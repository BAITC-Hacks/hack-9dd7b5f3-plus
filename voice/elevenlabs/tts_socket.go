package elevenlabs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// SocketOptions configures a streaming TTS WebSocket.
type SocketOptions struct {
	VoiceID           string
	Model             string // eleven_flash_v2_5 -> stream-input; eleven_v3* -> text-to-dialogue
	OutputFormat      string // pcm_16000 | pcm_8000 | ulaw_8000 ...
	Language          string // stream-input only (e.g. "ru")
	AutoMode          bool   // stream-input: generate as soon as a phrase arrives (lower latency)
	InactivityTimeout int    // stream-input seconds, default 60 (max 180)
	Settings          *VoiceSettings
	ChunkSchedule     []int
	Normalization     string
}

// SocketInfo describes a socket for logs and latency metrics.
type SocketInfo struct {
	Model      string
	Voice      string
	Format     string
	Opened     time.Time
	FirstText  time.Time
	FirstAudio time.Time
}

// Socket streams text in (LLM deltas) and audio out as soon as the model has
// it. One Socket speaks one reply; open the next one early to hide the handshake.
type Socket struct {
	dialogue bool
	voice    string
	info     SocketInfo

	conn      *websocket.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	wmu       sync.Mutex
	audio     chan []byte
	mu        sync.Mutex
	err       error
	finished  bool
	closeOnce sync.Once
}

// OpenSocket connects and sends the init message. For eleven_v3* models it
// uses the text-to-dialogue socket (the only realtime path with Kazakh);
// otherwise the classic stream-input socket.
func (c *Client) OpenSocket(ctx context.Context, o SocketOptions) (*Socket, error) {
	if o.Model == "" {
		o.Model = "eleven_flash_v2_5"
	}
	if o.OutputFormat == "" {
		o.OutputFormat = "pcm_16000"
	}
	dialogue := strings.HasPrefix(o.Model, "eleven_v3")
	q := url.Values{}
	q.Set("model_id", o.Model)
	q.Set("output_format", o.OutputFormat)
	path := "/v1/text-to-dialogue/stream-input"
	if !dialogue {
		path = "/v1/text-to-speech/" + url.PathEscape(o.VoiceID) + "/stream-input"
		if o.Language != "" {
			q.Set("language_code", o.Language)
		}
		if o.AutoMode {
			q.Set("auto_mode", "true")
		}
		it := o.InactivityTimeout
		if it <= 0 {
			it = 60
		}
		q.Set("inactivity_timeout", strconv.Itoa(it))
		if o.Normalization != "" {
			q.Set("apply_text_normalization", o.Normalization)
		}
	}
	dctx, dcancel := context.WithTimeout(ctx, 10*time.Second)
	conn, err := c.dial(dctx, path, q)
	dcancel()
	if err != nil {
		return nil, err
	}
	sctx, cancel := context.WithCancel(ctx)
	s := &Socket{
		dialogue: dialogue,
		voice:    o.VoiceID,
		info:     SocketInfo{Model: o.Model, Voice: o.VoiceID, Format: o.OutputFormat, Opened: time.Now()},
		conn:     conn,
		ctx:      sctx,
		cancel:   cancel,
		audio:    make(chan []byte, 128),
	}
	var init any
	if dialogue {
		init = map[string]any{"voices": []string{o.VoiceID}}
	} else {
		m := map[string]any{"text": " "}
		if o.Settings != nil {
			m["voice_settings"] = o.Settings
		}
		if len(o.ChunkSchedule) > 0 {
			m["generation_config"] = map[string]any{"chunk_length_schedule": o.ChunkSchedule}
		}
		init = m
	}
	if err := s.writeJSON(init); err != nil {
		s.Close()
		return nil, err
	}
	go s.readLoop()
	return s, nil
}

func (s *Socket) writeJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.wmu.Lock()
	defer s.wmu.Unlock()
	return s.conn.Write(s.ctx, websocket.MessageText, b)
}

// Send queues text for synthesis. flush forces generation of everything
// buffered so far (use it at sentence ends and for the first phrase).
func (s *Socket) Send(text string, flush bool) error {
	if err := s.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(text) != "" {
		s.mu.Lock()
		if s.info.FirstText.IsZero() {
			s.info.FirstText = time.Now()
		}
		s.mu.Unlock()
	}
	if s.dialogue {
		if strings.TrimSpace(text) != "" {
			in := map[string]any{"text": text, "voice_id": s.voice}
			if err := s.writeJSON(map[string]any{"inputs": []any{in}}); err != nil {
				return err
			}
		}
		if flush {
			return s.writeJSON(map[string]any{"flush": true})
		}
		return nil
	}
	if strings.TrimSpace(text) == "" && !flush {
		return nil
	}
	if !strings.HasSuffix(text, " ") {
		text += " "
	}
	m := map[string]any{"text": text}
	if flush {
		m["flush"] = true
	}
	return s.writeJSON(m)
}

// Finish tells the server no more text is coming; remaining audio is still
// delivered and then Audio is closed.
func (s *Socket) Finish() error {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return nil
	}
	s.finished = true
	s.mu.Unlock()
	if s.dialogue {
		if err := s.writeJSON(map[string]any{"flush": true}); err != nil {
			return err
		}
		return s.writeJSON(map[string]any{"close_socket": true})
	}
	return s.writeJSON(map[string]any{"text": ""})
}

// KeepAlive resets the server inactivity timer without producing audio.
func (s *Socket) KeepAlive() error {
	if s.dialogue {
		return s.writeJSON(map[string]any{"keep_alive": true})
	}
	return s.writeJSON(map[string]any{"text": " "})
}

type socketMessage struct {
	Audio      *string `json:"audio"`
	IsFinal    *bool   `json:"isFinal"`
	IsFinalAlt *bool   `json:"is_final"`
	Error      string  `json:"error"`
	Message    string  `json:"message"`
}

func (s *Socket) readLoop() {
	defer close(s.audio)
	for {
		_, data, err := s.conn.Read(s.ctx)
		if err != nil {
			if s.ctx.Err() == nil {
				switch websocket.CloseStatus(err) {
				case websocket.StatusNormalClosure, websocket.StatusGoingAway:
					if !s.isFinished() {
						s.setErr(fmt.Errorf("elevenlabs tts: socket closed before finish: %w", err))
					}
				default:
					s.setErr(fmt.Errorf("elevenlabs tts: %w", err))
				}
			}
			return
		}
		var m socketMessage
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		if m.Error != "" {
			s.setErr(errors.New("elevenlabs tts: " + m.Error + " " + m.Message))
			return
		}
		if m.Audio != nil && *m.Audio != "" {
			b, err := base64.StdEncoding.DecodeString(*m.Audio)
			if err == nil && len(b) > 0 {
				s.mu.Lock()
				if s.info.FirstAudio.IsZero() {
					s.info.FirstAudio = time.Now()
				}
				s.mu.Unlock()
				select {
				case s.audio <- b:
				case <-s.ctx.Done():
					return
				}
			}
		}
		final := (m.IsFinal != nil && *m.IsFinal) || (m.IsFinalAlt != nil && *m.IsFinalAlt)
		if final && s.isFinished() {
			return
		}
	}
}

func (s *Socket) isFinished() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finished
}

func (s *Socket) setErr(err error) {
	s.mu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.mu.Unlock()
}

// Audio yields raw audio chunks in the requested output format. It is closed
// when the reply is complete, the socket fails, or Close is called.
func (s *Socket) Audio() <-chan []byte { return s.audio }

// Err is the first error seen (nil on a clean finish).
func (s *Socket) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Info returns model/voice/format and first-text/first-audio timestamps.
func (s *Socket) Info() SocketInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.info
}

// Close drops the connection immediately (used for barge-in).
func (s *Socket) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		s.conn.CloseNow()
	})
	return nil
}
