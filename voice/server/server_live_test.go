package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"hackathon/voice/audio"
	"hackathon/voice/config"
)

// TestLiveWebSocketTurn drives the real gateway like the browser does:
// start -> PCM16 frames of a caller WAV (real-time pace) -> reply events + audio.
// Run: VOICE_LIVE=1 go test ./server/ -run Live -v
func TestLiveWebSocketTurn(t *testing.T) {
	if os.Getenv("VOICE_LIVE") != "1" {
		t.Skip("set VOICE_LIVE=1 to run the live gateway test")
	}
	cfg := config.Load()
	if cfg.ElevenLabsKey == "" {
		t.Skip("no ElevenLabs key")
	}
	cfg.LogDir = t.TempDir()
	cfg.Greeting = false
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	hs := httptest.NewServer(srv.Handler())
	defer hs.Close()

	b, err := os.ReadFile(filepath.Join("..", "demos", "04_ru_caller.wav"))
	if err != nil {
		t.Skip("demos/04_ru_caller.wav missing (run `go run ./cmd/voicedemo tts`)")
	}
	pcm, rate, err := audio.DecodeWAV(b)
	if err != nil || rate != 16000 {
		t.Fatalf("wav: %v rate=%d", err, rate)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(hs.URL, "http")+"/ws/voice", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	c.SetReadLimit(4 << 20)
	start, _ := json.Marshal(map[string]any{"type": "start", "mode": "vad", "greeting": false})
	if err := c.Write(ctx, websocket.MessageText, start); err != nil {
		t.Fatal(err)
	}

	events := make(chan map[string]any, 256)
	audioBytes := make(chan int, 256)
	go func() {
		for {
			typ, data, err := c.Read(ctx)
			if err != nil {
				close(events)
				return
			}
			if typ == websocket.MessageBinary {
				audioBytes <- len(data)
				continue
			}
			var m map[string]any
			if json.Unmarshal(data, &m) == nil {
				events <- m
			}
		}
	}()

	f := audio.MustFormat("pcm_16000")
	go func() {
		t0 := time.Now()
		frames := audio.Frames(pcm, f, 40*time.Millisecond)
		silence := f.Silence(40 * time.Millisecond)
		for i := 0; ctx.Err() == nil; i++ {
			fr := silence
			if i < len(frames) {
				fr = frames[i]
			}
			if c.Write(ctx, websocket.MessageBinary, fr) != nil {
				return
			}
			time.Sleep(time.Until(t0.Add(time.Duration(i+1) * 40 * time.Millisecond)))
		}
	}()

	seen := map[string]bool{}
	total := 0
	for {
		select {
		case n := <-audioBytes:
			total += n
		case m, ok := <-events:
			if !ok {
				t.Fatal("socket closed early")
			}
			typ, _ := m["type"].(string)
			seen[typ] = true
			switch typ {
			case "stt.final":
				t.Logf("stt.final: %v (%v ms)", m["text"], m["ms"])
			case "router.decision":
				t.Logf("decision: %v", m["decision"])
			case "tts.start":
				t.Logf("first audio: end_to_audio_ms=%v", m["end_to_audio_ms"])
			case "error":
				t.Fatalf("gateway error: %v", m["message"])
			case "voice.metrics":
				t.Logf("metrics: %v", m["latency_ms"])
				for _, want := range []string{"session.ready", "stt.final", "response.delta", "tts.start"} {
					if !seen[want] {
						t.Fatalf("missing event %s (seen %v)", want, seen)
					}
				}
				if total == 0 {
					t.Fatal("no audio received")
				}
				return
			}
		case <-ctx.Done():
			t.Fatalf("timeout; seen %v", seen)
		}
	}
}
