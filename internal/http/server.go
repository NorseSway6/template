package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	api "github.com/NorseSway6/template.git/internal/generated"
	middleware "github.com/NorseSway6/template.git/internal/http/middleware"
)

type Server struct {
	create  createHandler
	get     getHandler
	finish  finishHandler
	idem    idempotency
	db      pinger
	log     *slog.Logger
	metrics *middleware.Metrics
}

func NewServer(c createHandler, g getHandler, f finishHandler, i idempotency, db pinger, log *slog.Logger, metrics *middleware.Metrics) *Server {
	return &Server{create: c, get: g, finish: f, idem: i, db: db, log: log, metrics: metrics}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(
		middleware.Recover(s.log),
		middleware.Tracing,
		middleware.Logging(s.log),
		s.metrics.Middleware,
	)
	r.Get("/health", s.Health)
	r.Get("/ready", s.Ready)
	api.HandlerFromMux(s, r)
	return r
}

// Health проверяет, что процесс сервиса запущен.
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

// Ready проверяет доступность базы данных.
func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.db.Ping(ctx); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Unavailable})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, p api.CreateTripParams) {
	s.idem.Wrap(func(w http.ResponseWriter, r *http.Request) {
		s.create.Create(w, r, p)
	})(w, r)
}

func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	s.get.Get(w, r, id)
}

func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	s.finish.Finish(w, r, id)
}
