package brain

import (
	"context"
	"slices"
	"testing"
)

func TestEcho(t *testing.T) {
	var b Brain = Echo{}
	if b.Name() != "echo" || !b.Stateless() {
		t.Fatal("echo must be a stateless brain named echo")
	}
	for _, tc := range []struct{ lang, prefix string }{{"ru", "Вы сказали: "}, {"kk", "Сіз айттыңыз: "}} {
		emit, evs := record()
		res, err := b.Turn(context.Background(), Input{Text: "полис", Lang: tc.lang}, emit)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := kindsOf(*evs), []string{KindDecision, KindDelta, KindDelta}; !slices.Equal(got, want) {
			t.Fatalf("event kinds = %v, want %v", got, want)
		}
		if d := (*evs)[0].Decision; d.ScenarioID != "SYS_UNCLEAR" || d.Confidence != 0.5 {
			t.Errorf("decision = %+v", d)
		}
		if (*evs)[1].Text != tc.prefix || (*evs)[2].Text != "полис" || res.Reply != tc.prefix+"полис" {
			t.Errorf("%s reply = %q", tc.lang, res.Reply)
		}
	}
}
