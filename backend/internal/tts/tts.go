// Package tts holds the text-to-speech providers. Every provider streams raw
// PCM16 mono audio at SampleRate(); the engine forwards the bytes to the
// browser as they arrive so the first sentence plays while the rest is still
// being synthesized.
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNoServerTTS signals that the browser should speak the text itself.
var ErrNoServerTTS = errors.New("no server-side TTS configured")

type Provider interface {
	Name() string
	SampleRate() int
	// Synthesize returns a PCM16 stream for text in language lang ("ru"|"kk").
	Synthesize(ctx context.Context, text, lang string) (io.ReadCloser, error)
}

// Browser is the keyless provider: the UI uses window.speechSynthesis.
type Browser struct{}

func (Browser) Name() string    { return "browser" }
func (Browser) SampleRate() int { return 24000 }
func (Browser) Synthesize(context.Context, string, string) (io.ReadCloser, error) {
	return nil, ErrNoServerTTS
}

// ---------------------------------------------------------------- openai-compatible

// OpenAI calls /audio/speech (OpenAI gpt-4o-mini-tts, geko Tokay, speaches...).
type OpenAI struct {
	BaseURL string
	APIKey  string
	Model   string
	Voice   string
	ModelKK string
	VoiceKK string
	Rate    int
	Speed   float64
	HTTP    *http.Client
}

func NewOpenAI(baseURL, apiKey, model, voice, modelKK, voiceKK string, rate int, speed float64, timeout time.Duration) *OpenAI {
	return &OpenAI{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Model: model, Voice: voice, ModelKK: modelKK, VoiceKK: voiceKK, Rate: rate, Speed: speed, HTTP: &http.Client{Timeout: timeout}}
}

func (o *OpenAI) Name() string    { return "openai-compatible" }
func (o *OpenAI) SampleRate() int { return o.Rate }

func (o *OpenAI) Synthesize(ctx context.Context, text, lang string) (io.ReadCloser, error) {
	model, voice := o.Model, o.Voice
	if lang == "kk" {
		if o.ModelKK != "" {
			model = o.ModelKK
		}
		if o.VoiceKK != "" {
			voice = o.VoiceKK
		}
	}
	body := map[string]any{"model": model, "voice": voice, "input": text, "response_format": "pcm"}
	if o.Speed > 0 && o.Speed != 1 {
		body["speed"] = o.Speed
	}
	if strings.HasPrefix(model, "gpt-") {
		langName := map[string]string{"ru": "Russian", "kk": "Kazakh"}[lang]
		body["instructions"] = "You are a friendly, calm insurance contact-center operator. Speak " + langName + " naturally at a brisk conversational pace, with clear pronunciation."
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/audio/speech", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts request: %w", err)
	}
	if resp.StatusCode != 200 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("tts http %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "wav") {
		return &wavStripper{r: resp.Body}, nil
	}
	return resp.Body, nil
}

// wavStripper removes a RIFF header from a WAV stream (geko answers WAV).
type wavStripper struct {
	r       io.ReadCloser
	started bool
}

func (w *wavStripper) Read(p []byte) (int, error) {
	if !w.started {
		w.started = true
		hdr := make([]byte, 44)
		if _, err := io.ReadFull(w.r, hdr); err != nil {
			return 0, err
		}
	}
	return w.r.Read(p)
}
func (w *wavStripper) Close() error { return w.r.Close() }

// ---------------------------------------------------------------- elevenlabs

// ElevenLabs streams from /v1/text-to-speech/{voice}/stream as PCM.
type ElevenLabs struct {
	BaseURL string
	APIKey  string
	Model   string
	Voice   string
	ModelKK string
	VoiceKK string
	Rate    int
	Speed   float64
	HTTP    *http.Client
}

func NewElevenLabs(baseURL, apiKey, model, voice, modelKK, voiceKK string, rate int, speed float64, timeout time.Duration) *ElevenLabs {
	return &ElevenLabs{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Model: model, Voice: voice, ModelKK: modelKK, VoiceKK: voiceKK, Rate: rate, Speed: speed, HTTP: &http.Client{Timeout: timeout}}
}

func (e *ElevenLabs) Name() string    { return "elevenlabs" }
func (e *ElevenLabs) SampleRate() int { return e.Rate }

func (e *ElevenLabs) Synthesize(ctx context.Context, text, lang string) (io.ReadCloser, error) {
	model, voice := e.Model, e.Voice
	if lang == "kk" {
		if e.ModelKK != "" {
			model = e.ModelKK
		}
		if e.VoiceKK != "" {
			voice = e.VoiceKK
		}
	}
	body := map[string]any{"text": text, "model_id": model}
	// Flash/Turbo/Multilingual v2 accept an ISO language hint; Kazakh is only
	// covered by eleven_v3, which auto-detects, so we hint Russian only.
	if lang == "ru" && !strings.HasPrefix(model, "eleven_v3") {
		body["language_code"] = "ru"
	}
	vs := map[string]any{"stability": 0.5, "similarity_boost": 0.75}
	if e.Speed > 0 && e.Speed != 1 {
		vs["speed"] = e.Speed
	}
	body["voice_settings"] = vs
	b, _ := json.Marshal(body)
	u := fmt.Sprintf("%s/v1/text-to-speech/%s/stream?output_format=pcm_%d&optimize_streaming_latency=3", e.BaseURL, voice, e.Rate)
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", e.APIKey)
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts request: %w", err)
	}
	if resp.StatusCode != 200 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("elevenlabs tts http %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return resp.Body, nil
}
