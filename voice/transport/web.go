package transport

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// WebHandler serves the browser voice WebSocket (GET /ws/voice).
//
// Protocol: the client first sends {"type":"start","mode":"vad"|"ptt","greeting":bool,
// "lang":"ru"|"kk"|"","session_id":""}, then binary PCM16 LE 16 kHz mono frames,
// plus control messages {"type":"commit"|"text"|"interrupt"|"client.metrics"|"stop"}.
// The server answers with JSON events (stt.partial, stt.final, router.decision,
// response.delta, tts.start, tts.clear, voice.metrics, ...) and binary PCM16
// audio at the announced sample rate. See voice/README.md.
type WebHandler struct {
	Start          Starter
	AllowedOrigins []string // "*" allows any origin
	Logger         *slog.Logger
}

type webStart struct {
	Type      string `json:"type"`
	Mode      string `json:"mode"`
	Greeting  bool   `json:"greeting"`
	Lang      string `json:"lang"`
	SessionID string `json:"session_id"`
}

func (h *WebHandler) log() *slog.Logger {
	if h.Logger != nil {
		return h.Logger
	}
	return slog.Default()
}

func (h *WebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	opts := &websocket.AcceptOptions{}
	if allowsAny(h.AllowedOrigins) {
		opts.InsecureSkipVerify = true
	} else {
		opts.OriginPatterns = originHosts(h.AllowedOrigins)
	}
	c, err := websocket.Accept(w, r, opts)
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(1 << 20)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rctx, rcancel := context.WithTimeout(ctx, 15*time.Second)
	typ, data, err := c.Read(rctx)
	rcancel()
	var st webStart
	if err != nil || typ != websocket.MessageText || json.Unmarshal(data, &st) != nil || st.Type != "start" {
		c.Close(websocket.StatusPolicyViolation, `first message must be {"type":"start"}`)
		return
	}
	sid := st.SessionID
	if sid == "" {
		sid = NewID("web")
	}
	sink := newWebSink(ctx, c)
	defer sink.close()
	call := h.Start(ctx, CallInfo{
		SessionID: sid, Channel: "web", InputFormat: "pcm_16000", Manual: st.Mode == "ptt",
		Greeting: st.Greeting, Lang: st.Lang,
		Meta: map[string]any{"user_agent": r.UserAgent(), "remote": r.RemoteAddr, "mode": st.Mode},
	}, sink)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := call.Run(); err != nil {
			h.log().Warn("web call ended with error", "session", sid, "err", err)
		}
		cancel()
	}()

loop:
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			break
		}
		if typ == websocket.MessageBinary {
			call.PushAudio(data)
			continue
		}
		var m map[string]any
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m["type"] {
		case "commit":
			call.Commit()
		case "text":
			if s, _ := m["text"].(string); s != "" {
				call.SubmitText(s)
			}
		case "interrupt":
			call.Interrupt()
		case "client.metrics":
			if cm, ok := call.(interface{ ClientMetrics(map[string]any) }); ok {
				cm.ClientMetrics(m)
			}
		case "ping":
			sink.Send(map[string]any{"type": "pong", "t": m["t"]})
		case "stop", "hangup":
			break loop
		}
	}
	call.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
	c.Close(websocket.StatusNormalClosure, "bye")
}

type wsMsg struct {
	typ   websocket.MessageType
	data  []byte
	gen   int64
	audio bool
}

// webSink writes events and audio from one goroutine, in order. Clear bumps a
// generation counter so audio queued before a barge-in is dropped unsent.
type webSink struct {
	c    *websocket.Conn
	ctx  context.Context
	stop context.CancelFunc
	out  chan wsMsg
	gen  atomic.Int64
	done chan struct{}
}

func newWebSink(parent context.Context, c *websocket.Conn) *webSink {
	ctx, cancel := context.WithCancel(parent)
	s := &webSink{c: c, ctx: ctx, stop: cancel, out: make(chan wsMsg, 1024), done: make(chan struct{})}
	go s.writer()
	return s
}

func (s *webSink) writer() {
	defer close(s.done)
	for {
		select {
		case <-s.ctx.Done():
			return
		case m := <-s.out:
			if m.audio && m.gen != s.gen.Load() {
				continue
			}
			wctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
			err := s.c.Write(wctx, m.typ, m.data)
			cancel()
			if err != nil {
				s.stop()
				return
			}
		}
	}
}

func (s *webSink) enqueue(m wsMsg) {
	select {
	case s.out <- m:
	case <-s.ctx.Done():
	case <-time.After(2 * time.Second):
	}
}

func (s *webSink) OutputFormat() string { return "pcm_16000" }

func (s *webSink) Play(b []byte) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	s.enqueue(wsMsg{typ: websocket.MessageBinary, data: b, gen: s.gen.Load(), audio: true})
	return nil
}

func (s *webSink) Clear() { s.gen.Add(1) }

func (s *webSink) Send(ev map[string]any) {
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	s.enqueue(wsMsg{typ: websocket.MessageText, data: b})
}

func (s *webSink) Hangup() {
	s.Send(map[string]any{"type": "session.end"})
	go func() {
		time.Sleep(300 * time.Millisecond)
		s.c.Close(websocket.StatusNormalClosure, "hangup")
	}()
}

func (s *webSink) close() {
	s.stop()
	<-s.done
}

func allowsAny(origins []string) bool {
	if len(origins) == 0 {
		return true
	}
	for _, o := range origins {
		if o == "*" {
			return true
		}
	}
	return false
}

// originHosts turns "https://app.example.com" into "app.example.com" for the
// websocket origin check (which matches hosts).
func originHosts(origins []string) []string {
	out := make([]string, 0, len(origins))
	for _, o := range origins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			out = append(out, u.Host)
		} else if o != "" {
			out = append(out, o)
		}
	}
	return out
}
