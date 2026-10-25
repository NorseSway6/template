// middleware.go
package idempotency

import (
	"bytes"
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
}

// New создает новый экземпляр Middleware с заданным хранилищем.
func New(store store) *Middleware {
	return &Middleware{store: store}
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

		existing, err := m.store.Get(r.Context(), key)
		if err != nil {
			domainerror.WriteError(w, r, err)
			return
		}

		if existing != nil {
			if existing.RequestHash != hash {
				domainerror.WriteError(w, r, trip.ErrIdempotencyConflict)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(existing.ResponseCode)
			_, _ = w.Write(existing.ResponseBody)
			return
		}

		rec := &recorder{ResponseWriter: w}
		next(rec, r)

		if rec.status == http.StatusCreated {
			_ = m.store.Save(r.Context(), &model.IdempotencyRecord{
				Key:          key,
				RequestHash:  hash,
				ResponseCode: http.StatusOK,
				ResponseBody: rec.body.Bytes(),
			})
		}
	}
}

func hashBody(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
