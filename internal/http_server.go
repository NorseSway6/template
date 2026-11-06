package internal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	httpServer    *http.Server
	dbPool        *pgxpool.Pool
	repo          *TripRepository
	txManager     TxManager
	idempRepo     *IdempotencyRepository
	dbPingTimeout time.Duration
}

// NewServer создает и настраивает HTTP-сервер.
func NewServer(cfg config.Config, dbPool *pgxpool.Pool, repo *TripRepository, txManager TxManager, idempRepo *IdempotencyRepository) *Server {
	r := chi.NewRouter()

	srv := &Server{
		dbPool:        dbPool,
		repo:          repo,
		txManager:     txManager,
		idempRepo:     idempRepo,
		dbPingTimeout: cfg.DatabasePingTimeout,
	}

	r.Get("/health", srv.Health)
	r.Get("/ready", srv.Ready)

	api.HandlerWithOptions(srv, api.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid Request", err.Error())
		},
	})

	srv.httpServer = &http.Server{
		Addr:              cfg.HttpAddr,
		Handler:           r,
		ReadHeaderTimeout: cfg.HttpReadHeaderTimeout,
		ReadTimeout:       cfg.HttpReadTimeout,
		WriteTimeout:      cfg.HttpWriteTimeout,
		IdleTimeout:       cfg.HttpIdleTimeout,
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

// Close немедленно закрывает HTTP-сервер и активные соединения.
func (s *Server) Close() error {
	return s.httpServer.Close()
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

// hashBody хэширует тело запроса.
func hashBody(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

func writeStoredTripResponse(w http.ResponseWriter, rec *IdempotencyRecord) {
	w.Header().Set("Content-Type", "application/json")
	var trip api.Trip
	if json.Unmarshal(rec.ResponseBody, &trip) == nil {
		w.Header().Set("Location", fmt.Sprintf("/api/v1/trips/%s", trip.Id))
	}
	w.WriteHeader(rec.ResponseCode)
	_, _ = w.Write(rec.ResponseBody)
}

// Health GET /health
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

// Ready GET /ready
func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), s.dbPingTimeout)
	defer cancel()

	if err := s.dbPool.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Unavailable})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

// CreateTrip POST /api/v1/trips
func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	idempotencyKey := r.Header.Get("Idempotency-Key")

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid Request", "Failed to read body")
		return
	}

	var req api.CreateTripJSONRequestBody
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &fields); err != nil || fields == nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid Request", "Invalid JSON body")
		return
	}
	for _, name := range []string{"user_id", "driver_id", "start_point", "end_point", "price"} {
		value, ok := fields[name]
		if !ok || string(value) == "null" {
			writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid Request", "Missing required field: "+name)
			return
		}
	}
	for _, name := range []string{"start_point", "end_point"} {
		var point map[string]json.RawMessage
		if err := json.Unmarshal(fields[name], &point); err != nil || point == nil {
			writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid Request", "Invalid "+name)
			return
		}
		for _, coordinate := range []string{"latitude", "longitude"} {
			value, ok := point[coordinate]
			if !ok || string(value) == "null" {
				writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid Request", "Missing required field: "+name+"."+coordinate)
				return
			}
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid Request", "Invalid JSON body")
		return
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid Request", "Invalid JSON body")
		return
	}
	if req.UserId == uuid.Nil || req.DriverId == uuid.Nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid Request", "User and driver IDs must be non-zero UUIDs")
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

	// проверка на наличие ключа идемпотентности после валидации запроса
	if idempotencyKey != "" {
		existing, err := s.idempRepo.Get(r.Context(), idempotencyKey)
		if err != nil {
			writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Error", "Idempotency check failed")
			return
		}

		if existing != nil {
			reqHash := hashBody(bodyBytes)
			if existing.RequestHash != reqHash {
				writeProblem(w, r, http.StatusConflict, "idempotency_conflict", "Idempotency Conflict", "Idempotency key already used with different request body")
				return
			}

			writeStoredTripResponse(w, existing)
			return
		}
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
	respBody, err := json.Marshal(mapTripToAPI(tripModel))
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Error", "Failed to encode trip response")
		return
	}

	reqHash := hashBody(bodyBytes)

	// создание поездки
	err = s.txManager.Do(r.Context(), func(ctx context.Context) error {
		if idempotencyKey != "" {
			if err := s.idempRepo.Save(ctx, &IdempotencyRecord{
				Key:          idempotencyKey,
				RequestHash:  reqHash,
				ResponseCode: http.StatusOK,
				ResponseBody: respBody,
			}); err != nil {
				return err
			}
		}
		return s.repo.Create(ctx, tripModel)
	})

	if err != nil {
		if errors.Is(err, ErrIdempotencyKeyExists) {
			existing, getErr := s.idempRepo.Get(r.Context(), idempotencyKey)
			if getErr == nil && existing != nil {
				if existing.RequestHash != reqHash {
					writeProblem(w, r, http.StatusConflict, "idempotency_conflict", "Idempotency Conflict", "Idempotency key already used with different request body")
					return
				}
				writeStoredTripResponse(w, existing)
				return
			}
			writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Error", "Failed to retrieve idempotency result")
			return
		}
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
	_, _ = w.Write(respBody)
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
