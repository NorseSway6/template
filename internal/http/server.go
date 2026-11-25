package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/NorseSway6/template.git/internal/domain/trip"
	api "github.com/NorseSway6/template.git/internal/generated"
	"github.com/NorseSway6/template.git/internal/http/domainerror"
	middleware "github.com/NorseSway6/template.git/internal/http/middleware"
)

// Server представляет собой HTTP-сервер, который обрабатывает запросы к API.
type Server struct {
	create       createHandler
	posCreate    positionCreateHandler
	get          getHandler
	posGet       positionGetHandler
	finish       finishHandler
	idem         idempotency
	db           pinger
	log          *slog.Logger
	metrics      *middleware.Metrics
	readyTimeout time.Duration
}

// NewServer создает новый экземпляр Server с заданными обработчиками и зависимостями.
func NewServer(
	c createHandler,
	pc positionCreateHandler,
	g getHandler,
	pg positionGetHandler,
	f finishHandler,
	i idempotency,
	db pinger,
	log *slog.Logger,
	metrics *middleware.Metrics,
	readyTimeout time.Duration,
) *Server {
	return &Server{
		create: c, posCreate: pc, get: g, posGet: pg,
		finish: f, idem: i, db: db, log: log, metrics: metrics,
		readyTimeout: readyTimeout,
	}
}

// Handler возвращает HTTP-обработчик, который обрабатывает запросы к API.
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
	return api.HandlerWithOptions(s, api.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			domainerror.WriteError(w, r, &trip.ValidationError{Field: "request", Reason: err.Error()})
		},
	})
}

// Health проверяет, что процесс сервиса запущен.
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

// Ready проверяет доступность базы данных.
func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.readyTimeout)
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

// CreateTrip обрабатывает запрос на создание новой поездки.
func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, p api.CreateTripParams) {
	s.idem.Wrap(func(w http.ResponseWriter, r *http.Request) {
		s.create.Create(w, r, p)
	})(w, r)
}

// CreateTripPosition обрабатывает запрос на создание новой позиции поездки.
func (s *Server) CreateTripPosition(w http.ResponseWriter, r *http.Request, id api.TripId) {
	s.idem.Wrap(func(w http.ResponseWriter, r *http.Request) {
		s.posCreate.CreatePosition(w, r, id)
	})(w, r)
}

// GetTrip обрабатывает запрос на получение информации о поездке по ее идентификатору.
func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	s.get.Get(w, r, id)
}

// ListTripPositions обрабатывает запрос на получение списка позиций поездки по ее идентификатору.
func (s *Server) ListTripPositions(w http.ResponseWriter, r *http.Request, id api.TripId) {
	s.posGet.GetPosition(w, r, id)
}

// FinishTrip обрабатывает запрос на завершение поездки по ее идентификатору.
func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, id api.TripId) {
	s.finish.Finish(w, r, id)
}
