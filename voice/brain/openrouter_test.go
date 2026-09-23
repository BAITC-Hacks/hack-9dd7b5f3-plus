package brain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const fixtureScenarios = `{
  "meta": {"as_of_date": "2026-10-01"},
  "scenarios": [
    {"scenario_id": "SC12", "slug": "ogpo_victim_claim", "name": "Claim as victim under culprit's OGPO", "domain": "auto", "category": "claims",
     "description": "Client was hit by another driver and claims under the culprit's OGPO.",
     "not_this_if": [{"condition": "Accident is happening right now", "use_instead": "SC11"}],
     "priority": "high", "fast_path_eligible": false, "requires_identification": false,
     "slots": {"required": ["culprit_vehicle_plate", "incident_date"], "optional": ["iin"]},
     "actions": ["get_policy", "create_claim"], "requires_confirmation": true, "handoff": null,
     "examples": {"ru": ["Меня ударили"], "kk": ["Маған соғып кетті"]},
     "responses": {"ru": {"opening": "Сочувствую. Назовите госномер виновника.", "closing": "Заявление {claim_number} зарегистрировано."},
                   "kk": {"opening": "Түсінемін. Кінәлі көліктің нөмірін айтыңызшы.", "closing": "{claim_number} өтініші тіркелді."}}},
    {"scenario_id": "SC27", "slug": "policy_renewal", "name": "Policy renewal", "domain": "general", "category": "sales",
     "description": "Client wants to extend a policy that is about to expire.", "not_this_if": [],
     "priority": "normal", "fast_path_eligible": false, "requires_identification": true,
     "slots": {"required": ["policy_number"], "optional": []}, "actions": ["renew_policy"], "requires_confirmation": true, "handoff": null,
     "examples": {"ru": ["Хочу продлить полис"], "kk": ["Полисті ұзартқым келеді"]},
     "responses": {"ru": {"opening": "Продлим. Назовите номер полиса.", "closing": "Готово."}, "kk": {"opening": "Ұзартайық. Полис нөмірін айтыңызшы.", "closing": "Дайын."}}},
    {"scenario_id": "SC37", "slug": "human_operator", "name": "Request a human operator", "domain": "general", "category": "contact",
     "description": "Client asks for a person.", "not_this_if": [], "priority": "normal", "fast_path_eligible": true,
     "requires_identification": false, "slots": {"required": [], "optional": []}, "actions": ["transfer_to_operator"],
     "requires_confirmation": false, "handoff": {"when": "always", "queue": "operator_general"},
     "examples": {"ru": ["Соедините с оператором"], "kk": ["Операторға қосыңыз"]},
     "responses": {"ru": {"opening": "Соединяю с оператором.", "closing": ""}, "kk": {"opening": "Операторға қосамын.", "closing": ""}}}
  ],
  "system_intents": [
    {"id": "SYS_OUT_OF_SCOPE", "description": "Not about Saqta insurance.", "behavior": "Say politely you cannot help.", "response": {"ru": "С этим я не помогу.", "kk": "Бұған көмектесе алмаймын."}},
    {"id": "SYS_UNCLEAR", "description": "Intent unclear.", "behavior": "Ask one short question.", "response": {"ru": "Уточните, пожалуйста.", "kk": "Нақтылаңызшы."}},
    {"id": "SYS_GOODBYE", "description": "Client ends the call.", "behavior": "Thank and close.", "response": {"ru": "Спасибо!", "kk": "Рақмет!"}}
  ]
}`

const fixtureKnowledge = `{
  "meta": {"as_of_date": "2026-10-01"},
  "company": {"name": "Saqta Insurance", "contact_center": {"phone": "+7 727 000 7575"}}
}`

const fixtureBackend = `{
  "clients": [
    {"client_id": "C001", "full_name": "Arman Tulegenov", "phone": "+77010000001", "iin": "850314300121", "city": "Almaty", "bm_class": "7", "preferred_language": "ru"},
    {"client_id": "C002", "full_name": "Aigerim Bekova", "phone": "+77010000002", "iin": "920607400233", "city": "Astana", "bm_class": "3", "preferred_language": "kk"}
  ],
  "policies": [
    {"policy_number": "SQ-OGPO-104501", "client_id": "C001", "product": "ogpo", "end_date": "2027-03-14"},
    {"policy_number": "SQ-DMS-604220", "client_id": "C002", "product": "dms", "end_date": "2026-12-31"}
  ],
  "claims": [{"claim_number": "CL-500198", "client_id": "C001", "status": "paid"}],
  "payments": [{"payment_id": "P-2950", "client_id": "C002", "amount": 22800}]
}`

// fixture writes a small starter kit to a temp dir and loads it.
func fixture(t *testing.T) *Dataset {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"scenarios.json":      fixtureScenarios,
		"knowledge_base.json": fixtureKnowledge,
		"mock_backend.json":   fixtureBackend,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d, err := LoadDataset(dir)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// sentRequest is what the fake server decodes from a chat completion request.
type sentRequest struct {
	Model       string    `json:"model"`
	Models      []string  `json:"models"`
	Messages    []message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	Provider    struct {
		Sort string `json:"sort"`
	} `json:"provider"`
	Usage struct {
		Include bool `json:"include"`
	} `json:"usage"`

	raw    string
	header http.Header
}

// fakeLLM is an OpenRouter stand-in that streams canned SSE chunks.
type fakeLLM struct {
	chunks []string

	mu   sync.Mutex
	reqs []sentRequest
}

func (f *fakeLLM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
		http.NotFound(w, r)
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req sentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.raw, req.header = string(body), r.Header.Clone()
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "text/event-stream")
	for _, c := range f.chunks {
		io.WriteString(w, c)
		w.(http.Flusher).Flush()
	}
}

func (f *fakeLLM) last(t *testing.T) sentRequest {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.reqs) == 0 {
		t.Fatal("no request received")
	}
	return f.reqs[len(f.reqs)-1]
}

// sse formats one SSE data frame.
func sse(data string) string { return "data: " + data + "\n\n" }

func chunk(content string) string {
	b, _ := json.Marshal(content)
	return sse(`{"id":"gen-1","model":"google/gemini-2.5-flash-lite","choices":[{"index":0,"delta":{"role":"assistant","content":` + string(b) + `}}]}`)
}

var renewalChunks = []string{
	": OPENROUTER PROCESSING\n\n",
	chunk("[["),
	chunk("sc27|0."),
	chunk("91]]\nАрман, "),
	": OPENROUTER PROCESSING\n\n",
	chunk("продлим полис."),
	sse(`{"id":"gen-1","model":"google/gemini-2.5-flash-lite","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}],"usage":{"prompt_tokens":5120,"completion_tokens":17,"total_tokens":5137}}`),
	"data: [DONE]\n\n",
}

func newFake(t *testing.T, chunks []string) (*fakeLLM, *httptest.Server) {
	t.Helper()
	f := &fakeLLM{chunks: chunks}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

// record returns an emit func and the slice it appends to.
func record() (func(Event), *[]Event) {
	var evs []Event
	return func(e Event) { evs = append(evs, e) }, &evs
}

func TestOpenRouterTurn(t *testing.T) {
	f, srv := newFake(t, renewalChunks)
	o := NewOpenRouter("test-key", srv.URL, "test/model", nil, fixture(t))
	ctx := context.Background()

	text := "Здравствуйте, мой номер +7 701 000 00 01, хочу продлить полис"
	emit, evs := record()
	res, err := o.Turn(ctx, Input{SessionID: "s1", Channel: "web", Text: text, Lang: "ru"}, emit)
	if err != nil {
		t.Fatal(err)
	}

	// Events: decision first, then the reply deltas without the header.
	var kinds []string
	for _, e := range *evs {
		kinds = append(kinds, e.Kind)
	}
	if want := []string{KindDecision, KindDelta, KindDelta}; !slices.Equal(kinds, want) {
		t.Fatalf("event kinds = %v, want %v", kinds, want)
	}
	d := (*evs)[0].Decision
	if d.ScenarioID != "SC27" || d.Confidence != 0.91 || d.Status != "route" || d.Language != "ru" || !strings.HasPrefix(d.Reason, "Policy renewal: ") {
		t.Errorf("decision = %+v", d)
	}
	if (*evs)[1].Text != "Арман, " || (*evs)[2].Text != "продлим полис." {
		t.Errorf("deltas = %q, %q", (*evs)[1].Text, (*evs)[2].Text)
	}

	if res.Model != "google/gemini-2.5-flash-lite" || res.PromptTokens != 5120 || res.CompletionTokens != 17 {
		t.Errorf("result = %+v", res)
	}
	if res.Reply != "Арман, продлим полис." || res.Raw != "[[sc27|0.91]]\nАрман, продлим полис." {
		t.Errorf("reply = %q, raw = %q", res.Reply, res.Raw)
	}
	if res.TTFT <= 0 || res.Total < res.TTFT {
		t.Errorf("TTFT = %v, Total = %v", res.TTFT, res.Total)
	}

	req := f.last(t)
	if req.Model != "test/model" || req.Models != nil || !req.Stream || req.Temperature != 0.3 || req.MaxTokens != 180 {
		t.Errorf("request = model %q models %v stream %v temperature %v max_tokens %d", req.Model, req.Models, req.Stream, req.Temperature, req.MaxTokens)
	}
	if req.Provider.Sort != "latency" || !req.Usage.Include {
		t.Errorf("provider.sort = %q, usage.include = %v", req.Provider.Sort, req.Usage.Include)
	}
	if got := req.header.Get("Authorization"); got != "Bearer test-key" {
		t.Error("missing bearer token")
	}
	if req.header.Get("HTTP-Referer") != appReferer || req.header.Get("X-Title") != appTitle {
		t.Errorf("attribution headers = %q, %q", req.header.Get("HTTP-Referer"), req.header.Get("X-Title"))
	}
	if len(req.Messages) != 3 {
		t.Fatalf("got %d messages, want 3", len(req.Messages))
	}
	static, dynamic, user := req.Messages[0], req.Messages[1], req.Messages[2]
	if static.Role != "system" || !strings.Contains(static.Content, "OUTPUT FORMAT") || !strings.Contains(static.Content, "SC27 [normal] Policy renewal") {
		t.Errorf("static system message = %.120q", static.Content)
	}
	if strings.Contains(static.Content, "Tulegenov") || strings.Contains(static.Content, "850314300121") {
		t.Error("caller data leaked into the static (cacheable) prompt")
	}
	for _, want := range []string{"CALL CONTEXT", "Russian (ru)", "Channel: web", "Arman Tulegenov", "SQ-OGPO-104501", "CL-500198"} {
		if dynamic.Role != "system" || !strings.Contains(dynamic.Content, want) {
			t.Errorf("dynamic system message lacks %q: %s", want, dynamic.Content)
		}
	}
	if strings.Contains(dynamic.Content, "SQ-DMS-604220") {
		t.Error("card contains another client's policy")
	}
	if user.Role != "user" || user.Content != text {
		t.Errorf("user message = %+v", user)
	}
}

func TestOpenRouterHistory(t *testing.T) {
	f, srv := newFake(t, renewalChunks)
	o := NewOpenRouter("k", srv.URL, "", nil, fixture(t))
	ctx := context.Background()

	res, err := o.Turn(ctx, Input{SessionID: "s1", Text: "мой номер 8 701 000 00 01, продлить полис", Lang: "ru"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	o.Commit("s1", "мой номер 8 701 000 00 01, продлить полис", res.Raw)

	if _, err := o.Turn(ctx, Input{SessionID: "s1", Text: "Иә", Lang: "kk"}, nil); err != nil {
		t.Fatal(err)
	}
	req := f.last(t)
	if req.Model != DefaultModel {
		t.Errorf("model = %q, want default %q", req.Model, DefaultModel)
	}
	var roles []string
	for _, m := range req.Messages {
		roles = append(roles, m.Role)
	}
	if want := []string{"system", "system", "user", "assistant", "user"}; !slices.Equal(roles, want) {
		t.Fatalf("roles = %v, want %v", roles, want)
	}
	if req.Messages[3].Content != res.Raw {
		t.Errorf("assistant history = %q, want raw output with header", req.Messages[3].Content)
	}
	ctxMsg := req.Messages[1].Content
	if !strings.Contains(ctxMsg, "Kazakh (kk)") || !strings.Contains(ctxMsg, "Arman Tulegenov") {
		t.Errorf("second turn context should keep the identified caller and switch language: %s", ctxMsg)
	}

	// History is trimmed to MaxHistory messages and starts with a user turn.
	o.MaxHistory = 3
	o.Commit("s1", "Иә", "[[SC27|0.95]]\nГотово.")
	if _, err := o.Turn(ctx, Input{SessionID: "s1", Text: "Рақмет", Lang: "kk"}, nil); err != nil {
		t.Fatal(err)
	}
	msgs := f.last(t).Messages
	if len(msgs) != 5 || msgs[2].Content != "Иә" || msgs[3].Content != "[[SC27|0.95]]\nГотово." {
		t.Errorf("trimmed history = %+v", msgs[2:])
	}

	// End forgets the session: no history and no identified caller.
	o.End("s1")
	if _, err := o.Turn(ctx, Input{SessionID: "s1", Text: "Сәлем", Lang: "kk"}, nil); err != nil {
		t.Fatal(err)
	}
	msgs = f.last(t).Messages
	if len(msgs) != 3 || !strings.Contains(msgs[1].Content, "not identified yet") {
		t.Errorf("after End: %d messages, context %q", len(msgs), msgs[1].Content)
	}
}

func TestOpenRouterSpeculativeHasNoSideEffects(t *testing.T) {
	f, srv := newFake(t, renewalChunks)
	o := NewOpenRouter("k", srv.URL, "m", nil, fixture(t))
	ctx := context.Background()
	partial := "мой ИИН 850314300121"

	if _, err := o.Turn(ctx, Input{SessionID: "s", Text: partial, Lang: "ru", Speculative: true}, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.last(t).Messages[1].Content, "Arman Tulegenov") {
		t.Error("speculative turn should still see the caller card")
	}
	if _, err := o.Turn(ctx, Input{SessionID: "s", Text: "продлить", Lang: "ru"}, nil); err != nil {
		t.Fatal(err)
	}
	if msgs := f.last(t).Messages; len(msgs) != 3 || !strings.Contains(msgs[1].Content, "not identified yet") {
		t.Error("speculative turn changed session state")
	}

	// A promoted speculative result is committed: Commit identifies the caller.
	o.Commit("s", partial, "[[SC27|0.9]]\nАрман, слушаю.")
	if _, err := o.Turn(ctx, Input{SessionID: "s", Text: "да", Lang: "ru"}, nil); err != nil {
		t.Fatal(err)
	}
	if msgs := f.last(t).Messages; len(msgs) != 5 || !strings.Contains(msgs[1].Content, "Arman Tulegenov") {
		t.Errorf("after Commit: %d messages, context %q", len(msgs), msgs[1].Content)
	}
}

func TestOpenRouterIdentification(t *testing.T) {
	f, srv := newFake(t, renewalChunks)
	o := NewOpenRouter("k", srv.URL, "m", nil, fixture(t))
	ctx := context.Background()

	if _, err := o.Turn(ctx, Input{SessionID: "a", Channel: "twilio", CallerID: "+77010000002", Text: "Сәлеметсіз бе", Lang: "kk"}, nil); err != nil {
		t.Fatal(err)
	}
	if c := f.last(t).Messages[1].Content; !strings.Contains(c, "Aigerim Bekova") || !strings.Contains(c, "SQ-DMS-604220") {
		t.Errorf("caller ID not used: %s", c)
	}

	if _, err := o.Turn(ctx, Input{SessionID: "b", Text: "мой номер 8 701 999 99 99", Lang: "ru"}, nil); err != nil {
		t.Fatal(err)
	}
	if c := f.last(t).Messages[1].Content; !strings.Contains(c, "not in the client base") {
		t.Errorf("unknown number not reported: %s", c)
	}

	// Once identified, another person's IIN (e.g. a new driver) does not switch the caller.
	o.Commit("a", "Сәлеметсіз бе", "[[SC27|0.8]]\nАйгерім, тыңдап тұрмын.")
	if _, err := o.Turn(ctx, Input{SessionID: "a", CallerID: "+77010000002", Text: "жүргізуші ЖСН 850314300121", Lang: "kk"}, nil); err != nil {
		t.Fatal(err)
	}
	if c := f.last(t).Messages[1].Content; !strings.Contains(c, "Aigerim Bekova") {
		t.Errorf("identified caller switched: %s", c)
	}
}

func TestOpenRouterFallbacks(t *testing.T) {
	f, srv := newFake(t, renewalChunks)
	o := NewOpenRouter("k", srv.URL, "a/model", []string{"b/model", "", "a/model"}, nil)
	if _, err := o.Turn(context.Background(), Input{Text: "Привет", Lang: "ru"}, nil); err != nil {
		t.Fatal(err)
	}
	req := f.last(t)
	if !slices.Equal(req.Models, []string{"a/model", "b/model"}) || strings.Contains(req.raw, `"model":`) {
		t.Errorf("models = %v, raw body has model field: %v", req.Models, strings.Contains(req.raw, `"model":`))
	}
	if !strings.Contains(req.Messages[0].Content, "No scenario catalog is loaded") {
		t.Error("nil dataset should give the generic prompt")
	}
}

func TestOpenRouterErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, `{"error":{"message":"bad credentials: %s","code":401}}%s`, r.Header.Get("Authorization"), strings.Repeat("x", 600))
		}))
		defer srv.Close()
		o := NewOpenRouter("sk-or-secret", srv.URL, "m", nil, nil)
		_, err := o.Turn(ctx, Input{Text: "Привет", Lang: "ru"}, nil)
		if err == nil || !strings.Contains(err.Error(), "status 401") || !strings.Contains(err.Error(), "bad credentials") {
			t.Fatalf("err = %v", err)
		}
		if strings.Contains(err.Error(), "sk-or-secret") {
			t.Error("error leaks the API key")
		}
		if len(err.Error()) > 400 {
			t.Errorf("error body not truncated: %d bytes", len(err.Error()))
		}
	})

	t.Run("in-stream", func(t *testing.T) {
		_, srv := newFake(t, []string{
			chunk("[[SC01|0.9]]\nСейчас "),
			sse(`{"error":{"code":502,"message":"Provider returned error"},"choices":[{"index":0,"delta":{"content":""},"finish_reason":"error"}]}`),
		})
		o := NewOpenRouter("k", srv.URL, "m", nil, nil)
		emit, evs := record()
		_, err := o.Turn(ctx, Input{Text: "Сколько стоит ОГПО?", Lang: "ru"}, emit)
		if err == nil || !strings.Contains(err.Error(), "Provider returned error") {
			t.Fatalf("err = %v", err)
		}
		if len(*evs) != 2 {
			t.Errorf("events before the error = %d, want decision + delta", len(*evs))
		}
	})

	t.Run("empty", func(t *testing.T) {
		_, srv := newFake(t, []string{"data: [DONE]\n\n"})
		o := NewOpenRouter("k", srv.URL, "m", nil, nil)
		if _, err := o.Turn(ctx, Input{Text: "Алло", Lang: "ru"}, nil); err == nil {
			t.Fatal("want an error for an empty reply")
		}
	})

	t.Run("blank utterance", func(t *testing.T) {
		o := NewOpenRouter("k", "http://127.0.0.1:1", "m", nil, nil)
		if _, err := o.Turn(ctx, Input{Text: "  "}, nil); !errors.Is(err, ErrEmptyText) {
			t.Fatalf("err = %v, want ErrEmptyText", err)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, ": OPENROUTER PROCESSING\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}))
		defer srv.Close()
		o := NewOpenRouter("k", srv.URL, "m", nil, nil)
		ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		start := time.Now()
		if _, err := o.Turn(ctx, Input{Text: "Алло", Lang: "ru"}, nil); err == nil {
			t.Fatal("want a context error")
		}
		if time.Since(start) > 2*time.Second {
			t.Error("cancellation took too long")
		}
	})
}

func TestDecisionHelpers(t *testing.T) {
	d := fixture(t)
	for id, want := range map[string]string{"SC12": "route", "SC37": "handoff", "SYS_UNCLEAR": "clarify", "SYS_OUT_OF_SCOPE": "out_of_scope", "SYS_GOODBYE": "goodbye"} {
		if got := status(d, id); got != want {
			t.Errorf("status(%s) = %q, want %q", id, got, want)
		}
	}
	for in, want := range map[string]string{"SC27": "SC27", "UNCLEAR": "SYS_UNCLEAR", "SC99": "SC99"} {
		if got := d.canonicalID(in); got != want {
			t.Errorf("canonicalID(%s) = %q, want %q", in, got, want)
		}
	}
	if got := realData(t).canonicalID("SC7"); got != "SC07" {
		t.Errorf("canonicalID(SC7) = %q, want SC07", got)
	}
}

func TestOpenRouterWarm(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/key" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		auth = r.Header.Get("Authorization")
		if auth != "Bearer good" {
			http.Error(w, `{"error":{"message":"invalid key"}}`, http.StatusUnauthorized)
			return
		}
		io.WriteString(w, `{"data":{"label":"test"}}`)
	}))
	defer srv.Close()
	ctx := context.Background()
	if err := NewOpenRouter("good", srv.URL+"/", "m", nil, nil).Warm(ctx); err != nil {
		t.Fatalf("warm: %v", err)
	}
	if auth != "Bearer good" {
		t.Error("warm request not authenticated")
	}
	if err := NewOpenRouter("wrong-key", srv.URL, "m", nil, nil).Warm(ctx); err == nil || !strings.Contains(err.Error(), "status 401") {
		t.Fatalf("warm with a rejected key: %v", err)
	}
}
