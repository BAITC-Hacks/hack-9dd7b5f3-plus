package speech

import (
	"reflect"
	"strings"
	"testing"
)

func pieces(ps []Piece) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Text
	}
	return out
}

func TestChunkerFirstPieceAtClause(t *testing.T) {
	c := NewChunker()
	var got []Piece
	for _, d := range []string{"Сочувствую, ", "давайте оформим. ", "Назовите госномер машины виновника ", "и дату ДТП."} {
		got = append(got, c.Push(d)...)
	}
	got = append(got, c.Flush()...)
	want := []string{"Сочувствую, ", "давайте оформим. Назовите госномер машины виновника и дату ДТП."}
	if !reflect.DeepEqual(pieces(got), want) {
		t.Fatalf("got %q want %q", pieces(got), want)
	}
	if !got[0].Flush || !got[1].Flush {
		t.Fatal("first piece and final piece must flush")
	}
}

func TestChunkerShortFirstSentence(t *testing.T) {
	c := NewChunker()
	got := c.Push("Да. Полис действует до марта. ")
	if len(got) != 1 || got[0].Text != "Да. " || !got[0].Flush {
		t.Fatalf("got %+v", got)
	}
	// "Полис действует до марта." is under MinNext (30), so it waits for more text
	if rest := c.Push("Могу ещё чем-то помочь? "); len(rest) != 1 || !strings.HasPrefix(rest[0].Text, "Полис") {
		t.Fatalf("rest %+v", rest)
	}
}

func TestChunkerNoSplitInsideNumbers(t *testing.T) {
	c := NewChunker()
	got := c.Push("Стоимость 3.5 тысячи")
	if len(got) != 0 {
		t.Fatalf("must not split inside a number: %+v", got)
	}
	got = append(got, c.Flush()...)
	if len(got) != 1 || got[0].Text != "Стоимость 3.5 тысячи" {
		t.Fatalf("flush %+v", got)
	}
}

func TestChunkerKazakh(t *testing.T) {
	c := NewChunker()
	var got []Piece
	for _, r := range "Түсінемін, рәсімдейік. Кінәлі көліктің нөмірін айтыңызшы." {
		got = append(got, c.Push(string(r))...)
	}
	got = append(got, c.Flush()...)
	if len(got) != 2 || got[0].Text != "Түсінемін, " {
		t.Fatalf("got %q", pieces(got))
	}
}

func TestChunkerEmpty(t *testing.T) {
	c := NewChunker()
	if got := c.Push("   "); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
	if got := c.Flush(); got != nil {
		t.Fatalf("flush %+v", got)
	}
}
