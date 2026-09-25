package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/NorseSway6/template.git/internal/config"
	api "github.com/NorseSway6/template.git/internal/generated"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)


// Server оборачивает стандартный http.Server.
type Server struct {
	httpServer *http.Server
	dbPool     *pgxpool.Pool
	repo       *TripRepository
	txManager  TxManager
}

// NewServer создает и настраивает HTTP-сервер.
func NewServer(cfg config.Config, dbPool *pgxpool.Pool, repo *TripRepository, txManager TxManager) *Server {
	r := chi.NewRouter()

	srv := &Server{ 
		dbPool: dbPool,
		repo: repo,
		txManager: txManager,
	}

	r.Get("/health", srv.Health)
	r.Get("/ready", srv.Ready)

	api.HandlerFromMux(srv, r)

	srv.httpServer = &http.Server{
		Addr:              cfg.HttpAddr,
		Handler:           r,
	    ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	return srv
}

// Start стартует сервер.
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown делает graceful shutdown.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}


// writeProblem вспомогательная функция отправки ошибок в формате application/problem+json.
func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	prob := api.Problem{
		Type:     fmt.Sprintf("https://tripgo.example/problems/%s", code),
		Title:    title,
		Status:   int32(status),
		Detail:   &detail,
		Code:     code,
		Instance: &r.URL.Path,
	}

	_ = json.NewEncoder(w).Encode(prob)
}

// Health GET /health
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{ Status: api.Ok})
}

// Ready GET /ready
func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.dbPool.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Unavailable})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{ Status: api.Ok})
}

// CreateTrip POST /api/v1/trips
func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var req api.CreateTripJSONRequestBody

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid Request", "Invalid JSON body")
		return
	}

	// валидация входных данных	
	if req.StartPoint.Latitude < -90 || req.StartPoint.Latitude > 90 ||
		req.StartPoint.Longitude < -180 || req.StartPoint.Longitude > 180 ||
		req.EndPoint.Latitude < -90 || req.EndPoint.Latitude > 90 ||
		req.EndPoint.Longitude < -180 || req.EndPoint.Longitude > 180 {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid Request", "Coordinates out of bounds")
		return
	}
	if req.Price < 0 {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid Request", "Price must be >= 0")
		return
	}

	// формирование модели поездки
	tripID := uuid.New().String()
	now := time.Now().UTC()

	tripModel := &Trip{
		ID:             tripID,
		UserID:         req.UserId.String(),
		DriverID:       req.DriverId.String(),
		StartLatitude:  float64(req.StartPoint.Latitude),
		StartLongitude: float64(req.StartPoint.Longitude),
		EndLatitude:    float64(req.EndPoint.Latitude),
		EndLongitude:   float64(req.EndPoint.Longitude),
		Price:          int64(req.Price),
		Status:         string(api.Active),
		StartedAt:      now,
	}

	// создание поездки
	err := s.txManager.Do(r.Context(), func(ctx context.Context) error {
		return s.repo.Create(ctx, tripModel)
	})

	if err != nil {
		if errors.Is(err, ErrDriverBusy) {
			writeProblem(w, r, http.StatusConflict, "driver_busy", "Driver Busy", "Driver already has an active trip")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Error", "Failed to create trip")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", fmt.Sprintf("/api/v1/trips/%s", tripID))
	w.WriteHeader(http.StatusCreated)

	resp := mapTripToAPI(tripModel)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetTrip GET /api/v1/trips/{tripId}
func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	trip, err := s.repo.GetByID(r.Context(), tripId.String())
	if err != nil {
		if errors.Is(err, ErrTripNotFound) {
			writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip Not Found", "Trip not found")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Error", "Failed to fetch trip")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(mapTripToAPI(trip))
}

// FinishTrip POST /api/v1/trips/{tripId}/finish
func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {	
	now := time.Now().UTC()
	var finishedTrip *Trip

	err := s.txManager.Do(r.Context(), func(ctx context.Context) error {
		var txErr error
		finishedTrip, txErr = s.repo.Finish(ctx, tripId.String(), now)
		return txErr
	})

	if err != nil {
		if errors.Is(err, ErrTripNotFound) {
			writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip Not Found", "Trip not found")
			return
		}
		if errors.Is(err, ErrTripCompleted) {
			writeProblem(w, r, http.StatusConflict, "trip_completed", "Trip Completed", "Trip is already completed")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Error", "Failed to finish trip")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(mapTripToAPI(finishedTrip))
}

// mapTripToAPI маппит внутреннюю модель Trip в сгенерированный тип api.Trip.
func mapTripToAPI(t *Trip) api.Trip {
	userID := uuid.MustParse(t.UserID)
	driverID := uuid.MustParse(t.DriverID)
	tripID := uuid.MustParse(t.ID)

	return api.Trip{
		Id:       tripID,
		UserId:   userID,
		DriverId: driverID,
		StartPoint: api.Coordinates{
			Latitude:  t.StartLatitude,
			Longitude: t.StartLongitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  t.EndLatitude,
			Longitude: t.EndLongitude,
		},
		Price:      t.Price,
		Status:     api.TripStatus(t.Status),
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}
}
