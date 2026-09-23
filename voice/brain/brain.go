// Package brain turns one caller utterance into a routing decision (which of
// the Saqta scenarios applies) and a short spoken reply, streamed as text
// deltas so text-to-speech can start after the first clause.
package brain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Decision is the routing verdict for one turn.
type Decision struct {
	ScenarioID string  `json:"scenario_id"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status,omitempty"` // e.g. route|clarify|handoff (backend)
	Language   string  `json:"language,omitempty"`
	Reason     string  `json:"reason,omitempty"`
}

// Input is one caller utterance and its call context.
type Input struct {
	SessionID   string
	Channel     string    // "web" | "phone" | "twilio" | "demo"
	CallerID    string    // E.164 if known (phone)
	Text        string    // final user utterance
	Lang        string    // reply language chosen by the gateway: "ru" | "kk"
	SpeechEnd   time.Time // when the caller stopped speaking (for backend client_t0)
	STTms       float64
	Speculative bool // true when called on a not-yet-final transcript
}

// Event kinds.
const (
	KindDecision = "decision" // Event.Decision set (at most once per turn, before or during deltas)
	KindDelta    = "delta"    // Event.Text = next piece of the spoken reply (clean text, no header)
	KindRaw      = "raw"      // Event.Raw = an upstream event to relay to the UI unchanged (backend brain)
)

// Event is one streamed piece of a turn.
type Event struct {
	Kind     string
	Text     string
	Decision *Decision
	Raw      json.RawMessage
}

// Result summarizes a finished turn.
type Result struct {
	Model            string
	TTFT             time.Duration // request start -> first reply text delta
	Total            time.Duration
	PromptTokens     int
	CompletionTokens int
	Reply            string // spoken reply text (no header)
	Raw              string // full raw model output (with header) for history
}

// Brain produces a decision and a streamed reply for each caller turn.
type Brain interface {
	Name() string
	// Stateless reports whether the brain keeps history itself and a Turn has
	// no side effects until Commit, so it is safe to call speculatively.
	Stateless() bool
	// Turn handles one utterance; emit is called synchronously from Turn's goroutine.
	Turn(ctx context.Context, in Input, emit func(Event)) (Result, error)
	// Commit records a finished exchange (stateless brains); no-op otherwise.
	Commit(sessionID, user, assistant string)
	// End forgets session state.
	End(sessionID string)
}

// ErrEmptyText is returned by Turn for a blank utterance.
var ErrEmptyText = errors.New("brain: empty utterance")

var (
	_ Brain = (*OpenRouter)(nil)
	_ Brain = (*Backend)(nil)
	_ Brain = Echo{}
)

// replyLang normalizes a language hint to "ru" or "kk" (Russian by default).
func replyLang(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "kk") || strings.HasPrefix(s, "kz") || strings.HasPrefix(s, "kaz") {
		return "kk"
	}
	return "ru"
}
