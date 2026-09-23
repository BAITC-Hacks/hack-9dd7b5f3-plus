package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"hackathon/backend/internal/ai"
	"hackathon/backend/internal/domain"
	"hackathon/backend/internal/speech"
	"hackathon/backend/internal/store"
)

type Server struct {
	Catalog *domain.Catalog
	Router  ai.Router
	Store   store.Store
	Speech  *speech.Client
	locks   [256]sync.Mutex
	DBMode  string
}
type routeRequest struct {
	SessionID string   `json:"session_id"`
	Text      string   `json:"text"`
	InputKind string   `json:"input_kind"`
	STTMS     *float64 `json:"stt_ms,omitempty"`
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(s.cors)
	r.Get("/healthz", s.health)
	r.Get("/health", s.health)
	r.Get("/api/catalog", func(w http.ResponseWriter, r *http.Request) { write(w, 200, s.Catalog) })
	r.Post("/api/sessions", s.create)
	r.Get("/api/sessions", s.list)
	r.Get("/api/sessions/{id}", s.get)
	r.Post("/api/route", func(w http.ResponseWriter, r *http.Request) { s.route(w, r, false) })
	r.Post("/api/turns/stream", func(w http.ResponseWriter, r *http.Request) { s.route(w, r, true) })
	r.Post("/api/transcribe", s.transcribe)
	r.Post("/api/realtime/connect", s.connect)
	r.Get("/api/sessions/{id}/turns/{turn}/speech", s.speak)
	r.Post("/api/sessions/{id}/turns/{turn}/metrics", s.metrics)
	r.Get("/api/stats", s.stats)
	return r
}
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		for _, v := range strings.Split(ai.Env("CORS_ORIGINS", "http://localhost:3000"), ",") {
			if strings.TrimSpace(v) == origin {
				allowed = true
			}
		}
		if origin != "" && !allowed {
			fail(w, 403, "origin is not allowed")
			return
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	write(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		fail(w, 400, "invalid JSON body")
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		fail(w, 400, "expected exactly one JSON object")
		return false
	}
	return true
}
func id() string             { return rand.Text() }
func ms(t time.Time) float64 { return float64(time.Since(t).Microseconds()) / 1000 }
func (s *Server) lock(id string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return &s.locks[h.Sum32()%256]
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	write(w, 200, map[string]any{"status": "ok", "provider": s.Router.Name(), "model": s.Router.Model(), "speech_provider": ai.Env("SPEECH_PROVIDER", "browser"), "speech_ready": s.Speech.Enabled && s.Speech.Key != "", "realtime_model": s.Speech.RealtimeModel, "catalog_source": s.Catalog.Source, "catalog_count": len(s.Catalog.Scenarios), "catalog_hash": s.Catalog.Hash, "storage": s.DBMode, "max_turns": 10})
}
func (s *Server) newSession(ctx context.Context) (*domain.Session, error) {
	v := &domain.Session{ID: id(), Turns: []domain.Turn{}, Pending: []string{}, CreatedAt: time.Now().UTC()}
	return v, s.Store.Save(ctx, v)
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	v, e := s.newSession(r.Context())
	if e != nil {
		fail(w, 503, "could not create session")
		return
	}
	write(w, 201, v)
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	v, e := s.Store.List(r.Context())
	if e != nil {
		fail(w, 503, "storage unavailable")
		return
	}
	sort.Slice(v, func(i, j int) bool { return v[i].CreatedAt.After(v[j].CreatedAt) })
	write(w, 200, v)
}
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	v, e := s.Store.Load(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		if errors.Is(e, store.ErrNotFound) {
			fail(w, 404, "session not found")
		} else {
			fail(w, 503, "storage unavailable")
		}
		return
	}
	write(w, 200, v)
}
func (s *Server) route(w http.ResponseWriter, r *http.Request, stream bool) {
	var in routeRequest
	if !decode(w, r, &in) {
		return
	}
	in.Text = strings.TrimSpace(in.Text)
	if len([]rune(in.Text)) == 0 || len([]rune(in.Text)) > 4000 {
		fail(w, 400, "text must be 1..4000 characters")
		return
	}
	if in.InputKind == "" {
		in.InputKind = "text"
	}
	if in.InputKind != "text" && in.InputKind != "microphone" && in.InputKind != "audio_file" {
		fail(w, 400, "invalid input_kind")
		return
	}
	if in.STTMS != nil && (*in.STTMS < 0 || *in.STTMS > 120000 || math.IsNaN(*in.STTMS)) {
		fail(w, 400, "invalid stt_ms")
		return
	}
	if in.SessionID == "" {
		session, e := s.newSession(r.Context())
		if e != nil {
			fail(w, 503, "storage unavailable")
			return
		}
		in.SessionID = session.ID
	}
	lock := s.lock(in.SessionID)
	if !lock.TryLock() {
		fail(w, 409, "session is processing another turn; retry after it finishes")
		return
	}
	defer lock.Unlock()
	session, e := s.Store.Load(r.Context(), in.SessionID)
	if e != nil {
		if errors.Is(e, store.ErrNotFound) {
			fail(w, 404, "session not found")
		} else {
			fail(w, 503, "storage unavailable")
		}
		return
	}
	if len(session.Turns) >= 10 {
		fail(w, 409, "10-turn limit reached; start a new session")
		return
	}
	started := time.Now()
	emit := func(kind string, data any) {
		if !stream {
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"type": kind, "elapsed_ms": ms(started), "data": data})
		_ = http.NewResponseController(w).Flush()
	}
	if stream {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(200)
	}
	emit("input", map[string]any{"text": in.Text, "session_id": session.ID, "active": session.Active, "pending": session.Pending})
	emit("routing_started", map[string]any{"provider": s.Router.Name(), "model": s.Router.Model(), "catalog_count": len(s.Catalog.Scenarios), "catalog_hash": s.Catalog.Hash, "history_turns": len(session.Turns)})
	calls := []domain.Call{}
	routeStart := time.Now()
	d, err := s.Router.Route(r.Context(), domain.RouteInput{Text: in.Text, History: session.Turns, Active: session.Active, Pending: session.Pending}, func(call domain.Call) { calls = append(calls, call); emit("llm_attempt", call) })
	routingMS := ms(routeStart)
	source := s.Router.Name()
	warnings := []string{}
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		source = "provider_error"
		d = domain.Decision{Status: "handoff", Language: "ru", Reason: "Маршрутизатор недоступен или вернул некорректное решение.", Alternatives: []domain.Alternative{}, Slots: []domain.Slot{}, Pending: session.Pending}
		warnings = append(warnings, "LLM недоступен. Ответ fallback; метрика не доказывает качество модели.")
	}
	if source == "mock" {
		warnings = append(warnings, "MOCK: тестовый дубль без LLM. Его точность и задержки не являются результатом модели.")
	}
	if s.Catalog.Source == "synthetic_demo" {
		warnings = append(warnings, "Синтетический каталог: исходные 40 сценариев ещё не подключены.")
	}
	emit("routing_complete", d)
	policyStart := time.Now()
	if d.Status == "clarify" && len(session.Turns) >= 2 && session.Turns[len(session.Turns)-1].Decision.Status == "clarify" && session.Turns[len(session.Turns)-2].Decision.Status == "clarify" {
		d.Status = "handoff"
		d.Reason = "Повторное непонимание: требуется оператор."
	}
	reply, policyWarnings := s.Catalog.Reply(&d)
	warnings = append(warnings, policyWarnings...)
	turn := domain.Turn{ID: id(), Text: in.Text, Reply: reply, Decision: d, PreviousScenario: session.Active, TopicChanged: d.Status == "route" && session.Active != "" && session.Active != d.ScenarioID, Source: source, Calls: calls, Warnings: warnings, CreatedAt: time.Now().UTC(), Timing: domain.Timing{RoutingMS: routingMS, PolicyMS: ms(policyStart), STTMS: in.STTMS, InputKind: in.InputKind}}
	if scenario, ok := s.Catalog.Find(d.ScenarioID); ok {
		turn.ScenarioName = scenario.Name
	}
	if d.Status == "route" {
		session.Active = d.ScenarioID
	}
	session.Pending = d.Pending
	emit("policy", map[string]any{"status": d.Status, "reply": reply, "warnings": warnings, "topic_changed": turn.TopicChanged})
	turn.Timing.ServerMS = ms(started)
	session.Turns = append(session.Turns, turn)
	if e = s.Store.Save(r.Context(), session); e != nil {
		if stream {
			emit("error", map[string]string{"error": "session could not be saved"})
		} else {
			fail(w, 503, "session could not be saved")
		}
		return
	}
	result := map[string]any{"session_id": session.ID, "turn": turn}
	if stream {
		emit("result", result)
	} else {
		write(w, 200, result)
	}
	slog.Info("route_complete", "session_id", session.ID, "turn_id", turn.ID, "source", source, "scenario", d.ScenarioID, "status", d.Status, "routing_ms", routingMS)
}
func (s *Server) transcribe(w http.ResponseWriter, r *http.Request) {
	if !s.Speech.Enabled || s.Speech.Key == "" {
		fail(w, 503, "Для аудиофайла задайте SPEECH_PROVIDER=openai и OPENAI_API_KEY. Mock не подменяет распознавание текстом.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 24<<20)
	if err := r.ParseMultipartForm(24 << 20); err != nil {
		fail(w, 400, "invalid multipart or audio exceeds 24 MiB")
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, h, e := r.FormFile("audio")
	if e != nil {
		fail(w, 400, "audio file required")
		return
	}
	defer f.Close()
	start := time.Now()
	text, e := s.Speech.Transcribe(r.Context(), h.Filename, f)
	if e != nil {
		fail(w, 502, e.Error())
		return
	}
	write(w, 200, map[string]any{"text": text, "stt_ms": ms(start), "source": "openai"})
}
func (s *Server) connect(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	b, e := io.ReadAll(r.Body)
	if e != nil || !strings.HasPrefix(string(b), "v=0") {
		fail(w, 400, "SDP offer required")
		return
	}
	answer, e := s.Speech.Connect(r.Context(), string(b))
	if e != nil {
		fail(w, 502, e.Error())
		return
	}
	w.Header().Set("Content-Type", "application/sdp")
	_, _ = io.WriteString(w, answer)
}
func (s *Server) speak(w http.ResponseWriter, r *http.Request) {
	session, e := s.Store.Load(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		fail(w, 404, "session not found")
		return
	}
	var reply string
	for _, t := range session.Turns {
		if t.ID == chi.URLParam(r, "turn") {
			reply = t.Reply
			break
		}
	}
	if reply == "" {
		fail(w, 404, "turn not found")
		return
	}
	started := time.Now()
	resp, e := s.Speech.Speak(r.Context(), reply)
	if e != nil {
		fail(w, 502, e.Error())
		return
	}
	defer resp.Body.Close()
	// Wait for first PCM bytes before committing headers, so upstream failure is visible.
	buf := make([]byte, 4096)
	n, e := resp.Body.Read(buf)
	if n == 0 {
		fail(w, 502, "empty TTS stream")
		return
	}
	first := ms(started)
	w.Header().Set("Content-Type", "audio/pcm")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Audio-Sample-Rate", "24000")
	w.Header().Set("X-TTS-First-Byte-MS", fmt.Sprintf("%.2f", first))
	w.Header().Set("Access-Control-Expose-Headers", "X-TTS-First-Byte-MS, X-Audio-Sample-Rate")
	_, _ = w.Write(buf[:n])
	_ = http.NewResponseController(w).Flush()
	for e == nil {
		n, e = resp.Body.Read(buf)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return
			}
			_ = http.NewResponseController(w).Flush()
		}
	}
	slog.Info("tts_stream", "first_byte_ms", first, "error", e != nil && e != io.EOF)
}
func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	var input struct {
		EndToAudioMS   *float64 `json:"end_to_audio_ms"`
		TTSFirstByteMS *float64 `json:"tts_first_byte_ms"`
	}
	if !decode(w, r, &input) {
		return
	}
	for _, n := range []*float64{input.EndToAudioMS, input.TTSFirstByteMS} {
		if n != nil && (*n < 0 || *n > 120000 || math.IsNaN(*n)) {
			fail(w, 400, "invalid timing")
			return
		}
	}
	lock := s.lock(chi.URLParam(r, "id"))
	lock.Lock()
	defer lock.Unlock()
	session, e := s.Store.Load(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		fail(w, 404, "session not found")
		return
	}
	found := false
	for i := range session.Turns {
		t := &session.Turns[i]
		if t.ID == chi.URLParam(r, "turn") {
			if t.Timing.InputKind == "microphone" {
				t.Timing.EndToAudioMS = input.EndToAudioMS
			}
			t.Timing.TTSFirstByteMS = input.TTSFirstByteMS
			found = true
			break
		}
	}
	if !found {
		fail(w, 404, "turn not found")
		return
	}
	if s.Store.Save(r.Context(), session) != nil {
		fail(w, 503, "storage unavailable")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
func percentile(v []float64, p float64) any {
	if len(v) == 0 {
		return nil
	}
	sort.Float64s(v)
	i := int(math.Ceil(float64(len(v))*p)) - 1
	if i < 0 {
		i = 0
	}
	return v[i]
}
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	sessions, e := s.Store.List(r.Context())
	if e != nil {
		fail(w, 503, "storage unavailable")
		return
	}
	routing := []float64{}
	audio := []float64{}
	counts := map[string]int{}
	for _, session := range sessions {
		for _, t := range session.Turns {
			counts[t.Source]++
			if t.Source != "mock" && t.Source != "provider_error" {
				routing = append(routing, t.Timing.RoutingMS)
				if t.Timing.EndToAudioMS != nil {
					audio = append(audio, *t.Timing.EndToAudioMS)
				}
			}
		}
	}
	write(w, 200, map[string]any{"source_counts": counts, "routing_n": len(routing), "routing_p50_ms": percentile(routing, .5), "routing_p95_ms": percentile(routing, .95), "audio_n": len(audio), "end_to_audio_p50_ms": percentile(audio, .5), "end_to_audio_p95_ms": percentile(audio, .95), "note": "Mock excluded. Client audio timings are telemetry, not an independent benchmark. Latest 100 sessions with PostgreSQL."})
}
func Run() error {
	catalog, e := domain.LoadCatalog(ai.Env("DATA_DIR", "data/demo"))
	if e != nil {
		return fmt.Errorf("load catalog: %w", e)
	}
	router, e := ai.New(ai.ConfigFromEnv(), catalog)
	if e != nil {
		return e
	}
	var db store.Store
	mode := "memory"
	if url := os.Getenv("DATABASE_URL"); url != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db, e = store.NewPostgres(ctx, url)
		if e != nil {
			return fmt.Errorf("postgres: %w", e)
		}
		mode = "postgres"
	} else {
		db = store.NewMemory()
	}
	defer db.Close()
	s := &Server{Catalog: catalog, Router: router, Store: db, Speech: speech.New(), DBMode: mode}
	h := &http.Server{Addr: ":" + ai.Env("PORT", "8080"), Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 35 * time.Second, WriteTimeout: 65 * time.Second, IdleTimeout: 60 * time.Second}
	slog.Info("voice_router_ready", "addr", h.Addr, "provider", router.Name(), "catalog_count", len(catalog.Scenarios), "catalog_source", catalog.Source, "storage", mode)
	return h.ListenAndServe()
}
