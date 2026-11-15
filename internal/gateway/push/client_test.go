package push

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	api "github.com/NorseSway6/template.git/internal/generated"
)

func TestRequestPositionSendsExpectedPush(t *testing.T) {
	driverID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/push" {
			t.Errorf("request = %s %s, want POST /api/v1/push", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}

		var request api.PushRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if request.RequestId == "" {
			t.Error("request_id is empty")
		}
		if request.RecipientId != driverID {
			t.Errorf("recipient_id = %s, want %s", request.RecipientId, driverID)
		}
		if request.Kind != api.REQUESTPOSITION {
			t.Errorf("kind = %q, want %q", request.Kind, api.REQUESTPOSITION)
		}
		if request.Data == nil || (*request.Data)["trip_id"] != "trip-123" {
			t.Errorf("data = %#v, want trip_id trip-123", request.Data)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client, err := New(server.URL, time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	if err := client.RequestPosition(context.Background(), "trip-123", driverID.String()); err != nil {
		t.Fatalf("RequestPosition() error = %v", err)
	}
}

func TestRequestPositionReturnsTypedHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := New(server.URL, time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	err = client.RequestPosition(context.Background(), "trip-123", uuid.NewString())
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("RequestPosition() error = %v, want *HTTPStatusError", err)
	}
	if statusErr.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", statusErr.StatusCode, http.StatusServiceUnavailable)
	}
}
