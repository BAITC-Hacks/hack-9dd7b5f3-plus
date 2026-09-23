package dialog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"hackathon/backend/internal/catalog"
	"hackathon/backend/internal/config"
	"hackathon/backend/internal/events"
	"hackathon/backend/internal/lang"
	"hackathon/backend/internal/llm"
	"hackathon/backend/internal/mockbackend"
	"hackathon/backend/internal/retrieval"
	"hackathon/backend/internal/router"
	"hackathon/backend/internal/store"
	"hackathon/backend/internal/stt"
	"hackathon/backend/internal/triage"
	"hackathon/backend/internal/tts"
)

// ErrEmptyTranscript is returned when STT heard nothing.
var ErrEmptyTranscript = errors.New("empty transcript")

// Engine wires every layer together.
type Engine struct {
	cfg     config.Config
	cat     *catalog.Catalog
	ix      *retrieval.Index
	primary router.Router
	mock    *router.MockRouter
	policy  router.Policy
	tpl     *router.Templates
	be      *mockbackend.Backend
	stt     stt.Provider
	tts     tts.Provider
	store   *store.Store
	bus     *events.Bus
	today   string

	mu       sync.RWMutex
	sessions map[string]*Session
}

// New builds the engine.
func New(cfg config.Config, cat *catalog.Catalog, ix *retrieval.Index, primary router.Router, mock *router.MockRouter, be *mockbackend.Backend, sttP stt.Provider, ttsP tts.Provider, st *store.Store, bus *events.Bus) *Engine {
	return &Engine{
		cfg: cfg, cat: cat, ix: ix, primary: primary, mock: mock, be: be, stt: sttP, tts: ttsP, store: st, bus: bus,
		policy:   router.Policy{ProceedMin: cfg.Policy.ProceedMin, ClarifyMin: cfg.Policy.ClarifyMin, FastMinScore: cfg.Policy.FastPathMinScore, FastMinMargin: cfg.Policy.FastPathMinMargin, FastPathMode: cfg.FastPath},
		tpl:      router.NewTemplates(cat, ix.ScenarioName),
		today:    cat.AsOfDate,
		sessions: map[string]*Session{},
	}
}

func (e *Engine) emit(ev events.Event) { e.bus.Publish(ev) }

// Bus exposes the event bus.
func (e *Engine) Bus() *events.Bus { return e.bus }

// Store exposes the trace store.
func (e *Engine) Store() *store.Store { return e.store }

// Catalog exposes the catalog.
func (e *Engine) Catalog() *catalog.Catalog { return e.cat }

// Backend exposes the mock backend.
func (e *Engine) Backend() *mockbackend.Backend { return e.be }

// Config exposes the configuration.
func (e *Engine) Config() config.Config { return e.cfg }

// TTSSampleRate is the PCM rate sent to the browser.
func (e *Engine) TTSSampleRate() int {
	if e.tts == nil {
		return 24000
	}
	return e.tts.SampleRate()
}

// RouterName describes the primary router.
func (e *Engine) RouterName() string { return e.primary.Name() }

// SystemPrompt returns the LLM system prompt (or the mock description).
func (e *Engine) SystemPrompt() string {
	if r, ok := e.primary.(*router.LLMRouter); ok {
		return r.SystemPrompt()
	}
	return "LLM_PROVIDER=mock: the keyless lexical router is active, no prompt is sent. Set LLM_PROVIDER=openai (+ key) to see the real prompt.\n\nThe prompt that WOULD be sent:\n\n" + router.BuildSystemPrompt(e.cat)
}

// ReloadCatalog re-reads the kit JSON and rebuilds the index and prompt.
func (e *Engine) ReloadCatalog(lex *retrieval.Lexicon) error {
	if err := e.cat.Reload(); err != nil {
		return err
	}
	e.ix = retrieval.New(e.cat, lex)
	e.mock = router.NewMockRouter(e.cat, e.ix)
	e.tpl = router.NewTemplates(e.cat, e.ix.ScenarioName)
	if r, ok := e.primary.(*router.LLMRouter); ok {
		r.RefreshPrompt()
	}
	return nil
}

// NewSession creates and registers a session.
func (e *Engine) NewSession(channel string) *Session {
	s := &Session{ID: newID(), CreatedAt: time.Now(), Channel: channel, Slots: map[string]any{}}
	e.mu.Lock()
	e.sessions[s.ID] = s
	e.mu.Unlock()
	e.store.CreateSession(s.ID, channel)
	return s
}

// Session returns a live session or nil.
func (e *Engine) Session(id string) *Session {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sessions[id]
}

// TurnOptions control one turn.
type TurnOptions struct {
	Source         string // text | voice | browser_stt | audio_file
	Voice          bool   // synthesize the reply
	CollectAudio   bool   // keep the synthesized PCM in the trace (tests)
	TranscriptHint string // mock STT
	T0             time.Time
	ClientT0       int64 // browser timestamp of end of speech (ms)
	STTMs          int
	AudioMs        int
	STTProvider    string
	STTModel       string
}

// ActionRecord is an executed / previewed action in the trace.
type ActionRecord struct {
	Name   string         `json:"name"`
	Args   map[string]any `json:"args,omitempty"`
	Mode   string         `json:"mode"` // execute | preview | handoff
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
	Note   string         `json:"note,omitempty"`
	Ms     int            `json:"ms"`
}

// LLMInfo is the model call summary.
type LLMInfo struct {
	Provider string        `json:"provider"`
	Model    string        `json:"model"`
	Usage    *llm.Usage    `json:"usage,omitempty"`
	TTFTMs   int           `json:"ttft_ms"`
	TotalMs  int           `json:"total_ms"`
	Repaired bool          `json:"repaired,omitempty"`
	Raw      string        `json:"raw,omitempty"`
	Messages []llm.Message `json:"messages,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// Shadow is the asynchronous LLM verification of a fast-path decision.
type Shadow struct {
	LLMPrimary string  `json:"llm_primary"`
	Confidence float64 `json:"confidence"`
	Agree      bool    `json:"agree"`
	Ms         int     `json:"ms"`
	Error      string  `json:"error,omitempty"`
}

// Trace is the full explanation of one turn (shown in the trace panel).
type Trace struct {
	SessionID string    `json:"session_id"`
	Turn      int       `json:"turn"`
	At        time.Time `json:"at"`
	Input     struct {
		Source      string `json:"source"`
		Transcript  string `json:"transcript"`
		STTProvider string `json:"stt_provider,omitempty"`
		STTModel    string `json:"stt_model,omitempty"`
		AudioMs     int    `json:"audio_ms,omitempty"`
	} `json:"input"`
	Language struct {
		Detected string  `json:"detected"`
		KKShare  float64 `json:"kk_share"`
		Reply    string  `json:"reply"`
	} `json:"language"`
	Triage    triage.Signals        `json:"triage"`
	Retrieval []retrieval.Candidate `json:"retrieval"`
	Path      string                `json:"path"`
	FastPath  router.FastPathCheck  `json:"fast_path"`
	Decision  *router.Decision      `json:"decision"`
	Policy    router.Verdict        `json:"policy"`
	Facts     []router.Fact         `json:"facts,omitempty"`
	Actions   []ActionRecord        `json:"actions"`
	State     View                  `json:"state"`
	Reply     struct {
		Text      string `json:"text"`
		Lang      string `json:"lang"`
		Sentences int    `json:"sentences"`
		FollowUp  bool   `json:"follow_up"`
	} `json:"reply"`
	Timings map[string]int `json:"timings"`
	LLM     *LLMInfo       `json:"llm,omitempty"`
	Shadow  *Shadow        `json:"shadow,omitempty"`
	Errors  []string       `json:"errors,omitempty"`
	Audio   []byte         `json:"-"`
}

func ms(t time.Time) int { return int(time.Since(t).Milliseconds()) }

// missingSlots lists required slots of a scenario not yet known.
func (e *Engine) missingSlots(active string, slots map[string]any) []string {
	sc := e.cat.Scenario(active)
	if sc == nil {
		return nil
	}
	var missing []string
	for _, r := range sc.Slots.Required {
		if v, ok := slots[r]; !ok || v == nil || fmt.Sprint(v) == "" {
			missing = append(missing, r)
		}
	}
	return missing
}

// RunTurn processes one client utterance.
func (e *Engine) RunTurn(ctx context.Context, s *Session, text string, opts TurnOptions) (*Trace, error) {
	s.turnMu.Lock()
	defer s.turnMu.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.mu.Lock()
	s.cancelTurn = cancel
	s.TurnIndex++
	turn := s.TurnIndex
	hasPending := s.Pending != nil
	s.mu.Unlock()

	t0 := opts.T0
	if t0.IsZero() {
		t0 = time.Now()
	}
	tr := &Trace{SessionID: s.ID, Turn: turn, At: time.Now(), Timings: map[string]int{}, Actions: []ActionRecord{}}
	tr.Input.Source = opts.Source
	tr.Input.Transcript = text
	tr.Input.STTProvider = opts.STTProvider
	tr.Input.STTModel = opts.STTModel
	tr.Input.AudioMs = opts.AudioMs
	if opts.STTMs > 0 {
		tr.Timings["stt"] = opts.STTMs
	}
	e.emit(events.Event{Type: "turn_start", SessionID: s.ID, Turn: turn, Data: map[string]any{"text": text, "source": opts.Source}})

	// 1. triage
	st := time.Now()
	sig := triage.Analyze(text, hasPending)
	tr.Timings["triage"] = ms(st)
	tr.Triage = sig
	tr.Language.Detected = sig.Language
	tr.Language.KKShare = sig.KKShare
	e.emit(events.Event{Type: "triage", SessionID: s.ID, Turn: turn, Data: sig})

	// 2. lexical retrieval
	st = time.Now()
	cands := e.ix.Search(text, 6)
	tr.Timings["retrieval"] = ms(st)
	tr.Retrieval = cands
	e.emit(events.Event{Type: "retrieval", SessionID: s.ID, Turn: turn, Data: map[string]any{"candidates": cands, "ms": tr.Timings["retrieval"]}})

	// 3. confirmation gate + prefetch backend facts
	st = time.Now()
	facts := e.prefetch(s, sig, tr)
	tr.Timings["facts"] = ms(st)
	if len(facts) > 0 {
		tr.Facts = facts
		e.emit(events.Event{Type: "facts", SessionID: s.ID, Turn: turn, Data: facts})
	}

	in := router.Input{Utterance: text, Signals: sig, Candidates: cands, State: s.stateView(e.missingSlots), Facts: facts, Today: e.today}

	// 4. fast path?
	fc := e.policy.FastPath(e.cat, in)
	tr.FastPath = fc
	e.emit(events.Event{Type: "fast_path", SessionID: s.ID, Turn: turn, Data: fc})
	_, primaryIsMock := e.primary.(*router.MockRouter)
	useFast := fc.Eligible && e.cfg.FastPath == "on" && !primaryIsMock

	// 5. route (streaming)
	var sp *speaker
	var verdict router.Verdict
	suppress := false
	replyLang := lang.ReplyLanguage(sig.Language, sig.KKShare, in.State.Language)
	var replyMu sync.Mutex
	sink := router.Sink{
		OnLLMStart: func(model string, msgs []llm.Message) {
			data := map[string]any{"model": model, "prompt_chars": len(msgs[0].Content) + len(msgs[1].Content)}
			if e.cfg.Debug {
				data["messages"] = msgs
			}
			e.emit(events.Event{Type: "llm_start", SessionID: s.ID, Turn: turn, Data: data})
		},
		OnDelta: func(raw string) {
			if e.cfg.Debug {
				e.emit(events.Event{Type: "llm_delta", SessionID: s.ID, Turn: turn, Data: map[string]any{"text": raw}})
			}
		},
		OnDecision: func(d *router.Decision, routeMs int) {
			replyMu.Lock()
			defer replyMu.Unlock()
			dc := *d
			dc.Scenarios = append([]router.ScenarioPick{}, d.Scenarios...)
			dc.Alternatives = append([]router.Alt{}, d.Alternatives...)
			v := e.policy.Apply(e.cat, &dc, in.State)
			verdict = v
			if dc.Language == "ru" || dc.Language == "kk" {
				replyLang = dc.Language
			}
			tr.Timings["route"] = ms(t0)
			tr.Timings["llm_route"] = routeMs
			for _, n := range v.Notes {
				if strings.Contains(n, "treated as unclear") {
					suppress = true
				}
			}
			e.emit(events.Event{Type: "route", SessionID: s.ID, Turn: turn, Data: map[string]any{"decision": dc, "verdict": v, "route_ms": routeMs, "since_t0_ms": tr.Timings["route"], "language": replyLang}})
			if opts.Voice && sp == nil {
				sp = e.newSpeaker(ctx, s, replyLang, turn, t0, opts.CollectAudio)
				if suppress {
					sp.Feed(e.clarifyText(&dc, replyLang))
				}
			}
		},
		OnReplyDelta: func(txt string) {
			replyMu.Lock()
			defer replyMu.Unlock()
			if suppress {
				return
			}
			e.emit(events.Event{Type: "reply_delta", SessionID: s.ID, Turn: turn, Data: map[string]any{"text": txt}})
			if sp != nil {
				sp.Feed(txt)
			}
		},
	}

	path := "llm"
	var res *router.Result
	var err error
	switch {
	case useFast:
		path = "fast"
		res, err = e.mock.Route(ctx, in, sink)
	case primaryIsMock:
		path = "mock"
		res, err = e.primary.Route(ctx, in, sink)
	default:
		res, err = e.primary.Route(ctx, in, sink)
		if err != nil {
			tr.Errors = append(tr.Errors, "llm: "+err.Error())
			e.emit(events.Event{Type: "llm_error", SessionID: s.ID, Turn: turn, Data: map[string]any{"error": err.Error()}})
			if res != nil && res.Raw != "" {
				tr.LLM = &LLMInfo{Provider: res.Provider, Model: res.Model, Raw: res.Raw, Error: err.Error()}
			}
			path = "fallback"
			res, err = e.mock.Route(ctx, in, sink)
		}
	}
	if err != nil || res == nil || res.Decision == nil {
		if err == nil {
			err = errors.New("router returned nothing")
		}
		tr.Errors = append(tr.Errors, err.Error())
		e.emit(events.Event{Type: "error", SessionID: s.ID, Turn: turn, Data: map[string]any{"error": err.Error()}})
		return tr, err
	}
	d := res.Decision
	verdict = e.policy.Apply(e.cat, d, in.State)
	if d.Language == "ru" || d.Language == "kk" {
		replyLang = d.Language
	}
	tr.Decision = d
	tr.Policy = verdict
	tr.Path = path
	tr.Language.Reply = replyLang
	if _, ok := tr.Timings["route"]; !ok {
		tr.Timings["route"] = ms(t0)
	}
	if path == "llm" || (tr.LLM != nil) {
		info := &LLMInfo{Provider: res.Provider, Model: res.Model, Usage: res.Usage, TTFTMs: res.TTFTMs, TotalMs: res.TotalMs, Repaired: res.Repaired}
		if e.cfg.Debug {
			info.Raw = res.Raw
			info.Messages = router.BuildMessages("(system prompt: see /api/prompt)", in)
		}
		if tr.LLM != nil && tr.LLM.Error != "" {
			info.Error = tr.LLM.Error
			info.Raw = tr.LLM.Raw
		}
		tr.LLM = info
		tr.Timings["llm_ttft"] = res.TTFTMs
		tr.Timings["llm_total"] = res.TotalMs
	}

	// 6. executor
	followFacts, needFollowUp := e.execute(ctx, s, d, tr)
	replyText := d.Reply
	if suppress {
		replyText = e.clarifyText(d, replyLang)
	}

	// 7. follow-up model call when the model asked for data first
	if needFollowUp && path == "llm" && ctx.Err() == nil {
		in2 := in
		in2.Facts = append(append([]router.Fact{}, facts...), followFacts...)
		in2.State = s.stateView(e.missingSlots)
		in2.FollowUp = true
		fs := time.Now()
		var extra strings.Builder
		res2, err2 := e.primary.Route(ctx, in2, router.Sink{
			OnLLMStart: func(model string, msgs []llm.Message) {
				e.emit(events.Event{Type: "llm_start", SessionID: s.ID, Turn: turn, Data: map[string]any{"model": model, "follow_up": true}})
			},
			OnReplyDelta: func(txt string) {
				extra.WriteString(txt)
				e.emit(events.Event{Type: "reply_delta", SessionID: s.ID, Turn: turn, Data: map[string]any{"text": txt, "follow_up": true}})
				if sp != nil {
					sp.Feed(txt)
				}
			},
		})
		tr.Timings["followup_llm"] = ms(fs)
		if err2 != nil {
			tr.Errors = append(tr.Errors, "follow-up llm: "+err2.Error())
		} else if res2 != nil && res2.Decision != nil {
			tr.Reply.FollowUp = true
			if !strings.HasSuffix(replyText, " ") {
				replyText += " "
			}
			replyText += res2.Decision.Reply
			e.applyFollowUpActions(ctx, s, res2.Decision, tr)
		}
	}

	// 8. finish speaking
	if sp != nil {
		sp.Flush()
		tr.Reply.Sentences = sp.sentences
		if sp.ttsFirst > 0 {
			tr.Timings["tts_first_byte"] = sp.ttsFirst
		}
		if !sp.first.IsZero() {
			tr.Timings["first_audio"] = int(sp.first.Sub(t0).Milliseconds())
		}
		tr.Errors = append(tr.Errors, sp.errs...)
		tr.Audio = sp.collect
	} else if opts.Voice && replyText != "" {
		// decision never arrived through the sink (should not happen) — speak anyway
		sp = e.newSpeaker(ctx, s, replyLang, turn, t0, opts.CollectAudio)
		sp.Feed(replyText)
		sp.Flush()
		tr.Audio = sp.collect
	}
	tr.Reply.Text = strings.TrimSpace(replyText)
	tr.Reply.Lang = replyLang
	e.emit(events.Event{Type: "reply_done", SessionID: s.ID, Turn: turn, Data: map[string]any{"text": tr.Reply.Text, "lang": replyLang}})
	tr.Timings["total"] = ms(t0)

	s.mu.Lock()
	s.Turns = append(s.Turns, router.TurnView{Role: "client", Text: text}, router.TurnView{Role: "bot", Text: tr.Reply.Text})
	if len(s.Turns) > 24 {
		s.Turns = s.Turns[len(s.Turns)-24:]
	}
	s.mu.Unlock()
	tr.State = s.View()
	tr.State.Turns = nil
	e.emit(events.Event{Type: "turn_done", SessionID: s.ID, Turn: turn, Data: tr})

	if path == "fast" {
		go e.shadow(in, tr)
	} else {
		e.store.Append(e.record(tr))
	}
	return tr, nil
}

func (e *Engine) clarifyText(d *router.Decision, language string) string {
	a, b := "", ""
	if len(d.Alternatives) > 0 {
		a = d.Alternatives[0].ID
	}
	if len(d.Alternatives) > 1 {
		b = d.Alternatives[1].ID
	}
	return e.tpl.Clarify(language, a, b)
}

// prefetch runs the confirmation gate and identity lookups before the model
// is called, so the reply can already use backend data.
func (e *Engine) prefetch(s *Session, sig triage.Signals, tr *Trace) []router.Fact {
	var facts []router.Fact
	s.mu.Lock()
	pending := s.Pending
	s.mu.Unlock()
	if pending != nil && sig.Confirmation == "yes" {
		st := time.Now()
		res, aerr := e.be.Execute(pending.Name, pending.Args)
		rec := ActionRecord{Name: pending.Name, Args: pending.Args, Mode: "execute", Result: res, Ms: ms(st), Note: "executed after explicit client confirmation"}
		f := router.Fact{Name: pending.Name, Args: pending.Args, Result: res, Note: "EXECUTED after client confirmation"}
		if aerr != nil {
			rec.Error = aerr.Error()
			f.Error = aerr.Error()
		}
		tr.Actions = append(tr.Actions, rec)
		facts = append(facts, f)
		e.emit(events.Event{Type: "action", SessionID: s.ID, Turn: tr.Turn, Data: rec})
		s.mu.Lock()
		s.Pending = nil
		s.mu.Unlock()
	} else if pending != nil && sig.Confirmation == "no" {
		facts = append(facts, router.Fact{Name: pending.Name, Args: pending.Args, Note: "DECLINED by the client, nothing was changed"})
		tr.Actions = append(tr.Actions, ActionRecord{Name: pending.Name, Args: pending.Args, Mode: "cancelled", Note: "client declined"})
		s.mu.Lock()
		s.Pending = nil
		s.mu.Unlock()
	}

	// identity
	phone, iin := sig.Entities["phone"], sig.Entities["iin"]
	if phone != "" || iin != "" {
		s.mu.Lock()
		known := s.ClientID
		knownPhone := ""
		if s.Client != nil {
			knownPhone, _ = s.Client["phone"].(string)
		}
		s.mu.Unlock()
		if known == "" || (phone != "" && phone != knownPhone) {
			args := map[string]any{}
			if phone != "" {
				args["phone"] = phone
			} else {
				args["iin"] = iin
			}
			st := time.Now()
			res, aerr := e.be.Execute("find_client", args)
			f := router.Fact{Name: "find_client", Args: args, Result: res}
			rec := ActionRecord{Name: "find_client", Args: args, Mode: "execute", Result: res, Ms: ms(st), Note: "prefetched from the phone/IIN in the utterance"}
			if aerr != nil {
				f.Error = aerr.Error()
				rec.Error = aerr.Error()
			} else {
				cid := fmt.Sprint(res["client_id"])
				profile := e.be.ClientProfile(cid)
				s.mu.Lock()
				s.Client = profile
				s.ClientID = cid
				if pl, ok := profile["preferred_language"].(string); ok && s.Language == "" {
					s.Language = pl
				}
				s.mu.Unlock()
				e.store.SetClient(s.ID, cid)
			}
			facts = append(facts, f)
			tr.Actions = append(tr.Actions, rec)
			e.emit(events.Event{Type: "action", SessionID: s.ID, Turn: tr.Turn, Data: rec})
		}
	}
	for _, key := range []string{"claim_number", "policy_number", "vehicle_plate"} {
		v := sig.Entities[key]
		if v == "" {
			continue
		}
		name := "get_policy"
		if key == "claim_number" {
			name = "get_claim"
		}
		args := map[string]any{key: v}
		st := time.Now()
		res, aerr := e.be.Execute(name, args)
		f := router.Fact{Name: name, Args: args, Result: res}
		rec := ActionRecord{Name: name, Args: args, Mode: "execute", Result: res, Ms: ms(st), Note: "prefetched from the number in the utterance"}
		if aerr != nil {
			f.Error = aerr.Error()
			rec.Error = aerr.Error()
		}
		facts = append(facts, f)
		tr.Actions = append(tr.Actions, rec)
	}
	// entities are slots too
	s.mu.Lock()
	for k, v := range sig.Entities {
		s.Slots[k] = v
	}
	s.mu.Unlock()
	return facts
}

// execute applies the decision to the session state and runs actions.
func (e *Engine) execute(ctx context.Context, s *Session, d *router.Decision, tr *Trace) ([]router.Fact, bool) {
	primary := d.Primary()
	s.mu.Lock()
	if d.Language == "ru" || d.Language == "kk" {
		s.Language = d.Language
	}
	switch primary {
	case "SYS_GOODBYE":
		s.Closed = true
	case "SYS_UNCLEAR":
		s.ClarifyCount++
	case "SYS_OUT_OF_SCOPE":
	default:
		if e.cat.Scenario(primary) != nil {
			if d.IsContinuation && s.Active != "" {
				if primary != s.Active {
					s.Active = primary // quote → purchase style transition
				}
			} else if primary != s.Active {
				if s.Active != "" && !contains(s.Stack, s.Active) {
					s.Stack = append(s.Stack, s.Active)
				}
				s.Stack = remove(s.Stack, primary)
				s.Active = primary
			}
			for _, sec := range d.Scenarios[1:] {
				if e.cat.Scenario(sec.ID) != nil && sec.ID != s.Active && !contains(s.Stack, sec.ID) {
					s.Stack = append(s.Stack, sec.ID)
				}
			}
		}
	}
	for k, v := range d.Slots {
		if v == nil || fmt.Sprint(v) == "" {
			continue
		}
		s.Slots[k] = v
	}
	clientID := s.ClientID
	client := s.Client
	slots := map[string]any{}
	for k, v := range s.Slots {
		slots[k] = v
	}
	active := s.Active
	s.mu.Unlock()

	var follow []router.Fact
	need := false
	for _, a := range d.Actions {
		if ctx.Err() != nil {
			break
		}
		act := e.cat.Action(a.Name)
		if act == nil {
			tr.Errors = append(tr.Errors, "unknown action requested: "+a.Name)
			continue
		}
		args := map[string]any{}
		for k, v := range a.Args {
			args[k] = v
		}
		e.fillArgs(act, args, clientID, client, slots)
		if a.Name == "transfer_to_operator" {
			queue := fmt.Sprint(args["queue"])
			if queue == "" || queue == "<nil>" {
				if sc := e.cat.Scenario(active); sc != nil && sc.Handoff != nil {
					queue = sc.Handoff.Queue
				} else {
					queue = "operator_general"
				}
			}
			if d.Handoff == nil {
				d.Handoff = &router.Handoff{Queue: queue}
			} else if d.Handoff.Queue == "" {
				d.Handoff.Queue = queue
			}
			continue
		}
		if e.be.IsIrreversible(a.Name) {
			preview, perr := e.be.Preview(a.Name, args)
			rec := ActionRecord{Name: a.Name, Args: args, Mode: "preview", Result: preview, Note: "irreversible — waiting for explicit client confirmation"}
			if perr != nil {
				rec.Error = perr.Error()
			}
			s.mu.Lock()
			s.Pending = &router.PendingAction{Name: a.Name, Args: args, Preview: preview, ScenarioID: active}
			s.mu.Unlock()
			tr.Actions = append(tr.Actions, rec)
			e.emit(events.Event{Type: "action", SessionID: s.ID, Turn: tr.Turn, Data: rec})
			continue
		}
		st := time.Now()
		res, aerr := e.be.Execute(a.Name, args)
		rec := ActionRecord{Name: a.Name, Args: args, Mode: "execute", Result: res, Ms: ms(st)}
		f := router.Fact{Name: a.Name, Args: args, Result: res, Note: "requested by the model this turn"}
		if aerr != nil {
			rec.Error = aerr.Error()
			f.Error = aerr.Error()
		} else if a.Name == "find_client" {
			cid := fmt.Sprint(res["client_id"])
			profile := e.be.ClientProfile(cid)
			s.mu.Lock()
			s.Client = profile
			s.ClientID = cid
			s.mu.Unlock()
			e.store.SetClient(s.ID, cid)
		}
		tr.Actions = append(tr.Actions, rec)
		follow = append(follow, f)
		need = true
		e.emit(events.Event{Type: "action", SessionID: s.ID, Turn: tr.Turn, Data: rec})
	}
	if d.Handoff != nil && d.Handoff.Queue != "" {
		s.mu.Lock()
		s.Handoff = d.Handoff
		s.mu.Unlock()
		tr.Actions = append(tr.Actions, ActionRecord{Name: "transfer_to_operator", Args: map[string]any{"queue": d.Handoff.Queue}, Mode: "handoff", Note: d.Handoff.Summary})
		e.emit(events.Event{Type: "handoff", SessionID: s.ID, Turn: tr.Turn, Data: d.Handoff})
	}
	return follow, need
}

// applyFollowUpActions handles actions requested in the follow-up call
// (typically a preview after data was fetched).
func (e *Engine) applyFollowUpActions(ctx context.Context, s *Session, d *router.Decision, tr *Trace) {
	if len(d.Actions) == 0 && d.Handoff == nil {
		return
	}
	d2 := &router.Decision{Scenarios: []router.ScenarioPick{{ID: s.View().Active}}, Actions: d.Actions, Handoff: d.Handoff, Slots: d.Slots, IsContinuation: true, Language: d.Language}
	e.execute(ctx, s, d2, tr)
}

// fillArgs completes action inputs from the session (client_id, phone,
// policy number when the client has exactly one matching policy).
func (e *Engine) fillArgs(act *catalog.Action, args map[string]any, clientID string, client map[string]any, slots map[string]any) {
	needs := func(name string) bool {
		for _, in := range act.Inputs {
			for _, alt := range strings.Split(in, "|") {
				if alt == name {
					return true
				}
			}
		}
		return false
	}
	if needs("client_id") && emptyArg(args, "client_id") && clientID != "" {
		args["client_id"] = clientID
	}
	if needs("phone") && emptyArg(args, "phone") {
		if p, ok := slots["phone"]; ok {
			args["phone"] = p
		} else if client != nil {
			if p, ok := client["phone"]; ok {
				args["phone"] = p
			}
		}
	}
	if needs("policy_number") && emptyArg(args, "policy_number") {
		if p, ok := slots["policy_number"]; ok {
			args["policy_number"] = p
		} else if client != nil {
			if pols, ok := client["policies"].([]map[string]any); ok && len(pols) == 1 {
				args["policy_number"] = pols[0]["policy_number"]
			}
		}
	}
	if needs("claim_number") && emptyArg(args, "claim_number") {
		if c, ok := slots["claim_number"]; ok {
			args["claim_number"] = c
		} else if client != nil {
			if cls, ok := client["claims"].([]map[string]any); ok && len(cls) == 1 {
				args["claim_number"] = cls[0]["claim_number"]
			}
		}
	}
	for _, in := range act.Inputs {
		for _, alt := range strings.Split(in, "|") {
			if emptyArg(args, alt) {
				if v, ok := slots[alt]; ok {
					args[alt] = v
				}
			}
		}
	}
}

func emptyArg(args map[string]any, k string) bool {
	v, ok := args[k]
	return !ok || v == nil || fmt.Sprint(v) == ""
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func remove(list []string, s string) []string {
	out := list[:0]
	for _, x := range list {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

// shadow verifies a fast-path decision with the LLM in the background.
func (e *Engine) shadow(in router.Input, tr *Trace) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	in.RouteOnly = true
	st := time.Now()
	res, err := e.primary.Route(ctx, in, router.Sink{})
	sh := &Shadow{Ms: ms(st)}
	if err != nil || res == nil || res.Decision == nil {
		if err != nil {
			sh.Error = err.Error()
		}
	} else {
		e.policy.Apply(e.cat, res.Decision, in.State)
		sh.LLMPrimary = res.Decision.Primary()
		sh.Confidence = res.Decision.Confidence()
		sh.Agree = sh.LLMPrimary == tr.Decision.Primary()
	}
	tr.Shadow = sh
	e.emit(events.Event{Type: "shadow", SessionID: tr.SessionID, Turn: tr.Turn, Data: map[string]any{"fast_primary": tr.Decision.Primary(), "shadow": sh}})
	e.store.Append(e.record(tr))
}

func (e *Engine) record(tr *Trace) store.Record {
	raw, _ := json.Marshal(tr)
	r := store.Record{SessionID: tr.SessionID, Turn: tr.Turn, At: tr.At, Transcript: tr.Input.Transcript, Language: tr.Language.Detected,
		Path: tr.Path, PolicyAction: tr.Policy.Action, Timings: tr.Timings, Trace: raw}
	if tr.Decision != nil {
		r.Scenarios = tr.Decision.IDs()
		r.Confidence = tr.Decision.Confidence()
		r.Handoff = tr.Decision.Handoff != nil && tr.Decision.Handoff.Queue != ""
	}
	if tr.Shadow != nil && tr.Shadow.Error == "" {
		agree := tr.Shadow.Agree
		r.FastPathAgree = &agree
	}
	return r
}

// ---------------------------------------------------------------- voice

// StartUtterance resets the audio buffer and opens the realtime STT stream
// when the provider supports it.
func (e *Engine) StartUtterance(s *Session) {
	s.audioMu.Lock()
	s.audio = nil
	s.speechStart = time.Now()
	rt := s.rt
	s.audioMu.Unlock()
	if rt != nil {
		return
	}
	if rp, ok := e.stt.(stt.Realtime); ok {
		sess, err := rp.NewSession(context.Background(), func(text string) {
			e.emit(events.Event{Type: "stt_partial", SessionID: s.ID, Data: map[string]any{"text": text}})
		})
		if err != nil {
			e.emit(events.Event{Type: "stt_error", SessionID: s.ID, Data: map[string]any{"error": err.Error(), "stage": "realtime_connect", "fallback": "batch"}})
			return
		}
		s.audioMu.Lock()
		s.rt = sess
		s.audioMu.Unlock()
		e.emit(events.Event{Type: "stt_realtime", SessionID: s.ID, Data: map[string]any{"status": "connected"}})
	}
}

// AudioChunk buffers audio and forwards it to the realtime STT stream.
func (e *Engine) AudioChunk(s *Session, pcm []byte) {
	s.AppendAudio(pcm)
	s.audioMu.Lock()
	rt := s.rt
	s.audioMu.Unlock()
	if rt != nil {
		if err := rt.Send(pcm); err != nil {
			s.audioMu.Lock()
			s.rt = nil
			s.audioMu.Unlock()
			rt.Close()
			e.emit(events.Event{Type: "stt_error", SessionID: s.ID, Data: map[string]any{"error": err.Error(), "stage": "realtime_send", "fallback": "batch"}})
		}
	}
}

// EndUtterance finalizes STT for the buffered audio and runs the turn.
func (e *Engine) EndUtterance(ctx context.Context, s *Session, opts TurnOptions) (*Trace, error) {
	pcm, _ := s.takeAudio()
	if opts.T0.IsZero() {
		opts.T0 = time.Now()
	}
	opts.AudioMs = len(pcm) / 32
	e.emit(events.Event{Type: "stt_start", SessionID: s.ID, Data: map[string]any{"audio_ms": opts.AudioMs, "bytes": len(pcm)}})
	var result *stt.Result
	var err error
	s.audioMu.Lock()
	rt := s.rt
	s.audioMu.Unlock()
	if rt != nil {
		cctx, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
		result, err = rt.Commit(cctx)
		cancel()
		if err != nil {
			e.emit(events.Event{Type: "stt_error", SessionID: s.ID, Data: map[string]any{"error": err.Error(), "stage": "realtime_commit", "fallback": "batch"}})
			s.audioMu.Lock()
			s.rt = nil
			s.audioMu.Unlock()
			rt.Close()
			result = nil
		}
	}
	if result == nil {
		if e.stt == nil {
			return nil, stt.ErrNoServerSTT
		}
		if len(pcm) < 3200 && opts.TranscriptHint == "" {
			return nil, ErrEmptyTranscript
		}
		result, err = e.stt.Transcribe(ctx, pcm, 16000, opts.TranscriptHint)
		if err != nil {
			e.emit(events.Event{Type: "stt_error", SessionID: s.ID, Data: map[string]any{"error": err.Error(), "stage": "batch"}})
			return nil, err
		}
	}
	opts.STTMs = ms(opts.T0)
	opts.STTProvider = result.Provider
	opts.STTModel = result.Model
	if opts.Source == "" {
		opts.Source = "voice"
	}
	text := strings.TrimSpace(result.Text)
	e.emit(events.Event{Type: "stt_final", SessionID: s.ID, Data: map[string]any{"text": text, "ms": opts.STTMs, "provider": result.Provider, "model": result.Model, "language": result.Language, "audio_ms": opts.AudioMs}})
	if text == "" {
		return nil, ErrEmptyTranscript
	}
	return e.RunTurn(ctx, s, text, opts)
}

// TranscribeAndRun handles an uploaded PCM buffer (tests, audio files).
func (e *Engine) TranscribeAndRun(ctx context.Context, s *Session, pcm []byte, rate int, opts TurnOptions) (*Trace, error) {
	if rate != 16000 {
		pcm = stt.Resample(pcm, rate, 16000)
	}
	s.takeAudio()
	s.AppendAudio(pcm)
	if opts.Source == "" {
		opts.Source = "audio_file"
	}
	return e.EndUtterance(ctx, s, opts)
}

// CloseSession releases realtime resources.
func (e *Engine) CloseSession(s *Session) {
	s.audioMu.Lock()
	rt := s.rt
	s.rt = nil
	s.audioMu.Unlock()
	if rt != nil {
		rt.Close()
	}
}

// ---------------------------------------------------------------- stateless routing

// RouteResponse is the /api/route answer (used by evaluate.py integration).
type RouteResponse struct {
	Scenarios  []string              `json:"scenarios"`
	Confidence float64               `json:"confidence"`
	Path       string                `json:"path"`
	RouteMs    int                   `json:"route_ms"`
	Decision   *router.Decision      `json:"decision"`
	Verdict    router.Verdict        `json:"verdict"`
	Candidates []retrieval.Candidate `json:"candidates"`
	Signals    triage.Signals        `json:"signals"`
	FastPath   router.FastPathCheck  `json:"fast_path"`
	LLM        *LLMInfo              `json:"llm,omitempty"`
	Error      string                `json:"error,omitempty"`
}

// RouteText routes a single utterance with no dialogue state (evaluation).
func (e *Engine) RouteText(ctx context.Context, text string) *RouteResponse {
	sig := triage.Analyze(text, false)
	cands := e.ix.Search(text, 6)
	in := router.Input{Utterance: text, Signals: sig, Candidates: cands, State: router.StateView{}, Today: e.today, RouteOnly: true}
	fc := e.policy.FastPath(e.cat, in)
	_, primaryIsMock := e.primary.(*router.MockRouter)
	out := &RouteResponse{Candidates: cands, Signals: sig, FastPath: fc, Path: "llm"}
	var res *router.Result
	var err error
	switch {
	case fc.Eligible && e.cfg.FastPath == "on" && !primaryIsMock:
		out.Path = "fast"
		res, err = e.mock.Route(ctx, in, router.Sink{})
	case primaryIsMock:
		out.Path = "mock"
		res, err = e.primary.Route(ctx, in, router.Sink{})
	default:
		res, err = e.primary.Route(ctx, in, router.Sink{})
		if err != nil {
			out.Error = err.Error()
			out.Path = "fallback"
			res, err = e.mock.Route(ctx, in, router.Sink{})
		}
	}
	if err != nil || res == nil || res.Decision == nil {
		if err != nil {
			out.Error = err.Error()
		}
		out.Scenarios = []string{}
		return out
	}
	d := res.Decision
	out.Verdict = e.policy.Apply(e.cat, d, in.State)
	out.Decision = d
	out.Scenarios = d.IDs()
	out.Confidence = d.Confidence()
	out.RouteMs = res.RouteMs
	if out.Path == "llm" {
		out.LLM = &LLMInfo{Provider: res.Provider, Model: res.Model, Usage: res.Usage, TTFTMs: res.TTFTMs, TotalMs: res.TotalMs, Repaired: res.Repaired}
	}
	return out
}
