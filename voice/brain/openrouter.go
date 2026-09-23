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
	"slices"
	"strings"
	"sync"
	"time"
)

// DefaultModel is used when no model is configured.
const DefaultModel = "google/gemini-2.5-flash-lite"

const (
	defaultBaseURL     = "https://openrouter.ai/api/v1"
	defaultTemperature = 0.3
	defaultMaxTokens   = 180
	defaultMaxHistory  = 10
	sessionIdle        = 30 * time.Minute
	turnTimeout        = 30 * time.Second
	appReferer         = "https://github.com/BAITC-Hacks/hack-9dd7b5f3-plus"
	appTitle           = "Saqta Voice Router"
)

// OpenRouter is a Brain that streams replies from the OpenRouter chat
// completions API. The model writes a [[SCENARIO|CONFIDENCE]] header line
// followed by the spoken reply; the header becomes the Decision.
type OpenRouter struct {
	Key, BaseURL, Model string       // BaseURL default "https://openrouter.ai/api/v1"
	Fallbacks           []string     // sent as "models": [Model, Fallbacks...] when non-empty
	Data                *Dataset     // may be nil: generic prompt without catalog
	HTTP                *http.Client // default: shared tuned client (HTTP/2, keep-alive)
	Temperature         float64      // NewOpenRouter sets 0.3
	MaxTokens           int          // default 180
	MaxHistory          int          // messages kept per session, default 10

	mu        sync.Mutex
	sessions  map[string]*session
	lastPrune time.Time
}

// session is the per-call conversation state.
type session struct {
	history []message
	client  *Client // identified caller, once known
	seen    time.Time
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewOpenRouter returns an OpenRouter brain with defaults applied.
func NewOpenRouter(key, baseURL, model string, fallbacks []string, data *Dataset) *OpenRouter {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if model == "" {
		model = DefaultModel
	}
	return &OpenRouter{
		Key:         key,
		BaseURL:     baseURL,
		Model:       model,
		Fallbacks:   fallbacks,
		Data:        data,
		HTTP:        sharedHTTP(),
		Temperature: defaultTemperature,
		MaxTokens:   defaultMaxTokens,
		MaxHistory:  defaultMaxHistory,
	}
}

// Name implements Brain.
func (o *OpenRouter) Name() string { return "openrouter" }

// Stateless implements Brain: history changes only in Commit.
func (o *OpenRouter) Stateless() bool { return true }

// Warm sends a cheap authenticated GET {base}/key so the TLS connection is
// open before the first turn. It also reports a rejected key early.
func (o *OpenRouter) Warm(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL()+"/key", nil)
	if err != nil {
		return fmt.Errorf("openrouter: %w", err)
	}
	o.setHeaders(req)
	resp, err := o.client().Do(req)
	if err != nil {
		return fmt.Errorf("openrouter: warm: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("openrouter: warm: status %d: %s", resp.StatusCode, o.redact(snippet(resp)))
	}
	release(resp.Body)
	return nil
}

// Turn implements Brain. It identifies the caller (caller ID, or a phone
// number or IIN in the utterance), streams the model reply and emits the
// decision and reply deltas as they arrive.
func (o *OpenRouter) Turn(ctx context.Context, in Input, emit func(Event)) (Result, error) {
	start := time.Now()
	models := o.models()
	res := Result{Model: models[0]}
	if strings.TrimSpace(in.Text) == "" {
		return res, ErrEmptyText
	}
	if emit == nil {
		emit = func(Event) {}
	}
	lang := replyLang(in.Lang)
	history, client, note := o.prepare(in)

	msgs := make([]message, 0, len(history)+3)
	msgs = append(msgs,
		message{"system", o.Data.StaticPrompt()},
		message{"system", callContext(o.Data, in, lang, client, note)})
	msgs = append(msgs, history...)
	msgs = append(msgs, message{"user", strings.TrimSpace(in.Text)})
	body, err := o.requestBody(models, msgs)
	if err != nil {
		return res, fmt.Errorf("openrouter: %w", err)
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, turnTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return res, fmt.Errorf("openrouter: %w", err)
	}
	o.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := o.client().Do(req)
	if err != nil {
		return res, fmt.Errorf("openrouter: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return res, fmt.Errorf("openrouter: status %d: %s", resp.StatusCode, o.redact(snippet(resp)))
	}
	err = o.stream(resp.Body, start, lang, emit, &res)
	release(resp.Body)
	res.Total = time.Since(start)
	if err != nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		return res, fmt.Errorf("openrouter: %w", err)
	}
	return res, nil
}

// Commit implements Brain: it appends the exchange to the session history
// (trimmed to MaxHistory) and identifies the caller from the utterance if
// that has not happened yet.
func (o *OpenRouter) Commit(sessionID, user, assistant string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	s := o.sessionLocked(sessionID, time.Now())
	if s.client == nil {
		s.client, _ = identify(o.Data, "", user)
	}
	if u := strings.TrimSpace(user); u != "" {
		s.history = append(s.history, message{"user", u})
	}
	if a := strings.TrimSpace(assistant); a != "" {
		s.history = append(s.history, message{"assistant", a})
	}
	limit := o.MaxHistory
	if limit <= 0 {
		limit = defaultMaxHistory
	}
	if n := len(s.history) - limit; n > 0 {
		s.history = slices.Clone(s.history[n:])
	}
	for len(s.history) > 0 && s.history[0].Role != "user" {
		s.history = s.history[1:]
	}
}

// End implements Brain.
func (o *OpenRouter) End(sessionID string) {
	o.mu.Lock()
	delete(o.sessions, sessionID)
	o.mu.Unlock()
}

// prepare snapshots the session history and resolves the caller for this
// turn. Only a non-speculative turn remembers a newly identified caller.
func (o *OpenRouter) prepare(in Input) ([]message, *Client, string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	s := o.sessionLocked(in.SessionID, time.Now())
	history := slices.Clone(s.history)
	if s.client != nil {
		return history, s.client, ""
	}
	c, note := identify(o.Data, in.CallerID, in.Text)
	if c != nil && !in.Speculative {
		s.client = c
	}
	return history, c, note
}

// sessionLocked returns the session for id, creating it if needed, and
// prunes sessions idle for longer than 30 minutes. o.mu must be held.
func (o *OpenRouter) sessionLocked(id string, now time.Time) *session {
	if o.sessions == nil {
		o.sessions = make(map[string]*session)
	}
	if now.Sub(o.lastPrune) > time.Minute {
		for k, s := range o.sessions {
			if now.Sub(s.seen) > sessionIdle {
				delete(o.sessions, k)
			}
		}
		o.lastPrune = now
	}
	s := o.sessions[id]
	if s == nil {
		s = &session{}
		o.sessions[id] = s
	}
	s.seen = now
	return s
}

// identify finds the caller by caller ID, or by a phone number or IIN in
// text. The note explains a spoken identifier that matched no client.
func identify(d *Dataset, callerID, text string) (*Client, string) {
	if d == nil {
		return nil, ""
	}
	if c := d.ClientByPhone(callerID); c != nil {
		return c, ""
	}
	phone, iin := FindPhone(text), FindIIN(text)
	if c := d.ClientByPhone(phone); c != nil {
		return c, ""
	}
	if c := d.ClientByIIN(iin); c != nil {
		return c, ""
	}
	if phone != "" || iin != "" {
		return nil, notFoundNote
	}
	return nil, ""
}

type chatRequest struct {
	Model       string    `json:"model,omitempty"`
	Models      []string  `json:"models,omitempty"`
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
}

func (o *OpenRouter) requestBody(models []string, msgs []message) ([]byte, error) {
	r := chatRequest{Messages: msgs, Stream: true, Temperature: o.Temperature, MaxTokens: o.MaxTokens}
	if r.MaxTokens <= 0 {
		r.MaxTokens = defaultMaxTokens
	}
	if len(models) > 1 {
		r.Models = models
	} else {
		r.Model = models[0]
	}
	r.Provider.Sort = "latency"
	r.Usage.Include = true
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// chatChunk is one streamed chat completion chunk.
type chatChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Code    any    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// stream reads the SSE body, emitting the decision and reply deltas.
func (o *OpenRouter) stream(body io.Reader, start time.Time, lang string, emit func(Event), res *Result) error {
	var (
		hp         headerParser
		raw, reply strings.Builder
		decided    bool
	)
	out := func(d *Decision, text string) {
		if d != nil && !decided {
			decided = true
			d.ScenarioID = o.Data.canonicalID(d.ScenarioID)
			d.Language = lang
			d.Status = status(o.Data, d.ScenarioID)
			d.Reason = o.Data.describe(d.ScenarioID)
			emit(Event{Kind: KindDecision, Decision: d})
		}
		if text != "" {
			if reply.Len() == 0 {
				res.TTFT = time.Since(start)
			}
			reply.WriteString(text)
			emit(Event{Kind: KindDelta, Text: text})
		}
	}
	err := eachLine(body, func(line []byte) (bool, error) {
		data := payload(line)
		if data == nil {
			return false, nil
		}
		if string(data) == "[DONE]" {
			return true, nil
		}
		var ch chatChunk
		if json.Unmarshal(data, &ch) != nil {
			return false, nil
		}
		if e := ch.Error; e != nil {
			if e.Code != nil {
				return true, fmt.Errorf("stream error %v: %s", e.Code, e.Message)
			}
			return true, fmt.Errorf("stream error: %s", e.Message)
		}
		if ch.Model != "" {
			res.Model = ch.Model
		}
		if u := ch.Usage; u != nil {
			res.PromptTokens, res.CompletionTokens = u.PromptTokens, u.CompletionTokens
		}
		for _, c := range ch.Choices {
			if c.Index == 0 && c.Delta.Content != "" {
				raw.WriteString(c.Delta.Content)
				out(hp.feed(c.Delta.Content))
			}
		}
		return false, nil
	})
	if err == nil {
		out(hp.flush())
	}
	res.Raw = strings.TrimSpace(raw.String())
	res.Reply = strings.TrimSpace(reply.String())
	if err == nil && res.Raw == "" {
		err = errors.New("empty reply")
	}
	return err
}

func (o *OpenRouter) models() []string {
	out := []string{cmp.Or(o.Model, DefaultModel)}
	for _, m := range o.Fallbacks {
		if m = strings.TrimSpace(m); m != "" && !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	return out
}

func (o *OpenRouter) baseURL() string {
	return strings.TrimRight(cmp.Or(o.BaseURL, defaultBaseURL), "/")
}

func (o *OpenRouter) client() *http.Client {
	if o.HTTP != nil {
		return o.HTTP
	}
	return sharedHTTP()
}

func (o *OpenRouter) setHeaders(req *http.Request) {
	if o.Key != "" {
		req.Header.Set("Authorization", "Bearer "+o.Key)
	}
	req.Header.Set("HTTP-Referer", appReferer)
	req.Header.Set("X-Title", appTitle)
}

// redact removes the API key from text that is about to leave the package.
func (o *OpenRouter) redact(s string) string {
	if o.Key == "" {
		return s
	}
	return strings.ReplaceAll(s, o.Key, "[redacted]")
}
