package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"hackathon/backend/internal/dialog"
	"hackathon/backend/internal/events"
)

// wsClientMessage is what the browser sends as JSON (binary frames = audio).
type wsClientMessage struct {
	Type   string `json:"type"`
	Text   string `json:"text,omitempty"`
	Source string `json:"source,omitempty"`
	T      int64  `json:"t,omitempty"`
	Voice  *bool  `json:"voice,omitempty"`
	Hint   string `json:"transcript_hint,omitempty"`
}

// ws is the voice channel: audio in (binary PCM16 16 kHz), events out (JSON),
// TTS audio out (binary PCM16 at the configured rate).
func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	c.SetReadLimit(8 << 20)
	ctx := r.Context()
	id := r.URL.Query().Get("session_id")
	sess := s.e.Session(id)
	if sess == nil {
		sess = s.e.NewSession("web")
	}
	var wmu sync.Mutex
	writeJSON := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		wmu.Lock()
		defer wmu.Unlock()
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		c.Write(wctx, websocket.MessageText, b)
	}
	writeBin := func(b []byte) {
		wmu.Lock()
		defer wmu.Unlock()
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		c.Write(wctx, websocket.MessageBinary, b)
	}
	sess.SetAudioSink(writeBin)
	defer sess.SetAudioSink(nil)

	cfg := s.cfg.Public()
	cfg["router"] = s.e.RouterName()
	writeJSON(events.Event{Type: "session", SessionID: sess.ID, T: time.Now().UnixMilli(), Data: map[string]any{
		"session_id": sess.ID, "config": cfg, "audio_out": map[string]any{"sample_rate": s.e.TTSSampleRate(), "format": "pcm16"}, "state": sess.View()}})

	ch, cancel := s.e.Bus().Subscribe(sess.ID, 512)
	defer cancel()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-ch:
				writeJSON(ev)
			}
		}
	}()

	voice := true
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			break
		}
		if typ == websocket.MessageBinary {
			s.e.AudioChunk(sess, data)
			continue
		}
		var msg wsClientMessage
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		switch msg.Type {
		case "config":
			if msg.Voice != nil {
				voice = *msg.Voice
			}
		case "speech_start":
			sess.Cancel() // barge-in: stop the previous reply
			s.e.StartUtterance(sess)
		case "speech_end":
			t0 := time.Now()
			hint := msg.Hint
			go func() {
				tctx, tcancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer tcancel()
				_, err := s.e.EndUtterance(tctx, sess, dialog.TurnOptions{Source: "voice", Voice: voice, T0: t0, ClientT0: msg.T, TranscriptHint: hint})
				if err != nil && !errors.Is(err, context.Canceled) {
					writeJSON(events.Event{Type: "turn_error", SessionID: sess.ID, T: time.Now().UnixMilli(), Data: map[string]any{"error": err.Error(), "empty": errors.Is(err, dialog.ErrEmptyTranscript)}})
				}
			}()
		case "text":
			text := msg.Text
			source := msg.Source
			if source == "" {
				source = "text"
			}
			t0 := time.Now()
			if msg.T > 0 && source == "browser_stt" {
				// the browser measured end-of-speech itself; keep server t0
			}
			go func() {
				tctx, tcancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer tcancel()
				_, err := s.e.RunTurn(tctx, sess, text, dialog.TurnOptions{Source: source, Voice: voice, T0: t0, ClientT0: msg.T})
				if err != nil && !errors.Is(err, context.Canceled) {
					writeJSON(events.Event{Type: "turn_error", SessionID: sess.ID, T: time.Now().UnixMilli(), Data: map[string]any{"error": err.Error()}})
				}
			}()
		case "cancel":
			sess.Cancel()
		case "ping":
			writeJSON(map[string]any{"type": "pong", "t": time.Now().UnixMilli()})
		}
	}
	s.e.CloseSession(sess)
	c.Close(websocket.StatusNormalClosure, "")
}
