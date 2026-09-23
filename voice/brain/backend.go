package brain

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Backend is a Brain that delegates turns to the team backend
// (docs/API_CONTRACT.md): POST /api/turn streams the backend's own events,
// which are relayed raw and mapped to decisions and reply deltas.
type Backend struct {
	URL  string
	HTTP *http.Client // default: shared tuned client

	mu       sync.Mutex
	sessions map[string]string // voice session id -> backend session id
	legacy   atomic.Bool       // backend only serves POST /api/turns/stream
	plural   atomic.Bool       // backend creates sessions at /api/sessions
}

// NewBackend returns a Backend for the given base URL.
func NewBackend(url string) *Backend {
	return &Backend{URL: url, HTTP: sharedHTTP()}
}

// Name implements Brain.
func (b *Backend) Name() string { return "backend" }

// Stateless implements Brain: the backend keeps its own dialog state.
func (b *Backend) Stateless() bool { return false }

// Commit implements Brain; the backend records turns itself.
func (b *Backend) Commit(sessionID, user, assistant string) {}

// End implements Brain: it forgets the backend session mapping.
func (b *Backend) End(sessionID string) {
	b.mu.Lock()
	delete(b.sessions, sessionID)
	b.mu.Unlock()
}

// Healthy reports whether GET {URL}/healthz or {URL}/health answers 2xx
// within 1.5 s.
func (b *Backend) Healthy(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	for _, p := range []string{"/healthz", "/health"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.base()+p, nil)
		if err != nil {
			return false
		}
		resp, err := b.client().Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return false
			}
			continue
		}
		release(resp.Body)
		if resp.StatusCode/100 == 2 {
			return true
		}
	}
	return false
}

// Turn implements Brain.
func (b *Backend) Turn(ctx context.Context, in Input, emit func(Event)) (Result, error) {
	start := time.Now()
	res := Result{Model: "backend"}
	if strings.TrimSpace(in.Text) == "" {
		return res, ErrEmptyText
	}
	if emit == nil {
		emit = func(Event) {}
	}
	sid, err := b.session(ctx, in.SessionID)
	if err != nil {
		return res, err
	}
	legacy := b.legacy.Load()
	fellBack, retried := false, false
	for {
		path, resp, err := b.sendTurn(ctx, sid, in, legacy)
		if err != nil {
			return res, err
		}
		if resp.StatusCode/100 == 2 {
			if fellBack {
				b.legacy.Store(true)
			}
			err = b.read(resp.Body, start, emit, &res)
			release(resp.Body)
			res.Total = time.Since(start)
			if err != nil && ctx.Err() != nil {
				err = fmt.Errorf("backend: %w", ctx.Err())
			}
			return res, err
		}
		code, body := resp.StatusCode, snippet(resp)
		lower := strings.ToLower(body)
		notFound := code == http.StatusNotFound
		aboutSession := strings.Contains(lower, "session")
		switch {
		case notFound && !legacy && !aboutSession:
			// No /api/turn route: this backend speaks NDJSON at /api/turns/stream.
			legacy, fellBack = true, true
			continue
		case !retried && (code == http.StatusConflict || notFound && aboutSession):
			retried = true
			if notFound || strings.Contains(lower, "limit") {
				// Unknown or exhausted backend session: start a new one.
				b.End(in.SessionID)
				if sid, err = b.session(ctx, in.SessionID); err != nil {
					return res, err
				}
			} else if err := sleepCtx(ctx, 250*time.Millisecond); err != nil {
				return res, fmt.Errorf("backend: %w", err)
			}
			continue
		}
		return res, fmt.Errorf("backend: POST %s: status %d: %s", path, code, body)
	}
}

// turnRequest is the body of POST /api/turn.
type turnRequest struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	LangHint  string `json:"lang_hint,omitempty"`
	ClientT0  int64  `json:"client_t0"`
	TTS       bool   `json:"tts"`
}

// streamRequest is the body of POST /api/turns/stream.
type streamRequest struct {
	SessionID string  `json:"session_id"`
	Text      string  `json:"text"`
	InputKind string  `json:"input_kind"`
	STTms     float64 `json:"stt_ms"`
}

// sendTurn posts the utterance to /api/turn, or to /api/turns/stream for a
// legacy backend, and returns the path used.
func (b *Backend) sendTurn(ctx context.Context, sid string, in Input, legacy bool) (string, *http.Response, error) {
	text := strings.TrimSpace(in.Text)
	if legacy {
		body, _ := json.Marshal(streamRequest{SessionID: sid, Text: text, InputKind: "microphone", STTms: in.STTms})
		resp, err := b.post(ctx, "/api/turns/stream", body, "application/x-ndjson, text/event-stream")
		return "/api/turns/stream", resp, err
	}
	t0 := in.SpeechEnd
	if t0.IsZero() {
		t0 = time.Now()
	}
	body, _ := json.Marshal(turnRequest{SessionID: sid, Text: text, LangHint: in.Lang, ClientT0: t0.UnixMilli()})
	resp, err := b.post(ctx, "/api/turn", body, "text/event-stream")
	return "/api/turn", resp, err
}

// session returns the backend session for a voice session, creating it on
// first use.
func (b *Backend) session(ctx context.Context, voiceID string) (string, error) {
	b.mu.Lock()
	id, ok := b.sessions[voiceID]
	b.mu.Unlock()
	if ok {
		return id, nil
	}
	id, err := b.newSession(ctx)
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	if b.sessions == nil {
		b.sessions = make(map[string]string)
	}
	b.sessions[voiceID] = id
	b.mu.Unlock()
	return id, nil
}

// newSession creates a backend session via POST /api/session, falling back
// to /api/sessions.
func (b *Backend) newSession(ctx context.Context) (string, error) {
	paths := []string{"/api/session", "/api/sessions"}
	if b.plural.Load() {
		paths = paths[1:]
	}
	for i, p := range paths {
		resp, err := b.post(ctx, p, []byte("{}"), "application/json")
		if err != nil {
			return "", err
		}
		missing := resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed
		if missing && i+1 < len(paths) {
			snippet(resp)
			continue
		}
		if resp.StatusCode/100 != 2 {
			return "", fmt.Errorf("backend: POST %s: status %d: %s", p, resp.StatusCode, snippet(resp))
		}
		var v struct {
			SessionID json.RawMessage `json:"session_id"`
			ID        json.RawMessage `json:"id"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&v)
		release(resp.Body)
		if err != nil {
			return "", fmt.Errorf("backend: POST %s: %w", p, err)
		}
		id := jsonID(v.SessionID)
		if id == "" {
			id = jsonID(v.ID)
		}
		if id == "" {
			return "", fmt.Errorf("backend: POST %s: no session_id in response", p)
		}
		if p == "/api/sessions" {
			b.plural.Store(true)
		}
		return id, nil
	}
	return "", errors.New("backend: no session endpoint")
}

// backendEvent holds the fields of a backend stream event the brain reads.
type backendEvent struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Message  string          `json:"message"`
	Decision json.RawMessage `json:"decision"`
	Data     json.RawMessage `json:"data"`
}

// routerDecision is the "decision" of a router.decision event.
type routerDecision struct {
	Scenarios []struct {
		ScenarioID string  `json:"scenario_id"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	} `json:"scenarios"`
	Language string `json:"language"`
	Reason   string `json:"reason"`
	Model    string `json:"model"`
}

// eventData is the "data" of the NDJSON variant's events.
type eventData struct {
	ScenarioID string  `json:"scenario_id"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
	Language   string  `json:"language"`
	Reason     string  `json:"reason"`
	Reply      string  `json:"reply"`
	Message    string  `json:"message"`
}

// read consumes an SSE or NDJSON turn stream. Every JSON event is relayed as
// KindRaw first, then mapped to a decision or reply text.
func (b *Backend) read(body io.Reader, start time.Time, emit func(Event), res *Result) error {
	var reply strings.Builder
	decided, spoke := false, false
	decide := func(d *Decision) {
		if !decided && d.ScenarioID != "" {
			decided = true
			emit(Event{Kind: KindDecision, Decision: d})
		}
	}
	say := func(text string) {
		if text == "" {
			return
		}
		if !spoke {
			spoke = true
			res.TTFT = time.Since(start)
		}
		reply.WriteString(text)
		emit(Event{Kind: KindDelta, Text: text})
	}
	err := eachLine(body, func(line []byte) (bool, error) {
		data := payload(line)
		if string(data) == "[DONE]" {
			return true, nil
		}
		if data == nil || !json.Valid(data) {
			return false, nil
		}
		emit(Event{Kind: KindRaw, Raw: json.RawMessage(data)})
		var ev backendEvent
		_ = json.Unmarshal(data, &ev) // partial decoding is fine
		var d eventData
		_ = json.Unmarshal(ev.Data, &d)
		switch ev.Type {
		case "router.decision":
			var rd routerDecision
			if json.Unmarshal(ev.Decision, &rd) != nil {
				break
			}
			if rd.Model != "" {
				res.Model = rd.Model
			}
			if len(rd.Scenarios) > 0 {
				s := rd.Scenarios[0]
				decide(&Decision{ScenarioID: s.ScenarioID, Confidence: s.Confidence, Language: rd.Language, Reason: cmp.Or(s.Reason, rd.Reason)})
			}
		case "routing_complete":
			decide(&Decision{ScenarioID: d.ScenarioID, Confidence: d.Confidence, Status: d.Status, Language: d.Language, Reason: d.Reason})
		case "response.delta":
			say(ev.Text)
		case "policy":
			if !spoke {
				say(d.Reply)
			}
		case "response.final":
			if !spoke {
				say(ev.Text)
			}
		case "error":
			msg := cmp.Or(ev.Message, d.Message)
			if msg == "" {
				msg = truncate(string(data), 300)
			}
			return true, errors.New(msg)
		case "turn.done", "done":
			return true, nil
		}
		return false, nil
	})
	res.Reply = strings.TrimSpace(reply.String())
	res.Raw = res.Reply
	if err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	return nil
}

func (b *Backend) post(ctx context.Context, path string, body []byte, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.base()+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("backend: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)
	resp, err := b.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("backend: %w", err)
	}
	return resp, nil
}

func (b *Backend) base() string { return strings.TrimRight(b.URL, "/") }

func (b *Backend) client() *http.Client {
	if b.HTTP != nil {
		return b.HTTP
	}
	return sharedHTTP()
}

// jsonID reads a JSON string or number as an id.
func jsonID(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return ""
}
