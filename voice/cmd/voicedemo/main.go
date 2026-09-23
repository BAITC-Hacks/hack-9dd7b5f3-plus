// Command voicedemo exercises the ElevenLabs integration with the team key and
// records what it measured: demo WAV files plus demos/RESULTS.md.
//
//	go run ./cmd/voicedemo quota               remaining TTS characters on the plan
//	go run ./cmd/voicedemo voices              voices available to the key
//	go run ./cmd/voicedemo tts [-force]        synthesize demo phrases (RU / KZ / mixed) over WebSockets
//	go run ./cmd/voicedemo stt [file.wav ...]  stream WAVs to Scribe v2 Realtime at real-time pace
//	go run ./cmd/voicedemo report              rebuild demos/RESULTS.md from the saved measurements
//
// TTS output is cached as WAV: re-running tts without -force costs no credits.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hackathon/voice/audio"
	"hackathon/voice/config"
	"hackathon/voice/elevenlabs"
	"hackathon/voice/lang"
	"hackathon/voice/speech"
)

type phrase struct {
	Name string
	Lang lang.Lang
	Text string
}

// Short on purpose: the Starter plan has few characters left.
var demoPhrases = []phrase{
	{"01_ru_greeting", lang.RU, "Здравствуйте! Я голосовой помощник Saqta. Чем могу помочь?"},
	{"02_kk_greeting", lang.KK, "Сәлеметсіз бе! Мен Saqta дауыстық көмекшісімін. Қалай көмектесе аламын?"},
	{"03_mixed_caller", lang.Mixed, "Сәлеметсіз бе, кеше аварияға түстім, но я не виноват."},
}

// ttsResult is saved next to each WAV as <name>.tts.json.
type ttsResult struct {
	Name         string  `json:"name"`
	Lang         string  `json:"lang"`
	Text         string  `json:"text"`
	Model        string  `json:"model"`
	Voice        string  `json:"voice"`
	Format       string  `json:"format"`
	ConnectMS    float64 `json:"connect_ms"`     // WebSocket open + init
	FirstAudioMS float64 `json:"first_audio_ms"` // first text sent -> first audio chunk
	TotalMS      float64 `json:"total_ms"`       // first text sent -> last audio chunk
	AudioMS      float64 `json:"audio_ms"`       // duration of the produced audio
	Chars        int     `json:"chars"`
	At           string  `json:"at"`
}

// sttResult is saved as <name>.<mode>.stt.json.
type sttResult struct {
	Name           string  `json:"name"`
	Mode           string  `json:"mode"` // vad | manual
	Format         string  `json:"format"`
	AudioMS        float64 `json:"audio_ms"`
	FirstPartialMS float64 `json:"first_partial_ms"` // stream start -> first partial transcript
	FinalAfterMS   float64 `json:"final_after_ms"`   // end of audio -> committed transcript
	SpeechEndMS    float64 `json:"speech_end_ms"`    // last word end (from timestamps) -> committed transcript
	Text           string  `json:"text"`
	Language       string  `json:"language"`
	Expected       string  `json:"expected,omitempty"`
	At             string  `json:"at"`
}

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	out := fs.String("out", "demos", "output directory for WAVs and measurements")
	force := fs.Bool("force", false, "tts: re-synthesize even if the WAV exists (costs credits)")
	format := fs.String("format", "pcm_16000", "tts: output format (pcm_16000 | pcm_8000 | ulaw_8000)")
	_ = fs.Parse(os.Args[2:])

	cfg := config.Load()
	if cfg.ElevenLabsKey == "" {
		log.Fatal("ElevenLabs key missing: set ELEVENLABS_API_KEY (or eleven-labs=...) in .env")
	}
	el := elevenlabs.NewClient(cfg.ElevenLabsKey, cfg.ElevenLabsBase)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var err error
	switch cmd {
	case "quota":
		err = runQuota(ctx, el)
	case "voices":
		err = runVoices(ctx, el)
	case "tts":
		err = runTTS(ctx, el, cfg, *out, *format, *force)
	case "stt":
		err = runSTT(ctx, el, cfg, *out, fs.Args())
	case "report":
		err = writeReport(*out)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: voicedemo quota|voices|tts|stt|report [flags]")
}

func runQuota(ctx context.Context, el *elevenlabs.Client) error {
	s, err := el.Subscription(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("plan=%s characters used=%d limit=%d remaining=%d\n", s.Tier, s.CharacterCount, s.CharacterLimit, s.Remaining())
	return nil
}

func runVoices(ctx context.Context, el *elevenlabs.Client) error {
	vs, err := el.Voices(ctx)
	if err != nil {
		return err
	}
	for _, v := range vs {
		fmt.Printf("%s  %-40s %-10s %s/%s\n", v.ID, v.Name, v.Category, v.Labels["gender"], v.Labels["accent"])
	}
	return nil
}

func runTTS(ctx context.Context, el *elevenlabs.Client, cfg config.Config, out, format string, force bool) error {
	f, err := audio.ParseFormat(format)
	if err != nil || !f.Raw() {
		return fmt.Errorf("tts: need a raw format, got %q", format)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	tts := &speech.ElevenTTS{Client: el, Cfg: cfg}
	for _, p := range demoPhrases {
		wav := filepath.Join(out, p.Name+".wav")
		if _, err := os.Stat(wav); err == nil && !force {
			log.Printf("skip %s (exists; -force to re-synthesize)", wav)
			continue
		}
		t0 := time.Now()
		st, err := tts.Open(ctx, p.Lang, format)
		if err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
		connect := time.Since(t0)
		// simulate an LLM streaming the reply: pieces go out as soon as they exist
		ch := speech.NewChunker()
		for _, pc := range append(ch.Push(p.Text), ch.Flush()...) {
			if err := st.Send(pc.Text, pc.Flush); err != nil {
				st.Close()
				return err
			}
		}
		_ = st.Finish()
		var raw []byte
		var last time.Time
		for b := range st.Audio() {
			raw = append(raw, b...)
			last = time.Now()
		}
		st.Close()
		if err := st.Err(); err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
		info := st.Info()
		pcm, err := audio.Convert(raw, f, audio.MustFormat(fmt.Sprintf("pcm_%d", f.Rate)))
		if err != nil {
			return err
		}
		if err := os.WriteFile(wav, audio.EncodeWAV(pcm, f.Rate), 0o644); err != nil {
			return err
		}
		r := ttsResult{
			Name: p.Name, Lang: string(p.Lang), Text: p.Text, Model: info.Model, Voice: info.Voice, Format: format,
			ConnectMS:    ms(connect),
			FirstAudioMS: ms(info.FirstAudio.Sub(info.FirstText)),
			TotalMS:      ms(last.Sub(info.FirstText)),
			AudioMS:      ms(f.Duration(len(raw))),
			Chars:        len([]rune(p.Text)),
			At:           time.Now().Format(time.RFC3339),
		}
		if err := saveJSON(filepath.Join(out, p.Name+".tts.json"), r); err != nil {
			return err
		}
		log.Printf("%-16s %-26s connect=%4.0fms first_audio=%4.0fms total=%5.0fms audio=%5.0fms -> %s",
			p.Name, r.Model, r.ConnectMS, r.FirstAudioMS, r.TotalMS, r.AudioMS, wav)
	}
	return writeReport(out)
}

func runSTT(ctx context.Context, el *elevenlabs.Client, cfg config.Config, out string, files []string) error {
	if len(files) == 0 {
		matches, _ := filepath.Glob(filepath.Join(out, "*.wav"))
		for _, m := range matches {
			if !strings.Contains(filepath.Base(m), "_reply") {
				files = append(files, m)
			}
		}
	}
	if len(files) == 0 {
		return errors.New("stt: no WAV files (run `voicedemo tts` first or pass files)")
	}
	stt := &speech.ElevenSTT{Client: el, Cfg: cfg}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		pcm, rate, err := audio.DecodeWAV(b)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		if rate != 16000 && rate != 8000 {
			pcm = audio.SamplesToBytes(audio.Resample(audio.BytesToSamples(pcm), rate, 16000))
			rate = 16000
		}
		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		expected := expectedText(out, name)
		for _, manual := range []bool{false, true} {
			r, err := streamFile(ctx, stt, pcm, rate, manual)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			r.Name, r.Expected, r.At = name, expected, time.Now().Format(time.RFC3339)
			if err := saveJSON(filepath.Join(out, name+"."+r.Mode+".stt.json"), r); err != nil {
				return err
			}
			log.Printf("%-16s %-6s first_partial=%4.0fms final_after_audio=%4.0fms final_after_speech=%4.0fms lang=%s text=%q",
				name, r.Mode, r.FirstPartialMS, r.FinalAfterMS, r.SpeechEndMS, r.Language, r.Text)
		}
	}
	return writeReport(out)
}

// streamFile plays PCM into a realtime session in 20 ms frames at real-time
// pace (like a live caller), then waits for the committed transcript.
func streamFile(ctx context.Context, stt *speech.ElevenSTT, pcm []byte, rate int, manual bool) (sttResult, error) {
	format := fmt.Sprintf("pcm_%d", rate)
	f := audio.MustFormat(format)
	opts := stt.Options(speech.RecognizerOptions{Format: format, Manual: manual})
	sess, err := stt.Client.OpenSTT(ctx, opts)
	if err != nil {
		return sttResult{}, err
	}
	defer sess.Close()
	r := sttResult{Format: format, AudioMS: ms(f.Duration(len(pcm))), Mode: "vad"}
	if manual {
		r.Mode = "manual"
	}

	type final struct {
		ev           elevenlabs.STTEvent
		firstPartial time.Time
	}
	finals := make(chan final, 4)
	start := time.Now()
	go func() {
		var firstPartial time.Time
		for ev := range sess.Events() {
			switch ev.Type {
			case elevenlabs.EventPartial:
				if firstPartial.IsZero() && strings.TrimSpace(ev.Text) != "" {
					firstPartial = ev.At
				}
			case elevenlabs.EventCommittedTimestamps:
				finals <- final{ev, firstPartial}
			case elevenlabs.EventError:
				log.Printf("stt error: %v", ev.Err)
			}
		}
		close(finals)
	}()

	frames := audio.Frames(pcm, f, 20*time.Millisecond)
	for i, fr := range frames {
		if err := sess.Send(fr, false); err != nil {
			return r, err
		}
		time.Sleep(time.Until(start.Add(time.Duration(i+1) * 20 * time.Millisecond)))
	}
	audioEnd := time.Now()
	if manual {
		if err := sess.Commit(); err != nil {
			return r, err
		}
	}
	// keep the line alive with silence, as a real call would, until the final arrives
	silence := f.Silence(20 * time.Millisecond)
	deadline := time.After(6 * time.Second)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case fin, ok := <-finals:
			if !ok {
				return r, errors.New("stt closed before a final transcript")
			}
			r.Text, r.Language = fin.ev.Text, fin.ev.Language
			r.FinalAfterMS = ms(fin.ev.At.Sub(audioEnd))
			if end, ok := sess.SpeechEnd(fin.ev.Words); ok {
				r.SpeechEndMS = ms(fin.ev.At.Sub(end))
			}
			if !fin.firstPartial.IsZero() {
				r.FirstPartialMS = ms(fin.firstPartial.Sub(start))
			}
			return r, nil
		case <-tick.C:
			if !manual {
				_ = sess.Send(silence, false)
			}
		case <-deadline:
			return r, errors.New("no final transcript within 6s")
		}
	}
}

func expectedText(out, name string) string {
	var r ttsResult
	if b, err := os.ReadFile(filepath.Join(out, name+".tts.json")); err == nil && json.Unmarshal(b, &r) == nil {
		return r.Text
	}
	return ""
}

func writeReport(out string) error {
	var tts []ttsResult
	var stt []sttResult
	files, _ := filepath.Glob(filepath.Join(out, "*.json"))
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		switch {
		case strings.HasSuffix(f, ".tts.json"):
			var r ttsResult
			if json.Unmarshal(b, &r) == nil {
				tts = append(tts, r)
			}
		case strings.HasSuffix(f, ".stt.json"):
			var r sttResult
			if json.Unmarshal(b, &r) == nil {
				stt = append(stt, r)
			}
		}
	}
	var sb strings.Builder
	sb.WriteString("# Voice demo results\n\n")
	sb.WriteString("Measured with `go run ./cmd/voicedemo` on the team ElevenLabs key. Regenerate: `go run ./cmd/voicedemo tts -force && go run ./cmd/voicedemo stt`.\n\n")
	if len(tts) > 0 {
		sb.WriteString("## Text-to-speech (WebSocket streaming)\n\n")
		sb.WriteString("| Demo | Lang | Model | Connect ms | First audio ms | Total ms | Audio ms | Text |\n|---|---|---|---:|---:|---:|---:|---|\n")
		for _, r := range tts {
			fmt.Fprintf(&sb, "| [%s](%s.wav) | %s | `%s` | %.0f | **%.0f** | %.0f | %.0f | %s |\n", r.Name, r.Name, r.Lang, r.Model, r.ConnectMS, r.FirstAudioMS, r.TotalMS, r.AudioMS, r.Text)
		}
		sb.WriteString("\n*First audio* = first text sent -> first audio chunk received (what the caller waits for after the LLM starts talking). The socket is opened in advance during the call, so *Connect* is hidden.\n\n")
	}
	if len(stt) > 0 {
		sb.WriteString("## Speech-to-text (Scribe v2 Realtime, audio streamed at real-time pace)\n\n")
		sb.WriteString("| Input | Mode | Audio ms | First partial ms | Final after audio end ms | Final after last word ms | Lang | Transcript | Expected |\n|---|---|---:|---:|---:|---:|---|---|---|\n")
		for _, r := range stt {
			fmt.Fprintf(&sb, "| %s | %s | %.0f | %.0f | **%.0f** | %.0f | %s | %s | %s |\n", r.Name, r.Mode, r.AudioMS, r.FirstPartialMS, r.FinalAfterMS, r.SpeechEndMS, r.Language, r.Text, r.Expected)
		}
		sb.WriteString("\n*vad* = server end-of-turn detection (phone, hands-free; includes the configured silence window). *manual* = push-to-talk: the client commits on button release.\n")
	}
	path := filepath.Join(out, "RESULTS.md")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return err
	}
	log.Printf("wrote %s", path)
	return nil
}

func saveJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func ms(d time.Duration) float64 {
	if d < 0 {
		return 0
	}
	return float64(d.Round(100*time.Microsecond)) / float64(time.Millisecond)
}
