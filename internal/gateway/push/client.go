package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	api "github.com/NorseSway6/template.git/internal/generated"
)

// Client представляет собой клиент для взаимодействия с push-сервисом.
type Client struct {
	baseURL   string
	http      *http.Client
	transport *http.Transport
}

type HTTPStatusError struct {
	StatusCode int
}

// Error возвращает строковое представление ошибки HTTPStatusError.
func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("push service returned HTTP status %d", e.StatusCode)
}

// New создает новый экземпляр клиента push-сервиса.
func New(baseURL string, timeout time.Duration) (*Client, error) {
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse push service URL: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("push service URL must be an absolute HTTP or HTTPS URL")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("push service timeout must be positive, got %s", timeout)
	}

	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("default HTTP transport is not *http.Transport")
	}
	tr := transport.Clone()
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		transport: tr,
		http: &http.Client{
			Timeout:   timeout,
			Transport: otelhttp.NewTransport(tr),
		},
	}, nil
}

// RequestPosition отправляет запрос на получение позиции водителя для указанного tripID и driverID.
func (c *Client) RequestPosition(ctx context.Context, tripID, driverID string) error {
	recipientID, err := uuid.Parse(driverID)
	if err != nil {
		return fmt.Errorf("parse push recipient ID: %w", err)
	}
	data := map[string]string{"trip_id": tripID}
	payload := api.PushRequest{
		RequestId:   uuid.NewString(),
		RecipientId: recipientID,
		Kind:        api.REQUESTPOSITION,
		Data:        &data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode push request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/push", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send push request: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= http.StatusBadRequest {
		return &HTTPStatusError{StatusCode: resp.StatusCode}
	}
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("push service returned unexpected HTTP status %d", resp.StatusCode)
	}
	return nil
}

// Close закрывает соединения, используемые клиентом.
func (c *Client) Close() {
	c.transport.CloseIdleConnections()
}
