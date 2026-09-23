// Package llm is a minimal streaming client for OpenAI-compatible
// chat-completions endpoints (OpenAI, Groq, OpenRouter, Ollama, Gemini's
// OpenAI endpoint, ...). The router only needs Stream.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Messages    []Message
	Temperature float64
	MaxTokens   int
	Stop        []string
	JSON        bool
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	CachedTokens     int `json:"cached_tokens"`
}

// Chunk is one streamed piece. The last chunk has Done=true and may carry Usage.
type Chunk struct {
	Delta string
	Done  bool
	Usage *Usage
	Err   error
}

type Provider interface {
	Name() string
	Model() string
	Stream(ctx context.Context, req Request) (<-chan Chunk, error)
}

// OpenAI is the OpenAI-compatible client.
type OpenAI struct {
	BaseURL string
	APIKey  string
	ModelID string
	Timeout time.Duration
	HTTP    *http.Client

	mu           sync.Mutex
	useMaxCompl  bool // model rejected max_tokens -> use max_completion_tokens, no temperature
	noJSONMode   bool // endpoint rejected response_format
	noStreamOpts bool // endpoint rejected stream_options
}

func NewOpenAI(baseURL, apiKey, model string, timeout time.Duration) *OpenAI {
	return &OpenAI{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, ModelID: model, Timeout: timeout,
		HTTP: &http.Client{Timeout: timeout + 5*time.Second}}
}

func (o *OpenAI) Name() string  { return "openai-compatible" }
func (o *OpenAI) Model() string { return o.ModelID }

func (o *OpenAI) body(req Request) map[string]any {
	o.mu.Lock()
	defer o.mu.Unlock()
	m := map[string]any{"model": o.ModelID, "messages": req.Messages, "stream": true}
	if !o.noStreamOpts {
		m["stream_options"] = map[string]any{"include_usage": true}
	}
	if o.useMaxCompl {
		if req.MaxTokens > 0 {
			m["max_completion_tokens"] = req.MaxTokens
		}
	} else {
		if req.MaxTokens > 0 {
			m["max_tokens"] = req.MaxTokens
		}
		m["temperature"] = req.Temperature
	}
	if len(req.Stop) > 0 {
		m["stop"] = req.Stop
	}
	if req.JSON && !o.noJSONMode {
		m["response_format"] = map[string]any{"type": "json_object"}
	}
	return m
}

// Stream sends the request and returns a channel of chunks. It adapts once to
// common parameter incompatibilities (max_completion_tokens, response_format,
// stream_options) and retries once on a network error before the first token.
func (o *OpenAI) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	var resp *http.Response
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		resp, err = o.do(ctx, req)
		if err == nil {
			break
		}
		var pe *paramError
		if errors.As(err, &pe) {
			o.mu.Lock()
			switch pe.kind {
			case "max_completion_tokens":
				o.useMaxCompl = true
			case "response_format":
				o.noJSONMode = true
			case "stream_options":
				o.noStreamOpts = true
			}
			o.mu.Unlock()
			continue
		}
		if ctx.Err() != nil || attempt >= 1 {
			return nil, err
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		return nil, err
	}
	ch := make(chan Chunk, 64)
	go o.read(ctx, resp, ch)
	return ch, nil
}

type paramError struct {
	kind string
	msg  string
}

func (p *paramError) Error() string { return p.msg }

func (o *OpenAI) do(ctx context.Context, req Request) (*http.Response, error) {
	buf, _ := json.Marshal(o.body(req))
	hreq, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "text/event-stream")
	if o.APIKey != "" {
		hreq.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	resp, err := o.HTTP.Do(hreq)
	if err != nil {
		return nil, fmt.Errorf("llm request: %w", err)
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		msg := string(b)
		lower := strings.ToLower(msg)
		switch {
		case strings.Contains(lower, "max_completion_tokens") || strings.Contains(lower, "temperature") && strings.Contains(lower, "unsupported"):
			return nil, &paramError{kind: "max_completion_tokens", msg: msg}
		case strings.Contains(lower, "response_format"):
			return nil, &paramError{kind: "response_format", msg: msg}
		case strings.Contains(lower, "stream_options"):
			return nil, &paramError{kind: "stream_options", msg: msg}
		}
		return nil, fmt.Errorf("llm http %d: %s", resp.StatusCode, strings.TrimSpace(msg))
	}
	return resp, nil
}

type sseChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (o *OpenAI) read(ctx context.Context, resp *http.Response, ch chan<- Chunk) {
	defer close(ch)
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var usage *Usage
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var c sseChunk
		if err := json.Unmarshal([]byte(data), &c); err != nil {
			continue
		}
		if c.Error != nil {
			ch <- Chunk{Err: errors.New(c.Error.Message), Done: true}
			return
		}
		if c.Usage != nil {
			usage = &Usage{PromptTokens: c.Usage.PromptTokens, CompletionTokens: c.Usage.CompletionTokens}
			if c.Usage.PromptTokensDetails != nil {
				usage.CachedTokens = c.Usage.PromptTokensDetails.CachedTokens
			}
		}
		for _, choice := range c.Choices {
			if choice.Delta.Content != "" {
				select {
				case ch <- Chunk{Delta: choice.Delta.Content}:
				case <-ctx.Done():
					return
				}
			}
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		ch <- Chunk{Err: err, Done: true, Usage: usage}
		return
	}
	ch <- Chunk{Done: true, Usage: usage}
}

// Collect drains a stream into a string (for non-streaming callers).
func Collect(ch <-chan Chunk) (string, *Usage, error) {
	var b strings.Builder
	var usage *Usage
	for c := range ch {
		if c.Err != nil {
			return b.String(), usage, c.Err
		}
		b.WriteString(c.Delta)
		if c.Usage != nil {
			usage = c.Usage
		}
	}
	return b.String(), usage, nil
}
