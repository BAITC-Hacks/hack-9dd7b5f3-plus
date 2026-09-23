package dialog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"hackathon/backend/internal/catalog"
	"hackathon/backend/internal/config"
	"hackathon/backend/internal/events"
	"hackathon/backend/internal/llm"
	"hackathon/backend/internal/mockbackend"
	"hackathon/backend/internal/retrieval"
	"hackathon/backend/internal/router"
	"hackathon/backend/internal/store"
	"hackathon/backend/internal/stt"
	"hackathon/backend/internal/tts"
)

// fakeLLM is an OpenAI-compatible SSE server that streams canned decisions:
// first call requests a read-only action, the follow-up call uses FACTS.
func fakeLLM(t *testing.T, calls *int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []llm.Message `json:"messages"`
			Stream   bool          `json:"stream"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		n := atomic.AddInt32(calls, 1)
		user := req.Messages[len(req.Messages)-1].Content
		var out string
		switch {
		case strings.Contains(user, "The actions you requested have been executed"):
			out = `{"language":"ru","scenarios":[{"id":"SC17","confidence":0.95,"reason":"статус заявления"}],"alternatives":[],"is_continuation":true,"slots":{},"actions":[],"handoff":null,"reply":"Сергей, заявление CL-500330 на рассмотрении, решение до 9 октября."}`
		case strings.Contains(user, "плюс 7 701 000 00 07"):
			out = `{"language":"ru","scenarios":[{"id":"SC17","confidence":0.93,"reason":"клиент назвал телефон"}],"alternatives":[],"is_continuation":true,"slots":{"phone":"+77010000007"},"actions":[{"name":"get_claim","args":{"client_id":"C007"}}],"handoff":null,"reply":"Секунду, проверяю."}`
		default:
			out = `{"language":"ru","scenarios":[{"id":"SC17","confidence":0.9,"reason":"статус заявления по каско"}],"alternatives":[{"id":"SC13","confidence":0.2}],"is_continuation":false,"slots":{},"actions":[],"handoff":null,"reply":"Сейчас проверю. Назовите номер заявления или телефон."}`
		}
		_ = n
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		// stream in 12-byte chunks like a real model
		for i := 0; i < len(out); i += 12 {
			end := i + 12
			if end > len(out) {
				end = len(out)
			}
			chunk := map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": out[i:end]}}}}
			b, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
			time.Sleep(2 * time.Millisecond)
		}
		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[],"usage":{"prompt_tokens":6000,"completion_tokens":80,"prompt_tokens_details":{"cached_tokens":5800}}}`)
		fmt.Fprint(w, "data: [DONE]\n\n")
		fl.Flush()
	}))
}

func newTestEngine(t *testing.T, llmURL string) (*Engine, *events.Bus) {
	cat, err := catalog.Load("../../../data")
	if err != nil {
		t.Fatal(err)
	}
	lex, _ := retrieval.LoadLexicon("../../config/lexicon.json")
	ix := retrieval.New(cat, lex)
	be, _ := mockbackend.New(cat)
	mock := router.NewMockRouter(cat, ix)
	cfg := config.Config{FastPath: "on", Debug: true, Policy: config.PolicyConfig{ProceedMin: 0.55, ClarifyMin: 0.3, FastPathMinScore: 0.75, FastPathMinMargin: 0.35}}
	var primary router.Router = mock
	if llmURL != "" {
		p := llm.NewOpenAI(llmURL, "test", "fake-model", 5*time.Second)
		primary = router.NewLLMRouter(cat, p, 0.1, 400, true)
	}
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()
	return New(cfg, cat, ix, primary, mock, be, stt.Mock{}, tts.Browser{}, st, bus), bus
}

func TestLLMTurnWithFollowUp(t *testing.T) {
	var calls int32
	srv := fakeLLM(t, &calls)
	defer srv.Close()
	e, bus := newTestEngine(t, srv.URL)
	ch, cancel := bus.Subscribe("", 1024)
	defer cancel()
	s := e.NewSession("test")

	tr, err := e.RunTurn(context.Background(), s, "Здравствуйте, что с моим заявлением по каско?", TurnOptions{Source: "text", Voice: true})
	if err != nil {
		t.Fatal(err)
	}
	if tr.Path != "llm" || tr.Decision.Primary() != "SC17" || tr.Policy.Action != "proceed" {
		t.Fatalf("turn 1: path=%s primary=%s policy=%+v", tr.Path, tr.Decision.Primary(), tr.Policy)
	}
	if tr.Timings["route"] <= 0 || tr.Timings["route"] > tr.Timings["llm_total"] {
		t.Fatalf("route must be measured before the reply finished: %v", tr.Timings)
	}
	if tr.LLM == nil || tr.LLM.Usage == nil || tr.LLM.Usage.CachedTokens != 5800 {
		t.Fatalf("usage not captured: %+v", tr.LLM)
	}

	tr2, err := e.RunTurn(context.Background(), s, "Номер не помню, телефон плюс 7 701 000 00 07.", TurnOptions{Source: "text", Voice: true})
	if err != nil {
		t.Fatal(err)
	}
	if tr2.State.ClientName != "Sergey Popov" {
		t.Fatalf("client should be identified from the phone: %+v", tr2.State)
	}
	if !tr2.Reply.FollowUp || !strings.Contains(tr2.Reply.Text, "CL-500330") || !strings.HasPrefix(tr2.Reply.Text, "Секунду") {
		t.Fatalf("follow-up reply expected (holding phrase + data): %q follow_up=%v", tr2.Reply.Text, tr2.Reply.FollowUp)
	}
	found := false
	for _, a := range tr2.Actions {
		if a.Name == "get_claim" && a.Mode == "execute" && a.Result["claim_number"] == "CL-500330" {
			found = true
		}
	}
	if !found {
		t.Fatalf("get_claim should have been executed: %+v", tr2.Actions)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("expected 3 model calls (turn1, turn2, follow-up), got %d", calls)
	}
	// events: route must arrive before reply_delta, and speak events exist (browser TTS)
	var seq []string
	deadline := time.After(2 * time.Second)
loop:
	for {
		select {
		case ev := <-ch:
			seq = append(seq, ev.Type)
		case <-deadline:
			break loop
		default:
			if len(seq) > 0 {
				break loop
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	joined := strings.Join(seq, ",")
	if !strings.Contains(joined, "route") || !strings.Contains(joined, "reply_delta") || !strings.Contains(joined, "speak") || !strings.Contains(joined, "turn_done") {
		t.Fatalf("event stream incomplete: %s", joined)
	}
	if strings.Index(joined, "route") > strings.Index(joined, "reply_delta") {
		t.Fatalf("route event must precede reply streaming: %s", joined)
	}
}

func TestConfirmationGate(t *testing.T) {
	e, _ := newTestEngine(t, "")
	s := e.NewSession("test")
	// identify + cancel flow in keyless mode: the pending action is set by the executor
	_, err := e.RunTurn(context.Background(), s, "Телефон плюс 7 701 000 00 10", TurnOptions{Source: "text"})
	if err != nil {
		t.Fatal(err)
	}
	// simulate a model decision that previews cancel_policy
	d := &router.Decision{Language: "kk", Scenarios: []router.ScenarioPick{{ID: "SC28", Confidence: 0.9}}, Slots: map[string]any{"cancel_reason": "sold"},
		Actions: []router.ActionRequest{{Name: "cancel_policy", Args: map[string]any{"policy_number": "SQ-CASCO-204350"}, Mode: "preview"}}}
	tr := &Trace{Timings: map[string]int{}}
	e.execute(context.Background(), s, d, tr)
	if s.View().Pending == nil || s.View().Pending.Name != "cancel_policy" {
		t.Fatalf("pending confirmation expected: %+v", s.View())
	}
	if tr.Actions[0].Mode != "preview" || tr.Actions[0].Result["refund_amount"] != 163800.0 {
		t.Fatalf("preview should compute the refund without executing: %+v", tr.Actions)
	}
	tr2, err := e.RunTurn(context.Background(), s, "Иә, растаймын.", TurnOptions{Source: "text"})
	if err != nil {
		t.Fatal(err)
	}
	executed := false
	for _, a := range tr2.Actions {
		if a.Name == "cancel_policy" && a.Mode == "execute" && a.Result["refund_amount"] == 163800.0 {
			executed = true
		}
	}
	if !executed || s.View().Pending != nil {
		t.Fatalf("action should execute only after the explicit yes: %+v pending=%v", tr2.Actions, s.View().Pending)
	}
	// second cancel of the same policy must fail (already_done), never silently re-run
	if _, aerr := e.Backend().Execute("cancel_policy", map[string]any{"policy_number": "SQ-CASCO-204350"}); aerr == nil || aerr.Code != "already_done" {
		t.Fatalf("expected already_done, got %v", aerr)
	}
}
