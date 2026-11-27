package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// StatusError — Push Service ответил, но не принял команду.
type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("push service returned HTTP %d", e.StatusCode)
}

type Gateway struct {
	endpoint  string
	client    *http.Client
	transport *http.Transport
}

func New(baseURL string, timeout time.Duration) (*Gateway, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("push request timeout must be positive")
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse PUSH_HTTP_URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid PUSH_HTTP_URL %q", baseURL)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	return &Gateway{
		endpoint:  u.JoinPath("api/v1/push").String(),
		transport: transport,
		client: &http.Client{
			Timeout:   timeout,
			Transport: otelhttp.NewTransport(transport),
		},
	}, nil
}

func (g *Gateway) RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error {
	payload := struct {
		RequestID   string            `json:"request_id"`
		RecipientID string            `json:"recipient_id"`
		Kind        string            `json:"kind"`
		Data        map[string]string `json:"data"`
	}{
		RequestID:   uuid.NewString(),
		RecipientID: driverID.String(),
		Kind:        "REQUEST_POSITION",
		Data:        map[string]string{"trip_id": tripID.String()},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode push request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, g.endpoint, bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("push transport: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return &StatusError{StatusCode: resp.StatusCode}
	}
	return nil
}

func (g *Gateway) Close() {
	g.transport.CloseIdleConnections()
}
