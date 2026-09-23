package brain

import (
	"context"
	"strings"
	"time"
)

// Echo is a keyless Brain that repeats the caller's words, for testing the
// voice pipeline without an LLM.
type Echo struct{}

// Name implements Brain.
func (Echo) Name() string { return "echo" }

// Stateless implements Brain.
func (Echo) Stateless() bool { return true }

// Turn replies "Вы сказали: <text>" (or the Kazakh equivalent) in two deltas.
func (Echo) Turn(ctx context.Context, in Input, emit func(Event)) (Result, error) {
	start := time.Now()
	if err := ctx.Err(); err != nil {
		return Result{Model: "echo"}, err
	}
	if emit == nil {
		emit = func(Event) {}
	}
	lang := replyLang(in.Lang)
	prefix := "Вы сказали: "
	if lang == "kk" {
		prefix = "Сіз айттыңыз: "
	}
	text := strings.TrimSpace(in.Text)
	emit(Event{Kind: KindDecision, Decision: &Decision{ScenarioID: "SYS_UNCLEAR", Confidence: 0.5, Status: "clarify", Language: lang, Reason: "echo brain"}})
	emit(Event{Kind: KindDelta, Text: prefix})
	ttft := time.Since(start)
	emit(Event{Kind: KindDelta, Text: text})
	reply := prefix + text
	return Result{
		Model: "echo",
		TTFT:  ttft,
		Total: time.Since(start),
		Reply: reply,
		Raw:   "[[SYS_UNCLEAR|0.50]]\n" + reply,
	}, nil
}

// Commit implements Brain; Echo keeps no history.
func (Echo) Commit(sessionID, user, assistant string) {}

// End implements Brain.
func (Echo) End(sessionID string) {}
