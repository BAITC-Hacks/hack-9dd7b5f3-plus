package router

import (
	"fmt"
	"sort"
	"strings"

	"hackathon/backend/internal/catalog"
	"hackathon/backend/internal/lang"
)

// Policy holds the deterministic decision rules applied around the model.
type Policy struct {
	ProceedMin    float64
	ClarifyMin    float64
	FastMinScore  float64
	FastMinMargin float64
	FastPathMode  string // on | shadow | off
}

// FastPathCheck explains whether the fast path was taken.
type FastPathCheck struct {
	Eligible  bool    `json:"eligible"`
	Reason    string  `json:"reason"`
	Candidate string  `json:"candidate,omitempty"`
	Score     float64 `json:"score,omitempty"`
	Margin    float64 `json:"margin,omitempty"`
}

// FastPath decides whether an utterance is obvious enough to answer from the
// lexical shortlist without the LLM: informational scenario, a clear
// lexical winner, no multi-intent / urgency / confirmation context.
func (p Policy) FastPath(cat *catalog.Catalog, in Input) FastPathCheck {
	if p.FastPathMode == "off" {
		return FastPathCheck{Reason: "fast path disabled"}
	}
	if len(in.Candidates) == 0 {
		return FastPathCheck{Reason: "no lexical candidates"}
	}
	top := in.Candidates[0]
	margin := top.Score
	if len(in.Candidates) > 1 {
		margin = top.Score - in.Candidates[1].Score
	}
	fc := FastPathCheck{Candidate: top.ID, Score: top.Score, Margin: margin}
	sc := cat.Scenario(top.ID)
	sig := in.Signals
	switch {
	case sc == nil:
		fc.Reason = "top candidate unknown"
	case !sc.FastPathEligible:
		fc.Reason = fmt.Sprintf("%s is not fast_path_eligible", top.ID)
	case in.State.PendingConfirmation != nil:
		fc.Reason = "pending confirmation"
	case in.State.ActiveScenario != "" && len(in.State.MissingSlots) > 0:
		fc.Reason = "active scenario is collecting slots"
	case len(sig.MultiIntentMarkers) > 0:
		fc.Reason = "multi-intent markers present"
	case len(sig.Urgent) > 0:
		fc.Reason = "urgency markers present"
	case len(sig.OutOfScopeHints) > 0:
		fc.Reason = "out-of-scope words present"
	case sig.GreetingOnly:
		fc.Reason = "greeting only"
	case sig.OperatorRequest && top.ID == "SC37":
		fc.Eligible = true
		fc.Reason = "explicit operator request matches SC37"
	case top.Score < p.FastMinScore:
		fc.Reason = fmt.Sprintf("score %.2f < %.2f", top.Score, p.FastMinScore)
	case margin < p.FastMinMargin:
		fc.Reason = fmt.Sprintf("margin %.2f < %.2f", margin, p.FastMinMargin)
	case len(lang.Tokens(in.Utterance)) > 14:
		fc.Reason = "utterance too long for the fast path"
	default:
		fc.Eligible = true
		fc.Reason = fmt.Sprintf("clear lexical winner %s (score %.2f, margin %.2f), informational scenario", top.ID, top.Score, margin)
	}
	return fc
}

// Verdict is the policy outcome shown in the trace.
type Verdict struct {
	Action     string   `json:"action"` // proceed | clarify | handoff | out_of_scope | goodbye
	Confidence float64  `json:"confidence"`
	Notes      []string `json:"notes,omitempty"`
}

// Apply validates and normalizes a decision: unknown IDs are dropped, urgent
// scenarios move first, very low confidence becomes SYS_UNCLEAR, and the
// clarify/handoff action is derived from the thresholds.
func (p Policy) Apply(cat *catalog.Catalog, d *Decision, st StateView) Verdict {
	v := Verdict{}
	// 1. validate IDs
	kept := d.Scenarios[:0]
	seen := map[string]bool{}
	for _, s := range d.Scenarios {
		id := strings.ToUpper(strings.TrimSpace(s.ID))
		s.ID = id
		if !cat.IsKnown(id) {
			v.Notes = append(v.Notes, "dropped unknown scenario "+id)
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		kept = append(kept, s)
	}
	d.Scenarios = kept
	alts := d.Alternatives[:0]
	for _, a := range d.Alternatives {
		a.ID = strings.ToUpper(strings.TrimSpace(a.ID))
		if cat.IsKnown(a.ID) && !seen[a.ID] {
			alts = append(alts, a)
		}
	}
	d.Alternatives = alts
	if len(d.Scenarios) == 0 {
		d.Scenarios = []ScenarioPick{{ID: "SYS_UNCLEAR", Confidence: 0.5, Reason: "no valid scenario returned"}}
		v.Notes = append(v.Notes, "no valid scenario → SYS_UNCLEAR")
	}
	// 2. urgent first (stable)
	sort.SliceStable(d.Scenarios, func(i, j int) bool {
		pi, pj := 2, 2
		if s := cat.Scenario(d.Scenarios[i].ID); s != nil && s.Priority == "urgent" {
			pi = 0
		}
		if s := cat.Scenario(d.Scenarios[j].ID); s != nil && s.Priority == "urgent" {
			pj = 0
		}
		return pi < pj
	})
	// 3. thresholds
	primary := d.Scenarios[0]
	v.Confidence = primary.Confidence
	switch primary.ID {
	case "SYS_UNCLEAR":
		v.Action = "clarify"
		if st.ClarifyCount >= 2 {
			v.Action = "handoff"
			v.Notes = append(v.Notes, "third clarification → hand off to an operator")
			if d.Handoff == nil {
				d.Handoff = &Handoff{Queue: "operator_general", Summary: "Client's request could not be clarified after several attempts."}
			}
		}
	case "SYS_OUT_OF_SCOPE":
		v.Action = "out_of_scope"
	case "SYS_GOODBYE":
		v.Action = "goodbye"
	default:
		switch {
		case d.Handoff != nil && d.Handoff.Queue != "":
			v.Action = "handoff"
		case primary.Confidence < p.ClarifyMin && !d.IsContinuation:
			v.Action = "clarify"
			v.Notes = append(v.Notes, fmt.Sprintf("confidence %.2f < %.2f → treated as unclear", primary.Confidence, p.ClarifyMin))
			d.Alternatives = append([]Alt{{ID: primary.ID, Confidence: primary.Confidence}}, d.Alternatives...)
			d.Scenarios = []ScenarioPick{{ID: "SYS_UNCLEAR", Confidence: 1 - primary.Confidence, Reason: primary.Reason}}
		case primary.Confidence < p.ProceedMin && !d.IsContinuation:
			v.Action = "clarify"
			v.Notes = append(v.Notes, fmt.Sprintf("confidence %.2f < %.2f → keep scenario, ask to confirm", primary.Confidence, p.ProceedMin))
		default:
			v.Action = "proceed"
		}
	}
	// a handoff attached to a system intent is still a handoff
	if d.Handoff != nil && d.Handoff.Queue != "" && v.Action != "goodbye" {
		v.Action = "handoff"
	}
	return v
}
