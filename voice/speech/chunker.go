package speech

import (
	"strings"
	"unicode"
)

// Piece is a speakable part of a reply. Flush marks a sentence end: the TTS
// should generate everything buffered so far.
type Piece struct {
	Text  string
	Flush bool
}

// Chunker turns a stream of LLM text deltas into speakable pieces. The first
// piece is released at the first clause boundary so audio starts early; later
// pieces are cut at sentence ends to keep natural prosody.
type Chunker struct {
	MinFirst int // chars before a clause boundary may end the first piece (default 10)
	MinNext  int // chars before a sentence end may end a later piece (default 30)
	buf      []rune
	emitted  int
}

// NewChunker returns a Chunker with the default thresholds.
func NewChunker() *Chunker { return &Chunker{MinFirst: 10, MinNext: 30} }

// Push adds a delta and returns the pieces that became ready.
func (c *Chunker) Push(delta string) []Piece {
	c.buf = append(c.buf, []rune(delta)...)
	var out []Piece
	for {
		cut, sentence := c.boundary()
		if cut <= 0 {
			return out
		}
		text := string(c.buf[:cut])
		c.buf = c.buf[cut:]
		if strings.TrimSpace(text) == "" {
			continue
		}
		out = append(out, Piece{Text: text, Flush: sentence || c.emitted == 0})
		c.emitted++
	}
}

// Flush returns whatever is left as a final piece.
func (c *Chunker) Flush() []Piece {
	text := string(c.buf)
	c.buf = c.buf[:0]
	if strings.TrimSpace(text) == "" {
		return nil
	}
	c.emitted++
	return []Piece{{Text: text, Flush: true}}
}

// boundary returns the index just after a cut point (including the following
// space) and whether it is a sentence end; 0 if no cut is possible yet. A
// boundary needs the next rune to be whitespace, so "3.5" is never split.
func (c *Chunker) boundary() (int, bool) {
	minFirst, minNext := c.MinFirst, c.MinNext
	if minFirst <= 0 {
		minFirst = 10
	}
	if minNext <= 0 {
		minNext = 30
	}
	for i := 0; i+1 < len(c.buf); i++ {
		if !unicode.IsSpace(c.buf[i+1]) {
			continue
		}
		r := c.buf[i]
		switch {
		case strings.ContainsRune(".!?…", r):
			if c.emitted == 0 || i+1 >= minNext {
				return i + 2, true
			}
		case c.emitted == 0 && strings.ContainsRune(",;:—", r) && i+1 >= minFirst:
			return i + 2, false
		}
	}
	return 0, false
}
