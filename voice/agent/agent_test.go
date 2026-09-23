package agent

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"hackathon/voice/brain"
	"hackathon/voice/config"
	"hackathon/voice/elevenlabs"
	"hackathon/voice/lang"
	"hackathon/voice/speech"
	"hackathon/voice/voicelog"
)

// ---- fakes ----

type fakeRec struct {
	events chan elevenlabs.STTEvent
	mu     sync.Mutex
	sent   int
	commit int
}

func (f *fakeRec) Send(b []byte) error { f.mu.Lock(); f.sent++; f.mu.Unlock(); return nil }
func (f *fakeRec) Commit() error       { f.mu.Lock(); f.commit++; f.mu.Unlock(); return nil }
func (f *fakeRec) Events() <-chan elevenlabs.STTEvent {
	return f.events
}
func (f *fakeRec) SpeechEnd(w []elevenlabs.Word) (time.Time, bool) {
	return time.Now().Add(-50 * time.Millisecond), true
}
func (f *fakeRec) Close() error { return nil }

type fakeSTT struct{ rec *fakeRec }

func (f *fakeSTT) Open(ctx context.Context, o speech.RecognizerOptions) (speech.RecognizerSession, error) {
	return f.rec, nil
}

type fakeStream struct {
	audio  chan []byte
	mu     sync.Mutex
	done   bool
	opened time.Time
}

func (s *fakeStream) Send(text string, flush bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done || strings.TrimSpace(text) == "" {
		return nil
	}
	s.audio <- []byte(text)
	return nil
}
func (s *fakeStream) Finish() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.done {
		s.done = true
		close(s.audio)
	}
	return nil
}
func (s *fakeStream) Audio() <-chan []byte { return s.audio }
func (s *fakeStream) Err() error           { return nil }
func (s *fakeStream) Close() error         { return s.Finish() }
func (s *fakeStream) Info() elevenlabs.SocketInfo {
	return elevenlabs.SocketInfo{Model: "fake", Opened: s.opened}
}

type fakeTTS struct {
	mu    sync.Mutex
	opens int
}

func (f *fakeTTS) Open(ctx context.Context, l lang.Lang, format string) (speech.Stream, error) {
	f.mu.Lock()
	f.opens++
	f.mu.Unlock()
	return &fakeStream{audio: make(chan []byte, 64), opened: time.Now()}, nil
}
func (f *fakeTTS) Say(ctx context.Context, text string, l lang.Lang, format string) ([]byte, error) {
	return []byte("say:" + text), nil
}

type fakeBrain struct {
	stateless bool
	delay     time.Duration
	mu        sync.Mutex
	inputs    []brain.Input
	commits   []string
}

func (b *fakeBrain) Name() string    { return "fake" }
func (b *fakeBrain) Stateless() bool { return b.stateless }
func (b *fakeBrain) Turn(ctx context.Context, in brain.Input, emit func(brain.Event)) (brain.Result, error) {
	b.mu.Lock()
	b.inputs = append(b.inputs, in)
	b.mu.Unlock()
	select {
	case <-time.After(b.delay):
	case <-ctx.Done():
		return brain.Result{}, ctx.Err()
	}
	emit(brain.Event{Kind: brain.KindDecision, Decision: &brain.Decision{ScenarioID: "SC27", Confidence: 0.9}})
	reply := "Конечно, продлим полис. Назовите номер телефона."
	if in.Lang == "kk" {
		reply = "Әрине, полисті ұзартайық. Телефон нөміріңізді айтыңызшы."
	}
	for _, part := range strings.SplitAfter(reply, " ") {
		if ctx.Err() != nil {
			return brain.Result{}, ctx.Err()
		}
		emit(brain.Event{Kind: brain.KindDelta, Text: part})
	}
	return brain.Result{Model: "fake-model", Reply: reply, Raw: "[[SC27|0.9]]\n" + reply}, nil
}
func (b *fakeBrain) Commit(sessionID, user, assistant string) {
	b.mu.Lock()
	b.commits = append(b.commits, user)
	b.mu.Unlock()
}
func (b *fakeBrain) End(string) {}
func (b *fakeBrain) calls() []brain.Input {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]brain.Input(nil), b.inputs...)
}

type fakeTransport struct {
	mu     sync.Mutex
	played []byte
	events []map[string]any
	clears int
}

func (t *fakeTransport) OutputFormat() string { return "pcm_16000" }
func (t *fakeTransport) Play(b []byte) error {
	t.mu.Lock()
	t.played = append(t.played, b...)
	t.mu.Unlock()
	return nil
}
func (t *fakeTransport) Clear() { t.mu.Lock(); t.clears++; t.mu.Unlock() }
func (t *fakeTransport) Send(ev map[string]any) {
	t.mu.Lock()
	t.events = append(t.events, ev)
	t.mu.Unlock()
}
func (t *fakeTransport) Hangup() {}
func (t *fakeTransport) types() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []string
	for _, e := range t.events {
		out = append(out, e["type"].(string))
	}
	return out
}
func (t *fakeTransport) audio() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.played)
}
func (t *fakeTransport) find(typ string) map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, e := range t.events {
		if e["type"] == typ {
			return e
		}
	}
	return nil
}

type harness struct {
	a   *Agent
	rec *fakeRec
	tts *fakeTTS
	br  *fakeBrain
	tr  *fakeTransport
	log *voicelog.Logger
}

func newHarness(t *testing.T, br *fakeBrain, mut func(*config.Config)) *harness {
	t.Helper()
	cfg := config.Config{BargeIn: true, Speculative: true, SpeculateAfter: 40 * time.Millisecond, VADSilenceSecs: 0.5}
	if mut != nil {
		mut(&cfg)
	}
	h := &harness{rec: &fakeRec{events: make(chan elevenlabs.STTEvent, 32)}, tts: &fakeTTS{}, br: br, tr: &fakeTransport{}, log: voicelog.New(t.TempDir(), false)}
	h.a = New(context.Background(), Deps{Cfg: cfg, STT: &fakeSTT{h.rec}, TTS: h.tts, Brain: br, Log: h.log},
		Options{SessionID: "t1", Channel: "demo", InputFormat: "pcm_16000"}, h.tr)
	go h.a.Run()
	t.Cleanup(h.a.Close)
	return h
}

func (h *harness) partial(text string) {
	h.rec.events <- elevenlabs.STTEvent{Type: elevenlabs.EventPartial, Text: text, At: time.Now()}
}
func (h *harness) final(text string) {
	h.rec.events <- elevenlabs.STTEvent{Type: elevenlabs.EventCommittedTimestamps, Text: text, Language: "rus", At: time.Now(),
		Words: []elevenlabs.Word{{Text: "x", End: 0.1, Type: "word"}}}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

const ruReply = "Конечно, продлим полис. Назовите номер телефона."

// ---- tests ----

func TestTurnEndToEnd(t *testing.T) {
	h := newHarness(t, &fakeBrain{}, nil)
	h.partial("Здравствуйте хочу")
	h.final("Здравствуйте, хочу продлить полис")
	waitFor(t, "voice.metrics", func() bool { return h.tr.find("voice.metrics") != nil })
	if got := h.tr.audio(); got != ruReply {
		t.Fatalf("audio = %q", got)
	}
	types := strings.Join(h.tr.types(), ",")
	for _, want := range []string{"session.ready", "stt.partial", "stt.final", "router.decision", "response.delta", "tts.start", "response.final", "voice.metrics", "turn.done"} {
		if !strings.Contains(types, want) {
			t.Fatalf("missing %s in %s", want, types)
		}
	}
	if fin := h.tr.find("stt.final"); fin["language"] != "ru" {
		t.Fatalf("stt.final = %v", fin)
	}
	m := h.tr.find("voice.metrics")["metrics"].(voicelog.TurnMetrics)
	if m.Scenario != "SC27" || m.ReplyLang != "ru" || m.EndToAudioms <= 0 || m.Interrupted {
		t.Fatalf("metrics %+v", m)
	}
	if calls := h.br.calls(); len(calls) != 1 || calls[0].Lang != "ru" {
		t.Fatalf("brain calls %+v", calls)
	}
	waitFor(t, "commit", func() bool { h.br.mu.Lock(); defer h.br.mu.Unlock(); return len(h.br.commits) == 1 })
	if s := h.log.Stats().Snapshot(); s.Turns != 1 {
		t.Fatalf("stats %+v", s)
	}
}

func TestKazakhReply(t *testing.T) {
	h := newHarness(t, &fakeBrain{}, nil)
	h.final("Сәлеметсіз бе, полисімнің мерзімін ұзартқым келеді")
	waitFor(t, "voice.metrics", func() bool { return h.tr.find("voice.metrics") != nil })
	if calls := h.br.calls(); calls[0].Lang != "kk" {
		t.Fatalf("lang %q", calls[0].Lang)
	}
	if !strings.HasPrefix(h.tr.audio(), "Әрине") {
		t.Fatalf("audio %q", h.tr.audio())
	}
}

func TestBargeIn(t *testing.T) {
	h := newHarness(t, &fakeBrain{}, func(c *config.Config) { c.Speculative = false })
	h.final("Хочу продлить полис")
	waitFor(t, "audio", func() bool { return h.tr.audio() != "" })
	// fake audio bytes are "long" at 16 kHz? make sure the agent believes it is speaking
	h.a.mu.Lock()
	h.a.playUntil = time.Now().Add(2 * time.Second)
	h.a.mu.Unlock()
	h.partial("Подождите нет")
	waitFor(t, "tts.clear", func() bool { return h.tr.find("tts.clear") != nil })
	h.tr.mu.Lock()
	clears := h.tr.clears
	h.tr.mu.Unlock()
	if clears != 1 {
		t.Fatalf("clears = %d", clears)
	}
}

func TestSpeculationHit(t *testing.T) {
	br := &fakeBrain{stateless: true}
	h := newHarness(t, br, nil)
	h.partial("Хочу продлить полис")
	waitFor(t, "speculative call", func() bool { c := br.calls(); return len(c) == 1 && c[0].Speculative })
	time.Sleep(50 * time.Millisecond)
	if h.tr.audio() != "" {
		t.Fatal("speculative audio must be held until the final transcript")
	}
	h.final("Хочу продлить полис.")
	waitFor(t, "voice.metrics", func() bool { return h.tr.find("voice.metrics") != nil })
	if len(br.calls()) != 1 {
		t.Fatalf("brain called %d times, want 1", len(br.calls()))
	}
	if h.tr.audio() != ruReply {
		t.Fatalf("audio %q", h.tr.audio())
	}
	m := h.tr.find("voice.metrics")["metrics"].(voicelog.TurnMetrics)
	if !m.Speculative {
		t.Fatalf("metrics should mark speculative hit: %+v", m)
	}
}

func TestSpeculationMiss(t *testing.T) {
	br := &fakeBrain{stateless: true, delay: 20 * time.Millisecond}
	h := newHarness(t, br, nil)
	h.partial("Хочу продлить")
	waitFor(t, "speculative call", func() bool { return len(br.calls()) == 1 })
	h.final("Хочу продлить КАСКО на машину")
	waitFor(t, "voice.metrics", func() bool { return h.tr.find("voice.metrics") != nil })
	calls := br.calls()
	if len(calls) != 2 || calls[1].Speculative || calls[1].Text != "Хочу продлить КАСКО на машину" {
		t.Fatalf("calls %+v", calls)
	}
	if h.tr.audio() != ruReply {
		t.Fatalf("audio %q (speculative audio must be dropped)", h.tr.audio())
	}
}

func TestMergeSplitUtterance(t *testing.T) {
	br := &fakeBrain{delay: 300 * time.Millisecond}
	h := newHarness(t, br, func(c *config.Config) { c.Speculative = false })
	h.final("Хочу оформить")
	waitFor(t, "first call", func() bool { return len(br.calls()) == 1 })
	h.final("КАСКО на машину")
	waitFor(t, "voice.metrics", func() bool { return h.tr.find("voice.metrics") != nil })
	calls := br.calls()
	if last := calls[len(calls)-1]; last.Text != "Хочу оформить КАСКО на машину" {
		t.Fatalf("merged text = %q", last.Text)
	}
	if h.tr.audio() != ruReply {
		t.Fatalf("only the merged reply must be spoken: %q", h.tr.audio())
	}
}

func TestSubmitText(t *testing.T) {
	h := newHarness(t, &fakeBrain{}, nil)
	waitFor(t, "ready", func() bool { return h.tr.find("session.ready") != nil })
	h.a.SubmitText("Какие документы нужны?")
	waitFor(t, "voice.metrics", func() bool { return h.tr.find("voice.metrics") != nil })
	if h.tr.audio() != ruReply {
		t.Fatalf("audio %q", h.tr.audio())
	}
}

func TestGreetingAndFiller(t *testing.T) {
	br := &fakeBrain{delay: 200 * time.Millisecond}
	cfg := func(c *config.Config) { c.Speculative = false; c.FillerAfter = 30 * time.Millisecond }
	h := newHarness(t, br, cfg)
	h.a.greet() // Options.Greeting is false, so Run did not greet; call it directly
	if !strings.HasPrefix(h.tr.audio(), "say:"+Greeting) {
		t.Fatalf("greeting audio %q", h.tr.audio())
	}
	h.tr.mu.Lock()
	h.tr.played = nil
	h.tr.mu.Unlock()
	h.a.mu.Lock()
	h.a.playUntil = time.Time{}
	h.a.mu.Unlock()
	h.final("Хочу продлить полис")
	waitFor(t, "voice.metrics", func() bool { return h.tr.find("voice.metrics") != nil })
	if got := h.tr.audio(); !strings.HasPrefix(got, "say:Секунду.") || !strings.HasSuffix(got, ruReply) {
		t.Fatalf("audio %q", got)
	}
}

func TestSpecMatches(t *testing.T) {
	cases := []struct {
		spec, final string
		want        bool
	}{
		{"хочу продлить полис", "хочу продлить полис", true},
		{"я вчера оплатил полис деньги списались а полис н", "я вчера оплатил полис деньги списались а полис не пришёл", true},
		{"хочу продлить", "хочу продлить каско на машину", false},
		{"хочу продлить полис каско", "хочу оформить полис каско", false},
	}
	for _, c := range cases {
		if got := specMatches(normalize(c.spec), normalize(c.final)); got != c.want {
			t.Errorf("specMatches(%q, %q) = %v, want %v", c.spec, c.final, got, c.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	if normalize("Хочу, продлить  ПОЛИС!") != normalize("хочу продлить полис") {
		t.Fatal("normalize must ignore case and punctuation")
	}
	if wordCount("  да, нет ") != 2 {
		t.Fatal("wordCount")
	}
}
