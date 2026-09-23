package transport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// TwilioHandler connects Twilio phone calls through Media Streams.
// Configure the number's voice webhook (POST) to <public>/twilio/voice.
type TwilioHandler struct {
	Start     Starter
	PublicURL string // https://voice.example.com; derived from the request when empty
	Greeting  bool
	Logger    *slog.Logger
}

func (h *TwilioHandler) log() *slog.Logger {
	if h.Logger != nil {
		return h.Logger
	}
	return slog.Default()
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return strings.ReplaceAll(b.String(), `"`, "&quot;")
}

// TwiML answers the call with <Connect><Stream> to our WebSocket.
func (h *TwilioHandler) TwiML(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	base := strings.TrimRight(h.PublicURL, "/")
	if base == "" {
		proto := r.Header.Get("X-Forwarded-Proto")
		if proto == "" {
			proto = "http"
			if r.TLS != nil {
				proto = "https"
			}
		}
		host := r.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = r.Host
		}
		base = proto + "://" + host
	}
	ws := "wss://" + strings.TrimPrefix(base, "https://")
	if strings.HasPrefix(base, "http://") {
		ws = "ws://" + strings.TrimPrefix(base, "http://")
	}
	w.Header().Set("Content-Type", "text/xml")
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Response><Connect><Stream url="` + xmlEscape(ws+"/twilio/stream") + `">` +
		`<Parameter name="from" value="` + xmlEscape(r.FormValue("From")) + `"/>` +
		`<Parameter name="callSid" value="` + xmlEscape(r.FormValue("CallSid")) + `"/>` +
		`</Stream></Connect></Response>`))
}

type twilioMsg struct {
	Event     string `json:"event"`
	StreamSid string `json:"streamSid"`
	Start     struct {
		StreamSid        string            `json:"streamSid"`
		CallSid          string            `json:"callSid"`
		CustomParameters map[string]string `json:"customParameters"`
	} `json:"start"`
	Media struct {
		Track   string `json:"track"`
		Payload string `json:"payload"`
	} `json:"media"`
}

// Stream is the Media Streams WebSocket (mu-law 8 kHz both ways).
func (h *TwilioHandler) Stream(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(1 << 20)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var call Call
	var sink *twilioSink
	done := make(chan struct{})
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			break
		}
		var m twilioMsg
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.Event {
		case "start":
			if call != nil {
				continue
			}
			sid := m.Start.StreamSid
			if sid == "" {
				sid = m.StreamSid
			}
			sink = newTwilioSink(ctx, c, sid)
			from := m.Start.CustomParameters["from"]
			callSid := m.Start.CustomParameters["callSid"]
			if callSid == "" {
				callSid = m.Start.CallSid
			}
			call = h.Start(ctx, CallInfo{
				SessionID: "tw-" + callSid, Channel: "twilio", CallerID: NormalizeCaller(from), InputFormat: "ulaw_8000",
				Greeting: h.Greeting, Meta: map[string]any{"call_sid": callSid, "stream_sid": sid},
			}, sink)
			go func(call Call) {
				defer close(done)
				if err := call.Run(); err != nil {
					h.log().Warn("twilio call ended with error", "call", callSid, "err", err)
				}
				cancel()
			}(call)
		case "media":
			if call == nil || (m.Media.Track != "" && m.Media.Track != "inbound") {
				continue
			}
			if b, err := base64.StdEncoding.DecodeString(m.Media.Payload); err == nil {
				call.PushAudio(b)
			}
		case "stop":
			goto end
		}
	}
end:
	if call != nil {
		call.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}
	if sink != nil {
		sink.close()
	}
	c.Close(websocket.StatusNormalClosure, "bye")
}

// twilioSink writes media/clear messages from one goroutine.
type twilioSink struct {
	c    *websocket.Conn
	sid  string
	ctx  context.Context
	stop context.CancelFunc
	out  chan []byte
	done chan struct{}
}

func newTwilioSink(parent context.Context, c *websocket.Conn, sid string) *twilioSink {
	ctx, cancel := context.WithCancel(parent)
	s := &twilioSink{c: c, sid: sid, ctx: ctx, stop: cancel, out: make(chan []byte, 512), done: make(chan struct{})}
	go func() {
		defer close(s.done)
		for {
			select {
			case <-s.ctx.Done():
				return
			case b := <-s.out:
				wctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
				err := s.c.Write(wctx, websocket.MessageText, b)
				cancel()
				if err != nil {
					s.stop()
					return
				}
			}
		}
	}()
	return s
}

func (s *twilioSink) enqueue(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case s.out <- b:
	case <-s.ctx.Done():
	case <-time.After(2 * time.Second):
	}
}

func (s *twilioSink) OutputFormat() string { return "ulaw_8000" }

func (s *twilioSink) Play(b []byte) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	for len(b) > 0 {
		n := len(b)
		if n > 8000 {
			n = 8000
		}
		s.enqueue(map[string]any{"event": "media", "streamSid": s.sid, "media": map[string]string{"payload": base64.StdEncoding.EncodeToString(b[:n])}})
		b = b[n:]
	}
	return nil
}

func (s *twilioSink) Clear() {
	// drop what is still queued locally, then tell Twilio to flush its buffer
	for {
		select {
		case <-s.out:
			continue
		default:
		}
		break
	}
	s.enqueue(map[string]any{"event": "clear", "streamSid": s.sid})
}

func (s *twilioSink) Send(map[string]any) {}

func (s *twilioSink) Hangup() {
	s.stop()
	s.c.Close(websocket.StatusNormalClosure, "hangup")
}

func (s *twilioSink) close() {
	s.stop()
	<-s.done
}
