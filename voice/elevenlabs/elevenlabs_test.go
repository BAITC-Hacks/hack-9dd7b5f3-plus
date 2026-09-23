package elevenlabs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fakeAPI emulates the ElevenLabs endpoints the client uses.
func fakeAPI(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/speech-to-text/realtime", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("xi-api-key") != "test-key" {
			http.Error(w, `{"detail":"invalid api key"}`, http.StatusUnauthorized)
			return
		}
		q := r.URL.Query()
		if q.Get("model_id") != "scribe_v2_realtime" || q.Get("audio_format") != "pcm_16000" || q.Get("commit_strategy") != "vad" {
			http.Error(w, "bad params "+r.URL.RawQuery, http.StatusBadRequest)
			return
		}
		if got := q["keyterms"]; len(got) != 2 {
			http.Error(w, "keyterms not repeated", http.StatusBadRequest)
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		send := func(v any) { b, _ := json.Marshal(v); _ = c.Write(ctx, websocket.MessageText, b) }
		send(map[string]any{"message_type": "session_started", "session_id": "sess-1", "config": map[string]any{}})
		chunks := 0
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var m sttChunk
			if json.Unmarshal(data, &m) != nil || m.MessageType != "input_audio_chunk" || m.SampleRate != 16000 {
				send(map[string]any{"message_type": "input_error", "error": "bad chunk"})
				continue
			}
			if chunks == 0 && m.PreviousText != "контекст" {
				send(map[string]any{"message_type": "input_error", "error": "previous_text missing"})
			}
			chunks++
			if chunks == 3 {
				send(map[string]any{"message_type": "partial_transcript", "text": "Сәлеметсіз"})
			}
			if m.Commit {
				send(map[string]any{"message_type": "committed_transcript", "text": "Сәлеметсіз бе"})
				send(map[string]any{"message_type": "committed_transcript_with_timestamps", "text": "Сәлеметсіз бе", "language_code": "kaz",
					"words": []map[string]any{{"text": "Сәлеметсіз", "start": 0.0, "end": 0.04, "type": "word"}, {"text": " ", "start": 0.04, "end": 0.05, "type": "spacing"}, {"text": "бе", "start": 0.05, "end": 0.06, "type": "word"}}})
			}
		}
	})
	mux.HandleFunc("/v1/text-to-speech/voice-1/stream-input", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("auto_mode") != "true" || q.Get("language_code") != "ru" || q.Get("output_format") != "pcm_16000" {
			http.Error(w, "bad params "+r.URL.RawQuery, http.StatusBadRequest)
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		var got []map[string]any
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var m map[string]any
			_ = json.Unmarshal(data, &m)
			got = append(got, m)
			if len(got) == 1 {
				if m["text"] != " " {
					t.Errorf("init text = %q", m["text"])
				}
				continue
			}
			text, _ := m["text"].(string)
			if text == "" {
				b, _ := json.Marshal(map[string]any{"isFinal": true})
				_ = c.Write(ctx, websocket.MessageText, b)
				c.Close(websocket.StatusNormalClosure, "")
				return
			}
			if !strings.HasSuffix(text, " ") {
				t.Errorf("text must end with a space: %q", text)
			}
			b, _ := json.Marshal(map[string]any{"audio": base64.StdEncoding.EncodeToString([]byte(text)), "isFinal": nil})
			_ = c.Write(ctx, websocket.MessageText, b)
		}
	})
	mux.HandleFunc("/v1/text-to-dialogue/stream-input", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("model_id") != "eleven_v3_conversational" {
			http.Error(w, "bad model", http.StatusBadRequest)
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		first := true
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var m map[string]any
			_ = json.Unmarshal(data, &m)
			if first {
				first = false
				if v, _ := m["voices"].([]any); len(v) != 1 || v[0] != "voice-kk" {
					t.Errorf("init = %v", m)
				}
				continue
			}
			switch {
			case m["inputs"] != nil:
				in := m["inputs"].([]any)[0].(map[string]any)
				b, _ := json.Marshal(map[string]any{"audio": base64.StdEncoding.EncodeToString([]byte(in["text"].(string))), "is_final": false})
				_ = c.Write(ctx, websocket.MessageText, b)
			case m["close_socket"] == true:
				b, _ := json.Marshal(map[string]any{"is_final": true})
				_ = c.Write(ctx, websocket.MessageText, b)
				c.Close(websocket.StatusNormalClosure, "")
				return
			}
		}
	})
	mux.HandleFunc("/v1/text-to-speech/voice-1/stream", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model_id"] != "eleven_flash_v2_5" || body["language_code"] != "ru" || r.URL.Query().Get("output_format") != "ulaw_8000" {
			http.Error(w, "bad request", http.StatusUnprocessableEntity)
			return
		}
		w.Write([]byte("audio-bytes"))
	})
	mux.HandleFunc("/v1/single-use-token/realtime_scribe", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"token":"tok"}`))
	})
	return httptest.NewServer(mux)
}

func TestSTTRealtime(t *testing.T) {
	srv := fakeAPI(t)
	defer srv.Close()
	c := NewClient("test-key", srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := c.OpenSTT(ctx, STTOptions{Keyterms: []string{"Saqta", "ОГПО"}, IncludeTimestamps: true, PreviousText: "контекст"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.ID != "sess-1" {
		t.Fatalf("session id %q", s.ID)
	}
	frame := make([]byte, 640) // 20 ms of pcm_16000
	for i := 0; i < 3; i++ {
		if err := s.Send(frame, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Commit(); err != nil {
		t.Fatal(err)
	}
	var partial, committed, stamped bool
	for !(partial && committed && stamped) {
		select {
		case ev := <-s.Events():
			switch ev.Type {
			case EventPartial:
				partial = ev.Text == "Сәлеметсіз"
			case EventCommitted:
				committed = ev.Text == "Сәлеметсіз бе"
			case EventCommittedTimestamps:
				stamped = ev.Language == "kaz" && len(ev.Words) == 3
				end, ok := s.SpeechEnd(ev.Words)
				if !ok || time.Since(end) > 2*time.Second {
					t.Fatalf("speech end %v %v", end, ok)
				}
			case EventError:
				t.Fatal(ev.Err)
			}
		case <-ctx.Done():
			t.Fatalf("timeout: partial=%v committed=%v stamped=%v", partial, committed, stamped)
		}
	}
	if got := s.AudioSent(); got != 70*time.Millisecond {
		t.Fatalf("audio sent %v", got)
	}
}

func TestSTTAuthError(t *testing.T) {
	srv := fakeAPI(t)
	defer srv.Close()
	c := NewClient("wrong", srv.URL)
	_, err := c.OpenSTT(context.Background(), STTOptions{})
	var apiErr *APIError
	if err == nil || !asAPIError(err, &apiErr) || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("want 401 APIError, got %v", err)
	}
}

func asAPIError(err error, target **APIError) bool {
	e, ok := err.(*APIError)
	if ok {
		*target = e
	}
	return ok
}

func collect(t *testing.T, s *Socket) string {
	t.Helper()
	var sb strings.Builder
	timeout := time.After(5 * time.Second)
	for {
		select {
		case b, ok := <-s.Audio():
			if !ok {
				return sb.String()
			}
			sb.Write(b)
		case <-timeout:
			t.Fatal("timeout waiting for audio")
		}
	}
}

func TestStreamInputSocket(t *testing.T) {
	srv := fakeAPI(t)
	defer srv.Close()
	c := NewClient("test-key", srv.URL)
	s, err := c.OpenSocket(context.Background(), SocketOptions{VoiceID: "voice-1", Model: "eleven_flash_v2_5", Language: "ru", AutoMode: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Send("Здравствуйте,", true); err != nil {
		t.Fatal(err)
	}
	if err := s.Send("чем помочь?", true); err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(); err != nil {
		t.Fatal(err)
	}
	if got := collect(t, s); got != "Здравствуйте, чем помочь? " {
		t.Fatalf("audio = %q", got)
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	info := s.Info()
	if info.FirstAudio.IsZero() || info.FirstText.IsZero() || info.FirstAudio.Before(info.FirstText) {
		t.Fatalf("bad timings %+v", info)
	}
}

func TestDialogueSocket(t *testing.T) {
	srv := fakeAPI(t)
	defer srv.Close()
	c := NewClient("test-key", srv.URL)
	s, err := c.OpenSocket(context.Background(), SocketOptions{VoiceID: "voice-kk", Model: "eleven_v3_conversational"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_ = s.Send("Сәлеметсіз бе! ", true)
	_ = s.Send("Қалай көмектесе аламын?", false)
	_ = s.Finish()
	if got := collect(t, s); got != "Сәлеметсіз бе! Қалай көмектесе аламын?" {
		t.Fatalf("audio = %q", got)
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestTTSHTTPAndToken(t *testing.T) {
	srv := fakeAPI(t)
	defer srv.Close()
	c := NewClient("test-key", srv.URL)
	b, ttfb, err := c.TTS(context.Background(), TTSRequest{VoiceID: "voice-1", Text: "Привет", Language: "ru", OutputFormat: "ulaw_8000"})
	if err != nil || string(b) != "audio-bytes" || ttfb <= 0 {
		t.Fatalf("tts: %q %v %v", b, ttfb, err)
	}
	_, _, err = c.TTS(context.Background(), TTSRequest{VoiceID: "voice-1", Text: "x", OutputFormat: "pcm_16000"})
	if e, ok := err.(*APIError); !ok || e.Status != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %v", err)
	}
	tok, err := c.SingleUseToken(context.Background(), "realtime_scribe")
	if err != nil || tok != "tok" {
		t.Fatalf("token %q %v", tok, err)
	}
}

func TestAudioClock(t *testing.T) {
	var c audioClock
	t0 := time.Now()
	for i := 1; i <= 10; i++ { // 10 chunks of 100 ms sent every 100 ms
		c.add(100*time.Millisecond, t0.Add(time.Duration(i)*100*time.Millisecond))
	}
	// a word ending at 0.55 s was captured 50 ms before chunk 6 (end 0.6 s) was sent
	got, ok := c.wallAt(550 * time.Millisecond)
	if !ok || got.Sub(t0) != 550*time.Millisecond {
		t.Fatalf("wallAt = %v (%v)", got.Sub(t0), ok)
	}
	if _, ok := (&audioClock{}).wallAt(time.Second); ok {
		t.Fatal("empty clock must not map")
	}
}

var _ io.Reader = (*AudioStream)(nil)
