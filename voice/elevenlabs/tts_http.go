package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

// VoiceSettings tunes a voice. Nil fields use the voice's stored defaults.
// eleven_v3* models only accept stability 0.0 / 0.5 / 1.0; leave it nil there.
type VoiceSettings struct {
	Stability       *float64 `json:"stability,omitempty"`
	SimilarityBoost *float64 `json:"similarity_boost,omitempty"`
	Style           *float64 `json:"style,omitempty"`
	UseSpeakerBoost *bool    `json:"use_speaker_boost,omitempty"`
	Speed           *float64 `json:"speed,omitempty"`
}

// Float returns a pointer, for VoiceSettings literals.
func Float(v float64) *float64 { return &v }

// Bool returns a pointer, for VoiceSettings literals.
func Bool(v bool) *bool { return &v }

// TTSRequest is one text-to-speech request.
type TTSRequest struct {
	VoiceID       string
	Model         string // eleven_flash_v2_5 (fastest, no Kazakh) | eleven_v3_conversational (Kazakh + Russian)
	Text          string
	Language      string // ISO 639-1, e.g. "ru"; enforces the language on models that support it
	OutputFormat  string // pcm_16000 | pcm_8000 | ulaw_8000 | mp3_22050_32 ...
	PreviousText  string // prosody context
	NextText      string
	Settings      *VoiceSettings
	Normalization string // "auto" | "on" | "off" (apply_text_normalization)
}

// AudioStream is a streaming TTS response; read audio as it arrives.
type AudioStream struct {
	io.ReadCloser
	Started time.Time
	Header  time.Time // response headers received
}

// TTSStream starts HTTP streaming synthesis: the first audio bytes arrive
// after roughly one model TTFB, while the rest is still being generated.
func (c *Client) TTSStream(ctx context.Context, r TTSRequest) (*AudioStream, error) {
	if r.Model == "" {
		r.Model = "eleven_flash_v2_5"
	}
	if r.OutputFormat == "" {
		r.OutputFormat = "pcm_16000"
	}
	body := map[string]any{"text": r.Text, "model_id": r.Model}
	if r.Language != "" {
		body["language_code"] = r.Language
	}
	if r.PreviousText != "" {
		body["previous_text"] = r.PreviousText
	}
	if r.NextText != "" {
		body["next_text"] = r.NextText
	}
	if r.Settings != nil {
		body["voice_settings"] = r.Settings
	}
	if r.Normalization != "" {
		body["apply_text_normalization"] = r.Normalization
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	path := "/v1/text-to-speech/" + url.PathEscape(r.VoiceID) + "/stream?output_format=" + url.QueryEscape(r.OutputFormat)
	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	return &AudioStream{ReadCloser: resp.Body, Started: started, Header: time.Now()}, nil
}

// TTS synthesizes the whole text and returns the audio plus time-to-first-byte.
func (c *Client) TTS(ctx context.Context, r TTSRequest) ([]byte, time.Duration, error) {
	s, err := c.TTSStream(ctx, r)
	if err != nil {
		return nil, 0, err
	}
	defer s.Close()
	var buf bytes.Buffer
	chunk := make([]byte, 16<<10)
	var ttfb time.Duration
	for {
		n, err := s.Read(chunk)
		if n > 0 {
			if ttfb == 0 {
				ttfb = time.Since(s.Started)
			}
			buf.Write(chunk[:n])
		}
		if err == io.EOF {
			return buf.Bytes(), ttfb, nil
		}
		if err != nil {
			return buf.Bytes(), ttfb, err
		}
	}
}

// Transcript is a batch speech-to-text result.
type Transcript struct {
	Text                string  `json:"text"`
	LanguageCode        string  `json:"language_code"`
	LanguageProbability float64 `json:"language_probability"`
	Words               []Word  `json:"words"`
}

// BatchOptions configures batch transcription.
type BatchOptions struct {
	Model    string // default "scribe_v2"
	Language string // "" = auto
}

// Transcribe uploads a whole recording (webm/opus, wav, mp3, ...) to Scribe v2.
// Use it when the client only has a finished recording (e.g. MediaRecorder);
// for live calls use OpenSTT, which is much faster.
func (c *Client) Transcribe(ctx context.Context, filename string, r io.Reader, o BatchOptions) (*Transcript, error) {
	if o.Model == "" {
		o.Model = "scribe_v2"
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("model_id", o.Model)
	if o.Language != "" {
		_ = mw.WriteField("language_code", o.Language)
	}
	_ = mw.WriteField("tag_audio_events", "false")
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, r); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/v1/speech-to-text", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var t Transcript
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Voice is an entry of the account's voice list.
type Voice struct {
	ID       string            `json:"voice_id"`
	Name     string            `json:"name"`
	Category string            `json:"category"`
	Labels   map[string]string `json:"labels"`
}

// Voices lists the voices available to the API key (premade, cloned, library).
func (c *Client) Voices(ctx context.Context) ([]Voice, error) {
	var out struct {
		Voices []Voice `json:"voices"`
	}
	if err := c.getJSON(ctx, "/v1/voices", &out); err != nil {
		return nil, err
	}
	return out.Voices, nil
}

// Subscription reports the plan and character usage (TTS credit guard).
type Subscription struct {
	Tier           string `json:"tier"`
	CharacterCount int    `json:"character_count"`
	CharacterLimit int    `json:"character_limit"`
}

// Remaining characters in the current billing period.
func (s Subscription) Remaining() int { return s.CharacterLimit - s.CharacterCount }

// Subscription returns the current plan usage.
func (c *Client) Subscription(ctx context.Context) (Subscription, error) {
	var s Subscription
	err := c.getJSON(ctx, "/v1/user/subscription", &s)
	return s, err
}

// SingleUseToken mints a short-lived token so a browser can connect to a
// realtime endpoint directly without seeing the API key.
// kind: "realtime_scribe" | "tts_websocket".
func (c *Client) SingleUseToken(ctx context.Context, kind string) (string, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/v1/single-use-token/"+url.PathEscape(kind), bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Token, nil
}
