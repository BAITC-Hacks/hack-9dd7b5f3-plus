package router

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var reReplyKey = regexp.MustCompile(`"reply"\s*:\s*"`)

// StreamParser consumes LLM tokens and yields the Decision as soon as the
// JSON prefix before "reply" is complete, then the reply text character by
// character (decoding JSON string escapes on the fly).
type StreamParser struct {
	buf          strings.Builder
	decided      bool
	replyStarted bool
	replyEnded   bool
	pos          int // byte cursor into buf for reply decoding
	pending      string
	decision     *Decision
	Reply        strings.Builder
}

func NewStreamParser() *StreamParser { return &StreamParser{} }

// Feed appends a token. It returns the decision the first time it becomes
// parseable and any newly decoded reply text.
func (p *StreamParser) Feed(delta string) (*Decision, string) {
	p.buf.WriteString(delta)
	var d *Decision
	if !p.decided {
		s := p.buf.String()
		if m := reReplyKey.FindStringIndex(s); m != nil {
			prefix := strings.TrimSpace(s[:m[0]])
			prefix = strings.TrimRight(prefix, ", \n\t")
			if dec, err := parseDecision(closeJSON(prefix)); err == nil {
				p.decided = true
				p.replyStarted = true
				p.decision = dec
				p.pos = m[1]
				d = dec
			}
		}
	}
	var out string
	if p.replyStarted && !p.replyEnded {
		out = p.decodeReply()
	}
	return d, out
}

// decodeReply decodes JSON string content from p.pos until an unescaped quote.
func (p *StreamParser) decodeReply() string {
	s := p.buf.String()
	var out strings.Builder
	i := p.pos
	for i < len(s) {
		c := s[i]
		if c == '"' {
			p.replyEnded = true
			i++
			break
		}
		if c == '\\' {
			if i+1 >= len(s) {
				break // wait for more
			}
			n := s[i+1]
			switch n {
			case 'u':
				if i+6 > len(s) {
					goto done
				}
				if v, err := strconv.ParseUint(s[i+2:i+6], 16, 32); err == nil {
					out.WriteRune(rune(v))
				}
				i += 6
				continue
			case 'n':
				out.WriteByte('\n')
			case 't':
				out.WriteByte(' ')
			case 'r':
			case '"', '\\', '/':
				out.WriteByte(n)
			default:
				out.WriteByte(n)
			}
			i += 2
			continue
		}
		// avoid splitting multi-byte runes: only emit complete UTF-8 sequences
		if c >= 0x80 {
			size := utf8Len(c)
			if i+size > len(s) {
				break
			}
			out.WriteString(s[i : i+size])
			i += size
			continue
		}
		out.WriteByte(c)
		i++
	}
done:
	p.pos = i
	p.Reply.WriteString(out.String())
	return out.String()
}

func utf8Len(b byte) int {
	switch {
	case b&0xE0 == 0xC0:
		return 2
	case b&0xF0 == 0xE0:
		return 3
	case b&0xF8 == 0xF0:
		return 4
	}
	return 1
}

// Finish parses the complete buffer. It repairs truncated JSON (e.g. when a
// stop sequence cut the output before "reply") and falls back to the
// streamed prefix decision.
func (p *StreamParser) Finish() (*Decision, bool, error) {
	raw := strings.TrimSpace(p.buf.String())
	if d, err := parseDecision(raw); err == nil {
		if d.Reply == "" && p.Reply.Len() > 0 {
			d.Reply = p.Reply.String()
		}
		return d, false, nil
	}
	if d, err := parseDecision(closeJSON(strings.TrimRight(raw, ", \n\t"))); err == nil {
		if d.Reply == "" && p.Reply.Len() > 0 {
			d.Reply = p.Reply.String()
		}
		return d, true, nil
	}
	if p.decision != nil {
		d := *p.decision
		d.Reply = p.Reply.String()
		return &d, true, nil
	}
	return nil, true, errors.New("router: could not parse model output as JSON")
}

// Raw returns the accumulated model output.
func (p *StreamParser) Raw() string { return p.buf.String() }

func parseDecision(s string) (*Decision, error) {
	s = stripFence(s)
	start := strings.Index(s, "{")
	if start < 0 {
		return nil, errors.New("no json object")
	}
	s = s[start:]
	var d Decision
	dec := json.NewDecoder(strings.NewReader(s))
	if err := dec.Decode(&d); err != nil {
		return nil, err
	}
	if d.Slots == nil {
		d.Slots = map[string]any{}
	}
	return &d, nil
}

func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}

// closeJSON appends the closing brackets a truncated JSON document is missing.
func closeJSON(s string) string {
	var stack []byte
	inStr := false
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{', '[':
			stack = append(stack, c)
		case '}', ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if inStr {
		s += `"`
	}
	// a dangling key like  ,"actions":  → remove it
	s = strings.TrimRight(s, " \n\t")
	if strings.HasSuffix(s, ":") {
		if i := strings.LastIndex(s, ","); i >= 0 {
			s = s[:i]
		}
	}
	s = strings.TrimRight(s, ", \n\t")
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == '{' {
			s += "}"
		} else {
			s += "]"
		}
	}
	return s
}
