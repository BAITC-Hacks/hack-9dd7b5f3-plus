// Package server wires the voice gateway: one HTTP server (browser WebSocket,
// Twilio, admin live events, TTS/STT helper endpoints) plus an optional
// Asterisk AudioSocket listener for the Kazakhstan phone number.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"hackathon/voice/agent"
	"hackathon/voice/audio"
	"hackathon/voice/brain"
	"hackathon/voice/config"
	"hackathon/voice/elevenlabs"
	"hackathon/voice/lang"
	"hackathon/voice/speech"
	"hackathon/voice/transport"
	"hackathon/voice/voicelog"
	"hackathon/voice/web"
)

// Server is the voice gateway.
type Server struct {
	Cfg      config.Config
	EL       *elevenlabs.Client
	TTS      *speech.ElevenTTS
	Deps     agent.Deps
	Log      *voicelog.Logger
	Registry *transport.Registry
	started  time.Time
}

// New builds the gateway from config. The brain is chosen by VOICE_BRAIN:
// backend (team API), openrouter (built-in Saqta agent), echo, or auto.
func New(cfg config.Config) (*Server, error) {
	if cfg.ElevenLabsKey == "" {
		return nil, errors.New("ElevenLabs key missing: set ELEVENLABS_API_KEY")
	}
	el := elevenlabs.NewClient(cfg.ElevenLabsKey, cfg.ElevenLabsBase)
	lg := voicelog.New(cfg.LogDir, cfg.RecordAudio)
	tts := &speech.ElevenTTS{Client: el, Cfg: cfg}
	s := &Server{Cfg: cfg, EL: el, TTS: tts, Log: lg, Registry: transport.NewRegistry(), started: time.Now()}
	s.Deps = agent.Deps{Cfg: cfg, STT: &speech.ElevenSTT{Client: el, Cfg: cfg}, TTS: tts, Brain: PickBrain(cfg), Log: lg}
	return s, nil
}

// PickBrain selects the reply generator.
func PickBrain(cfg config.Config) brain.Brain {
	var data *brain.Dataset
	if cfg.DataDir != "" {
		d, err := brain.LoadDataset(cfg.DataDir)
		if err != nil {
			slog.Warn("dataset not loaded", "dir", cfg.DataDir, "err", err)
		} else {
			data = d
		}
	}
	openrouter := func() brain.Brain {
		return brain.NewOpenRouter(cfg.OpenRouterKey, cfg.OpenRouterBase, cfg.OpenRouterModel, cfg.OpenRouterFallbacks, data)
	}
	switch cfg.Brain {
	case "backend":
		return brain.NewBackend(cfg.BackendURL)
	case "openrouter":
		return openrouter()
	case "echo":
		return brain.Echo{}
	}
	if cfg.BackendURL != "" {
		be := brain.NewBackend(cfg.BackendURL)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		healthy := be.Healthy(ctx)
		cancel()
		if healthy {
			return be
		}
		slog.Warn("backend not reachable, falling back", "url", cfg.BackendURL)
	}
	if cfg.OpenRouterKey != "" {
		return openrouter()
	}
	return brain.Echo{}
}

// Starter adapts transport calls to the conversation engine.
func (s *Server) Starter() transport.Starter {
	return func(ctx context.Context, info transport.CallInfo, sink transport.Sink) transport.Call {
		return agent.New(ctx, s.Deps, agent.Options{
			SessionID: info.SessionID, Channel: info.Channel, CallerID: info.CallerID,
			InputFormat: info.InputFormat, Manual: info.Manual, Greeting: info.Greeting && s.Cfg.Greeting,
			Lang: lang.Lang(info.Lang), Meta: info.Meta,
		}, sink)
	}
}

// Handler returns all HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.index)
	mux.HandleFunc("GET /call", s.callPage)
	mux.HandleFunc("GET /phone", s.callPage)
	mux.HandleFunc("GET /healthz", s.health)
	mux.Handle("GET /ws/voice", &transport.WebHandler{Start: s.Starter(), AllowedOrigins: s.Cfg.AllowedOrigins})
	tw := &transport.TwilioHandler{Start: s.Starter(), PublicURL: s.Cfg.PublicURL, Greeting: true}
	mux.HandleFunc("POST /twilio/voice", tw.TwiML)
	mux.HandleFunc("GET /twilio/stream", tw.Stream)
	mux.Handle("GET /asterisk/call", s.Registry)
	mux.HandleFunc("GET /api/voice/events", s.Log.Hub().ServeSSE)
	mux.HandleFunc("GET /api/voice/stats", s.stats)
	mux.HandleFunc("GET /api/voice/sessions", s.sessions)
	mux.HandleFunc("POST /api/voice/tts", s.tts)
	mux.HandleFunc("POST /api/voice/stt", s.stt)
	return s.cors(mux)
}

// Run serves until ctx is done.
func (s *Server) Run(ctx context.Context) error {
	go s.warm(ctx)
	srv := &http.Server{Addr: s.Cfg.HTTPAddr, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 2)
	go func() { errc <- srv.ListenAndServe() }()
	if s.Cfg.AudioSocketAddr != "" {
		as := &transport.AudioSocketServer{Start: s.Starter(), Registry: s.Registry, Greeting: true}
		go func() { errc <- as.ListenAndServe(ctx, s.Cfg.AudioSocketAddr) }()
	}
	slog.Info("voice gateway listening", "http", s.Cfg.HTTPAddr, "audiosocket", s.Cfg.AudioSocketAddr,
		"brain", s.Deps.Brain.Name(), "tts_ru", s.Cfg.TTSModelRU, "tts_kk", s.Cfg.TTSModelKK, "stt", s.Cfg.STTModel)
	select {
	case <-ctx.Done():
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(sctx)
	case err := <-errc:
		return err
	}
}

// warm opens provider connections and pre-synthesizes the fixed phrases for
// the formats in use, so the first call pays no extra latency.
func (s *Server) warm(ctx context.Context) {
	wctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.EL.Warm(wctx); err != nil {
		slog.Warn("elevenlabs warm-up failed", "err", err)
	}
	if w, ok := s.Deps.Brain.(interface{ Warm(context.Context) error }); ok {
		if err := w.Warm(wctx); err != nil {
			slog.Warn("brain warm-up failed", "err", err)
		}
	}
	formats := []string{"pcm_16000"}
	if s.Cfg.AudioSocketAddr != "" {
		formats = append(formats, "pcm_8000")
	}
	if s.Cfg.PublicURL != "" {
		formats = append(formats, "ulaw_8000")
	}
	for _, f := range formats {
		if s.Cfg.Greeting {
			if _, err := s.TTS.Say(wctx, agent.Greeting, lang.Mixed, f); err != nil {
				slog.Warn("greeting pre-synthesis failed", "format", f, "err", err)
			}
		}
		if s.Cfg.FillerAfter > 0 {
			for _, l := range []lang.Lang{lang.RU, lang.KK} {
				_, _ = s.TTS.Say(wctx, agent.FillerText(l), l, f)
			}
		}
	}
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(web.Index)
}

// callPage serves the phone-style call screen (mobile, hands-free, captions).
func (s *Server) callPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(web.Call)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "brain": s.Deps.Brain.Name(), "stt_model": s.Cfg.STTModel,
		"tts_model_ru": s.Cfg.TTSModelRU, "tts_model_kk": s.Cfg.TTSModelKK, "voice": s.Cfg.VoiceID,
		"audiosocket": s.Cfg.AudioSocketAddr != "", "uptime_s": int(time.Since(s.started).Seconds()),
		"warnings": s.Cfg.Validate(),
	})
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Log.Stats().Snapshot())
}

func (s *Server) sessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Log.Sessions())
}

type ttsRequest struct {
	Text   string `json:"text"`
	Lang   string `json:"lang"`   // ru | kk (default: detected from text)
	Format string `json:"format"` // pcm_16000 | pcm_8000 | ulaw_8000 | mp3_22050_32 | wav
}

// tts streams synthesized audio: POST /api/voice/tts {"text","lang","format"}.
func (s *Server) tts(w http.ResponseWriter, r *http.Request) {
	var req ttsRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": `body must be {"text":"...","lang":"ru|kk","format":"pcm_16000|ulaw_8000|mp3_22050_32|wav"}`})
		return
	}
	l := lang.Lang(req.Lang)
	if l != lang.RU && l != lang.KK {
		l = (&lang.Policy{}).Choose(req.Text, "").Reply
	}
	format := req.Format
	wav := format == "wav"
	if format == "" || wav {
		format = "pcm_16000"
	}
	f, err := audio.ParseFormat(format)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	model := s.TTS.Model(l)
	treq := elevenlabs.TTSRequest{VoiceID: s.TTS.Voice(l), Model: model, Text: req.Text, OutputFormat: format}
	if l == lang.RU && !strings.HasPrefix(model, "eleven_v3") {
		treq.Language = "ru"
	}
	start := time.Now()
	if wav {
		b, ttfb, err := s.EL.TTS(r.Context(), treq)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Header().Set("X-TTS-Model", model)
		w.Header().Set("X-TTS-TTFB-Ms", itoa(ttfb.Milliseconds()))
		_, _ = w.Write(audio.EncodeWAV(b, f.Rate))
		return
	}
	st, err := s.EL.TTSStream(r.Context(), treq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer st.Close()
	w.Header().Set("Content-Type", contentType(f))
	w.Header().Set("X-TTS-Model", model)
	w.Header().Set("X-Audio-Format", format)
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 8<<10)
	first := true
	for {
		n, err := st.Read(buf)
		if n > 0 {
			if first {
				first = false
				slog.Debug("api tts first byte", "ms", time.Since(start).Milliseconds(), "model", model)
			}
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

// stt transcribes a whole recording: POST /api/voice/stt with the audio as the
// body (any container ElevenLabs accepts: webm/opus, wav, mp3, ...) or as the
// multipart field "file". Returns {"text","language","language_code","ms"}.
func (s *Server) stt(w http.ResponseWriter, r *http.Request) {
	body := io.Reader(http.MaxBytesReader(w, r.Body, 25<<20))
	name := "audio.webm"
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); strings.HasPrefix(mt, "multipart/") {
		f, hdr, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "multipart field 'file' missing"})
			return
		}
		defer f.Close()
		body, name = f, hdr.Filename
	} else if ext := extFor(mt); ext != "" {
		name = "audio" + ext
	}
	start := time.Now()
	t, err := s.EL.Transcribe(r.Context(), name, body, elevenlabs.BatchOptions{Language: s.Cfg.STTLanguage})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"text": t.Text, "language": string(lang.Analyze(t.Text).Language()), "language_code": t.LanguageCode,
		"language_probability": t.LanguageProbability, "ms": time.Since(start).Milliseconds(),
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			origin := r.Header.Get("Origin")
			if origin != "" && originAllowed(s.Cfg.AllowedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			} else if originAllowed(s.Cfg.AllowedOrigins, "*") {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Expose-Headers", "X-TTS-Model, X-TTS-TTFB-Ms, X-Audio-Format")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func originAllowed(allowed []string, origin string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(strings.TrimRight(a, "/"), strings.TrimRight(origin, "/")) {
			return true
		}
	}
	return false
}

func contentType(f audio.Format) string {
	switch f.Codec {
	case "pcm":
		return "audio/L16; rate=" + itoa(int64(f.Rate)) + "; channels=1"
	case "ulaw":
		return "audio/basic"
	case "mp3":
		return "audio/mpeg"
	case "opus":
		return "audio/ogg"
	}
	return "application/octet-stream"
}

func extFor(mediaType string) string {
	switch {
	case strings.Contains(mediaType, "webm"):
		return ".webm"
	case strings.Contains(mediaType, "wav"):
		return ".wav"
	case strings.Contains(mediaType, "mpeg"), strings.Contains(mediaType, "mp3"):
		return ".mp3"
	case strings.Contains(mediaType, "ogg"):
		return ".ogg"
	case strings.Contains(mediaType, "mp4"), strings.Contains(mediaType, "m4a"):
		return ".m4a"
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func itoa(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
