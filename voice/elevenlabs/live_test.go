package elevenlabs_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"hackathon/voice/audio"
	"hackathon/voice/config"
	"hackathon/voice/elevenlabs"
)

// Live tests hit the real API and spend a few dozen TTS characters.
// Run: VOICE_LIVE=1 go test ./elevenlabs/ -run Live -v
func liveClient(t *testing.T) (*elevenlabs.Client, config.Config) {
	t.Helper()
	if os.Getenv("VOICE_LIVE") != "1" {
		t.Skip("set VOICE_LIVE=1 to run live ElevenLabs tests")
	}
	cfg := config.Load()
	if cfg.ElevenLabsKey == "" {
		t.Skip("no ElevenLabs key")
	}
	return elevenlabs.NewClient(cfg.ElevenLabsKey, cfg.ElevenLabsBase), cfg
}

func speak(t *testing.T, c *elevenlabs.Client, o elevenlabs.SocketOptions, text string) ([]byte, elevenlabs.SocketInfo) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := c.OpenSocket(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Send(text, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(); err != nil {
		t.Fatal(err)
	}
	var out []byte
	for b := range s.Audio() {
		out = append(out, b...)
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	info := s.Info()
	t.Logf("%s: first audio %v after first text, %d bytes", o.Model, info.FirstAudio.Sub(info.FirstText).Round(time.Millisecond), len(out))
	return out, info
}

func TestLiveFlashRussian(t *testing.T) {
	c, cfg := liveClient(t)
	pcm, _ := speak(t, c, elevenlabs.SocketOptions{VoiceID: cfg.VoiceID, Model: cfg.TTSModelRU, OutputFormat: "pcm_16000", Language: "ru", AutoMode: true}, "Здравствуйте, чем могу помочь?")
	if len(pcm) < 16000 { // < 0.5 s of audio
		t.Fatalf("too little audio: %d bytes", len(pcm))
	}
}

func TestLiveKazakhRoundTrip(t *testing.T) {
	c, cfg := liveClient(t)
	text := "Сәлеметсіз бе, қалай көмектесе аламын?"
	pcm, _ := speak(t, c, elevenlabs.SocketOptions{VoiceID: cfg.VoiceIDKK, Model: cfg.TTSModelKK, OutputFormat: "pcm_16000"}, text)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := c.OpenSTT(ctx, elevenlabs.STTOptions{AudioFormat: "pcm_16000", Language: cfg.STTLanguage, CommitStrategy: "vad", VADSilenceSecs: 0.5, IncludeTimestamps: true, IncludeLanguageDetection: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f := audio.MustFormat("pcm_16000")
	start := time.Now()
	for i, fr := range audio.Frames(pcm, f, 20*time.Millisecond) {
		if err := s.Send(fr, false); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Until(start.Add(time.Duration(i+1) * 20 * time.Millisecond)))
	}
	audioEnd := time.Now()
	silence := f.Silence(20 * time.Millisecond)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case ev, ok := <-s.Events():
			if !ok {
				t.Fatal("stt closed")
			}
			switch ev.Type {
			case elevenlabs.EventPartial:
				t.Logf("partial +%v: %s", ev.At.Sub(start).Round(time.Millisecond), ev.Text)
			case elevenlabs.EventCommittedTimestamps:
				end, _ := s.SpeechEnd(ev.Words)
				t.Logf("final %v after audio end, %v after last word: %q lang=%s", ev.At.Sub(audioEnd).Round(time.Millisecond), ev.At.Sub(end).Round(time.Millisecond), ev.Text, ev.Language)
				if !strings.Contains(strings.ToLower(ev.Text), "сәлеметсіз") {
					t.Fatalf("unexpected transcript %q", ev.Text)
				}
				return
			case elevenlabs.EventError:
				t.Fatal(ev.Err)
			}
		case <-tick.C:
			_ = s.Send(silence, false)
		case <-ctx.Done():
			t.Fatal("no final transcript")
		}
	}
}
