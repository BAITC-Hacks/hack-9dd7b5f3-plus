package stt

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestWAVRoundTrip(t *testing.T) {
	pcm := make([]byte, 3200)
	for i := range pcm {
		pcm[i] = byte(i)
	}
	w := WAV(pcm, 16000)
	got, rate, err := ParseWAV(w)
	if err != nil || rate != 16000 || len(got) != len(pcm) || got[100] != pcm[100] {
		t.Fatalf("round trip failed: %v rate=%d len=%d", err, rate, len(got))
	}
	if r := Resample(pcm, 16000, 24000); len(r) != 4800 {
		t.Fatalf("resample length %d", len(r))
	}
}

// TestElevenLabsRealtime drives the client against a fake Scribe realtime
// server: partials while audio streams, a committed transcript after commit.
func TestElevenLabsRealtime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("xi-api-key") != "k" {
			http.Error(w, "no key", 401)
			return
		}
		if r.URL.Query().Get("audio_format") != "pcm_16000" || r.URL.Query().Get("commit_strategy") != "manual" {
			http.Error(w, "bad query "+r.URL.RawQuery, 400)
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx := context.Background()
		c.Write(ctx, websocket.MessageText, []byte(`{"message_type":"session_started","session_id":"s1"}`))
		total := 0
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var m struct {
				MessageType string `json:"message_type"`
				Audio       string `json:"audio_base_64"`
				Commit      bool   `json:"commit"`
				SampleRate  int    `json:"sample_rate"`
			}
			json.Unmarshal(data, &m)
			b, _ := base64.StdEncoding.DecodeString(m.Audio)
			total += len(b)
			if m.SampleRate != 16000 {
				c.Write(ctx, websocket.MessageText, []byte(`{"message_type":"error","message":"bad sample rate"}`))
				return
			}
			if m.Commit {
				c.Write(ctx, websocket.MessageText, []byte(`{"message_type":"committed_transcript","text":"полисім жарамды ма","language_code":"kk"}`))
			} else {
				c.Write(ctx, websocket.MessageText, []byte(`{"message_type":"partial_transcript","text":"полисім"}`))
			}
		}
	}))
	defer srv.Close()
	p := NewElevenLabsRealtime(srv.URL, "k", "ru", "kk", 5*time.Second)
	var partials []string
	sess, err := p.NewSession(context.Background(), func(text string) { partials = append(partials, text) })
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	if err := sess.Send(make([]byte, 3200)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res, err := sess.Commit(ctx)
	if err != nil || res.Text != "полисім жарамды ма" || res.Language != "kk" {
		t.Fatalf("commit: %v %+v", err, res)
	}
	time.Sleep(20 * time.Millisecond)
	if len(partials) == 0 {
		t.Fatalf("expected partial transcripts")
	}
}

func TestOpenAITranscribe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" || !strings.HasSuffix(r.URL.Path, "/audio/transcriptions") {
			http.Error(w, "bad request", 400)
			return
		}
		r.ParseMultipartForm(1 << 20)
		f, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "no file", 400)
			return
		}
		defer f.Close()
		buf := make([]byte, 4)
		f.Read(buf)
		if string(buf) != "RIFF" || r.FormValue("model") != "gpt-4o-mini-transcribe" {
			http.Error(w, "bad form", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"Сколько стоит ОГПО?"}`))
	}))
	defer srv.Close()
	p := NewOpenAI(srv.URL, "k", "gpt-4o-mini-transcribe", "", "Saqta", 5*time.Second)
	res, err := p.Transcribe(context.Background(), make([]byte, 3200), 16000, "")
	if err != nil || res.Text != "Сколько стоит ОГПО?" {
		t.Fatalf("transcribe: %v %+v", err, res)
	}
}

func TestElevenLabsBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(1 << 20)
		if r.Header.Get("xi-api-key") != "k" || r.FormValue("model_id") != "scribe_v2" {
			http.Error(w, "bad", 400)
			return
		}
		w.Write([]byte(`{"language_code":"ru","language_probability":0.98,"text":"Где ваш офис?","words":[]}`))
	}))
	defer srv.Close()
	p := NewElevenLabs(srv.URL, "k", "scribe_v2", "", 5*time.Second)
	res, err := p.Transcribe(context.Background(), make([]byte, 3200), 16000, "")
	if err != nil || res.Text != "Где ваш офис?" || res.Language != "ru" {
		t.Fatalf("batch: %v %+v", err, res)
	}
}
