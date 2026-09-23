package ai

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"hackathon/backend/internal/domain"
)

//go:embed prompts/router.md
var prompt string

type Router interface {
	Route(context.Context, domain.RouteInput, func(domain.Call)) (domain.Decision, error)
	Name() string
	Model() string
}
type Config struct {
	Provider, BaseURL, Key, Model string
	Timeout                       time.Duration
	Strict                        bool
}

func Env(k, def string) string {
	if s := os.Getenv(k); s != "" {
		return s
	}
	return def
}
func ConfigFromEnv() Config {
	c := Config{Provider: Env("LLM_PROVIDER", "mock"), Timeout: 8 * time.Second, Strict: Env("LLM_JSON_SCHEMA", "true") == "true"}
	if d, e := time.ParseDuration(Env("LLM_TIMEOUT", "8s")); e == nil && d > 0 && d <= 30*time.Second {
		c.Timeout = d
	}
	switch c.Provider {
	case "openai":
		c.BaseURL = Env("OPENAI_BASE_URL", "https://api.openai.com/v1")
		c.Key = os.Getenv("OPENAI_API_KEY")
		c.Model = Env("OPENAI_MODEL", "gpt-4.1-mini")
	case "nvidia":
		c.BaseURL = "https://integrate.api.nvidia.com/v1"
		c.Key = os.Getenv("NVIDIA_API_KEY")
		c.Model = Env("NVIDIA_MODEL", "meta/llama-3.3-70b-instruct")
	case "openai_compatible":
		c.BaseURL = os.Getenv("LLM_BASE_URL")
		c.Key = os.Getenv("LLM_API_KEY")
		c.Model = os.Getenv("LLM_MODEL")
	}
	return c
}

type Client struct {
	config  Config
	catalog *domain.Catalog
	http    *http.Client
	system  string
	schema  map[string]any
}

func New(c Config, cat *domain.Catalog) (Router, error) {
	if c.Provider == "mock" {
		return &Mock{catalog: cat}, nil
	}
	if c.Provider != "openai" && c.Provider != "nvidia" && c.Provider != "openai_compatible" {
		return nil, errors.New("unknown LLM_PROVIDER")
	}
	if c.BaseURL == "" || c.Model == "" || (c.Key == "" && c.Provider != "openai_compatible") {
		return nil, errors.New("LLM provider requires base URL, model and API key (local compatible endpoints may omit key)")
	}
	if c.Timeout <= 0 || c.Timeout > 30*time.Second {
		c.Timeout = 8 * time.Second
	}
	schema := Schema(cat)
	js, _ := json.Marshal(schema)
	return &Client{config: c, catalog: cat, http: &http.Client{Timeout: c.Timeout}, schema: schema, system: prompt + "\nSCHEMA:\n" + string(js) + "\nCATALOG:\n" + string(cat.Raw)}, nil
}
func (c *Client) Name() string  { return c.config.Provider }
func (c *Client) Model() string { return c.config.Model }
func (c *Client) Route(ctx context.Context, in domain.RouteInput, observe func(domain.Call)) (domain.Decision, error) {
	// Bound BOTH attempts by one deadline; never double the advertised timeout.
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	history := make([]map[string]any, 0, len(in.History))
	for _, t := range in.History {
		history = append(history, map[string]any{"user": t.Text, "assistant": t.Reply, "scenario_id": t.Decision.ScenarioID, "status": t.Decision.Status})
	}
	user, _ := json.Marshal(map[string]any{"text": in.Text, "history": history, "active": in.Active, "pending": in.Pending})
	messages := []map[string]string{{"role": "system", "content": c.system}, {"role": "user", "content": string(user)}}
	var last error
	for attempt := 1; attempt <= 2; attempt++ {
		if ctx.Err() != nil {
			return domain.Decision{}, ctx.Err()
		}
		format := map[string]any{"type": "json_object"}
		if c.config.Strict {
			format = map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "routing_decision", "strict": true, "schema": c.schema}}
		}
		body, _ := json.Marshal(map[string]any{"model": c.config.Model, "temperature": 0, "max_tokens": 420, "messages": messages, "response_format": format})
		req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.config.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return domain.Decision{}, err
		}
		req.Header.Set("Content-Type", "application/json")
		if c.config.Key != "" {
			req.Header.Set("Authorization", "Bearer "+c.config.Key)
		}
		started := time.Now()
		call := domain.Call{Attempt: attempt, Provider: c.Name(), Model: c.Model()}
		resp, e := c.http.Do(req)
		var decision domain.Decision
		retry := true
		if e != nil {
			last = errors.New("LLM network error or deadline exceeded")
		} else {
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if readErr != nil {
				last = errors.New("LLM response read failed")
			} else if resp.StatusCode != 200 {
				last = fmt.Errorf("LLM HTTP %d", resp.StatusCode)
				retry = resp.StatusCode == 429 || resp.StatusCode >= 500
			} else {
				var out struct {
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
						FinishReason string `json:"finish_reason"`
					} `json:"choices"`
					Usage struct {
						Prompt     int `json:"prompt_tokens"`
						Completion int `json:"completion_tokens"`
						Details    struct {
							Cached int `json:"cached_tokens"`
						} `json:"prompt_tokens_details"`
					} `json:"usage"`
				}
				last = json.Unmarshal(data, &out)
				call.PromptTokens = out.Usage.Prompt
				call.CompletionTokens = out.Usage.Completion
				call.CachedTokens = out.Usage.Details.Cached
				if last == nil {
					if len(out.Choices) == 0 || out.Choices[0].FinishReason == "length" {
						last = errors.New("missing or truncated structured output")
					} else {
						raw := out.Choices[0].Message.Content
						last = DecodeDecision(raw, c.catalog, &decision)
					}
				}
			}
		}
		call.LatencyMS = float64(time.Since(started).Microseconds()) / 1000
		if last != nil {
			call.Error = last.Error()
		}
		slog.Info("llm_call", "provider", call.Provider, "model", call.Model, "attempt", attempt, "latency_ms", call.LatencyMS, "prompt_tokens", call.PromptTokens, "completion_tokens", call.CompletionTokens, "cached_tokens", call.CachedTokens, "error", call.Error)
		observe(call)
		if last == nil {
			return decision, nil
		}
		if !retry {
			break
		}
		messages = append(messages, map[string]string{"role": "user", "content": "The previous attempt failed schema validation or transport. Return exactly one complete object matching SCHEMA, using only catalog IDs."})
	}
	return domain.Decision{}, last
}
func DecodeDecision(raw string, cat *domain.Catalog, d *domain.Decision) error {
	var keys map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &keys) != nil {
		return errors.New("invalid decision JSON")
	}
	for _, k := range []string{"scenario_id", "status", "confidence", "language", "reason", "alternatives", "slots", "pending"} {
		if v, ok := keys[k]; !ok || string(v) == "null" {
			return fmt.Errorf("missing field %s", k)
		}
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(d); err != nil {
		return errors.New("decision schema mismatch")
	}
	return cat.Validate(*d)
}
func Schema(cat *domain.Catalog) map[string]any {
	ids := []string{""}
	for _, s := range cat.Scenarios {
		ids = append(ids, s.ID)
	}
	str := map[string]any{"type": "string"}
	obj := func(p map[string]any, required []string) map[string]any {
		return map[string]any{"type": "object", "properties": p, "required": required, "additionalProperties": false}
	}
	return obj(map[string]any{"scenario_id": map[string]any{"type": "string", "enum": ids}, "status": map[string]any{"type": "string", "enum": []string{"route", "clarify", "handoff"}}, "confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1}, "language": map[string]any{"type": "string", "enum": []string{"ru", "kk", "mixed"}}, "reason": str, "alternatives": map[string]any{"type": "array", "items": obj(map[string]any{"scenario_id": map[string]any{"type": "string", "enum": ids[1:]}, "reason": str}, []string{"scenario_id", "reason"})}, "slots": map[string]any{"type": "array", "items": obj(map[string]any{"name": str, "value": str}, []string{"name", "value"})}, "pending": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": ids[1:]}}}, []string{"scenario_id", "status", "confidence", "language", "reason", "alternatives", "slots", "pending"})
}
