package brain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeBackend is a team-backend stand-in that records what it receives.
type fakeBackend struct {
	mu     sync.Mutex
	hits   map[string]int
	bodies map[string][]map[string]any
	mux    *http.ServeMux
}

func newFakeBackend(t *testing.T) (*fakeBackend, *httptest.Server) {
	t.Helper()
	f := &fakeBackend{hits: map[string]int{}, bodies: map[string][]map[string]any{}, mux: http.NewServeMux()}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.hits[r.URL.Path]++
		f.bodies[r.URL.Path] = append(f.bodies[r.URL.Path], body)
		f.mu.Unlock()
		f.mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeBackend) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[path]
}

func (f *fakeBackend) body(path string, i int) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	if i < 0 {
		i += len(f.bodies[path])
	}
	return f.bodies[path][i]
}

// writeSSE streams events as SSE frames.
func writeSSE(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, e := range events {
		fmt.Fprintf(w, "data: %s\n\n", e)
		w.(http.Flusher).Flush()
	}
}

// writeNDJSON streams events as JSON lines.
func writeNDJSON(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	for _, e := range events {
		fmt.Fprintln(w, e)
		w.(http.Flusher).Flush()
	}
}

var contractEvents = []string{
	`{"type":"turn.start","turn":1,"session_id":"b-1","t0":1}`,
	`{"type":"router.decision","decision":{"scenarios":[{"scenario_id":"SC30","confidence":0.86,"reason":"money charged, policy not issued"}],"alternatives":[],"language":"ru","reason":"payment issue","model":"gpt-4.1-mini"},"ms":120}`,
	`{"type":"response.delta","text":"Вижу списание. "}`,
	`{"type":"response.delta","text":"Проверю платёж."}`,
	`{"type":"response.final","text":"Вижу списание. Проверю платёж.","language":"ru","ms":300}`,
	`{"type":"turn.done","trace":{"turn":1},"latency_ms":{"total":420}}`,
}

func kindsOf(evs []Event) []string {
	var out []string
	for _, e := range evs {
		out = append(out, e.Kind)
	}
	return out
}

func TestBackendTurnSSE(t *testing.T) {
	f, srv := newFakeBackend(t)
	f.mux.HandleFunc("POST /api/session", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"session_id":"b-%d","state":{}}`, f.count(r.URL.Path))
	})
	f.mux.HandleFunc("POST /api/turn", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "text/event-stream" {
			http.Error(w, "want SSE", http.StatusNotAcceptable)
			return
		}
		writeSSE(w, contractEvents...)
	})

	b := NewBackend(srv.URL + "/")
	if b.Name() != "backend" || b.Stateless() {
		t.Fatal("backend must be a stateful brain named backend")
	}
	speechEnd := time.UnixMilli(1758630000000)
	text := "Я вчера оплатил, деньги списались, а полис не пришёл"
	emit, evs := record()
	res, err := b.Turn(context.Background(), Input{SessionID: "v1", Text: text, Lang: "ru", SpeechEnd: speechEnd}, emit)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{KindRaw, KindRaw, KindDecision, KindRaw, KindDelta, KindRaw, KindDelta, KindRaw, KindRaw}
	if got := kindsOf(*evs); !slices.Equal(got, want) {
		t.Fatalf("event kinds = %v, want %v", got, want)
	}
	if raw := string((*evs)[1].Raw); !strings.Contains(raw, `"router.decision"`) {
		t.Errorf("raw event = %s", raw)
	}
	d := (*evs)[2].Decision
	if d.ScenarioID != "SC30" || d.Confidence != 0.86 || d.Language != "ru" || d.Reason != "money charged, policy not issued" {
		t.Errorf("decision = %+v", d)
	}
	if (*evs)[4].Text != "Вижу списание. " || (*evs)[6].Text != "Проверю платёж." {
		t.Errorf("deltas = %q %q", (*evs)[4].Text, (*evs)[6].Text)
	}
	if res.Reply != "Вижу списание. Проверю платёж." || res.Raw != res.Reply || res.Model != "gpt-4.1-mini" || res.TTFT <= 0 {
		t.Errorf("result = %+v", res)
	}

	body := f.body("/api/turn", 0)
	if body["session_id"] != "b-1" || body["text"] != text || body["lang_hint"] != "ru" || body["tts"] != false || body["client_t0"] != float64(speechEnd.UnixMilli()) {
		t.Errorf("turn body = %v", body)
	}

	// The backend session is reused, and End drops the mapping.
	if _, err := b.Turn(context.Background(), Input{SessionID: "v1", Text: "И адрес поменять", Lang: "ru"}, nil); err != nil {
		t.Fatal(err)
	}
	if n := f.count("/api/session"); n != 1 {
		t.Errorf("sessions created = %d, want 1", n)
	}
	b.End("v1")
	if _, err := b.Turn(context.Background(), Input{SessionID: "v1", Text: "Алло", Lang: "ru"}, nil); err != nil {
		t.Fatal(err)
	}
	if n := f.count("/api/session"); n != 2 || f.body("/api/turn", -1)["session_id"] != "b-2" {
		t.Errorf("after End: sessions = %d, session_id = %v", n, f.body("/api/turn", -1)["session_id"])
	}
}

func TestBackendTurnNDJSON(t *testing.T) {
	f, srv := newFakeBackend(t)
	f.mux.HandleFunc("POST /api/session", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":42}`)
	})
	f.mux.HandleFunc("POST /api/turn", func(w http.ResponseWriter, r *http.Request) {
		writeNDJSON(w,
			`{"type":"routing_complete","data":{"scenario_id":"SC17","confidence":0.9,"status":"route","language":"kk","reason":"claim status"}}`,
			`{"type":"policy","data":{"reply":"Өтінішіңіз қаралуда."}}`,
		)
	})
	emit, evs := record()
	res, err := NewBackend(srv.URL).Turn(context.Background(), Input{SessionID: "v", Text: "Өтінішім қандай күйде?", Lang: "kk"}, emit)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := kindsOf(*evs), []string{KindRaw, KindDecision, KindRaw, KindDelta}; !slices.Equal(got, want) {
		t.Fatalf("event kinds = %v, want %v", got, want)
	}
	d := (*evs)[1].Decision
	if d.ScenarioID != "SC17" || d.Status != "route" || d.Language != "kk" || d.Confidence != 0.9 {
		t.Errorf("decision = %+v", d)
	}
	if res.Reply != "Өтінішіңіз қаралуда." {
		t.Errorf("reply = %q", res.Reply)
	}
	if f.body("/api/turn", 0)["session_id"] != "42" {
		t.Errorf("numeric session id not used: %v", f.body("/api/turn", 0))
	}
}

func TestBackendFallbackToTurnsStream(t *testing.T) {
	f, srv := newFakeBackend(t)
	f.mux.HandleFunc("POST /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id":"legacy-7"}`)
	})
	f.mux.HandleFunc("POST /api/turns/stream", func(w http.ResponseWriter, r *http.Request) {
		writeNDJSON(w,
			`{"type":"routing_complete","data":{"scenario_id":"SC33","confidence":0.8,"status":"route","language":"ru"}}`,
			`{"type":"policy","data":{"reply":"Офис на проспекте Абая."}}`,
		)
	})
	b := NewBackend(srv.URL)
	ctx := context.Background()
	res, err := b.Turn(ctx, Input{SessionID: "v", Text: "Где ваш офис?", Lang: "ru", STTms: 120}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Reply != "Офис на проспекте Абая." {
		t.Errorf("reply = %q", res.Reply)
	}
	body := f.body("/api/turns/stream", 0)
	if body["session_id"] != "legacy-7" || body["text"] != "Где ваш офис?" || body["input_kind"] != "microphone" || body["stt_ms"] != float64(120) {
		t.Errorf("stream body = %v", body)
	}

	// Both fallbacks are remembered: no more 404 round trips.
	b.End("v")
	if _, err := b.Turn(ctx, Input{SessionID: "v", Text: "А часы работы?", Lang: "ru"}, nil); err != nil {
		t.Fatal(err)
	}
	if a, b, c, d := f.count("/api/turn"), f.count("/api/session"), f.count("/api/sessions"), f.count("/api/turns/stream"); a != 1 || b != 1 || c != 2 || d != 2 {
		t.Errorf("hits: /api/turn %d, /api/session %d, /api/sessions %d, /api/turns/stream %d", a, b, c, d)
	}
}

func TestBackendConflictRetry(t *testing.T) {
	t.Run("busy", func(t *testing.T) {
		f, srv := newFakeBackend(t)
		f.mux.HandleFunc("POST /api/session", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"session_id":"s-1"}`)
		})
		f.mux.HandleFunc("POST /api/turn", func(w http.ResponseWriter, r *http.Request) {
			if f.count(r.URL.Path) == 1 {
				http.Error(w, `{"error":"turn already in progress"}`, http.StatusConflict)
				return
			}
			writeSSE(w, contractEvents...)
		})
		start := time.Now()
		res, err := NewBackend(srv.URL).Turn(context.Background(), Input{SessionID: "v", Text: "Алло", Lang: "ru"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if n := f.count("/api/turn"); res.Reply == "" || n != 2 || time.Since(start) < 250*time.Millisecond {
			t.Errorf("reply %q after %d turn requests in %v", res.Reply, n, time.Since(start))
		}
	})

	t.Run("turn limit", func(t *testing.T) {
		f, srv := newFakeBackend(t)
		f.mux.HandleFunc("POST /api/session", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{"session_id":"s-%d"}`, f.count(r.URL.Path))
		})
		f.mux.HandleFunc("POST /api/turn", func(w http.ResponseWriter, r *http.Request) {
			if f.body(r.URL.Path, -1)["session_id"] == "s-1" {
				http.Error(w, `{"error":"turn limit reached"}`, http.StatusConflict)
				return
			}
			writeSSE(w, contractEvents...)
		})
		b := NewBackend(srv.URL)
		if _, err := b.Turn(context.Background(), Input{SessionID: "v", Text: "Алло", Lang: "ru"}, nil); err != nil {
			t.Fatal(err)
		}
		if n := f.count("/api/session"); n != 2 || f.body("/api/turn", -1)["session_id"] != "s-2" {
			t.Errorf("sessions = %d, last session_id = %v", n, f.body("/api/turn", -1)["session_id"])
		}
	})
}

func TestBackendErrors(t *testing.T) {
	f, srv := newFakeBackend(t)
	f.mux.HandleFunc("POST /api/session", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"session_id":"s"}`)
	})
	f.mux.HandleFunc("POST /api/turn", func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, `{"type":"turn.start","turn":1}`, `{"type":"error","message":"router exploded"}`)
	})
	b := NewBackend(srv.URL)
	if _, err := b.Turn(context.Background(), Input{SessionID: "v", Text: "Алло"}, nil); err == nil || !strings.Contains(err.Error(), "router exploded") {
		t.Fatalf("err = %v", err)
	}
	if _, err := b.Turn(context.Background(), Input{SessionID: "v", Text: " "}, nil); !errors.Is(err, ErrEmptyText) {
		t.Fatalf("blank text: err = %v", err)
	}
}

func TestBackendHealthy(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		path string
		want bool
	}{{"/healthz", true}, {"/health", true}, {"/other", false}} {
		mux := http.NewServeMux()
		mux.HandleFunc("GET "+tc.path, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"status":"ok"}`) })
		srv := httptest.NewServer(mux)
		if got := NewBackend(srv.URL).Healthy(ctx); got != tc.want {
			t.Errorf("Healthy with %s = %v, want %v", tc.path, got, tc.want)
		}
		srv.Close()
	}
	start := time.Now()
	if NewBackend("http://127.0.0.1:1").Healthy(ctx) {
		t.Error("unreachable backend reported healthy")
	}
	if time.Since(start) > 2*time.Second {
		t.Error("Healthy exceeded its time budget")
	}
}
