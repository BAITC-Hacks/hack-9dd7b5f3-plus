package elevenlabs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"hackathon/voice/audio"
)

// STTOptions configures a Scribe v2 Realtime session.
type STTOptions struct {
	Model                    string   // default "scribe_v2_realtime"
	AudioFormat              string   // pcm_8000 | pcm_16000 | ... | ulaw_8000 (default pcm_16000)
	Language                 string   // "" = auto-detect (needed for RU/KZ code-switching)
	SecondaryLanguages       []string // optional extra languages
	CommitStrategy           string   // "vad" (server end-of-turn) | "manual" (push-to-talk)
	VADSilenceSecs           float64  // silence that ends a turn in vad mode (0 = server default)
	VADThreshold             float64  // 0 = server default
	MinSpeechMS              int      // 0 = server default
	MinSilenceMS             int      // 0 = server default
	IncludeTimestamps        bool     // word timings: used to measure the real end of speech
	IncludeLanguageDetection bool     // language_code on committed segments
	Keyterms                 []string // domain words to bias recognition
	PreviousText             string   // context sent with the first chunk
}

// Word is one recognised token with timings in seconds from the session start.
type Word struct {
	Text    string  `json:"text"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Type    string  `json:"type"`
	Logprob float64 `json:"logprob"`
}

// STT event types.
const (
	EventSessionStarted      = "session_started"
	EventPartial             = "partial"
	EventCommitted           = "committed"
	EventCommittedTimestamps = "committed_timestamps"
	EventWarning             = "warning"
	EventError               = "error"
	EventClosed              = "closed"
)

// STTEvent is one message from the recogniser.
type STTEvent struct {
	Type     string
	Text     string
	Language string // committed_timestamps only, when language detection is on
	Words    []Word // committed_timestamps only
	Code     string // server message_type for errors (auth_error, quota_exceeded, ...)
	Err      error
	At       time.Time
}

// STTSession is an open realtime transcription stream. Send audio as it is
// captured; read partial and committed transcripts from Events.
type STTSession struct {
	ID string

	conn      *websocket.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	opts      STTOptions
	format    audio.Format
	events    chan STTEvent
	wmu       sync.Mutex
	sentFirst bool
	clock     audioClock
	closeOnce sync.Once
}

type sttMessage struct {
	MessageType  string  `json:"message_type"`
	SessionID    string  `json:"session_id"`
	Text         string  `json:"text"`
	LanguageCode *string `json:"language_code"`
	Words        []Word  `json:"words"`
	Error        string  `json:"error"`
	Warning      string  `json:"warning"`
}

type sttChunk struct {
	MessageType  string `json:"message_type"`
	Audio        string `json:"audio_base_64"`
	Commit       bool   `json:"commit"`
	SampleRate   int    `json:"sample_rate"`
	PreviousText string `json:"previous_text,omitempty"`
}

// OpenSTT connects to Scribe v2 Realtime and waits for session_started, so
// auth or parameter errors surface here instead of mid-call.
func (c *Client) OpenSTT(ctx context.Context, o STTOptions) (*STTSession, error) {
	if o.Model == "" {
		o.Model = "scribe_v2_realtime"
	}
	if o.AudioFormat == "" {
		o.AudioFormat = "pcm_16000"
	}
	if o.CommitStrategy == "" {
		o.CommitStrategy = "vad"
	}
	f, err := audio.ParseFormat(o.AudioFormat)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("model_id", o.Model)
	q.Set("audio_format", o.AudioFormat)
	q.Set("commit_strategy", o.CommitStrategy)
	if o.Language != "" {
		q.Set("language_code", o.Language)
	}
	for _, l := range o.SecondaryLanguages {
		q.Add("secondary_languages", l)
	}
	if o.VADSilenceSecs > 0 {
		q.Set("vad_silence_threshold_secs", strconv.FormatFloat(o.VADSilenceSecs, 'f', 2, 64))
	}
	if o.VADThreshold > 0 {
		q.Set("vad_threshold", strconv.FormatFloat(o.VADThreshold, 'f', 2, 64))
	}
	if o.MinSpeechMS > 0 {
		q.Set("min_speech_duration_ms", strconv.Itoa(o.MinSpeechMS))
	}
	if o.MinSilenceMS > 0 {
		q.Set("min_silence_duration_ms", strconv.Itoa(o.MinSilenceMS))
	}
	if o.IncludeTimestamps {
		q.Set("include_timestamps", "true")
	}
	if o.IncludeLanguageDetection {
		q.Set("include_language_detection", "true")
	}
	for _, k := range o.Keyterms {
		if k = strings.TrimSpace(k); k != "" {
			q.Add("keyterms", k)
		}
	}

	dctx, dcancel := context.WithTimeout(ctx, 10*time.Second)
	conn, err := c.dial(dctx, "/v1/speech-to-text/realtime", q)
	dcancel()
	if err != nil {
		return nil, err
	}
	sctx, cancel := context.WithCancel(ctx)
	s := &STTSession{conn: conn, ctx: sctx, cancel: cancel, opts: o, format: f, events: make(chan STTEvent, 256)}
	started := make(chan error, 1)
	go s.readLoop(started)
	select {
	case err := <-started:
		if err != nil {
			s.Close()
			return nil, err
		}
	case <-time.After(8 * time.Second):
		s.Close()
		return nil, errors.New("elevenlabs stt: no session_started within 8s")
	case <-ctx.Done():
		s.Close()
		return nil, ctx.Err()
	}
	return s, nil
}

func (s *STTSession) readLoop(started chan<- error) {
	defer close(s.events)
	first := true
	signal := func(err error) {
		if first {
			first = false
			started <- err
		}
	}
	for {
		_, data, err := s.conn.Read(s.ctx)
		if err != nil {
			signal(fmt.Errorf("elevenlabs stt: %w", err))
			ev := STTEvent{Type: EventClosed, At: time.Now()}
			if s.ctx.Err() == nil && websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				ev.Err = err
			}
			s.emit(ev)
			return
		}
		var m sttMessage
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		now := time.Now()
		switch m.MessageType {
		case "session_started":
			s.ID = m.SessionID
			signal(nil)
			s.emit(STTEvent{Type: EventSessionStarted, At: now})
		case "partial_transcript":
			s.emit(STTEvent{Type: EventPartial, Text: m.Text, At: now})
		case "committed_transcript":
			s.emit(STTEvent{Type: EventCommitted, Text: m.Text, At: now})
		case "committed_transcript_with_timestamps":
			ev := STTEvent{Type: EventCommittedTimestamps, Text: m.Text, Words: m.Words, At: now}
			if m.LanguageCode != nil {
				ev.Language = *m.LanguageCode
			}
			s.emit(ev)
		case "committed_transcript_entities":
		case "warning":
			s.emit(STTEvent{Type: EventWarning, Text: m.Warning, At: now})
		default:
			if m.Error != "" {
				e := fmt.Errorf("elevenlabs stt %s: %s", m.MessageType, m.Error)
				signal(e)
				s.emit(STTEvent{Type: EventError, Code: m.MessageType, Err: e, At: now})
			}
		}
	}
}

func (s *STTSession) emit(ev STTEvent) {
	select {
	case s.events <- ev:
	case <-s.ctx.Done():
	}
}

// Send streams one chunk of audio in the session's format. commit=true ends
// the current segment immediately (push-to-talk release).
func (s *STTSession) Send(chunk []byte, commit bool) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	m := sttChunk{
		MessageType: "input_audio_chunk",
		Audio:       base64.StdEncoding.EncodeToString(chunk),
		Commit:      commit,
		SampleRate:  s.format.Rate,
	}
	if !s.sentFirst {
		m.PreviousText = s.opts.PreviousText
		s.sentFirst = true
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	s.clock.add(s.format.Duration(len(chunk)), time.Now())
	return s.conn.Write(s.ctx, websocket.MessageText, b)
}

// Commit ends the current segment now (sends 10 ms of silence with commit=true).
func (s *STTSession) Commit() error {
	return s.Send(s.format.Silence(10*time.Millisecond), true)
}

// Events delivers transcripts until the session closes.
func (s *STTSession) Events() <-chan STTEvent { return s.events }

// Format is the input audio format of the session.
func (s *STTSession) Format() audio.Format { return s.format }

// SpeechEnd maps the end of the last recognised word to wall-clock time, i.e.
// the moment the caller actually stopped speaking. This is the start of the
// "end of speech -> first audio" latency we optimise.
func (s *STTSession) SpeechEnd(words []Word) (time.Time, bool) {
	last := -1.0
	for _, w := range words {
		if (w.Type == "" || w.Type == "word") && w.End > last {
			last = w.End
		}
	}
	if last < 0 {
		return time.Time{}, false
	}
	return s.clock.wallAt(time.Duration(last * float64(time.Second)))
}

// AudioSent is the total duration of audio streamed so far.
func (s *STTSession) AudioSent() time.Duration { return s.clock.sent() }

// Close ends the session immediately.
func (s *STTSession) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		s.conn.CloseNow()
	})
	return nil
}

// audioClock remembers when each chunk was sent so that stream audio time
// (used by word timestamps) can be mapped back to wall-clock time.
type audioClock struct {
	mu    sync.Mutex
	total time.Duration
	marks []clockMark
}

type clockMark struct {
	end  time.Duration // stream audio time at the end of the chunk
	wall time.Time     // when the chunk was sent
}

func (c *audioClock) add(d time.Duration, wall time.Time) {
	c.mu.Lock()
	c.total += d
	c.marks = append(c.marks, clockMark{end: c.total, wall: wall})
	if len(c.marks) > 8192 {
		c.marks = append([]clockMark(nil), c.marks[len(c.marks)-4096:]...)
	}
	c.mu.Unlock()
}

func (c *audioClock) sent() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total
}

// wallAt: a sample at stream time t travelled in the first chunk whose end is
// >= t; live audio is sent as soon as it is captured, so it was captured about
// (chunkEnd - t) before that chunk was sent.
func (c *audioClock) wallAt(t time.Duration) (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.marks) == 0 {
		return time.Time{}, false
	}
	i := sort.Search(len(c.marks), func(i int) bool { return c.marks[i].end >= t })
	if i == len(c.marks) {
		i--
	}
	m := c.marks[i]
	return m.wall.Add(-(m.end - t)), true
}
