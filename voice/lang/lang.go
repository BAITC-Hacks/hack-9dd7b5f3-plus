// Package lang tags Russian/Kazakh words in a caller's utterance and
// decides which language the voice robot should reply in.
//
// Callers to the insurance contact center speak Russian, Kazakh, or
// switch between the two mid-phrase; both languages share the Cyrillic
// script, so words are classified letter-by-letter rather than by
// script alone.
package lang

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lang identifies the language of a word, an utterance, or a reply.
type Lang string

const (
	RU      Lang = "ru"
	KK      Lang = "kk"
	Mixed   Lang = "mixed"
	Unknown Lang = ""
)

// WordLang tags one word. Rules, in order:
//
//  1. strip non-letters, lowercase; empty -> Unknown
//  2. any Kazakh-only letter (ә ғ қ ң ө ұ ү һ і, any case) -> KK
//  3. no Cyrillic letter at all (Latin, digits) -> Unknown
//  4. word is in a built-in list of frequent Kazakh words spelled with
//     Russian-alphabet letters only -> KK
//  5. Kazakh person/case suffixes on words with >= 5 letters, or the
//     instrumental -мен/-пен/-бен on words with >= 6 letters (excluding a
//     small set of Russian look-alikes) -> KK
//  6. otherwise RU
func WordLang(w string) Lang {
	cleaned := cleanWord(w)
	if cleaned == "" {
		return Unknown
	}

	hasCyrillic := false
	for _, r := range cleaned {
		if kazakhOnlyLetters[r] {
			return KK
		}
		if unicode.Is(unicode.Cyrillic, r) {
			hasCyrillic = true
		}
	}
	if !hasCyrillic {
		return Unknown
	}

	if kazakhWordsRU[cleaned] {
		return KK
	}

	n := utf8.RuneCountInString(cleaned)
	if n >= 5 && hasAnySuffix(cleaned, kazakhSuffixes) {
		return KK
	}
	if n >= 6 && hasAnySuffix(cleaned, instrumentalSuffixes) && !instrumentalExceptions[cleaned] {
		return KK
	}

	return RU
}

// cleanWord strips everything that is not a letter and lowercases the rest.
func cleanWord(w string) string {
	var b strings.Builder
	for _, r := range w {
		if unicode.IsLetter(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// hasAnySuffix reports whether s ends with any of the given suffixes.
func hasAnySuffix(s string, suffixes []string) bool {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

// Tagged is one word together with its detected language.
type Tagged struct {
	Word string `json:"w"`
	Lang Lang   `json:"lang"`
}

// Mix summarizes the language makeup of an utterance.
type Mix struct {
	KK       int      `json:"kk"`
	RU       int      `json:"ru"`
	Share    float64  `json:"kk_share"`  // KK / (KK+RU), 0 when no tagged words
	Dominant Lang     `json:"dominant"`  // KK if Share >= 0.6, RU if Share <= 0.4, Mixed otherwise, Unknown if no tagged words
	Strong   bool     `json:"strong_kk"` // at least one word contains a Kazakh-only letter
	Words    []Tagged `json:"words,omitempty"`
}

// boundaryRunes are punctuation marks that split words in addition to
// whitespace.
var boundaryRunes = map[rune]bool{
	',': true, '.': true, '!': true, '?': true, ';': true, ':': true,
	'(': true, ')': true, '"': true, '«': true, '»': true, '…': true,
	'—': true, '-': true,
}

func isBoundary(r rune) bool {
	return unicode.IsSpace(r) || boundaryRunes[r]
}

// Analyze splits text on whitespace and punctuation (, . ! ? ; : ( ) " «
// » … — -) and tags each word; words tagged Unknown and single-letter
// words (я, а, и, в, с, у, к, о) are ignored.
func Analyze(text string) Mix {
	var m Mix

	for _, tok := range strings.FieldsFunc(text, isBoundary) {
		wl := WordLang(tok)
		if wl == Unknown {
			continue
		}
		cleaned := cleanWord(tok)
		if ignoredWords[cleaned] {
			continue
		}

		switch wl {
		case KK:
			m.KK++
		case RU:
			m.RU++
		}
		if !m.Strong {
			for _, r := range cleaned {
				if kazakhOnlyLetters[r] {
					m.Strong = true
					break
				}
			}
		}
		m.Words = append(m.Words, Tagged{Word: tok, Lang: wl})
	}

	if total := m.KK + m.RU; total > 0 {
		m.Share = float64(m.KK) / float64(total)
		switch {
		case m.Share >= 0.6:
			m.Dominant = KK
		case m.Share <= 0.4:
			m.Dominant = RU
		default:
			m.Dominant = Mixed
		}
	}

	return m
}

// Language is the utterance language for UI/trace: Unknown if no words;
// Mixed if both KK and RU words are present and the minority language has
// at least 2 words or at least 20% of words; otherwise Dominant (KK or
// RU).
func (m Mix) Language() Lang {
	total := m.KK + m.RU
	if total == 0 {
		return Unknown
	}
	if m.KK > 0 && m.RU > 0 {
		minority := m.KK
		if m.RU < minority {
			minority = m.RU
		}
		if minority >= 2 || float64(minority)/float64(total) >= 0.2 {
			return Mixed
		}
	}
	return m.Dominant
}

// ExplicitRequest detects a request to switch language, case-insensitive.
// If both a Kazakh and a Russian trigger phrase appear, the later
// occurrence in the text wins. Returns Unknown when neither appears.
func ExplicitRequest(text string) Lang {
	lower := strings.ToLower(text)

	kkPos := lastIndexOfAny(lower, kkTriggers)
	ruPos := lastIndexOfAny(lower, ruTriggers)

	switch {
	case kkPos < 0 && ruPos < 0:
		return Unknown
	case ruPos < 0 || kkPos > ruPos:
		return KK
	default:
		return RU
	}
}

// lastIndexOfAny returns the rightmost starting index, in s, of any of the
// given phrases, or -1 if none occur.
func lastIndexOfAny(s string, phrases []string) int {
	best := -1
	for _, p := range phrases {
		if i := strings.LastIndex(s, p); i > best {
			best = i
		}
	}
	return best
}

// FromSTT maps a speech-to-text language code to a Lang, case-insensitive.
// Unrecognized codes return Unknown.
func FromSTT(code string) Lang {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "kk", "kaz", "kazakh":
		return KK
	case "ru", "rus", "russian":
		return RU
	default:
		return Unknown
	}
}

// Policy decides the reply language for a session.
type Policy struct {
	Current   Lang    // session reply language, "" until decided
	Threshold float64 // default 0.6
	MinWords  int     // tagged words needed to switch by dominance once a session language exists (default 2)
}

// Choice is the outcome of one language decision, with the evidence
// behind it.
type Choice struct {
	Reply    Lang   `json:"reply"`    // always RU or KK
	Detected Lang   `json:"detected"` // Mix.Language()
	Rule     string `json:"rule"`     // "explicit" | "dominant" | "sticky" | "stt" | "any-kazakh" | "default"
	Mix      Mix    `json:"mix"`
}

const (
	defaultThreshold = 0.6
	defaultMinWords  = 2
)

// threshold returns p.Threshold, or its default when unset.
func (p Policy) threshold() float64 {
	if p.Threshold == 0 {
		return defaultThreshold
	}
	return p.Threshold
}

// minWords returns p.MinWords, or its default when unset.
func (p Policy) minWords() int {
	if p.MinWords == 0 {
		return defaultMinWords
	}
	return p.MinWords
}

// Peek computes the choice without changing Current. Rules in order:
//
//  1. ExplicitRequest != Unknown -> that language, rule "explicit"
//  2. if tagged words >= MinWords, or (Current == Unknown and tagged
//     words >= 1): Share >= Threshold -> KK "dominant"; Share <=
//     1-Threshold -> RU "dominant"
//  3. Current != Unknown -> Current, "sticky"
//  4. FromSTT(sttCode) != Unknown -> it, "stt"
//  5. any KK word -> KK "any-kazakh"; else RU "default"
func (p Policy) Peek(text, sttCode string) Choice {
	mix := Analyze(text)
	detected := mix.Language()

	if explicit := ExplicitRequest(text); explicit != Unknown {
		return Choice{Reply: explicit, Detected: detected, Rule: "explicit", Mix: mix}
	}

	tagged := mix.KK + mix.RU
	if tagged >= p.minWords() || (p.Current == Unknown && tagged >= 1) {
		threshold := p.threshold()
		switch {
		case mix.Share >= threshold:
			return Choice{Reply: KK, Detected: detected, Rule: "dominant", Mix: mix}
		case mix.Share <= 1-threshold:
			return Choice{Reply: RU, Detected: detected, Rule: "dominant", Mix: mix}
		}
	}

	if p.Current != Unknown {
		return Choice{Reply: p.Current, Detected: detected, Rule: "sticky", Mix: mix}
	}

	if stt := FromSTT(sttCode); stt != Unknown {
		return Choice{Reply: stt, Detected: detected, Rule: "stt", Mix: mix}
	}

	if mix.KK > 0 {
		return Choice{Reply: KK, Detected: detected, Rule: "any-kazakh", Mix: mix}
	}
	return Choice{Reply: RU, Detected: detected, Rule: "default", Mix: mix}
}

// Choose is Peek plus setting p.Current = choice.Reply.
func (p *Policy) Choose(text, sttCode string) Choice {
	choice := p.Peek(text, sttCode)
	p.Current = choice.Reply
	return choice
}
