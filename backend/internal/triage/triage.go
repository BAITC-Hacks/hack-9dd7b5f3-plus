// Package triage is the deterministic first layer of the router: language,
// normalized entities (phones, IINs, plates, policy/claim numbers), urgency
// and multi-intent signals, confirmations and goodbyes. It runs in
// microseconds and its output is shown in the trace and given to the LLM as
// SIGNALS — it never decides the scenario by itself.
package triage

import (
	"regexp"
	"strings"

	"hackathon/backend/internal/lang"
)

// Signals is the triage output for one utterance.
type Signals struct {
	Language           string            `json:"language"`
	KKShare            float64           `json:"kk_share"`
	Normalized         string            `json:"normalized"`
	Entities           map[string]string `json:"entities"`
	Urgent             []string          `json:"urgent"`
	MultiIntentMarkers []string          `json:"multi_intent_markers"`
	Confirmation       string            `json:"confirmation"` // "yes" | "no" | ""
	Goodbye            bool              `json:"goodbye"`
	GreetingOnly       bool              `json:"greeting_only"`
	OperatorRequest    bool              `json:"operator_request"`
	RobotQuestion      bool              `json:"robot_question"`
	OutOfScopeHints    []string          `json:"out_of_scope_hints"`
}

var (
	rePhone   = regexp.MustCompile(`(?:\+?7|8)\d{10}`)
	rePlate   = regexp.MustCompile(`\b(\d{3})\s?([a-z]{2,3})\s?(\d{2})\b`)
	reClaim   = regexp.MustCompile(`\b(?:cl|цл|сл)[\s-]*(\d{6})\b`)
	rePolicy  = regexp.MustCompile(`\b(?:sq|ск|эс кью)[\s-]*(ogpo|casco|trvl|prop|ns|dms|огпо|каско)[\s-]*(\d{6})\b`)
	reEmail   = regexp.MustCompile(`[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
	reDigits  = regexp.MustCompile(`\+?\d[\d ]*\d`)
	reSpaces  = regexp.MustCompile(`\s+`)
	cyrToLat  = strings.NewReplacer("а", "a", "в", "b", "с", "c", "е", "e", "н", "h", "к", "k", "м", "m", "о", "o", "р", "p", "т", "t", "х", "x", "у", "y")
	policyMap = map[string]string{"огпо": "OGPO", "каско": "CASCO"}
)

var urgentMarkers = []string{
	"только что", "прямо сейчас", "стою на дороге", "на месте дтп", "на месте аварии", "қазір ғана", "қазір соқтығыс", "жолдың ортасында",
	"за границей", "сейчас в ", "шетелде", "отравил", "сломал ногу за", "мошенн", "код из смс", "код из sms", "перевести деньги", "алаяқ", "күдікті", "сілтемеге",
	"представился вашим", "сіздерден деп", "пострадавш", "112",
}

var multiIntentMarkers = []string{
	"и ещё", "и еще", "а ещё", "а еще", "и заодно", "заодно", "а также", "и также", "плюс ещё", "и ещё скажите", "и скажите", "и подскажите",
	"әрі", "және", "сосын", "оған қоса", "и второе", "второй вопрос", "ещё вопрос", "еще вопрос", "тағы бір сұрақ", "тағы да",
	"и какие", "а какие", "и как", "а как", "и сколько", "а сколько", "и где", "а где", "и когда", "а когда", "и можно", "а можно", "и покрывает", "и себя", "и заодно",
}

var yesWords = []string{"да", "ага", "угу", "верно", "все верно", "всё верно", "правильно", "подтверждаю", "оформляйте", "оформляй", "давайте", "конечно", "согласен", "согласна", "ок", "окей", "хорошо",
	"иә", "иа", "дұрыс", "растаймын", "растайын", "жарайды", "болады", "келісемін", "рәсімдеңіз", "тіркеңіз", "жазыңыз"}
var noWords = []string{"нет", "не надо", "не нужно", "отмена", "отменить", "передумал", "не подтверждаю", "неверно", "не верно", "жоқ", "керек емес", "болмайды", "қажет емес"}

var goodbyeWords = []string{"до свидания", "спасибо, всё", "спасибо все", "всего доброго", "пока", "это всё", "это все", "больше ничего", "нет, спасибо", "рақмет", "сау болыңыз", "жоқ, рақмет", "отлично, спасибо", "хорошо, спасибо", "спасибо"}
var greetingWords = []string{"здравствуйте", "добрый день", "доброе утро", "добрый вечер", "алло", "привет", "сәлеметсіз бе", "сәлем", "қайырлы күн", "қайырлы таң"}
var operatorWords = []string{"с оператором", "оператора позовите", "позовите оператора", "нужен оператор", "дайте оператора", "соедини", "соедините", "переключи", "переведите на", "с человеком", "живым человеком", "живого человека", "с живым", "со специалистом", "с менеджером", "операторға қос", "операторды қос", "оператормен", "адаммен сөйлес", "маған оператор"}
var robotWords = []string{"ты робот", "вы робот", "это робот", "робот?", "бот?", "сен роботсың", "робот па"}

var outOfScopeMarkers = []string{"кредит", "займ", "ипотек", "депозит", "вклад", "погод", "работу", "работать у вас", "ваканси", "жұмысқа", "өмірді сақтандыру", "өмір сақтандыру", "страхование жизни", "страховку жизни", "пенси", "аннуитет", "зейнетақы", "несие", "курс доллара", "такси"}

// Analyze runs all deterministic checks over one utterance.
func Analyze(text string, hasPendingConfirmation bool) Signals {
	detected, share := lang.Detect(text)
	withDigits := SpokenToDigits(text)
	norm := lang.Normalize(withDigits)
	s := Signals{
		Language:   detected,
		KKShare:    share,
		Normalized: norm,
		Entities:   map[string]string{},
	}
	s.extractEntities(norm)

	lower := strings.ToLower(text)
	for _, m := range urgentMarkers {
		if strings.Contains(lower, m) {
			s.Urgent = append(s.Urgent, m)
		}
	}
	for _, m := range multiIntentMarkers {
		if containsWord(norm, m) {
			s.MultiIntentMarkers = append(s.MultiIntentMarkers, m)
		}
	}
	for _, m := range outOfScopeMarkers {
		if strings.Contains(lower, m) {
			s.OutOfScopeHints = append(s.OutOfScopeHints, m)
		}
	}
	for _, m := range operatorWords {
		if strings.Contains(lower, m) {
			s.OperatorRequest = true
			break
		}
	}
	for _, m := range robotWords {
		if strings.Contains(lower, m) {
			s.RobotQuestion = true
			break
		}
	}
	words := lang.Tokens(text)
	if hasPendingConfirmation {
		s.Confirmation = detectConfirmation(norm, words)
	}
	// greeting only: every token is part of a greeting phrase
	if len(words) > 0 && len(words) <= 4 {
		stripped := norm
		for _, g := range greetingWords {
			stripped = strings.ReplaceAll(stripped, g, "")
		}
		if strings.TrimSpace(stripped) == "" {
			s.GreetingOnly = true
		}
	}
	if len(words) <= 6 {
		for _, g := range goodbyeWords {
			if containsWord(norm, g) {
				s.Goodbye = true
				break
			}
		}
		// "спасибо" alone or with a closing word only
		if s.Goodbye && (strings.Contains(norm, "?") || strings.Contains(norm, "хочу") || strings.Contains(norm, "керек")) {
			s.Goodbye = false
		}
	}
	return s
}

func containsWord(norm, phrase string) bool {
	if !strings.Contains(norm, phrase) {
		return false
	}
	idx := strings.Index(norm, phrase)
	before := idx == 0 || norm[idx-1] == ' '
	end := idx + len(phrase)
	after := end == len(norm) || norm[end] == ' '
	return before && after
}

func detectConfirmation(norm string, words []string) string {
	if len(words) > 8 {
		return ""
	}
	for _, n := range noWords {
		if containsWord(norm, n) {
			return "no"
		}
	}
	for _, y := range yesWords {
		if containsWord(norm, y) {
			return "yes"
		}
	}
	return ""
}

func (s *Signals) extractEntities(norm string) {
	// digit runs (phones / IIN) — join spaced groups first
	for _, m := range reDigits.FindAllString(norm, -1) {
		d := reSpaces.ReplaceAllString(m, "")
		plus := strings.HasPrefix(d, "+")
		d = strings.TrimPrefix(d, "+")
		switch {
		case len(d) == 11 && (d[0] == '7' || d[0] == '8'):
			s.Entities["phone"] = "+7" + d[1:]
		case len(d) == 10 && plus:
			s.Entities["phone"] = "+7" + d
		case len(d) == 10 && (d[0] == '7' && !plus):
			s.Entities["phone"] = "+7" + d[1:] + "?" // ambiguous, keep marker
			delete(s.Entities, "phone")
		case len(d) == 12 && !plus:
			s.Entities["iin"] = d
		}
	}
	if m := rePhone.FindString(strings.ReplaceAll(norm, " ", "")); m != "" && s.Entities["phone"] == "" {
		d := strings.TrimPrefix(m, "+")
		s.Entities["phone"] = "+7" + d[len(d)-10:]
	}
	if m := reClaim.FindStringSubmatch(norm); m != nil {
		s.Entities["claim_number"] = "CL-" + m[1]
	}
	if m := rePolicy.FindStringSubmatch(norm); m != nil {
		prod := strings.ToUpper(m[1])
		if p, ok := policyMap[m[1]]; ok {
			prod = p
		}
		s.Entities["policy_number"] = "SQ-" + prod + "-" + m[2]
	}
	if m := reEmail.FindString(norm); m != "" {
		s.Entities["email"] = m
	}
	// plates: transliterate Cyrillic look-alikes, then match
	lat := cyrToLat.Replace(norm)
	if m := rePlate.FindStringSubmatch(lat); m != nil {
		s.Entities["vehicle_plate"] = strings.ToUpper(m[1] + m[2] + m[3])
	}
}
