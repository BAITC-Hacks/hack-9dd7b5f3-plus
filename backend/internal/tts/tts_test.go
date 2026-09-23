package tts

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestElevenLabsStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("xi-api-key") != "k" || !strings.Contains(r.URL.Path, "/v1/text-to-speech/voice1/stream") || r.URL.Query().Get("output_format") != "pcm_24000" {
			http.Error(w, "bad request "+r.URL.String(), 400)
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["model_id"] != "eleven_flash_v2_5" || body["language_code"] != "ru" {
			http.Error(w, "bad body", 400)
			return
		}
		w.Header().Set("Content-Type", "audio/pcm")
		w.Write(make([]byte, 4800))
	}))
	defer srv.Close()
	p := NewElevenLabs(srv.URL, "k", "eleven_flash_v2_5", "voice1", "", "", 24000, 1, 5*time.Second)
	rc, err := p.Synthesize(context.Background(), "Здравствуйте", "ru")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if len(b) != 4800 {
		t.Fatalf("got %d bytes", len(b))
	}
}

func TestOpenAISpeechStripsWAV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["response_format"] != "pcm" || body["voice"] != "Aigerim" || body["model"] != "tokay-kk-v1" {
			http.Error(w, "bad body", 400)
			return
		}
		// geko-style: answers WAV even for pcm requests
		w.Header().Set("Content-Type", "audio/wav")
		hdr := make([]byte, 44)
		copy(hdr, "RIFF")
		w.Write(append(hdr, make([]byte, 2400)...))
	}))
	defer srv.Close()
	p := NewOpenAI(srv.URL, "", "gpt-4o-mini-tts", "coral", "tokay-kk-v1", "Aigerim", 24000, 1, 5*time.Second)
	rc, err := p.Synthesize(context.Background(), "Сәлеметсіз бе", "kk")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	if len(b) != 2400 {
		t.Fatalf("WAV header should be stripped: got %d bytes", len(b))
	}
}
