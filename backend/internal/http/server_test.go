package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"hackathon/backend/internal/ai"
	"hackathon/backend/internal/domain"
	"hackathon/backend/internal/speech"
	"hackathon/backend/internal/store"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type stubRouter struct {
	inputs    []domain.RouteInput
	decisions []domain.Decision
	err       error
	block     chan struct{}
}

func (s *stubRouter) Name() string  { return "test_llm" }
func (s *stubRouter) Model() string { return "test" }
func (s *stubRouter) Route(ctx context.Context, in domain.RouteInput, observe func(domain.Call)) (domain.Decision, error) {
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return domain.Decision{}, ctx.Err()
		}
	}
	s.inputs = append(s.inputs, in)
	if s.err != nil {
		return domain.Decision{}, s.err
	}
	i := len(s.inputs) - 1
	if i >= len(s.decisions) {
		i = len(s.decisions) - 1
	}
	return s.decisions[i], nil
}
func decision(id string) domain.Decision {
	return domain.Decision{ScenarioID: id, Status: "route", Language: "ru", Confidence: .9, Reason: "Маршрут по смыслу", Alternatives: []domain.Alternative{}, Slots: []domain.Slot{}, Pending: []string{}}
}
func app(router ai.Router) *Server {
	return &Server{Catalog: &domain.Catalog{Source: "test", Scenarios: []domain.Scenario{{ID: "a", Name: "A", ResponseRU: "Ответ A"}, {ID: "b", Name: "B", ResponseRU: "Ответ B"}}}, Router: router, Store: store.NewMemory(), Speech: &speech.Client{}, DBMode: "memory"}
}
func post(t *testing.T, h http.Handler, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}
func TestContextSwitchReturnAndLimit(t *testing.T) {
	a := decision("a")
	b := decision("b")
	b.Pending = []string{"a"}
	router := &stubRouter{decisions: []domain.Decision{a, b, a}}
	s := app(router)
	h := s.Handler()
	var id string
	for i := 0; i < 10; i++ {
		w, out := post(t, h, "/api/route", map[string]any{"session_id": id, "text": "Реплика"})
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		id = out["session_id"].(string)
		if i == 2 {
			turn := out["turn"].(map[string]any)
			if turn["topic_changed"] != true {
				t.Fatal("return should be a topic change")
			}
		}
	}
	if len(router.inputs[2].History) != 2 || router.inputs[2].Active != "b" || router.inputs[2].Pending[0] != "a" {
		t.Fatal("lost context")
	}
	w, _ := post(t, h, "/api/route", map[string]any{"session_id": id, "text": "11"})
	if w.Code != 409 {
		t.Fatal("missing ten turn cap")
	}
}
func TestFailureIsExplicitHandoffNotMock(t *testing.T) {
	s := app(&stubRouter{err: errors.New("secret upstream error")})
	w, out := post(t, s.Handler(), "/api/route", map[string]string{"text": "Привет"})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	turn := out["turn"].(map[string]any)
	if turn["source"] != "provider_error" || turn["decision"].(map[string]any)["status"] != "handoff" || strings.Contains(w.Body.String(), "secret") {
		t.Fatal(w.Body.String())
	}
}
func TestStreamFlushesBeforeLLMCompletes(t *testing.T) {
	blocked := make(chan struct{})
	s := app(&stubRouter{decisions: []domain.Decision{decision("a")}, block: blocked})
	server := httptest.NewServer(s.Handler())
	defer server.Close()
	client := &http.Client{Timeout: time.Second}
	resp, e := client.Post(server.URL+"/api/turns/stream", "application/json", strings.NewReader(`{"text":"test"}`))
	if e != nil {
		close(blocked)
		t.Fatal(e)
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	line, e := reader.ReadString('\n')
	close(blocked)
	if e != nil || !strings.Contains(line, `"type":"input"`) {
		t.Fatalf("not flushed: %s %v", line, e)
	}
	rest, _ := io.ReadAll(reader)
	if !strings.Contains(string(rest), `"type":"result"`) {
		t.Fatal(string(rest))
	}
}
func TestMetricsDoNotTreatTextAsEndOfSpeech(t *testing.T) {
	s := app(&stubRouter{decisions: []domain.Decision{decision("a")}})
	h := s.Handler()
	_, out := post(t, h, "/api/route", map[string]string{"text": "x"})
	sid := out["session_id"].(string)
	tid := out["turn"].(map[string]any)["id"].(string)
	w, _ := post(t, h, "/api/sessions/"+sid+"/turns/"+tid+"/metrics", map[string]int{"end_to_audio_ms": 300, "tts_first_byte_ms": 100})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	session, _ := s.Store.Load(context.Background(), sid)
	if session.Turns[0].Timing.EndToAudioMS != nil {
		t.Fatal("text entered audio latency metric")
	}
}
func TestUnknownSessionAndInvalidInput(t *testing.T) {
	s := app(&stubRouter{decisions: []domain.Decision{decision("a")}})
	for _, body := range []map[string]any{{"text": " "}, {"text": "x", "stt_ms": -1}, {"text": "x", "input_kind": "fake"}} {
		w, _ := post(t, s.Handler(), "/api/route", body)
		if w.Code != 400 {
			t.Fatalf("accepted %#v", body)
		}
	}
	w, _ := post(t, s.Handler(), "/api/route", map[string]string{"text": "x", "session_id": "unknown"})
	if w.Code != 404 {
		t.Fatal("unknown session accepted")
	}
}
