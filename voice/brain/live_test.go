package brain

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLiveOpenRouterKazakh calls the real OpenRouter API. It runs only with
// VOICE_LIVE=1 and OPENROUTER_API_KEY set.
func TestLiveOpenRouterKazakh(t *testing.T) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if os.Getenv("VOICE_LIVE") != "1" || key == "" {
		t.Skip("set VOICE_LIVE=1 and OPENROUTER_API_KEY to run")
	}
	data, err := LoadDataset(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Logf("starter kit not loaded, using the generic prompt: %v", err)
	}
	o := NewOpenRouter(key, "", "google/gemini-2.5-flash-lite", nil, data)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := o.Warm(ctx); err != nil {
		t.Fatalf("warm: %v", err)
	}

	var dec *Decision
	res, err := o.Turn(ctx, Input{
		SessionID: "live-kk",
		Channel:   "demo",
		Text:      "Сәлеметсіз бе, полисімнің мерзімін ұзартқым келеді",
		Lang:      "kk",
		SpeechEnd: time.Now(),
	}, func(e Event) {
		if e.Kind == KindDecision {
			dec = e.Decision
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if dec == nil {
		t.Fatalf("no decision; raw output: %q", res.Raw)
	}
	if res.Reply == "" {
		t.Fatalf("empty reply; raw output: %q", res.Raw)
	}
	t.Logf("decision %s (%.2f, %s) model %s", dec.ScenarioID, dec.Confidence, dec.Status, res.Model)
	t.Logf("TTFT %v, total %v, tokens %d prompt / %d completion", res.TTFT, res.Total, res.PromptTokens, res.CompletionTokens)
	t.Logf("reply: %s", res.Reply)
}
