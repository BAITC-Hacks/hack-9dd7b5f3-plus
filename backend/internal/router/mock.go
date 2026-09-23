package router

import (
	"context"
	"math"
	"strings"
	"time"

	"hackathon/backend/internal/catalog"
	"hackathon/backend/internal/lang"
	"hackathon/backend/internal/retrieval"
)

// MockRouter is the keyless, deterministic router: lexical retrieval over
// the catalog examples plus the triage signals. It exists so reviewers can
// run the whole product without any API key, and it is the fast-path engine.
// It is deliberately simple and its accuracy is reported honestly as the
// baseline the LLM router improves on.
type MockRouter struct {
	cat *catalog.Catalog
	ix  *retrieval.Index
	tpl *Templates
}

func NewMockRouter(cat *catalog.Catalog, ix *retrieval.Index) *MockRouter {
	return &MockRouter{cat: cat, ix: ix, tpl: NewTemplates(cat, ix.ScenarioName)}
}

func (m *MockRouter) Name() string { return "mock-lexical" }

const (
	minSegmentScore    = 0.25
	minSegmentEvidence = 2.6 // ~ one specific word or two medium ones; below it we ask instead of guessing
)

func (m *MockRouter) Route(_ context.Context, in Input, sink Sink) (*Result, error) {
	start := time.Now()
	sig := in.Signals
	replyLang := lang.ReplyLanguage(sig.Language, sig.KKShare, in.State.Language)
	d := &Decision{Language: replyLang, Slots: map[string]any{}, Alternatives: []Alt{}, Actions: []ActionRequest{}}
	for k, v := range sig.Entities {
		d.Slots[k] = v
	}
	active := in.State.ActiveScenario
	oos := m.ix.OutOfScope(in.Utterance)
	top := in.Candidates
	topScore := 0.0
	if len(top) > 0 {
		topScore = top[0].Score
	}

	switch {
	case in.State.PendingConfirmation != nil && sig.Confirmation != "":
		d.IsContinuation = true
		d.Scenarios = []ScenarioPick{{ID: in.State.PendingConfirmation.ScenarioID, Confidence: 0.95, Reason: "confirmation of the previewed action"}}
	case sig.Goodbye && topScore < 0.4:
		d.Scenarios = []ScenarioPick{{ID: "SYS_GOODBYE", Confidence: 0.9, Reason: "client ends the conversation"}}
	case sig.GreetingOnly:
		d.Scenarios = []ScenarioPick{{ID: "SYS_UNCLEAR", Confidence: 0.9, Reason: "greeting only"}}
	case len(oos) > 0 && (topScore < 0.6 || !hasConcept(top)):
		d.Scenarios = []ScenarioPick{{ID: "SYS_OUT_OF_SCOPE", Confidence: 0.85, Reason: "out-of-scope words: " + strings.Join(oos, ", ")}}
	case sig.OperatorRequest && (topScore < 0.5 || (len(top) > 0 && top[0].ID == "SC37")):
		d.Scenarios = []ScenarioPick{{ID: "SC37", Confidence: 0.9, Reason: "explicit operator request"}}
	case active != "" && (len(sig.Entities) > 0 || sig.Confirmation != "" || (len(lang.Tokens(in.Utterance)) <= 4 && topScore < 0.45)):
		d.IsContinuation = true
		d.Scenarios = []ScenarioPick{{ID: active, Confidence: 0.85, Reason: "continuation of the active scenario"}}
	default:
		segs := retrieval.Segments(sig.Normalized, sig.MultiIntentMarkers)
		seen := map[string]bool{}
		for _, seg := range segs {
			cands := m.ix.Search(seg, 5)
			if len(cands) == 0 || cands[0].Score < minSegmentScore || cands[0].Evidence < minSegmentEvidence {
				continue
			}
			c := cands[0]
			if seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			margin := c.Score
			if len(cands) > 1 {
				margin = c.Score - cands[1].Score
			}
			conf := math.Min(0.95, 0.35+0.65*c.Score)
			if margin < 0.12 {
				conf *= 0.8
			}
			d.Scenarios = append(d.Scenarios, ScenarioPick{ID: c.ID, Confidence: round2(conf), Reason: "lexical match: " + strings.Join(c.Terms, ", ")})
		}
		for _, c := range top {
			if !seen[c.ID] && len(d.Alternatives) < 3 {
				d.Alternatives = append(d.Alternatives, Alt{ID: c.ID, Confidence: round2(c.Score)})
			}
		}
		if len(d.Scenarios) == 0 {
			d.Scenarios = []ScenarioPick{{ID: "SYS_UNCLEAR", Confidence: 0.7, Reason: "no confident lexical match"}}
		}
	}

	// reply
	primary := d.Scenarios[0].ID
	switch primary {
	case "SYS_UNCLEAR":
		a, b := "", ""
		if len(d.Alternatives) > 0 {
			a = d.Alternatives[0].ID
		}
		if len(d.Alternatives) > 1 {
			b = d.Alternatives[1].ID
		}
		d.Reply = m.tpl.Clarify(replyLang, a, b)
	case "SYS_OUT_OF_SCOPE", "SYS_GOODBYE":
		d.Reply = m.tpl.Opening(primary, replyLang)
	default:
		if sc := m.cat.Scenario(primary); sc != nil && sc.FastPathEligible {
			d.Reply = m.tpl.Info(primary, replyLang, in.Utterance)
		} else {
			d.Reply = m.tpl.Opening(primary, replyLang)
		}
		if d.IsContinuation && in.State.PendingConfirmation != nil && sig.Confirmation == "yes" {
			d.Reply = map[string]string{"ru": "Готово, выполнила. Чем ещё помочь?", "kk": "Дайын, орындалды. Тағы немен көмектесейін?"}[replyLang]
		} else if d.IsContinuation && in.State.PendingConfirmation != nil && sig.Confirmation == "no" {
			d.Reply = map[string]string{"ru": "Хорошо, ничего не меняю. Чем ещё помочь?", "kk": "Жарайды, ештеңе өзгертпеймін. Тағы немен көмектесейін?"}[replyLang]
		} else if d.IsContinuation && !in.State.PendingConfirmation.isNil() {
			// keep opening
		} else if d.IsContinuation {
			d.Reply = map[string]string{"ru": "Спасибо, записала. Продолжаем.", "kk": "Рақмет, жазып алдым. Жалғастырамыз."}[replyLang]
			if name := identifiedThisTurn(in); name != "" {
				d.Reply = map[string]string{"ru": "Спасибо, " + name + ", нашла ваш профиль. ", "kk": "Рақмет, " + name + ", профиліңізді таптым. "}[replyLang] + m.tpl.Opening(primary, replyLang)
			}
		}
		if len(d.Scenarios) > 1 {
			d.Reply += map[string]string{"ru": " Второй вопрос тоже решим сразу после этого.", "kk": " Екінші сұрағыңызды да осыдан кейін бірден шешеміз."}[replyLang]
		}
		if sc := m.cat.Scenario(primary); sc != nil && sc.Handoff != nil && (sig.OperatorRequest || primary == "SC37") {
			d.Handoff = &Handoff{Queue: sc.Handoff.Queue, Summary: "Client asked for an operator: " + in.Utterance}
		}
	}
	routeMs := int(time.Since(start).Milliseconds())
	if sink.OnDecision != nil {
		sink.OnDecision(d, routeMs)
	}
	if sink.OnReplyDelta != nil && !in.RouteOnly {
		sink.OnReplyDelta(d.Reply)
	}
	return &Result{Decision: d, Model: "lexical-bm25", Provider: "mock", RouteMs: routeMs, TotalMs: routeMs}, nil
}

func (p *PendingAction) isNil() bool { return p == nil }

// identifiedThisTurn returns the client's first name when find_client
// succeeded in this turn's facts.
func identifiedThisTurn(in Input) string {
	for _, f := range in.Facts {
		if f.Name == "find_client" && f.Error == "" {
			if n, ok := f.Result["full_name"].(string); ok {
				return strings.Fields(n)[0]
			}
		}
	}
	return ""
}

// hasConcept reports whether the top candidate matched a domain concept
// (not just generic words).
func hasConcept(c []retrieval.Candidate) bool {
	if len(c) == 0 {
		return false
	}
	for _, t := range c[0].Terms {
		if strings.HasPrefix(t, "#") {
			return true
		}
	}
	return false
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }
