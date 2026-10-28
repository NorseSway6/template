package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/NorseSway6/template.git/internal/http/domainerror"
)

type recoveryContextKey struct{}

type recoveryState struct {
	mu          sync.Mutex
	onRecover   func(int)
	spanContext trace.SpanContext
}

func (s *recoveryState) setCallback(callback func(int)) {
	s.mu.Lock()
	s.onRecover = callback
	s.mu.Unlock()
}

func (s *recoveryState) setSpanContext(spanContext trace.SpanContext) {
	s.mu.Lock()
	s.spanContext = spanContext
	s.mu.Unlock()
}

func (s *recoveryState) getSpanContext() trace.SpanContext {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.spanContext
}

func (s *recoveryState) complete(status int) {
	s.mu.Lock()
	callback := s.onRecover
	s.mu.Unlock()
	if callback != nil {
		callback(status)
	}
}

// Recover возвращает middleware, который перехватывает паники в обработчиках HTTP-запросов, логирует их и возвращает клиенту ошибку 500 Internal Server Error.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			state := &recoveryState{}
			ctx := context.WithValue(r.Context(), recoveryContextKey{}, state)
			r = r.WithContext(ctx)
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				logCtx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
				if spanContext := state.getSpanContext(); spanContext.IsValid() {
					logCtx = trace.ContextWithSpanContext(logCtx, spanContext)
				}
				log.ErrorContext(logCtx, "panic recovered",
					"panic_type", fmt.Sprintf("%T", rec),
					"stack", string(debug.Stack()),
				)
				domainerror.WriteError(w, r, errors.New("internal error"))
				state.complete(http.StatusInternalServerError)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func recoveryStateFromContext(ctx context.Context) *recoveryState {
	state, _ := ctx.Value(recoveryContextKey{}).(*recoveryState)
	return state
}
