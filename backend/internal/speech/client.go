package speech

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hackathon/backend/internal/ai"
)

type Client struct {
	Key, Base, STTModel, TTSModel, Voice, RealtimeModel string
	HTTP                                                *http.Client
	Enabled                                             bool
}

func New() *Client {
	return &Client{Key: os.Getenv("OPENAI_API_KEY"), Base: ai.Env("OPENAI_BASE_URL", "https://api.openai.com/v1"), STTModel: ai.Env("STT_MODEL", "gpt-4o-mini-transcribe"), TTSModel: ai.Env("TTS_MODEL", "gpt-4o-mini-tts"), Voice: ai.Env("TTS_VOICE", "coral"), RealtimeModel: ai.Env("REALTIME_MODEL", "gpt-live-transcribe"), HTTP: &http.Client{Timeout: 30 * time.Second}, Enabled: ai.Env("SPEECH_PROVIDER", "browser") == "openai"}
}
func (c *Client) request(ctx context.Context, path, ct string, b io.Reader) (*http.Response, error) {
	if !c.Enabled || c.Key == "" {
		return nil, errors.New("speech is unavailable: set SPEECH_PROVIDER=openai and OPENAI_API_KEY")
	}
	req, e := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.Base, "/")+path, b)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", ct)
	resp, e := c.HTTP.Do(req)
	if e != nil {
		return nil, errors.New("speech network error or timeout")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("speech provider HTTP %d", resp.StatusCode)
	}
	return resp, nil
}
func (c *Client) Transcribe(ctx context.Context, name string, audio io.Reader) (string, error) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, e := w.CreateFormFile("file", filepath.Base(name))
	if e != nil {
		return "", e
	}
	if _, e = io.Copy(part, audio); e != nil {
		return "", e
	}
	_ = w.WriteField("model", c.STTModel)
	_ = w.WriteField("response_format", "json")
	_ = w.WriteField("prompt", "Страховая компания. Русская и казахская речь, включая смешанные фразы. Сақтандыру, полис, өтемақы. Не переводить.")
	w.Close()
	resp, e := c.request(ctx, "/audio/transcriptions", w.FormDataContentType(), &b)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	var result struct {
		Text string `json:"text"`
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); e != nil {
		return "", errors.New("invalid STT response")
	}
	if strings.TrimSpace(result.Text) == "" {
		return "", errors.New("speech was empty")
	}
	return result.Text, nil
}
func (c *Client) Speak(ctx context.Context, text string) (*http.Response, error) {
	b, _ := json.Marshal(map[string]any{"model": c.TTSModel, "voice": c.Voice, "input": text, "response_format": "pcm"})
	return c.request(ctx, "/audio/speech", "application/json", bytes.NewReader(b))
}
func (c *Client) Connect(ctx context.Context, sdp string) (string, error) {
	transcription := map[string]any{"model": c.RealtimeModel, "prompt": "Russian and Kazakh insurance support conversation. Preserve the spoken language, do not translate."}
	if c.RealtimeModel == "gpt-live-transcribe" {
		transcription["delay"] = "low"
	}
	session := map[string]any{"type": "transcription", "audio": map[string]any{"input": map[string]any{"transcription": transcription, "turn_detection": nil}}}
	data, _ := json.Marshal(session)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("sdp", sdp)
	_ = w.WriteField("session", string(data))
	w.Close()
	resp, e := c.request(ctx, "/realtime/calls", w.FormDataContentType(), &body)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	answer, e := io.ReadAll(io.LimitReader(resp.Body, 128<<10))
	return string(answer), e
}
