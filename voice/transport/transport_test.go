package transport

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type fakeCall struct {
	mu     sync.Mutex
	frames [][]byte
	closed chan struct{}
	once   sync.Once
}

func newFakeCall() *fakeCall { return &fakeCall{closed: make(chan struct{})} }

func (f *fakeCall) PushAudio(b []byte) {
	f.mu.Lock()
	f.frames = append(f.frames, append([]byte(nil), b...))
	f.mu.Unlock()
}
func (f *fakeCall) Commit()           {}
func (f *fakeCall) SubmitText(string) {}
func (f *fakeCall) Interrupt()        {}
func (f *fakeCall) Close()            { f.once.Do(func() { close(f.closed) }) }
func (f *fakeCall) Run() error        { <-f.closed; return nil }
func (f *fakeCall) count() int        { f.mu.Lock(); defer f.mu.Unlock(); return len(f.frames) }

type captured struct {
	mu   sync.Mutex
	info CallInfo
	sink Sink
	call *fakeCall
	got  chan struct{}
}

func (c *captured) starter() Starter {
	c.got = make(chan struct{})
	return func(ctx context.Context, info CallInfo, sink Sink) Call {
		c.mu.Lock()
		c.info, c.sink, c.call = info, sink, newFakeCall()
		call := c.call
		c.mu.Unlock()
		close(c.got)
		return call
	}
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/asterisk/call?uuid=ABC&caller=87010000001&called=7172", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code %d", rec.Code)
	}
	caller, called, ok := r.Take("abc")
	if !ok || caller != "87010000001" || called != "7172" {
		t.Fatalf("take %q %q %v", caller, called, ok)
	}
	if _, _, ok := r.Take("abc"); ok {
		t.Fatal("take must remove the entry")
	}
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/asterisk/call", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing uuid: %d", rec.Code)
	}
	for in, want := range map[string]string{"87010000001": "+77010000001", "+7 (701) 000-00-01": "+77010000001", "77010000001": "+77010000001", "": ""} {
		if got := NormalizeCaller(in); got != want {
			t.Fatalf("NormalizeCaller(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAudioSocket(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	uuidBytes := []byte{0x40, 0x32, 0x5e, 0xc2, 0x5e, 0xfd, 0x4b, 0xd3, 0x80, 0x5f, 0x53, 0x57, 0x6e, 0x58, 0x1d, 0x13}
	reg.Put("40325ec2-5efd-4bd3-805f-53576e581d13", "87010000001", "7172")
	var cap captured
	srv := &AudioSocketServer{Start: cap.starter(), Registry: reg}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Serve(ctx, ln)

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := writeMessage(conn, asUUID, uuidBytes); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_ = writeMessage(conn, asAudio, make([]byte, frameBytes))
	}
	<-cap.got
	waitUntil(t, "3 frames", func() bool { return cap.call.count() == 3 })
	if cap.info.SessionID != "ast-40325ec2-5efd-4bd3-805f-53576e581d13" || cap.info.CallerID != "+77010000001" || cap.info.InputFormat != "pcm_8000" {
		t.Fatalf("info %+v", cap.info)
	}
	// 1000 bytes -> 4 frames of 320 (last zero-padded), paced ~20 ms apart
	start := time.Now()
	if err := cap.sink.Play(make([]byte, 1000)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		kind, payload, err := readMessage(conn)
		if err != nil || kind != asAudio || len(payload) != frameBytes {
			t.Fatalf("frame %d: kind=%x len=%d err=%v", i, kind, len(payload), err)
		}
	}
	if el := time.Since(start); el < 30*time.Millisecond {
		t.Fatalf("frames not paced: 4 frames in %v", el)
	}
	// barge-in: Clear drops the rest of a long buffer
	_ = cap.sink.Play(make([]byte, 16000)) // 1 s
	time.Sleep(50 * time.Millisecond)
	cap.sink.Clear()
	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	n := 0
	for {
		if _, _, err := readMessage(conn); err != nil {
			break
		}
		n++
	}
	if n > 10 {
		t.Fatalf("Clear did not stop playback: %d frames after clear", n)
	}
	_ = conn.SetReadDeadline(time.Time{})
	_ = writeMessage(conn, asHangup, nil)
	select {
	case <-cap.call.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("call not closed on hangup")
	}
}

func TestTwilio(t *testing.T) {
	var cap captured
	h := &TwilioHandler{Start: cap.starter(), PublicURL: "https://voice.example.com"}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/twilio/voice", strings.NewReader("From=%2B77010000001&CallSid=CA1%26x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.TwiML(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, `url="wss://voice.example.com/twilio/stream"`) || !strings.Contains(body, `value="+77010000001"`) || !strings.Contains(body, "CA1&amp;x") {
		t.Fatalf("twiml %s", body)
	}

	srv := httptest.NewServer(http.HandlerFunc(h.Stream))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	send := func(v any) { b, _ := json.Marshal(v); _ = c.Write(ctx, websocket.MessageText, b) }
	send(map[string]any{"event": "connected"})
	send(map[string]any{"event": "start", "streamSid": "MZ1", "start": map[string]any{"streamSid": "MZ1", "callSid": "CA1", "customParameters": map[string]string{"from": "+77010000001"}}})
	for i := 0; i < 2; i++ {
		send(map[string]any{"event": "media", "streamSid": "MZ1", "media": map[string]any{"track": "inbound", "payload": base64.StdEncoding.EncodeToString([]byte{0xff, 0xff})}})
	}
	<-cap.got
	waitUntil(t, "2 frames", func() bool { return cap.call.count() == 2 })
	if cap.info.SessionID != "tw-CA1" || cap.info.InputFormat != "ulaw_8000" || cap.info.CallerID != "+77010000001" {
		t.Fatalf("info %+v", cap.info)
	}
	_ = cap.sink.Play([]byte{1, 2, 3})
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	media, _ := m["media"].(map[string]any)
	if m["event"] != "media" || m["streamSid"] != "MZ1" || media["payload"] != base64.StdEncoding.EncodeToString([]byte{1, 2, 3}) {
		t.Fatalf("media msg %s", data)
	}
	cap.sink.Clear()
	_, data, _ = c.Read(ctx)
	if !strings.Contains(string(data), `"clear"`) {
		t.Fatalf("clear msg %s", data)
	}
	send(map[string]any{"event": "stop"})
	select {
	case <-cap.call.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("call not closed on stop")
	}
}
