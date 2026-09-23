package router

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"hackathon/backend/internal/catalog"
	"hackathon/backend/internal/llm"
)

// LLMRouter is the production decision layer: one streaming chat completion
// whose JSON output carries the decision first and the spoken reply last.
type LLMRouter struct {
	cat         *catalog.Catalog
	provider    llm.Provider
	system      string
	Temperature float64
	MaxTokens   int
	JSONMode    bool
}

func NewLLMRouter(cat *catalog.Catalog, p llm.Provider, temperature float64, maxTokens int, jsonMode bool) *LLMRouter {
	return &LLMRouter{cat: cat, provider: p, system: BuildSystemPrompt(cat), Temperature: temperature, MaxTokens: maxTokens, JSONMode: jsonMode}
}

func (r *LLMRouter) Name() string { return "llm:" + r.provider.Model() }

// SystemPrompt exposes the cached prompt (for the debug page).
func (r *LLMRouter) SystemPrompt() string { return r.system }

// RefreshPrompt rebuilds the system prompt after a catalog reload.
func (r *LLMRouter) RefreshPrompt() { r.system = BuildSystemPrompt(r.cat) }

func (r *LLMRouter) Route(ctx context.Context, in Input, sink Sink) (*Result, error) {
	messages := BuildMessages(r.system, in)
	req := llm.Request{Messages: messages, Temperature: r.Temperature, MaxTokens: r.MaxTokens, JSON: r.JSONMode}
	if in.RouteOnly {
		req.Stop = []string{`"reply"`}
		req.MaxTokens = 350
	}
	if sink.OnLLMStart != nil {
		sink.OnLLMStart(r.provider.Model(), messages)
	}
	start := time.Now()
	ch, err := r.provider.Stream(ctx, req)
	if err != nil {
		return nil, err
	}
	parser := NewStreamParser()
	res := &Result{Model: r.provider.Model(), Provider: r.provider.Name()}
	decided := false
	first := true
	for c := range ch {
		if c.Err != nil {
			if parser.Raw() == "" {
				return nil, c.Err
			}
			break
		}
		if c.Usage != nil {
			res.Usage = c.Usage
		}
		if c.Delta == "" {
			continue
		}
		if first {
			first = false
			res.TTFTMs = int(time.Since(start).Milliseconds())
		}
		if sink.OnDelta != nil {
			sink.OnDelta(c.Delta)
		}
		d, replyDelta := parser.Feed(c.Delta)
		if d != nil && !decided {
			decided = true
			res.RouteMs = int(time.Since(start).Milliseconds())
			if sink.OnDecision != nil {
				sink.OnDecision(d, res.RouteMs)
			}
		}
		if replyDelta != "" && sink.OnReplyDelta != nil && !in.RouteOnly {
			sink.OnReplyDelta(replyDelta)
		}
	}
	res.TotalMs = int(time.Since(start).Milliseconds())
	res.Raw = parser.Raw()
	final, repaired, perr := parser.Finish()
	if perr != nil {
		return res, fmt.Errorf("%w (raw: %s)", perr, truncate(res.Raw, 300))
	}
	res.Repaired = repaired
	if !decided {
		res.RouteMs = res.TotalMs
		if sink.OnDecision != nil {
			sink.OnDecision(final, res.RouteMs)
		}
		if final.Reply != "" && sink.OnReplyDelta != nil && !in.RouteOnly {
			sink.OnReplyDelta(final.Reply)
		}
	}
	if final.Language == "" {
		final.Language = "ru"
	}
	res.Decision = final
	if len(final.Scenarios) == 0 {
		return res, errors.New("router: model returned no scenarios")
	}
	return res, nil
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
