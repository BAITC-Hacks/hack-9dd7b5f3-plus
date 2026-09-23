// Package voicelog provides structured, durable logging for a real-time
// voice agent: one JSONL file per call/session with per-stage timings, a
// live event hub that fans log lines out to an admin console over SSE, and
// rolling latency statistics (p50/p90/p95) over recent turns.
//
// Every exported type is safe for concurrent use. Session methods are
// no-ops on a nil *Session, so a Session obtained from a disabled logger
// (or simply not yet opened) can be used without a nil check at every call
// site.
package voicelog

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"hackathon/voice/audio"
)

// maxSessions bounds how many SessionSummary entries Logger keeps in memory.
const maxSessions = 200

// Logger is the entry point of the package: it opens per-session log
// files, owns the live event Hub and the rolling latency Stats, and
// remembers a bounded history of recent session summaries for an admin
// console.
type Logger struct {
	dir    string
	record bool

	hub   *Hub
	stats *Stats

	mu       sync.Mutex
	sessions []*SessionSummary // bounded to maxSessions, oldest first
}

// New creates a Logger that writes session logs under dir (subdirectories
// and files are created lazily per session) and, when record is true, also
// writes raw in/out audio to WAV files alongside each session's log.
func New(dir string, record bool) *Logger {
	return &Logger{
		dir:    dir,
		record: record,
		hub:    NewHub(256),
		stats:  NewStats(1000),
	}
}

// Hub returns the live event hub that every Session.Log call publishes to.
func (l *Logger) Hub() *Hub { return l.hub }

// Stats returns the rolling latency/quality statistics collector that
// every Session.Turn call feeds.
func (l *Logger) Stats() *Stats { return l.stats }

// Sessions returns a snapshot of the most recent session summaries, most
// recent first.
func (l *Logger) Sessions() []SessionSummary {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]SessionSummary, len(l.sessions))
	n := len(l.sessions)
	for i, sum := range l.sessions {
		out[n-1-i] = *sum
	}
	return out
}

// addSummary registers a new session summary, evicting the oldest entry
// once more than maxSessions are held.
func (l *Logger) addSummary(sum *SessionSummary) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sessions = append(l.sessions, sum)
	if len(l.sessions) > maxSessions {
		drop := len(l.sessions) - maxSessions
		l.sessions = append(l.sessions[:0], l.sessions[drop:]...)
	}
}

// setTurns updates a summary's turn count under the logger lock.
func (l *Logger) setTurns(sum *SessionSummary, turns int) {
	l.mu.Lock()
	sum.Turns = turns
	l.mu.Unlock()
}

// finishSummary marks a summary as ended under the logger lock.
func (l *Logger) finishSummary(sum *SessionSummary, end time.Time, reason string, turns int) {
	l.mu.Lock()
	sum.End = end
	sum.Reason = reason
	sum.Turns = turns
	l.mu.Unlock()
}

// SessionSummary is a lightweight, JSON-serializable snapshot of one
// session's lifecycle, suitable for an admin console list view.
type SessionSummary struct {
	ID      string         `json:"id"`
	Channel string         `json:"channel"`
	Start   time.Time      `json:"start"`
	End     time.Time      `json:"end,omitempty"`
	Turns   int            `json:"turns"`
	File    string         `json:"log_file"`
	Reason  string         `json:"end_reason,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// Session represents one open call/conversation. Every method is safe for
// concurrent use and is a no-op on a nil *Session.
type Session struct {
	ID      string
	Channel string
	Start   time.Time

	logger  *Logger
	summary *SessionSummary

	mu     sync.Mutex
	file   *os.File
	closed bool
	turns  int

	recordEnabled bool
	baseWAVPath   string

	inMu  sync.Mutex
	inWav *audio.WAVWriter

	outMu  sync.Mutex
	outWav *audio.WAVWriter
}

// Open starts a session log file at
// <dir>/<YYYY-MM-DD>/<HHMMSS>_<channel>_<sanitized id>.jsonl and logs a
// "session.start" record with the meta fields. If the file cannot be
// created, the returned Session still works (hub + stats only) and a
// warning is logged via log/slog.
func (l *Logger) Open(id, channel string, meta map[string]any) *Session {
	start := time.Now().UTC()

	dateDir := start.Format("2006-01-02")
	timePart := start.Format("150405")
	fileName := fmt.Sprintf("%s_%s_%s.jsonl", timePart, channel, sanitizeID(id))
	dirPath := filepath.Join(l.dir, dateDir)
	fullPath := filepath.Join(dirPath, fileName)

	s := &Session{
		ID:            id,
		Channel:       channel,
		Start:         start,
		logger:        l,
		recordEnabled: l.record,
		baseWAVPath:   strings.TrimSuffix(fullPath, filepath.Ext(fullPath)),
	}

	var loggedPath string
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		slog.Warn("voicelog: failed to create session directory", "dir", dirPath, "err", err)
	} else if f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err != nil {
		slog.Warn("voicelog: failed to create session log file", "path", fullPath, "err", err)
	} else {
		s.file = f
		loggedPath = fullPath
	}

	var metaCopy map[string]any
	if len(meta) > 0 {
		metaCopy = make(map[string]any, len(meta))
		for k, v := range meta {
			metaCopy[k] = v
		}
	}

	sum := &SessionSummary{
		ID:      id,
		Channel: channel,
		Start:   start,
		File:    loggedPath,
		Meta:    metaCopy,
	}
	s.summary = sum
	l.addSummary(sum)

	s.Log("session.start", meta)

	return s
}

// Log writes one JSON line containing fields plus "ts" (RFC3339Nano UTC),
// "t_ms" (milliseconds since session start, rounded to 1 decimal),
// "session", "channel" and "kind". Each write goes straight to the
// underlying file (no buffering), so a concurrent `tail -f` sees it right
// away; the same JSON payload is published to the hub.
//
// Log is safe to call after Close: the file write is skipped and only the
// hub publish happens. A nil *Session is a no-op.
func (s *Session) Log(kind string, fields map[string]any) {
	if s == nil {
		return
	}

	now := time.Now().UTC()
	rec := make(map[string]any, len(fields)+5)
	for k, v := range fields {
		rec[k] = v
	}
	rec["ts"] = now.Format(time.RFC3339Nano)
	rec["t_ms"] = msSince(s.Start, now)
	rec["session"] = s.ID
	rec["channel"] = s.Channel
	rec["kind"] = kind

	payload, err := json.Marshal(rec)
	if err != nil {
		slog.Warn("voicelog: failed to marshal log record", "session", s.ID, "kind", kind, "err", err)
		return
	}

	s.mu.Lock()
	if s.file != nil {
		line := make([]byte, len(payload)+1)
		copy(line, payload)
		line[len(payload)] = '\n'
		if _, werr := s.file.Write(line); werr != nil {
			slog.Warn("voicelog: failed to write session log", "session", s.ID, "err", werr)
		}
	}
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Hub().Publish(payload)
	}
}

// Turn logs a "turn" record for m, feeds it into the logger's rolling
// Stats, and increments the session's turn counter. A nil *Session is a
// no-op.
func (s *Session) Turn(m TurnMetrics) {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.turns++
	turns := s.turns
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.setTurns(s.summary, turns)
		if stats := s.logger.Stats(); stats != nil {
			stats.Add(m)
		}
	}

	s.Log("turn", turnFields(m))
}

// RecordIn appends b to <base>_in.wav, decoding mu-law to PCM16 first;
// compressed formats are ignored. It only does anything when the Logger
// was created with record=true. A nil *Session is a no-op.
func (s *Session) RecordIn(b []byte, f audio.Format) {
	if s == nil || !s.recordEnabled {
		return
	}
	s.writeAudio(&s.inMu, &s.inWav, s.baseWAVPath+"_in.wav", b, f)
}

// RecordOut appends b to <base>_out.wav. See RecordIn for format handling.
func (s *Session) RecordOut(b []byte, f audio.Format) {
	if s == nil || !s.recordEnabled {
		return
	}
	s.writeAudio(&s.outMu, &s.outWav, s.baseWAVPath+"_out.wav", b, f)
}

// writeAudio normalizes b to PCM16 and appends it to the WAV file at path,
// lazily creating the writer (sized by f.Rate) on first use.
func (s *Session) writeAudio(mu *sync.Mutex, w **audio.WAVWriter, path string, b []byte, f audio.Format) {
	pcm, ok := toPCM16(b, f)
	if !ok || len(pcm) == 0 {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if *w == nil {
		writer, err := audio.CreateWAV(path, f.Rate)
		if err != nil {
			slog.Warn("voicelog: failed to create wav file", "session", s.ID, "path", path, "err", err)
			return
		}
		*w = writer
	}
	if _, err := (*w).Write(pcm); err != nil {
		slog.Warn("voicelog: failed to write wav data", "session", s.ID, "path", path, "err", err)
	}
}

// toPCM16 normalizes b to signed 16-bit PCM according to f's codec:
// passthrough for PCM, mu-law decoded via audio.MulawToPCM, and anything
// else (compressed codecs, per f.Raw()) ignored.
func toPCM16(b []byte, f audio.Format) ([]byte, bool) {
	if len(b) == 0 || !f.Raw() {
		return nil, false
	}
	if f.Codec == "ulaw" {
		return audio.MulawToPCM(b), true
	}
	return b, true // pcm
}

// Close logs a "session.end" record (reason, turns, duration_ms), flushes
// and closes the log file and any WAV writers, and updates the session's
// summary. Close is idempotent and a no-op on a nil *Session.
func (s *Session) Close(reason string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	turns := s.turns
	s.mu.Unlock()

	end := time.Now().UTC()

	s.Log("session.end", map[string]any{
		"reason":      reason,
		"turns":       turns,
		"duration_ms": msSince(s.Start, end),
	})

	s.mu.Lock()
	file := s.file
	s.file = nil
	s.mu.Unlock()
	if file != nil {
		if err := file.Close(); err != nil {
			slog.Warn("voicelog: failed to close session log file", "session", s.ID, "err", err)
		}
	}

	s.inMu.Lock()
	inWav := s.inWav
	s.inWav = nil
	s.inMu.Unlock()
	if inWav != nil {
		if err := inWav.Close(); err != nil {
			slog.Warn("voicelog: failed to close input wav", "session", s.ID, "err", err)
		}
	}

	s.outMu.Lock()
	outWav := s.outWav
	s.outWav = nil
	s.outMu.Unlock()
	if outWav != nil {
		if err := outWav.Close(); err != nil {
			slog.Warn("voicelog: failed to close output wav", "session", s.ID, "err", err)
		}
	}

	if s.logger != nil {
		s.logger.finishSummary(s.summary, end, reason, turns)
	}
}

// sanitizeID makes id safe to embed as a single filename component,
// replacing anything but letters, digits, '-', '_' and '.' with '_'.
func sanitizeID(id string) string {
	if id == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(id))
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// round1 rounds v to one decimal place.
func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// msSince returns the milliseconds between start and now, rounded to one
// decimal place.
func msSince(start, now time.Time) float64 {
	return round1(float64(now.Sub(start).Nanoseconds()) / 1e6)
}

// turnFields converts m to a map keyed by its JSON tags, so Log("turn", …)
// emits exactly the fields TurnMetrics defines, honoring its omitempty
// tags.
func turnFields(m TurnMetrics) map[string]any {
	b, err := json.Marshal(m)
	if err != nil {
		slog.Warn("voicelog: failed to marshal turn metrics", "err", err)
		return map[string]any{}
	}
	fields := make(map[string]any)
	if err := json.Unmarshal(b, &fields); err != nil {
		slog.Warn("voicelog: failed to decode turn metrics", "err", err)
		return map[string]any{}
	}
	return fields
}
