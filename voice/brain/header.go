package brain

import (
	"math"
	"strconv"
	"strings"
)

// headerMax bounds how far into the reply the routing header may end.
const headerMax = 80

// headerTrim is what may surround the header: whitespace and stray markdown.
const headerTrim = " \t\r\n`*"

// headerParser splits a streamed model reply into its routing header
// ("[[SC12|0.88]]") and the spoken text after it. Text that does not start
// with a header passes through unchanged and without a decision. Extra
// header lines right after the first one (a model listing several intents)
// are dropped.
type headerParser struct {
	buf   string
	found bool // a header has been parsed
	pass  bool // passing text through
}

// feed consumes the next delta. It returns the decision once, when the header
// is complete, and any reply text that can be spoken now.
func (p *headerParser) feed(s string) (*Decision, string) {
	if p.pass {
		return nil, s
	}
	p.buf += s
	return p.sniff(false)
}

// flush ends the stream and returns whatever is still buffered.
func (p *headerParser) flush() (*Decision, string) {
	if p.pass {
		return nil, ""
	}
	return p.sniff(true)
}

func (p *headerParser) sniff(final bool) (*Decision, string) {
	t := strings.TrimLeft(p.buf, headerTrim)
	switch {
	case t == "":
		if final {
			p.passThrough("")
		}
		return nil, ""
	case t[0] != '[' && p.found:
		return nil, p.passThrough(t) // reply after a header: drop the leading space
	case t[0] != '[':
		return nil, p.passThrough(p.buf) // no header: unchanged
	}
	end := strings.IndexByte(t, ']')
	if end < 0 || end >= headerMax {
		if final || len(t) >= headerMax {
			return nil, p.passThrough(p.text(t))
		}
		return nil, "" // wait for the closing bracket
	}
	stop := end + 1
	switch {
	case stop < len(t) && t[stop] == ']':
		stop++
	case stop == len(t) && !final:
		return nil, "" // the second ']' may arrive with the next delta
	}
	d, ok := parseHeader(t[:end])
	if !ok {
		return nil, p.passThrough(p.text(t))
	}
	p.buf = t[stop:]
	if p.found {
		d = nil // an extra header line: drop it silently
	}
	p.found = true
	_, rest := p.sniff(final)
	return d, rest
}

// text is the buffer to pass on: unchanged before a header, trimmed after one.
func (p *headerParser) text(trimmed string) string {
	if p.found {
		return trimmed
	}
	return p.buf
}

// passThrough switches to pass-through mode and returns s.
func (p *headerParser) passThrough(s string) string {
	p.buf, p.pass = "", true
	return s
}

// parseHeader parses the inside of a header, e.g. "[[sc12 | 0.88".
// It tolerates spaces, lowercase ids, a missing confidence and percentages.
func parseHeader(s string) (*Decision, bool) {
	s = strings.TrimSpace(strings.TrimLeft(s, "[ \t"))
	var id, conf string
	if i := strings.IndexByte(s, '|'); i >= 0 {
		id, conf = s[:i], s[i+1:]
	} else if f := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == ',' || r == ';' || r == '\t' }); len(f) > 0 {
		id = f[0]
		if len(f) > 1 {
			conf = f[1]
		}
	}
	id = strings.ToUpper(strings.TrimSpace(id))
	if !validID(id) {
		return nil, false
	}
	d := &Decision{ScenarioID: id}
	conf = strings.TrimSpace(conf)
	pct := strings.HasSuffix(conf, "%")
	conf = strings.Replace(strings.TrimSuffix(conf, "%"), ",", ".", 1)
	if f, err := strconv.ParseFloat(conf, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
		if pct || f > 1 {
			f /= 100
		}
		d.Confidence = min(max(f, 0), 1)
	}
	return d, true
}

// validID reports whether s looks like a scenario id (SC12, SYS_UNCLEAR):
// A-Z, digits and underscores, starting with a letter, with a digit or an
// underscore somewhere.
func validID(s string) bool {
	if len(s) < 2 || len(s) > 40 || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	marked := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9', c == '_':
			marked = true
		default:
			return false
		}
	}
	return marked
}
