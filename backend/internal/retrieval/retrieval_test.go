package retrieval

import (
	"encoding/json"
	"os"
	"testing"

	"hackathon/backend/internal/catalog"
)

// TestLexicalBaseline guards the keyless router's quality: the lexical
// top-1 must match the primary expected scenario for most single-intent
// utterances of the official dev set (the LLM router improves on this).
func TestLexicalBaseline(t *testing.T) {
	cat, err := catalog.Load("../../../data")
	if err != nil {
		t.Fatal(err)
	}
	lex, err := LoadLexicon("../../config/lexicon.json")
	if err != nil {
		t.Fatal(err)
	}
	ix := New(cat, lex)
	b, err := os.ReadFile("../../../data/dev_utterances.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Utterances []struct {
			ID       string   `json:"id"`
			Text     string   `json:"text"`
			Expected []string `json:"expected"`
			Type     string   `json:"type"`
		} `json:"utterances"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	n, hit := 0, 0
	for _, u := range f.Utterances {
		if u.Type != "single" {
			continue
		}
		n++
		c := ix.Search(u.Text, 3)
		if len(c) > 0 && c[0].ID == u.Expected[0] {
			hit++
		} else {
			t.Logf("miss %s: %v", u.ID, c)
		}
	}
	acc := float64(hit) / float64(n)
	t.Logf("lexical top-1 on single-intent dev utterances: %d/%d = %.3f", hit, n, acc)
	if acc < 0.85 {
		t.Fatalf("lexical baseline dropped to %.3f (< 0.85)", acc)
	}
}
