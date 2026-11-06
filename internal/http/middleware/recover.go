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
	spanContext trace.SpanContext
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
				status, body := domainerror.Problem(errors.New("internal error"))
				domainerror.WriteProblem(w, r, status, body)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func recoveryStateFromContext(ctx context.Context) *recoveryState {
	state, _ := ctx.Value(recoveryContextKey{}).(*recoveryState)
	return state
}
