// Package httpapi exposes the engine over REST, WebSocket (voice) and SSE
// (debug event stream). Contract: docs/SPEC.md.
package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"hackathon/backend/internal/config"
	"hackathon/backend/internal/dialog"
	"hackathon/backend/internal/eval"
	"hackathon/backend/internal/retrieval"
	"hackathon/backend/internal/stt"
	"hackathon/backend/internal/tts"
)

// Server holds the handlers.
type Server struct {
	cfg    config.Config
	e      *dialog.Engine
	lex    *retrieval.Lexicon
	ttsP   tts.Provider
	router http.Handler

	evalMu      sync.Mutex
	evalRunning bool
	evalLast    *eval.Report
	evalProg    [2]int
	started     time.Time
}

// New builds the HTTP handler.
func New(cfg config.Config, e *dialog.Engine, lex *retrieval.Lexicon, ttsP tts.Provider) *Server {
	s := &Server{cfg: cfg, e: e, lex: lex, ttsP: ttsP, started: time.Now()}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(s.cors)
	r.Get("/health", s.health)
	r.Get("/healthz", s.health)
	r.Get("/ws", s.ws)
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.health)
		r.Get("/config", s.config)
		r.Get("/prompt", s.prompt)
		r.Post("/sessions", s.createSession)
		r.Get("/sessions", s.listSessions)
		r.Get("/sessions/{id}", s.getSession)
		r.Post("/sessions/{id}/turn", s.turn)
		r.Post("/sessions/{id}/turn/audio", s.turnAudio)
		r.Post("/turn", s.turn)
		r.Post("/route", s.route)
		r.Post("/eval/run", s.evalRun)
		r.Get("/eval/last", s.evalLastHandler)
		r.Get("/supervisor/stats", s.stats)
		r.Get("/supervisor/actions", s.actionLog)
		r.Get("/catalog", s.catalog)
		r.Post("/catalog/reload", s.catalogReload)
		r.Put("/catalog/scenarios/{id}", s.catalogUpdate)
		r.Get("/lexicon", s.lexicon)
		r.Get("/debug/events", s.debugEvents)
		r.Post("/tts", s.ttsHandler)
	})
	s.router = r
	return s
}

// Handler returns the root handler.
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := ""
		for _, o := range s.cfg.CORSOrigins {
			if o == "*" || o == origin {
				allowed = origin
				if o == "*" && origin == "" {
					allowed = "*"
				}
				break
			}
		}
		if allowed != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"ok": true, "status": "ok", "mock_mode": s.cfg.LLM.Provider == "mock", "router": s.e.RouterName(),
		"stt": s.cfg.STT.Provider, "tts": s.cfg.TTS.Provider, "uptime_s": int(time.Since(s.started).Seconds())})
}

func (s *Server) config(w http.ResponseWriter, r *http.Request) {
	pub := s.cfg.Public()
	pub["router"] = s.e.RouterName()
	pub["tts_sample_rate"] = s.e.TTSSampleRate()
	pub["as_of_date"] = s.e.Catalog().AsOfDate
	writeJSON(w, 200, pub)
}

func (s *Server) prompt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, s.e.SystemPrompt())
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Channel string `json:"channel"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Channel == "" {
		body.Channel = "api"
	}
	sess := s.e.NewSession(body.Channel)
	writeJSON(w, 201, map[string]any{"session_id": sess.ID, "session": sess.View()})
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}
	writeJSON(w, 200, map[string]any{"sessions": s.e.Store().Sessions(limit)})
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sess, recs := s.e.Store().Session(id)
	if sess == nil {
		writeErr(w, 404, "session not found")
		return
	}
	out := map[string]any{"session": sess, "turns": recs}
	if live := s.e.Session(id); live != nil {
		out["state"] = live.View()
	}
	writeJSON(w, 200, out)
}

type turnRequest struct {
	SessionID    string `json:"session_id"`
	Text         string `json:"text"`
	Voice        *bool  `json:"voice"`
	CollectAudio bool   `json:"collect_audio"`
	Source       string `json:"source"`
}

func (s *Server) sessionFor(r *http.Request, id string) *dialog.Session {
	if id == "" {
		id = chi.URLParam(r, "id")
	}
	if id != "" {
		if live := s.e.Session(id); live != nil {
			return live
		}
		return nil
	}
	return s.e.NewSession("api")
}

func (s *Server) turn(w http.ResponseWriter, r *http.Request) {
	var req turnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid json: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeErr(w, 400, "text is required")
		return
	}
	sess := s.sessionFor(r, req.SessionID)
	if sess == nil {
		writeErr(w, 404, "session not found")
		return
	}
	voice := req.Voice != nil && *req.Voice
	source := req.Source
	if source == "" {
		source = "text"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	tr, err := s.e.RunTurn(ctx, sess, req.Text, dialog.TurnOptions{Source: source, Voice: voice, CollectAudio: req.CollectAudio && voice, T0: time.Now()})
	s.writeTrace(w, tr, err)
}

func (s *Server) writeTrace(w http.ResponseWriter, tr *dialog.Trace, err error) {
	if err != nil && tr == nil {
		writeErr(w, 500, err.Error())
		return
	}
	out := map[string]any{"session_id": tr.SessionID, "turn": tr.Turn, "reply": tr.Reply.Text, "language": tr.Reply.Lang,
		"scenarios": tr.Decision.IDs(), "confidence": tr.Decision.Confidence(), "path": tr.Path, "timings": tr.Timings, "trace": tr}
	if tr.Decision == nil {
		out["scenarios"] = []string{}
	}
	if len(tr.Audio) > 0 {
		out["audio_wav_base64"] = base64.StdEncoding.EncodeToString(stt.WAV(tr.Audio, s.e.TTSSampleRate()))
		out["audio_sample_rate"] = s.e.TTSSampleRate()
	}
	if err != nil {
		out["error"] = err.Error()
	}
	writeJSON(w, 200, out)
}

// turnAudio accepts a WAV (PCM16) upload: multipart "file" plus optional
// "transcript_hint" (mock STT), "voice", "collect_audio".
func (s *Server) turnAudio(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, 400, "multipart form expected: "+err.Error())
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, 400, "file is required")
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 32<<20))
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	pcm, rate, err := stt.ParseWAV(raw)
	if err != nil {
		writeErr(w, 400, "wav: "+err.Error())
		return
	}
	sess := s.sessionFor(r, r.FormValue("session_id"))
	if sess == nil {
		writeErr(w, 404, "session not found")
		return
	}
	voice := r.FormValue("voice") == "1" || r.FormValue("voice") == "true"
	collect := r.FormValue("collect_audio") == "1" || r.FormValue("collect_audio") == "true"
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	tr, err := s.e.TranscribeAndRun(ctx, sess, pcm, rate, dialog.TurnOptions{Source: "audio_file", Voice: voice, CollectAudio: collect && voice, TranscriptHint: r.FormValue("transcript_hint"), T0: time.Now()})
	if err != nil && tr == nil {
		writeJSON(w, 200, map[string]any{"session_id": sess.ID, "error": err.Error(), "scenarios": []string{}})
		return
	}
	s.writeTrace(w, tr, err)
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		writeErr(w, 400, "text is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	writeJSON(w, 200, s.e.RouteText(ctx, req.Text))
}

func (s *Server) evalRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Concurrency int `json:"concurrency"`
		Limit       int `json:"limit"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Concurrency <= 0 {
		req.Concurrency = 6
	}
	s.evalMu.Lock()
	if s.evalRunning {
		s.evalMu.Unlock()
		writeErr(w, 409, "evaluation already running")
		return
	}
	s.evalRunning = true
	s.evalMu.Unlock()
	defer func() {
		s.evalMu.Lock()
		s.evalRunning = false
		s.evalMu.Unlock()
	}()
	utts, err := eval.LoadDevSet(s.cfg.DataDir)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if req.Limit > 0 && req.Limit < len(utts) {
		utts = utts[:req.Limit]
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	rep := eval.Run(ctx, utts, func(ctx context.Context, text string) eval.Outcome {
		rr := s.e.RouteText(ctx, text)
		reason := ""
		if rr.Decision != nil && len(rr.Decision.Scenarios) > 0 {
			reason = rr.Decision.Scenarios[0].Reason
		}
		return eval.Outcome{IDs: rr.Scenarios, Confidence: rr.Confidence, Path: rr.Path, RouteMs: rr.RouteMs, Reason: reason, Error: rr.Error}
	}, s.e.RouterName(), req.Concurrency, func(done, total int) {
		s.evalMu.Lock()
		s.evalProg = [2]int{done, total}
		s.evalMu.Unlock()
	})
	s.evalMu.Lock()
	s.evalLast = rep
	s.evalMu.Unlock()
	writeJSON(w, 200, rep)
}

func (s *Server) evalLastHandler(w http.ResponseWriter, r *http.Request) {
	s.evalMu.Lock()
	defer s.evalMu.Unlock()
	writeJSON(w, 200, map[string]any{"running": s.evalRunning, "progress": s.evalProg, "report": s.evalLast})
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.e.Store().Stats(s.cfg.Policy.ProceedMin))
}

func (s *Server) actionLog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"actions": s.e.Backend().Log()})
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	sc, si := s.e.Catalog().Snapshot()
	writeJSON(w, 200, map[string]any{"as_of_date": s.e.Catalog().AsOfDate, "scenarios": sc, "system_intents": si, "dir": s.e.Catalog().Dir()})
}

func (s *Server) catalogReload(w http.ResponseWriter, r *http.Request) {
	if err := s.e.ReloadCatalog(s.lex); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	sc, _ := s.e.Catalog().Snapshot()
	writeJSON(w, 200, map[string]any{"ok": true, "scenarios": len(sc)})
}

// catalogUpdate edits one scenario (description, not_this_if, examples,
// priority, fast_path_eligible), persists the edited catalog to
// VAR_DIR/catalog/scenarios.json and rebuilds the index and the prompt.
func (s *Server) catalogUpdate(w http.ResponseWriter, r *http.Request) {
	id := strings.ToUpper(chi.URLParam(r, "id"))
	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeErr(w, 400, "invalid json: "+err.Error())
		return
	}
	sc, err := s.e.Catalog().UpdateScenario(id, patch)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := s.e.ReloadCatalog(s.lex); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "scenario": sc, "saved_to": s.e.Catalog().OverridePath()})
}

func (s *Server) lexicon(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.lex)
}

// debugEvents streams every pipeline event as SSE (curl -N .../api/debug/events).
func (s *Server) debugEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	session := r.URL.Query().Get("session")
	ch, cancel := s.e.Bus().Subscribe(session, 256)
	defer cancel()
	send := func(v any) bool {
		b, err := json.Marshal(v)
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return false
		}
		fl.Flush()
		return true
	}
	if r.URL.Query().Get("replay") != "0" {
		for _, ev := range s.e.Bus().Recent(100) {
			if session == "" || ev.SessionID == session {
				if !send(ev) {
					return
				}
			}
		}
	}
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			if !send(ev) {
				return
			}
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		}
	}
}

// ttsHandler synthesizes text (returns WAV) — handy for tests.
func (s *Server) ttsHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
		Lang string `json:"lang"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		writeErr(w, 400, "text is required")
		return
	}
	if req.Lang == "" {
		req.Lang = "ru"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	rc, err := s.ttsP.Synthesize(ctx, req.Text, req.Lang)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	defer rc.Close()
	pcm, err := io.ReadAll(rc)
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Write(stt.WAV(pcm, s.ttsP.SampleRate()))
}
