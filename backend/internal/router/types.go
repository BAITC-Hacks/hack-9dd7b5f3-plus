// Package router is the LLM decision layer. It turns one client utterance
// plus the dialogue state into a Decision (scenarios, confidence, slots,
// actions, handoff) and a spoken reply. Two implementations share the same
// contract: LLMRouter (production) and MockRouter (keyless baseline).
package router

import (
	"context"

	"hackathon/backend/internal/llm"
	"hackathon/backend/internal/retrieval"
	"hackathon/backend/internal/triage"
)

// ScenarioPick is one routed scenario with the model's confidence and reason.
type ScenarioPick struct {
	ID         string  `json:"id"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason,omitempty"`
}

// Alt is a rejected alternative shown to the supervisor.
type Alt struct {
	ID         string  `json:"id"`
	Confidence float64 `json:"confidence"`
}

// ActionRequest is a backend action the model wants to run.
type ActionRequest struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
	Mode string         `json:"mode,omitempty"` // execute | preview
}

// Handoff asks to transfer the call to a human queue with context.
type Handoff struct {
	Queue   string `json:"queue"`
	Summary string `json:"summary,omitempty"`
}

// Decision is the router's JSON contract. Field order matters: everything
// before "reply" is streamed first, so the scenario is known before the
// spoken answer is generated.
type Decision struct {
	Language       string          `json:"language"`
	Scenarios      []ScenarioPick  `json:"scenarios"`
	Alternatives   []Alt           `json:"alternatives"`
	IsContinuation bool            `json:"is_continuation"`
	Slots          map[string]any  `json:"slots"`
	Actions        []ActionRequest `json:"actions"`
	Handoff        *Handoff        `json:"handoff"`
	Reply          string          `json:"reply"`
}

// Primary returns the first scenario ID or "".
func (d *Decision) Primary() string {
	if d == nil || len(d.Scenarios) == 0 {
		return ""
	}
	return d.Scenarios[0].ID
}

// Confidence returns the primary confidence.
func (d *Decision) Confidence() float64 {
	if d == nil || len(d.Scenarios) == 0 {
		return 0
	}
	return d.Scenarios[0].Confidence
}

// IDs returns the ordered scenario IDs.
func (d *Decision) IDs() []string {
	if d == nil {
		return nil
	}
	out := make([]string, 0, len(d.Scenarios))
	for _, s := range d.Scenarios {
		out = append(out, s.ID)
	}
	return out
}

// Fact is a prefetched or executed backend action shown to the model.
type Fact struct {
	Name   string         `json:"name"`
	Args   map[string]any `json:"args,omitempty"`
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
	Note   string         `json:"note,omitempty"` // e.g. "EXECUTED after client confirmation"
}

// PendingAction is an irreversible action previewed in the previous turn.
type PendingAction struct {
	Name       string         `json:"name"`
	Args       map[string]any `json:"args"`
	Preview    map[string]any `json:"preview,omitempty"`
	ScenarioID string         `json:"scenario_id"`
}

// TurnView is a recent turn for the prompt.
type TurnView struct {
	Role string `json:"role"` // client | bot
	Text string `json:"text"`
}

// StateView is the dialogue state the router sees.
type StateView struct {
	Client              map[string]any `json:"client,omitempty"`
	ActiveScenario      string         `json:"active_scenario,omitempty"`
	ActiveSlots         map[string]any `json:"active_slots,omitempty"`
	MissingSlots        []string       `json:"missing_slots,omitempty"`
	Stack               []string       `json:"stack,omitempty"`
	PendingConfirmation *PendingAction `json:"pending_confirmation,omitempty"`
	RecentTurns         []TurnView     `json:"recent_turns,omitempty"`
	Language            string         `json:"language,omitempty"`
	ClarifyCount        int            `json:"clarify_count"`
	TurnIndex           int            `json:"turn_index"`
}

// Input is everything a router gets for one turn.
type Input struct {
	Utterance  string
	Signals    triage.Signals
	Candidates []retrieval.Candidate
	State      StateView
	Facts      []Fact
	Today      string
	RouteOnly  bool // evaluation: stop before generating the reply
	FollowUp   bool // second call after actions ran; finish the reply
}

// Sink receives streaming callbacks (all optional).
type Sink struct {
	OnLLMStart   func(model string, messages []llm.Message)
	OnDelta      func(raw string)
	OnDecision   func(d *Decision, routeMs int)
	OnReplyDelta func(text string)
}

// Result is the routing outcome with measurements.
type Result struct {
	Decision *Decision  `json:"decision"`
	Raw      string     `json:"raw,omitempty"`
	Usage    *llm.Usage `json:"usage,omitempty"`
	Model    string     `json:"model"`
	Provider string     `json:"provider"`
	TTFTMs   int        `json:"ttft_ms"`
	RouteMs  int        `json:"route_ms"`
	TotalMs  int        `json:"total_ms"`
	Repaired bool       `json:"repaired,omitempty"` // JSON needed repair
}

// Router is the decision layer contract.
type Router interface {
	Name() string
	Route(ctx context.Context, in Input, sink Sink) (*Result, error)
}
