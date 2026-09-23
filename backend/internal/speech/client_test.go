package speech

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAudioActuallyReachesSTTAndPCMStreams(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio/transcriptions":
			if e := r.ParseMultipartForm(1 << 20); e != nil {
				t.Error(e)
			}
			f, _, e := r.FormFile("file")
			if e != nil {
				t.Error(e)
				return
			}
			defer f.Close()
			b, _ := io.ReadAll(f)
			if !bytes.Equal(b, []byte("real-audio-bytes")) {
				t.Error("audio replaced")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"text":"Полисімді ұзарту керек"}`))
		case "/audio/speech":
			_, _ = w.Write([]byte{0, 0, 1, 0})
		case "/realtime/calls":
			_ = r.ParseMultipartForm(1 << 20)
			if !strings.Contains(r.FormValue("session"), `"turn_detection":null`) {
				t.Error("expected manual VAD")
			}
			_, _ = w.Write([]byte("v=0\r\nanswer"))
		}
	}))
	defer s.Close()
	c := &Client{Key: "placeholder", Base: s.URL, STTModel: "test", TTSModel: "test", RealtimeModel: "gpt-live-transcribe", Enabled: true, HTTP: &http.Client{Timeout: time.Second}}
	text, e := c.Transcribe(context.Background(), "audio.wav", strings.NewReader("real-audio-bytes"))
	if e != nil || text != "Полисімді ұзарту керек" {
		t.Fatalf("%s %v", text, e)
	}
	resp, e := c.Speak(context.Background(), "answer")
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if len(b) != 4 {
		t.Fatal("missing pcm")
	}
	if _, e = c.Connect(context.Background(), "v=0"); e != nil {
		t.Fatal(e)
	}
}
