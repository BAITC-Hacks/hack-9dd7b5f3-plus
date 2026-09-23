package voicelog

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"hackathon/voice/audio"
)

const floatEps = 1e-9

func floatEq(a, b float64) bool { return math.Abs(a-b) < floatEps }

func readLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	var out []map[string]any
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("bad JSONL line %q: %v", line, err)
		}
		out = append(out, m)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return out
}

func findSessionLogFile(t *testing.T, dir string) string {
	t.Helper()
	var found string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".jsonl") {
			found = path
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	if found == "" {
		t.Fatalf("no .jsonl file found under %s", dir)
	}
	return found
}

func pcmFormat() audio.Format {
	return audio.Format{Name: "pcm_16000", Codec: "pcm", Rate: 16000, BytesPerSample: 2}
}

// --- Log / file naming -------------------------------------------------

func TestOpenLogWritesJSONLWithExpectedNameAndFields(t *testing.T) {
	dir := t.TempDir()
	l := New(dir, false)

	sess := l.Open("call/123 weird id", "phone", map[string]any{"foo": "bar"})
	if sess == nil {
		t.Fatal("Open returned nil Session")
	}

	sess.Log("greeting", map[string]any{"text": "hello"})

	path := findSessionLogFile(t, dir)

	rel, err := filepath.Rel(dir, path)
	if err != nil {
		t.Fatal(err)
	}
	rel = filepath.ToSlash(rel)

	pattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}/\d{6}_phone_call_123_weird_id\.jsonl$`)
	if !pattern.MatchString(rel) {
		t.Fatalf("unexpected log file path %q", rel)
	}

	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %v", len(lines), lines)
	}

	first := lines[0]
	if first["kind"] != "session.start" {
		t.Errorf("first record kind = %v, want session.start", first["kind"])
	}
	if first["foo"] != "bar" {
		t.Errorf("first record missing meta field foo=bar: %v", first)
	}
	for _, key := range []string{"ts", "t_ms", "session", "channel", "kind"} {
		if _, ok := first[key]; !ok {
			t.Errorf("first record missing required key %q: %v", key, first)
		}
	}
	if first["session"] != "call/123 weird id" {
		t.Errorf("session field = %v", first["session"])
	}
	if first["channel"] != "phone" {
		t.Errorf("channel field = %v", first["channel"])
	}
	if _, err := time.Parse(time.RFC3339Nano, first["ts"].(string)); err != nil {
		t.Errorf("ts not RFC3339Nano: %v", err)
	}

	second := lines[1]
	if second["kind"] != "greeting" || second["text"] != "hello" {
		t.Errorf("second record = %v", second)
	}

	sess.Close("caller_hangup")
}

// --- Close idempotency ---------------------------------------------------

func TestCloseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	l := New(dir, false)
	sess := l.Open("s1", "web", nil)

	sess.Close("done")
	sess.Close("done-again") // must not panic or double-write

	path := findSessionLogFile(t, dir)
	lines := readLines(t, path)

	endCount := 0
	for _, ln := range lines {
		if ln["kind"] == "session.end" {
			endCount++
			if ln["reason"] != "done" {
				t.Errorf("session.end reason = %v, want done", ln["reason"])
			}
		}
	}
	if endCount != 1 {
		t.Fatalf("want exactly 1 session.end record, got %d", endCount)
	}

	sizeBefore, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	sess.Log("post-close", map[string]any{"x": 1}) // must publish-only, not write
	sizeAfter, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if sizeAfter.Size() != sizeBefore.Size() {
		t.Errorf("Log after Close wrote to file: size %d -> %d", sizeBefore.Size(), sizeAfter.Size())
	}
}

// --- nil Session ----------------------------------------------------------

func TestNilSessionIsNoOp(t *testing.T) {
	var s *Session
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil *Session method panicked: %v", r)
		}
	}()
	s.Log("x", map[string]any{"a": 1})
	s.Turn(TurnMetrics{Turn: 1})
	s.RecordIn([]byte{1, 2, 3}, pcmFormat())
	s.RecordOut([]byte{1, 2, 3}, pcmFormat())
	s.Close("reason")
}

// --- Turn / Stats integration ---------------------------------------------

func TestTurnUpdatesStats(t *testing.T) {
	dir := t.TempDir()
	l := New(dir, false)
	sess := l.Open("s1", "web", nil)

	sess.Turn(TurnMetrics{
		Turn:         1,
		Channel:      "web",
		ReplyLang:    "ru",
		Brain:        "router",
		EndToAudioms: 900,
		STTms:        200,
	})

	snap := l.Stats().Snapshot()
	if snap.Turns != 1 {
		t.Fatalf("snap.Turns = %d, want 1", snap.Turns)
	}
	if snap.EndToAudio.Count != 1 || snap.EndToAudio.P50 != 900 {
		t.Errorf("EndToAudio = %+v", snap.EndToAudio)
	}
	if snap.ByReplyLang["ru"] != 1 {
		t.Errorf("ByReplyLang = %v", snap.ByReplyLang)
	}

	sess.Close("done")

	sums := l.Sessions()
	if len(sums) != 1 || sums[0].Turns != 1 {
		t.Errorf("Sessions() = %+v", sums)
	}
}

// --- Stats.Snapshot: quantiles, zeros ignored, ShareUnder1500, maps ------

func TestStatsSnapshotQuantilesIgnoreZerosAndShareUnder1500(t *testing.T) {
	s := NewStats(100)

	// 0 must be ignored as "not measured", not as an instant response.
	values := []float64{0, 100, 200, 300, 400, 500, 600, 700, 800, 2000}
	for i, v := range values {
		s.Add(TurnMetrics{
			Turn:         i + 1,
			EndToAudioms: v,
			ReplyLang:    "ru",
			Channel:      "web",
			Scenario:     "faq",
		})
	}

	snap := s.Snapshot()

	if snap.Turns != len(values) {
		t.Fatalf("Turns = %d, want %d", snap.Turns, len(values))
	}

	// Non-zero, sorted: 100 200 300 400 500 600 700 800 2000 (n=9)
	// nearest-rank: p50 -> ceil(4.5)=5th -> 500; p90 -> ceil(8.1)=9th -> 2000;
	// p95 -> ceil(8.55)=9th -> 2000.
	if snap.EndToAudio.Count != 9 {
		t.Fatalf("EndToAudio.Count = %d, want 9", snap.EndToAudio.Count)
	}
	if !floatEq(snap.EndToAudio.P50, 500) {
		t.Errorf("P50 = %v, want 500", snap.EndToAudio.P50)
	}
	if !floatEq(snap.EndToAudio.P90, 2000) {
		t.Errorf("P90 = %v, want 2000", snap.EndToAudio.P90)
	}
	if !floatEq(snap.EndToAudio.P95, 2000) {
		t.Errorf("P95 = %v, want 2000", snap.EndToAudio.P95)
	}
	if !floatEq(snap.EndToAudio.Max, 2000) {
		t.Errorf("Max = %v, want 2000", snap.EndToAudio.Max)
	}

	// ShareUnder1500: turns with 0 < EndToAudio <= 1500, over ALL turns.
	// 100..800 (8 values) qualify; 2000 doesn't; 0 doesn't.
	wantShare := 8.0 / float64(len(values))
	if !floatEq(snap.ShareUnder1500, wantShare) {
		t.Errorf("ShareUnder1500 = %v, want %v", snap.ShareUnder1500, wantShare)
	}

	if snap.ByReplyLang["ru"] != len(values) {
		t.Errorf("ByReplyLang[ru] = %d, want %d", snap.ByReplyLang["ru"], len(values))
	}
	if snap.ByChannel["web"] != len(values) {
		t.Errorf("ByChannel[web] = %d, want %d", snap.ByChannel["web"], len(values))
	}
	if snap.ByScenario["faq"] != len(values) {
		t.Errorf("ByScenario[faq] = %d, want %d", snap.ByScenario["faq"], len(values))
	}

	if len(snap.Recent) != 10 {
		t.Fatalf("Recent len = %d, want 10", len(snap.Recent))
	}
	if snap.Recent[0].EndToAudioms != 2000 {
		t.Errorf("Recent[0].EndToAudioms = %v, want 2000 (newest first)", snap.Recent[0].EndToAudioms)
	}
}

func TestStatsRingEvictionAndRecentCap(t *testing.T) {
	s := NewStats(5) // small ring to exercise eviction cheaply
	for i := 1; i <= 8; i++ {
		s.Add(TurnMetrics{Turn: i, EndToAudioms: float64(i * 100)})
	}
	snap := s.Snapshot()
	if snap.Turns != 5 {
		t.Fatalf("Turns = %d, want 5 (ring capacity)", snap.Turns)
	}
	if snap.Recent[0].Turn != 8 {
		t.Errorf("newest turn = %d, want 8", snap.Recent[0].Turn)
	}
	if snap.Recent[len(snap.Recent)-1].Turn != 4 {
		t.Errorf("oldest remaining turn = %d, want 4 (1-3 evicted)", snap.Recent[len(snap.Recent)-1].Turn)
	}
}

func TestMsSinceRoundsToOneDecimal(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start.Add(123450 * time.Microsecond) // 123.45ms
	got := msSince(start, now)
	if !floatEq(got*10, math.Round(got*10)) {
		t.Errorf("msSince(%v) not rounded to 1 decimal", got)
	}
	if !floatEq(got, round1(123.45)) {
		t.Errorf("msSince = %v, want %v", got, round1(123.45))
	}
}

// --- Hub: publish/subscribe/backlog/cancel/non-blocking -------------------

func backlogStrings(b [][]byte) []string {
	out := make([]string, len(b))
	for i, v := range b {
		out[i] = string(v)
	}
	return out
}

func TestHubPublishSubscribeBacklogAndCancel(t *testing.T) {
	h := NewHub(2)

	h.Publish([]byte(`{"n":1}`))
	h.Publish([]byte(`{"n":2}`))
	h.Publish([]byte(`{"n":3}`)) // backlog capacity 2: only n=2,3 retained

	ch, backlog, cancel := h.Subscribe()
	defer cancel()

	if len(backlog) != 2 {
		t.Fatalf("backlog len = %d, want 2 (got %v)", len(backlog), backlogStrings(backlog))
	}
	if string(backlog[0]) != `{"n":2}` || string(backlog[1]) != `{"n":3}` {
		t.Fatalf("backlog = %v", backlogStrings(backlog))
	}

	h.Publish([]byte(`{"n":4}`))
	select {
	case got := <-ch:
		if string(got) != `{"n":4}` {
			t.Errorf("got %s, want {\"n\":4}", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live publish")
	}

	cancel()
	cancel() // idempotent, must not panic

	h.Publish([]byte(`{"n":5}`))
	select {
	case got, ok := <-ch:
		t.Fatalf("received after cancel: %q ok=%v", string(got), ok)
	case <-time.After(50 * time.Millisecond):
		// expected: nothing delivered to a cancelled subscriber
	}
}

func TestHubPublishNeverBlocksOnFullSubscriber(t *testing.T) {
	h := NewHub(0)
	ch, _, cancel := h.Subscribe()
	defer cancel()

	for i := 0; i < subscriberBuffer+10; i++ {
		h.Publish([]byte("x")) // fills, then overflows, the subscriber buffer
	}

	done := make(chan struct{})
	go func() {
		h.Publish([]byte("y"))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a full subscriber")
	}

	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// --- Hub.ServeSSE -----------------------------------------------------------

func TestHubServeSSE(t *testing.T) {
	h := NewHub(10)
	h.Publish([]byte(`{"a":1}`))
	h.Publish([]byte(`{"a":2}`))

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	// Only this goroutine touches `cancel`; ServeSSE runs synchronously in
	// the test goroutine, so there is no concurrent access to `rec`.
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	h.ServeSSE(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if xa := rec.Header().Get("X-Accel-Buffering"); xa != "no" {
		t.Errorf("X-Accel-Buffering = %q", xa)
	}
	if ao := rec.Header().Get("Access-Control-Allow-Origin"); ao != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q", ao)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "data: {\"a\":1}\n\n") {
		t.Errorf("body missing first backlog event: %q", body)
	}
	if !strings.Contains(body, "data: {\"a\":2}\n\n") {
		t.Errorf("body missing second backlog event: %q", body)
	}
}

// --- RecordIn/RecordOut WAV writing ----------------------------------------

func assertStartsWithRIFF(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(b) < 4 || string(b[:4]) != "RIFF" {
		t.Fatalf("%s does not start with RIFF: %v", path, b[:min(len(b), 4)])
	}
}

func TestRecordInAndOutWriteWAV(t *testing.T) {
	dir := t.TempDir()
	l := New(dir, true)
	sess := l.Open("wavtest", "phone", nil)

	format := pcmFormat()
	pcm := make([]byte, 320) // 10ms @ 16kHz/16-bit mono
	sess.RecordIn(pcm, format)
	sess.RecordOut(pcm, format)

	sess.Close("done")

	assertStartsWithRIFF(t, sess.baseWAVPath+"_in.wav")
	assertStartsWithRIFF(t, sess.baseWAVPath+"_out.wav")
}

func TestRecordInNoopWhenRecordingDisabled(t *testing.T) {
	dir := t.TempDir()
	l := New(dir, false)
	sess := l.Open("norecord", "phone", nil)

	sess.RecordIn(make([]byte, 320), pcmFormat())
	sess.Close("done")

	if _, err := os.Stat(sess.baseWAVPath + "_in.wav"); !os.IsNotExist(err) {
		t.Fatalf("expected no wav file when record=false, stat err = %v", err)
	}
}

// --- Concurrency ------------------------------------------------------------

func TestSessionConcurrentLogAndTurn(t *testing.T) {
	dir := t.TempDir()
	l := New(dir, false)
	sess := l.Open("concurrent", "web", nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				sess.Log("event", map[string]any{"i": i})
			} else {
				sess.Turn(TurnMetrics{Turn: i})
			}
		}(i)
	}
	wg.Wait()
	sess.Close("done")

	path := findSessionLogFile(t, dir)
	lines := readLines(t, path) // a parse failure here would flag corrupted/interleaved writes
	if len(lines) != 52 {       // session.start + 50 + session.end
		t.Fatalf("want 52 lines, got %d", len(lines))
	}
}

func TestLoggerConcurrentSessions(t *testing.T) {
	dir := t.TempDir()
	l := New(dir, true)

	var wg sync.WaitGroup
	format := pcmFormat()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sess := l.Open(fmt.Sprintf("sess-%d", i), "web", map[string]any{"i": i})
			for j := 0; j < 5; j++ {
				sess.Log("event", map[string]any{"j": j})
				sess.Turn(TurnMetrics{Turn: j, EndToAudioms: float64(100 * j)})
				sess.RecordIn(make([]byte, 160), format)
				sess.RecordOut(make([]byte, 160), format)
			}
			sess.Close("done")
		}(i)
	}
	wg.Wait()

	if got := len(l.Sessions()); got != 20 {
		t.Errorf("Sessions() len = %d, want 20", got)
	}
	if snap := l.Stats().Snapshot(); snap.Turns != 100 {
		t.Errorf("Stats turns = %d, want 100", snap.Turns)
	}
}
