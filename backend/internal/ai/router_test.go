package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"hackathon/backend/internal/domain"
)

func catalog() *domain.Catalog {
	return &domain.Catalog{Scenarios: []domain.Scenario{{ID: "renew", Name: "Продление", ResponseRU: "Уточните срок", ResponseKK: "Мерзімді айтыңыз"}}, Raw: json.RawMessage(`[{"id":"renew","name":"Продление"}]`)}
}

const valid = `{"scenario_id":"renew","status":"route","confidence":0.9,"language":"mixed","reason":"Клиент продлевает существующий полис.","alternatives":[],"slots":[],"pending":[]}`

func completion(w http.ResponseWriter, s string) {
	_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": s}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 30, "completion_tokens": 12, "prompt_tokens_details": map[string]int{"cached_tokens": 10}}})
}
func TestMalformedDecisionRetriesWithContext(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("missing body")
		}
		if len(body.Messages) < 2 || !strings.Contains(body.Messages[1].Content, "interrupted") {
			t.Error("context not sent")
		}
		if calls.Add(1) == 1 {
			completion(w, strings.Replace(valid, "renew", "invented", 1))
		} else {
			completion(w, valid)
		}
	}))
	defer server.Close()
	router, err := New(Config{Provider: "openai", BaseURL: server.URL, Key: "test-placeholder", Model: "test", Timeout: time.Second, Strict: true}, catalog())
	if err != nil {
		t.Fatal(err)
	}
	observed := []domain.Call{}
	d, err := router.Route(context.Background(), domain.RouteInput{Text: "И всё-таки продлить", Active: "renew", Pending: []string{"interrupted"}}, func(c domain.Call) { observed = append(observed, c) })
	if err != nil || d.ScenarioID != "renew" || calls.Load() != 2 {
		t.Fatalf("result=%+v err=%v calls=%d", d, err, calls.Load())
	}
	if len(observed) != 2 || observed[0].Error == "" || observed[1].CachedTokens != 10 {
		t.Fatalf("invalid trace: %+v", observed)
	}
}
func TestAuthenticationDoesNotRetryOrExposeBody(t *testing.T) {
	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(401)
		_, _ = w.Write([]byte("private provider response"))
	}))
	defer s.Close()
	c, _ := New(Config{Provider: "openai", BaseURL: s.URL, Key: "placeholder", Model: "test", Timeout: time.Second}, catalog())
	_, err := c.Route(context.Background(), domain.RouteInput{Text: "test"}, func(domain.Call) {})
	if err == nil || n.Load() != 1 || strings.Contains(err.Error(), "private") {
		t.Fatalf("err=%v attempts=%d", err, n.Load())
	}
}
func TestTotalDeadlineBoundsRetries(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(60 * time.Millisecond); w.WriteHeader(500) }))
	defer s.Close()
	c, _ := New(Config{Provider: "openai", BaseURL: s.URL, Key: "placeholder", Model: "test", Timeout: 90 * time.Millisecond}, catalog())
	start := time.Now()
	_, err := c.Route(context.Background(), domain.RouteInput{Text: "x"}, func(domain.Call) {})
	if err == nil || time.Since(start) > 180*time.Millisecond {
		t.Fatalf("deadline not respected: %v", err)
	}
}
func TestSchemaRejectsPartialNullUnknownAndUnbounded(t *testing.T) {
	for _, raw := range []string{`{}`, strings.Replace(valid, `"slots":[]`, `"slots":null`, 1), strings.Replace(valid, `"confidence":0.9`, `"confidence":4`, 1), strings.Replace(valid, `"pending":[]`, `"pending":["unknown"]`, 1), strings.Replace(valid, `"status":"route"`, `"status":"executed"`, 1), strings.Replace(valid, `"slots":[]`, `"slots":[],"execute":true`, 1)} {
		var d domain.Decision
		if DecodeDecision(raw, catalog(), &d) == nil {
			t.Errorf("accepted invalid output %s", raw)
		}
	}
}
