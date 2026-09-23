package ai

import (
	"context"
	"sort"
	"strings"
	"unicode"

	"hackathon/backend/internal/domain"
)

// Mock is a deterministic test double, never used as fallback for a real LLM.
// It scores catalog text, not held-out utterances; its accuracy is NOT an LLM result.
type Mock struct{ catalog *domain.Catalog }

func (m *Mock) Name() string  { return "mock" }
func (m *Mock) Model() string { return "catalog-test-double" }
func (m *Mock) Route(ctx context.Context, in domain.RouteInput, _ func(domain.Call)) (domain.Decision, error) {
	if err := ctx.Err(); err != nil {
		return domain.Decision{}, err
	}
	d := domain.Decision{Status: "clarify", Language: "ru", Confidence: 0.3, Reason: "Mock: недостаточно совпадений с текстом каталога.", Alternatives: []domain.Alternative{}, Slots: []domain.Slot{}, Pending: []string{}}
	if strings.ContainsAny(in.Text, "әіңғүұқөһӘІҢҒҮҰҚӨҺ") {
		d.Language = "kk"
	}
	lower := strings.ToLower(in.Text)
	if strings.Contains(lower, "оператор") || strings.Contains(lower, "адаммен") {
		d.Status = "handoff"
		d.Reason = "Mock: клиент просит оператора."
		return d, nil
	}
	words := tokens(lower)
	type candidate struct {
		id    string
		score int
	}
	candidates := []candidate{}
	for _, s := range m.catalog.Scenarios {
		corpus := tokens(strings.ToLower(s.Name + " " + s.Description + " " + strings.Join(s.Examples, " ")))
		score := 0
		for w := range words {
			if corpus[w] {
				score++
			}
		}
		if score > 0 {
			candidates = append(candidates, candidate{s.ID, score})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	for i, c := range candidates {
		if i >= 2 {
			break
		}
		d.Alternatives = append(d.Alternatives, domain.Alternative{ScenarioID: c.id, Reason: "Mock: совпадения с описанием; не оценка LLM."})
	}
	if len(candidates) > 0 && (len(candidates) == 1 || candidates[0].score > candidates[1].score) {
		d.Status = "route"
		d.ScenarioID = candidates[0].id
		d.Confidence = 0.8
		d.Reason = "Mock: детерминированное совпадение с каталогом. Для смыслового выбора включите LLM."
	}
	return d, nil
}
func tokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) }) {
		r := []rune(w)
		if len(r) < 4 {
			continue
		}
		if len(r) > 5 {
			r = r[:5]
		}
		out[string(r)] = true
	}
	return out
}
