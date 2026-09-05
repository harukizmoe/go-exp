package agent_trace_langfuse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ScoreClient writes trace-level scores to Langfuse's public API.
type ScoreClient struct {
	endpoint   string
	publicKey  string
	secretKey  string
	httpClient *http.Client
}

// Score is the payload accepted by Langfuse's score endpoint.
type Score struct {
	TraceID       string  `json:"traceId"`
	ObservationID string  `json:"observationId,omitempty"`
	Name          string  `json:"name"`
	Value         float64 `json:"value"`
	DataType      string  `json:"dataType"`
	Comment       string  `json:"comment,omitempty"`
}

// NewScoreClient creates a client for the Langfuse public score endpoint.
func NewScoreClient(baseURL, publicKey, secretKey string, timeout time.Duration) (*ScoreClient, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, errors.New("Langfuse base URL is required for score ingestion")
	}
	if strings.TrimSpace(publicKey) == "" || strings.TrimSpace(secretKey) == "" {
		return nil, errors.New("Langfuse public and secret keys are required for score ingestion")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	endpoint := base + "/api/public/scores"
	if strings.HasSuffix(base, "/api/public") {
		endpoint = base + "/scores"
	} else if strings.HasSuffix(base, "/api/public/scores") {
		endpoint = base
	}

	return &ScoreClient{
		endpoint:   endpoint,
		publicKey:  strings.TrimSpace(publicKey),
		secretKey:  strings.TrimSpace(secretKey),
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// WriteScore writes one score using the common evaluator sink contract.
func (c *ScoreClient) WriteScore(
	ctx context.Context,
	traceID string,
	name string,
	value float64,
	dataType string,
	comment string,
) error {
	return c.CreateScore(ctx, Score{
		TraceID:  traceID,
		Name:     name,
		Value:    value,
		DataType: dataType,
		Comment:  comment,
	})
}

// CreateScore sends one trace-level score and preserves HTTP failures.
func (c *ScoreClient) CreateScore(ctx context.Context, score Score) error {
	if strings.TrimSpace(score.TraceID) == "" {
		return errors.New("score traceId is required")
	}
	if strings.TrimSpace(score.Name) == "" {
		return errors.New("score name is required")
	}
	if score.DataType == "" {
		score.DataType = "NUMERIC"
	}

	body, err := json.Marshal(score)
	if err != nil {
		return fmt.Errorf("marshal Langfuse score: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Langfuse score request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth(c.publicKey, c.secretKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send Langfuse score: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read Langfuse score response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Langfuse score API status=%d body=%s", response.StatusCode, truncate(string(responseBody), 1000))
	}
	return nil
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}
