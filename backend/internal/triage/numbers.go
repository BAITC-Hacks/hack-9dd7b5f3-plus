package triage

import (
	"strconv"
	"strings"
)

// Spoken number words for Russian and Kazakh. Values >= 100 are "hundreds"
// words, 20..90 are tens, 0..19 are units.
var numberWords = map[string]int{
	// ru
	"ноль": 0, "нуль": 0, "один": 1, "одна": 1, "одно": 1, "раз": 1, "два": 2, "две": 2, "три": 3, "четыре": 4, "пять": 5,
	"шесть": 6, "семь": 7, "восемь": 8, "девять": 9, "десять": 10, "одиннадцать": 11, "двенадцать": 12, "тринадцать": 13,
	"четырнадцать": 14, "пятнадцать": 15, "шестнадцать": 16, "семнадцать": 17, "восемнадцать": 18, "девятнадцать": 19,
	"двадцать": 20, "тридцать": 30, "сорок": 40, "пятьдесят": 50, "шестьдесят": 60, "семьдесят": 70, "восемьдесят": 80, "девяносто": 90,
	"сто": 100, "двести": 200, "триста": 300, "четыреста": 400, "пятьсот": 500, "шестьсот": 600, "семьсот": 700, "восемьсот": 800, "девятьсот": 900,
	// kk
	"нөл": 0, "бір": 1, "екі": 2, "үш": 3, "төрт": 4, "бес": 5, "алты": 6, "жеті": 7, "сегіз": 8, "тоғыз": 9, "он": 10,
	"жиырма": 20, "отыз": 30, "қырық": 40, "елу": 50, "алпыс": 60, "жетпіс": 70, "сексен": 80, "тоқсан": 90,
}

// multipliers that follow a unit in Kazakh ("жеті жүз" = 700) or stand alone.
var multiplierWords = map[string]int{"жүз": 100, "мың": 1000, "тысяча": 1000, "тысячи": 1000, "тысяч": 1000}

// SpokenToDigits rewrites sequences of number words into digit groups so that
// "плюс семь семьсот семь сто двадцать три сорок пять шестьдесят семь" becomes
// "+7 707 123 45 67". Non-number words are kept as-is.
func SpokenToDigits(text string) string {
	words := strings.Fields(text)
	out := make([]string, 0, len(words))
	i := 0
	for i < len(words) {
		w := strings.Trim(strings.ToLower(words[i]), ",.;:!?")
		if w == "плюс" || w == "плюс," {
			out = append(out, "+")
			i++
			continue
		}
		if _, ok := numberWords[w]; !ok {
			if _, ok2 := multiplierWords[w]; !ok2 {
				out = append(out, words[i])
				i++
				continue
			}
		}
		// parse one spoken number group
		val, consumed := parseGroup(words, i)
		if consumed == 1 && ambiguousAlone[w] {
			// "он" (kk: ten / ru: he), "раз" (once), "одна" — only digits when part of a number sequence
			out = append(out, words[i])
			i++
			continue
		}
		out = append(out, strconv.Itoa(val))
		i += consumed
	}
	return strings.Join(out, " ")
}

// ambiguousAlone are number words that are ordinary words on their own.
var ambiguousAlone = map[string]bool{"он": true, "раз": true, "одна": true, "одно": true, "три": true, "сто": true}

// parseGroup reads a single number (e.g. "семьсот семь", "сто двадцать три",
// "жеті жүз бір", "сорок пять", "ноль") starting at words[i].
func parseGroup(words []string, i int) (int, int) {
	val := 0
	consumed := 0
	state := 0 // 0=start,1=after hundreds,2=after tens,3=after unit
	for i+consumed < len(words) {
		w := strings.Trim(strings.ToLower(words[i+consumed]), ",.;:!?")
		if m, ok := multiplierWords[w]; ok {
			if consumed == 0 {
				val = m
			} else {
				val *= m
			}
			consumed++
			state = 1
			if m == 1000 {
				// allow hundreds/tens/units after a thousand
				state = 0
			}
			continue
		}
		n, ok := numberWords[w]
		if !ok {
			break
		}
		switch {
		case n >= 100:
			if state != 0 {
				return val, consumed
			}
			val += n
			state = 1
		case n >= 20:
			if state >= 2 {
				return val, consumed
			}
			val += n
			state = 2
		case n >= 10: // 10..19 are terminal
			if state >= 2 {
				return val, consumed
			}
			val += n
			consumed++
			return val, consumed
		default: // 0..9
			if state == 3 {
				return val, consumed
			}
			// a unit followed by a multiplier ("жеті жүз") multiplies; peek
			if i+consumed+1 < len(words) {
				nw := strings.Trim(strings.ToLower(words[i+consumed+1]), ",.;:!?")
				if m, ok := multiplierWords[nw]; ok && n > 0 && state == 0 {
					val += n * m
					consumed += 2
					state = 1
					continue
				}
			}
			val += n
			state = 3
		}
		consumed++
	}
	return val, consumed
}
