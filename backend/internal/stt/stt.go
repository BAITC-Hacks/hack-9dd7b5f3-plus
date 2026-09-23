// Package stt holds the speech-to-text providers behind one interface:
// batch transcription of a PCM16 buffer, and (for ElevenLabs Scribe v2
// Realtime) a streaming session that returns partial transcripts while the
// client is still speaking. Any OpenAI-compatible /audio/transcriptions
// endpoint (OpenAI, geko Seta, speaches/whisper) works as the "openai" kind.
package stt

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// Result is one final transcript.
type Result struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// Provider transcribes a complete utterance.
type Provider interface {
	Name() string
	Transcribe(ctx context.Context, pcm16 []byte, sampleRate int, hint string) (*Result, error)
}

// Realtime is implemented by providers that can stream partial transcripts.
type Realtime interface {
	Provider
	NewSession(ctx context.Context, onPartial func(text string)) (Session, error)
}

// Session is a live transcription stream for one browser session.
type Session interface {
	Send(pcm16 []byte) error
	// Commit finalizes the current utterance and returns its transcript.
	Commit(ctx context.Context) (*Result, error)
	Close() error
}

// ErrNoServerSTT is returned by the browser/mock providers when no audio can
// be transcribed on the server.
var ErrNoServerSTT = errors.New("no server-side STT configured (use browser STT or set STT_PROVIDER)")

// WAV wraps PCM16 mono samples in a RIFF header.
func WAV(pcm []byte, sampleRate int) []byte {
	buf := &bytes.Buffer{}
	dataLen := uint32(len(pcm))
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataLen))
	buf.WriteString("WAVEfmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(buf, binary.LittleEndian, uint16(1)) // mono
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate*2))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, dataLen)
	buf.Write(pcm)
	return buf.Bytes()
}

// ParseWAV extracts PCM16 mono samples and the sample rate from a WAV file.
// Stereo input is down-mixed; other encodings are rejected.
func ParseWAV(b []byte) ([]byte, int, error) {
	if len(b) < 44 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, 0, errors.New("not a RIFF/WAVE file")
	}
	pos := 12
	var channels, bits int
	var rate int
	for pos+8 <= len(b) {
		id := string(b[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(b[pos+4 : pos+8]))
		body := pos + 8
		if id == "fmt " {
			if size < 16 {
				return nil, 0, errors.New("bad fmt chunk")
			}
			format := binary.LittleEndian.Uint16(b[body : body+2])
			channels = int(binary.LittleEndian.Uint16(b[body+2 : body+4]))
			rate = int(binary.LittleEndian.Uint32(b[body+4 : body+8]))
			bits = int(binary.LittleEndian.Uint16(b[body+14 : body+16]))
			if format != 1 || bits != 16 {
				return nil, 0, fmt.Errorf("unsupported WAV encoding (format %d, %d bit); use PCM16", format, bits)
			}
		}
		if id == "data" {
			end := body + size
			if end > len(b) {
				end = len(b)
			}
			data := b[body:end]
			if channels == 2 {
				mono := make([]byte, len(data)/2)
				for i := 0; i+3 < len(data); i += 4 {
					l := int16(binary.LittleEndian.Uint16(data[i:]))
					r := int16(binary.LittleEndian.Uint16(data[i+2:]))
					binary.LittleEndian.PutUint16(mono[i/2:], uint16((int32(l)+int32(r))/2))
				}
				data = mono
			}
			return data, rate, nil
		}
		pos = body + size + (size & 1)
	}
	return nil, 0, errors.New("no data chunk")
}

// Resample converts PCM16 mono between sample rates (linear interpolation).
func Resample(pcm []byte, from, to int) []byte {
	if from == to || from <= 0 || to <= 0 {
		return pcm
	}
	n := len(pcm) / 2
	outN := int(float64(n) * float64(to) / float64(from))
	out := make([]byte, outN*2)
	for i := 0; i < outN; i++ {
		pos := float64(i) * float64(from) / float64(to)
		j := int(pos)
		frac := pos - float64(j)
		a := int16(binary.LittleEndian.Uint16(pcm[2*j:]))
		bv := a
		if j+1 < n {
			bv = int16(binary.LittleEndian.Uint16(pcm[2*(j+1):]))
		}
		v := float64(a)*(1-frac) + float64(bv)*frac
		binary.LittleEndian.PutUint16(out[2*i:], uint16(int16(v)))
	}
	return out
}

// ---------------------------------------------------------------- mock

// Mock returns the transcript hint the caller passed (tests pass the expected
// transcript alongside the audio file) so the whole pipeline runs keyless.
type Mock struct{}

func (Mock) Name() string { return "mock" }
func (Mock) Transcribe(_ context.Context, _ []byte, _ int, hint string) (*Result, error) {
	if strings.TrimSpace(hint) == "" {
		return nil, ErrNoServerSTT
	}
	return &Result{Text: hint, Provider: "mock", Model: "transcript-hint"}, nil
}

// ---------------------------------------------------------------- openai-compatible

// OpenAI posts a WAV to an OpenAI-compatible /audio/transcriptions endpoint.
type OpenAI struct {
	BaseURL  string
	APIKey   string
	Model    string
	Language string
	Prompt   string
	HTTP     *http.Client
}

func NewOpenAI(baseURL, apiKey, model, language, prompt string, timeout time.Duration) *OpenAI {
	return &OpenAI{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Model: model, Language: language, Prompt: prompt, HTTP: &http.Client{Timeout: timeout}}
}

func (o *OpenAI) Name() string { return "openai-compatible" }

func (o *OpenAI) Transcribe(ctx context.Context, pcm []byte, rate int, _ string) (*Result, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, _ := w.CreateFormFile("file", "audio.wav")
	fw.Write(WAV(pcm, rate))
	w.WriteField("model", o.Model)
	w.WriteField("response_format", "json")
	if o.Language != "" {
		w.WriteField("language", o.Language)
	}
	if o.Prompt != "" {
		w.WriteField("prompt", o.Prompt)
	}
	w.Close()
	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/audio/transcriptions", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stt request: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("stt http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		// some servers answer text/plain
		return &Result{Text: strings.TrimSpace(string(b)), Provider: o.Name(), Model: o.Model}, nil
	}
	return &Result{Text: strings.TrimSpace(out.Text), Language: out.Language, Provider: o.Name(), Model: o.Model}, nil
}

// ---------------------------------------------------------------- elevenlabs batch

// ElevenLabs posts a WAV to /v1/speech-to-text (Scribe v2).
type ElevenLabs struct {
	BaseURL  string
	APIKey   string
	Model    string
	Language string
	HTTP     *http.Client
}

func NewElevenLabs(baseURL, apiKey, model, language string, timeout time.Duration) *ElevenLabs {
	if model == "" || model == "scribe_v2_realtime" {
		model = "scribe_v2"
	}
	return &ElevenLabs{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Model: model, Language: language, HTTP: &http.Client{Timeout: timeout}}
}

func (e *ElevenLabs) Name() string { return "elevenlabs" }

func (e *ElevenLabs) Transcribe(ctx context.Context, pcm []byte, rate int, _ string) (*Result, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, _ := w.CreateFormFile("file", "audio.wav")
	fw.Write(WAV(pcm, rate))
	w.WriteField("model_id", e.Model)
	w.WriteField("tag_audio_events", "false")
	w.WriteField("diarize", "false")
	if e.Language != "" {
		w.WriteField("language_code", e.Language)
	}
	w.Close()
	req, err := http.NewRequestWithContext(ctx, "POST", e.BaseURL+"/v1/speech-to-text", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("xi-api-key", e.APIKey)
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stt request: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("elevenlabs stt http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out struct {
		Text         string `json:"text"`
		LanguageCode string `json:"language_code"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("elevenlabs stt: bad json: %w", err)
	}
	return &Result{Text: strings.TrimSpace(out.Text), Language: out.LanguageCode, Provider: e.Name(), Model: e.Model}, nil
}
