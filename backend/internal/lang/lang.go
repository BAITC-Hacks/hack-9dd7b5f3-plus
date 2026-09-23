// Package lang holds language detection and text normalization for Russian
// and Kazakh, including mixed (code-switched) utterances.
package lang

import (
	"strings"
	"unicode"
)

// Kazakh-only Cyrillic letters (not part of the Russian alphabet).
const kazakhLetters = "әғқңөұүһі"

// Russian function words that practically never occur in Kazakh speech.
var ruMarkers = map[string]bool{
	"и": true, "а": true, "но": true, "ещё": true, "еще": true, "я": true, "не": true, "мне": true, "вы": true, "у": true, "вас": true,
	"на": true, "в": true, "с": true, "что": true, "как": true, "где": true, "когда": true, "можно": true, "нужно": true, "надо": true,
	"хочу": true, "подскажите": true, "скажите": true, "пожалуйста": true, "помогите": true, "действует": true, "ли": true, "есть": true,
	"для": true, "по": true, "это": true, "мой": true, "моя": true, "мои": true, "меня": true, "уже": true, "также": true, "заодно": true,
	"или": true, "если": true, "только": true, "сейчас": true, "вчера": true, "завтра": true, "через": true, "какие": true, "какой": true,
	"сколько": true, "почему": true, "нет": true, "да": true, "ваш": true, "ваша": true, "вашего": true, "здравствуйте": true, "спасибо": true,
	"приходит": true, "пришёл": true, "пришел": true, "виноват": true, "виновник": true, "застрахован": true, "поменять": true, "проверьте": true,
}

// Kazakh function words that practically never occur in Russian speech.
var kkMarkers = map[string]bool{
	"керек": true, "ма": true, "ме": true, "ба": true, "бе": true, "па": true, "пе": true, "және": true, "әрі": true, "мен": true, "менің": true,
	"маған": true, "сіз": true, "сіздер": true, "қалай": true, "қанша": true, "неге": true, "қайда": true, "бар": true, "жоқ": true, "иә": true,
	"рақмет": true, "сәлеметсіз": true, "болады": true, "бола": true, "керегі": true, "үшін": true, "туралы": true, "бойынша": true,
	"кеше": true, "ертең": true, "бүгін": true, "қазір": true, "тексеріп": true, "беріңізші": true, "айтыңызшы": true, "жатырмын": true,
}

// Detect returns "ru", "kk" or "mixed" plus the share of Kazakh-marked tokens.
func Detect(text string) (string, float64) {
	toks := Tokens(text)
	if len(toks) == 0 {
		return "ru", 0
	}
	kk, ru := 0, 0
	for _, t := range toks {
		isKK := strings.ContainsAny(t, kazakhLetters) || kkMarkers[t]
		isRU := !isKK && (ruMarkers[t] || hasRussianEnding(t))
		if isKK {
			kk++
		} else if isRU {
			ru++
		}
	}
	total := float64(len(toks))
	share := float64(kk) / total
	switch {
	case kk == 0:
		return "ru", 0
	case ru == 0:
		return "kk", share
	case kk >= 1 && ru >= 2, kk >= 2 && ru >= 1:
		return "mixed", share
	case share >= 0.5:
		return "kk", share
	default:
		return "ru", share
	}
}

func hasRussianEnding(t string) bool {
	for _, suf := range []string{"ить", "ать", "ется", "ится", "ите", "айте", "ого", "ему", "ому", "ыми", "ими", "ую", "ая", "ое", "ые", "ий", "ый", "ой", "ли"} {
		if len(t) > len(suf)+2 && strings.HasSuffix(t, suf) {
			return true
		}
	}
	return false
}

// Normalize lowercases, folds ё→е and strips punctuation.
func Normalize(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	prevSpace := true
	for _, r := range strings.ToLower(text) {
		switch {
		case r == 'ё':
			r = 'е'
		case r == '+':
			// keep the plus for phone numbers
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '@' || r == '.':
		default:
			r = ' '
		}
		if r == ' ' {
			if prevSpace {
				continue
			}
			prevSpace = true
		} else {
			prevSpace = false
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// Tokens splits normalized text into word tokens (letters/digits only).
func Tokens(text string) []string {
	norm := Normalize(text)
	fields := strings.FieldsFunc(norm, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	out := fields[:0]
	for _, f := range fields {
		if len([]rune(f)) >= 1 {
			out = append(out, f)
		}
	}
	return out
}

// Stem folds inflected forms by truncating long tokens to a 5-rune prefix
// after trimming a few very common endings. It is deliberately crude: both
// Russian and Kazakh keep their lexical root at the start of the word.
func Stem(tok string) string {
	r := []rune(tok)
	if len(r) <= 4 {
		return tok
	}
	for _, suf := range []string{"ларыңыз", "леріңіз", "ларым", "лерім", "дарым", "дерім", "ымыз", "іміз", "ыңыз", "іңіз", "ғым", "гім", "қым", "кім", "лар", "лер", "дар", "дер", "тар", "тер", "ями", "ами", "ого", "его", "ому", "ему"} {
		sr := []rune(suf)
		if len(r) > len(sr)+3 && strings.HasSuffix(tok, suf) {
			r = r[:len(r)-len(sr)]
			break
		}
	}
	if len(r) > 5 {
		r = r[:5]
	}
	return string(r)
}

// Stems returns the stemmed tokens of a text.
func Stems(text string) []string {
	toks := Tokens(text)
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		out = append(out, Stem(t))
	}
	return out
}

// ReplyLanguage picks the language to answer in: Kazakh only when the
// utterance is predominantly Kazakh; mixed speech gets the dominant side.
func ReplyLanguage(detected string, kkShare float64, sessionPref string) string {
	switch detected {
	case "kk":
		return "kk"
	case "ru":
		return "ru"
	default:
		if kkShare >= 0.5 {
			return "kk"
		}
		if sessionPref == "kk" && kkShare >= 0.3 {
			return "kk"
		}
		return "ru"
	}
}
