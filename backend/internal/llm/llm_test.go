package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestAdaptsToReasoningModels: a first 400 about max_tokens/temperature makes
// the client switch to max_completion_tokens (no temperature) and retry.
func TestAdaptsToReasoningModels(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		n := atomic.AddInt32(&calls, 1)
		if _, has := body["max_tokens"]; has {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":{"message":"Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead."}}`)
			return
		}
		if _, has := body["temperature"]; has {
			t.Errorf("temperature must be dropped for reasoning models")
		}
		if body["max_completion_tokens"] == nil {
			t.Errorf("max_completion_tokens expected on retry (call %d)", n)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"a\\\":1}\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":3}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	c := NewOpenAI(srv.URL, "k", "gpt-5-mini", 3*time.Second)
	ch, err := c.Stream(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 50, Temperature: 0.1, JSON: true})
	if err != nil {
		t.Fatal(err)
	}
	text, usage, err := Collect(ch)
	if err != nil || text != `{"a":1}` || usage == nil || usage.CompletionTokens != 3 {
		t.Fatalf("stream: %q %+v %v", text, usage, err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 1 failed + 1 adapted call, got %d", calls)
	}
	// second request must not hit the 400 again (adaptation is sticky)
	ch, err = c.Stream(context.Background(), Request{Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 50})
	if err != nil {
		t.Fatal(err)
	}
	Collect(ch)
	if atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("adaptation should be sticky, got %d calls", calls)
	}
}
