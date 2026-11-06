// middleware.go
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	"github.com/NorseSway6/template.git/internal/http/domainerror"
	"github.com/NorseSway6/template.git/internal/model"
)

// headerKey представляет имя заголовка, в котором клиент передает ключ идемпотентности.
const headerKey = "Idempotency-Key"

// Middleware обеспечивает идемпотентность для HTTP-запросов.
type Middleware struct {
	store store
	tx    txManager
}

// New создает новый экземпляр Middleware с заданным хранилищем.
func New(store store, tx txManager) *Middleware {
	return &Middleware{store: store, tx: tx}
}

// Wrap оборачивает HTTP-обработчик, обеспечивая идемпотентность для запросов с указанным ключом.
func (m *Middleware) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(headerKey)
		if key == "" {
			next(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			domainerror.WriteError(w, r, &trip.ValidationError{Field: "body", Reason: "failed to read body"})
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		hash := hashBody(body)

		var rec *recorder
		var existing *model.IdempotencyRecord
		err = m.tx.Do(r.Context(), func(ctx context.Context) error {
			claimed, record, err := m.store.Claim(ctx, key, hash)
			if err != nil {
				return err
			}
			if !claimed {
				if record.RequestHash != hash {
					return trip.ErrIdempotencyConflict
				}
				existing = record
				return nil
			}

			rec = newRecorder()
			next(rec, r.WithContext(ctx))
			if rec.status == http.StatusCreated {
				return m.store.Complete(ctx, &model.IdempotencyRecord{
					Key:          key,
					RequestHash:  hash,
					ResponseCode: http.StatusOK,
					ResponseBody: rec.body.Bytes(),
				})
			}
			return m.store.Release(ctx, key, hash)
		})
		if err != nil {
			domainerror.WriteError(w, r, err)
			return
		}

		if existing != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(existing.ResponseCode)
			_, _ = w.Write(existing.ResponseBody) //nolint:gosec // Воспроизводит JSON, ранее созданный этим сервисом.
			return
		}

		rec.flush(w)
	}
}

func hashBody(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
