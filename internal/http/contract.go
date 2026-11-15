package http

import (
	"context"
	"net/http"

	api "github.com/NorseSway6/template.git/internal/generated"
)

type (
	createHandler interface {
		Create(w http.ResponseWriter, r *http.Request, params api.CreateTripParams)
	}

	positionCreateHandler interface {
		CreatePosition(w http.ResponseWriter, r *http.Request, id api.TripId)
	}

	getHandler interface {
		Get(w http.ResponseWriter, r *http.Request, id api.TripId)
	}

	positionGetHandler interface {
		GetPosition(w http.ResponseWriter, r *http.Request, id api.TripId)
	}

	finishHandler interface {
		Finish(w http.ResponseWriter, r *http.Request, id api.TripId)
	}

	idempotency interface {
		Wrap(next http.HandlerFunc) http.HandlerFunc
	}

	pinger interface {
		Ping(ctx context.Context) error
	}
)
