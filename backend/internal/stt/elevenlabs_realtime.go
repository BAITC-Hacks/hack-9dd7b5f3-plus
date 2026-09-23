package stt

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// ElevenLabsRealtime streams PCM to Scribe v2 Realtime over WebSocket and
// gets partial transcripts back while the client is still speaking. Batch
// Transcribe is delegated to the HTTP endpoint (used as a fallback).
type ElevenLabsRealtime struct {
	*ElevenLabs
	WSBase             string
	SecondaryLanguages string
	SampleRate         int
}

func NewElevenLabsRealtime(baseURL, apiKey, language, secondary string, timeout time.Duration) *ElevenLabsRealtime {
	ws := strings.Replace(strings.Replace(strings.TrimRight(baseURL, "/"), "https://", "wss://", 1), "http://", "ws://", 1)
	return &ElevenLabsRealtime{ElevenLabs: NewElevenLabs(baseURL, apiKey, "scribe_v2", language, timeout), WSBase: ws, SecondaryLanguages: secondary, SampleRate: 16000}
}

func (e *ElevenLabsRealtime) Name() string { return "elevenlabs_realtime" }

type rtSession struct {
	e         *ElevenLabsRealtime
	conn      *websocket.Conn
	onPartial func(string)
	mu        sync.Mutex
	committed chan *Result
	errs      chan error
	closed    bool
	lastText  string
}

// NewSession opens the realtime WebSocket.
func (e *ElevenLabsRealtime) NewSession(ctx context.Context, onPartial func(string)) (Session, error) {
	q := url.Values{}
	q.Set("model_id", "scribe_v2_realtime")
	q.Set("audio_format", fmt.Sprintf("pcm_%d", e.SampleRate))
	q.Set("commit_strategy", "manual")
	q.Set("include_language_detection", "true")
	if e.Language != "" {
		q.Set("language_code", e.Language)
	}
	for _, l := range strings.Split(e.SecondaryLanguages, ",") {
		if l = strings.TrimSpace(l); l != "" {
			q.Add("secondary_languages", l)
		}
	}
	u := e.WSBase + "/v1/speech-to-text/realtime?" + q.Encode()
	dialCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(dialCtx, u, &websocket.DialOptions{HTTPHeader: http.Header{"xi-api-key": []string{e.APIKey}}})
	if err != nil {
		status := ""
		if resp != nil {
			status = fmt.Sprintf(" (http %d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("elevenlabs realtime dial: %w%s", err, status)
	}
	conn.SetReadLimit(4 << 20)
	s := &rtSession{e: e, conn: conn, onPartial: onPartial, committed: make(chan *Result, 4), errs: make(chan error, 4)}
	go s.readLoop()
	return s, nil
}

type rtMessage struct {
	MessageType  string `json:"message_type"`
	Text         string `json:"text"`
	LanguageCode string `json:"language_code"`
	Message      string `json:"message"`
	Error        string `json:"error"`
	Detail       any    `json:"detail"`
}

func (s *rtSession) readLoop() {
	ctx := context.Background()
	for {
		_, data, err := s.conn.Read(ctx)
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if !closed {
				select {
				case s.errs <- fmt.Errorf("elevenlabs realtime read: %w", err):
				default:
				}
			}
			return
		}
		var m rtMessage
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.MessageType {
		case "partial_transcript":
			s.mu.Lock()
			s.lastText = m.Text
			s.mu.Unlock()
			if s.onPartial != nil && m.Text != "" {
				s.onPartial(m.Text)
			}
		case "committed_transcript", "committed_transcript_with_timestamps":
			select {
			case s.committed <- &Result{Text: strings.TrimSpace(m.Text), Language: m.LanguageCode, Provider: "elevenlabs_realtime", Model: "scribe_v2_realtime"}:
			default:
			}
		case "session_started", "warning", "":
			// ignore
		case "committed_transcript_entities":
		default:
			if strings.Contains(m.MessageType, "error") || strings.Contains(m.MessageType, "exceeded") || strings.Contains(m.MessageType, "limited") || strings.Contains(m.MessageType, "insufficient") {
				msg := m.Message
				if msg == "" {
					msg = m.Error
				}
				if msg == "" {
					msg = string(data)
				}
				select {
				case s.errs <- fmt.Errorf("elevenlabs realtime %s: %s", m.MessageType, msg):
				default:
				}
			}
		}
	}
}

func (s *rtSession) send(pcm []byte, commit bool) error {
	msg := map[string]any{
		"message_type":  "input_audio_chunk",
		"audio_base_64": base64.StdEncoding.EncodeToString(pcm),
		"commit":        commit,
		"sample_rate":   s.e.SampleRate,
	}
	b, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.conn.Write(ctx, websocket.MessageText, b)
}

func (s *rtSession) Send(pcm []byte) error {
	if len(pcm) == 0 {
		return nil
	}
	return s.send(pcm, false)
}

// Commit sends a final (silent) chunk with commit=true and waits for the
// committed transcript.
func (s *rtSession) Commit(ctx context.Context) (*Result, error) {
	// drain stale results
	for {
		select {
		case <-s.committed:
			continue
		default:
		}
		break
	}
	silence := make([]byte, s.e.SampleRate/10*2) // 100 ms of silence
	if err := s.send(silence, true); err != nil {
		return nil, err
	}
	select {
	case r := <-s.committed:
		return r, nil
	case err := <-s.errs:
		return nil, err
	case <-ctx.Done():
		s.mu.Lock()
		last := s.lastText
		s.mu.Unlock()
		if last != "" {
			return &Result{Text: last, Provider: "elevenlabs_realtime", Model: "scribe_v2_realtime (partial)"}, nil
		}
		return nil, errors.New("elevenlabs realtime: commit timed out")
	}
}

func (s *rtSession) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return s.conn.Close(websocket.StatusNormalClosure, "bye")
}
