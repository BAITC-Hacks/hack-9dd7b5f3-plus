package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"hackathon/voice/agent"
	"hackathon/voice/audio"
	"hackathon/voice/brain"
	"hackathon/voice/config"
	"hackathon/voice/elevenlabs"
	"hackathon/voice/speech"
	"hackathon/voice/voicelog"
)

// e2eResult is saved as <name>.e2e.json.
type e2eResult struct {
	Name         string  `json:"name"`
	Brain        string  `json:"brain"`
	Model        string  `json:"model"`
	Transcript   string  `json:"transcript"`
	InLang       string  `json:"in_lang"`
	ReplyLang    string  `json:"reply_lang"`
	Scenario     string  `json:"scenario"`
	Confidence   float64 `json:"confidence"`
	Reply        string  `json:"reply"`
	STTms        float64 `json:"stt_ms"`
	BrainTTFTms  float64 `json:"brain_ttft_ms"`
	TTSTTFBms    float64 `json:"tts_ttfb_ms"`
	EndToAudioms float64 `json:"end_to_audio_ms"` // first sound, filler included
	EndToReplyms float64 `json:"end_to_reply_ms"` // first sound of the reply itself
	Speculative  bool    `json:"speculative"`
	Filler       bool    `json:"filler"`
	At           string  `json:"at"`
}

// recorder is an agent transport that keeps the reply audio and events.
type recorder struct {
	mu      sync.Mutex
	audio   []byte
	events  []map[string]any
	metrics chan voicelog.TurnMetrics
	ready   chan struct{}
	once    sync.Once
}

func (r *recorder) OutputFormat() string { return "pcm_16000" }
func (r *recorder) Play(b []byte) error {
	r.mu.Lock()
	r.audio = append(r.audio, b...)
	r.mu.Unlock()
	return nil
}
func (r *recorder) Clear()  {}
func (r *recorder) Hangup() {}
func (r *recorder) Send(ev map[string]any) {
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.mu.Unlock()
	switch ev["type"] {
	case "session.ready":
		r.once.Do(func() { close(r.ready) })
	case "voice.metrics":
		if m, ok := ev["metrics"].(voicelog.TurnMetrics); ok {
			select {
			case r.metrics <- m:
			default:
			}
		}
	}
}

func runE2E(ctx context.Context, el *elevenlabs.Client, cfg config.Config, out string, files []string) error {
	if len(files) == 0 {
		for _, n := range []string{"03_mixed_caller", "04_ru_caller", "05_kk_caller"} {
			if _, err := os.Stat(filepath.Join(out, n+".wav")); err == nil {
				files = append(files, filepath.Join(out, n+".wav"))
			}
		}
	}
	if len(files) == 0 {
		return errors.New("e2e: no caller WAVs (run `voicedemo tts` first or pass files)")
	}
	var data *brain.Dataset
	if cfg.DataDir != "" {
		if d, err := brain.LoadDataset(cfg.DataDir); err == nil {
			data = d
		} else {
			log.Printf("dataset: %v", err)
		}
	}
	var br brain.Brain = brain.Echo{}
	if cfg.OpenRouterKey != "" {
		or := brain.NewOpenRouter(cfg.OpenRouterKey, cfg.OpenRouterBase, cfg.OpenRouterModel, cfg.OpenRouterFallbacks, data)
		_ = or.Warm(ctx)
		br = or
	}
	_ = el.Warm(ctx)
	lg := voicelog.New(filepath.Join(out, "e2e-logs"), false)
	deps := agent.Deps{Cfg: cfg, STT: &speech.ElevenSTT{Client: el, Cfg: cfg}, TTS: &speech.ElevenTTS{Client: el, Cfg: cfg}, Brain: br, Log: lg}
	f := audio.MustFormat("pcm_16000")

	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		pcm, rate, err := audio.DecodeWAV(b)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		if rate != 16000 {
			pcm = audio.SamplesToBytes(audio.Resample(audio.BytesToSamples(pcm), rate, 16000))
		}
		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		rec := &recorder{metrics: make(chan voicelog.TurnMetrics, 1), ready: make(chan struct{})}
		cctx, cancel := context.WithCancel(ctx)
		ag := agent.New(cctx, deps, agent.Options{SessionID: "e2e-" + name, Channel: "demo", InputFormat: "pcm_16000"}, rec)
		go ag.Run()
		select {
		case <-rec.ready:
		case <-time.After(10 * time.Second):
			cancel()
			return errors.New("e2e: STT session not ready")
		}
		// caller speaks in real time, then the line stays open (silence) until the reply is done
		start := time.Now()
		frames := audio.Frames(pcm, f, 20*time.Millisecond)
		for i, fr := range frames {
			ag.PushAudio(fr)
			time.Sleep(time.Until(start.Add(time.Duration(i+1) * 20 * time.Millisecond)))
		}
		silence := f.Silence(20 * time.Millisecond)
		var m voicelog.TurnMetrics
		tick := time.NewTicker(20 * time.Millisecond)
		deadline := time.After(20 * time.Second)
	wait:
		for {
			select {
			case m = <-rec.metrics:
				break wait
			case <-tick.C:
				ag.PushAudio(silence)
			case <-deadline:
				tick.Stop()
				cancel()
				return fmt.Errorf("e2e %s: no reply within 20s", name)
			}
		}
		tick.Stop()
		cancel()
		rec.mu.Lock()
		reply := append([]byte(nil), rec.audio...)
		rec.mu.Unlock()
		if err := os.WriteFile(filepath.Join(out, name+"_reply.wav"), audio.EncodeWAV(reply, 16000), 0o644); err != nil {
			return err
		}
		r := e2eResult{
			Name: name, Brain: m.Brain, Model: m.Model, Transcript: m.UserText, InLang: m.InLang, ReplyLang: m.ReplyLang,
			Scenario: m.Scenario, Confidence: m.Confidence, Reply: m.ReplyText, STTms: m.STTms, BrainTTFTms: m.BrainTTFTms,
			TTSTTFBms: m.TTSTTFBms, EndToAudioms: m.EndToAudioms, EndToReplyms: m.EndToReplyms, Speculative: m.Speculative, Filler: m.Filler,
			At: time.Now().Format(time.RFC3339),
		}
		if err := saveJSON(filepath.Join(out, name+".e2e.json"), r); err != nil {
			return err
		}
		log.Printf("%-16s end_to_reply=%5.0fms first_sound=%5.0fms stt=%4.0fms llm_first=%4.0fms tts_first=%4.0fms spec=%v filler=%v %s %.2f | %q -> %q",
			name, r.EndToReplyms, r.EndToAudioms, r.STTms, r.BrainTTFTms, r.TTSTTFBms, r.Speculative, r.Filler, r.Scenario, r.Confidence, r.Transcript, r.Reply)
		time.Sleep(300 * time.Millisecond) // let the session log close
	}
	return writeReport(out)
}
