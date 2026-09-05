package agent_trace_langfuse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestScoreClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/scores" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != "pk-test" || password != "sk-test" {
			t.Fatalf("unexpected basic auth: %q %q %t", user, password, ok)
		}
		var score Score
		if err := json.NewDecoder(r.Body).Decode(&score); err != nil {
			t.Fatalf("decode score: %v", err)
		}
		if score.TraceID != "trace-1" || score.Name != "task_success" || score.DataType != "BOOLEAN" || score.Value != 1 {
			t.Fatalf("unexpected score: %+v", score)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"score-1"}`))
	}))
	defer server.Close()

	client, err := NewScoreClient(server.URL, "pk-test", "sk-test", time.Second)
	if err != nil {
		t.Fatalf("NewScoreClient() error = %v", err)
	}
	if err := client.WriteScore(context.Background(), "trace-1", "task_success", 1, "BOOLEAN", "passed"); err != nil {
		t.Fatalf("WriteScore() error = %v", err)
	}
}
