// Package retrieval builds a lexical (BM25-style) index over the scenario
// examples plus an editable domain lexicon. It returns a ranked SHORTLIST with
// scores and matched terms. It is retrieval, not classification: the LLM sees
// the whole catalog and the shortlist is only a hint, a fast-path trigger and
// the keyless (mock) baseline.
package retrieval

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"hackathon/backend/internal/catalog"
	"hackathon/backend/internal/lang"
)

// Lexicon is the editable vocabulary file (config/lexicon.json).
type Lexicon struct {
	Concepts      map[string][]string          `json:"concepts"`
	OutOfScope    []string                     `json:"out_of_scope"`
	ScenarioNames map[string]map[string]string `json:"scenario_names"`
}

// LoadLexicon reads the lexicon JSON.
func LoadLexicon(path string) (*Lexicon, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var l Lexicon
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("lexicon: %w", err)
	}
	return &l, nil
}

// Candidate is one shortlist entry.
type Candidate struct {
	ID       string   `json:"id"`
	Score    float64  `json:"score"`    // normalized 0..1 (share of query information matched)
	BM25     float64  `json:"bm25"`     // raw BM25
	Evidence float64  `json:"evidence"` // absolute matched information (sum of idf), guards tiny queries
	Terms    []string `json:"terms"`    // matched terms (stems and concept tokens)
}

type doc struct {
	id  string
	tf  map[string]float64
	len float64
}

// Index is the in-memory lexical index.
type Index struct {
	lex    *Lexicon
	docs   []doc
	df     map[string]int
	avgLen float64
	n      int
	// variant -> concept, split by matching mode
	exact  map[string]string
	prefix map[string]string
	phrase map[string]string
}

const (
	k1            = 1.2
	b             = 0.75
	conceptWeight = 1.4
)

// Function words carry no routing information; they are dropped from both
// the documents and the query.
var stopwords = map[string]bool{
	"и": true, "а": true, "но": true, "в": true, "во": true, "на": true, "с": true, "со": true, "по": true, "у": true, "к": true, "о": true, "об": true, "от": true,
	"до": true, "за": true, "из": true, "для": true, "про": true, "при": true, "без": true, "под": true, "над": true, "я": true, "мы": true, "вы": true, "он": true, "она": true,
	"они": true, "мне": true, "меня": true, "мой": true, "моя": true, "мои": true, "моим": true, "моему": true, "моего": true, "нам": true, "вам": true, "вас": true, "ваш": true, "ваша": true,
	"ваши": true, "вашего": true, "вашей": true, "наш": true, "его": true, "ее": true, "их": true, "себя": true, "себе": true,
	"это": true, "этот": true, "эта": true, "то": true, "там": true, "тут": true, "так": true, "же": true, "ли": true,
	"бы": true, "ни": true, "уже": true, "еще": true, "ещё": true, "тоже": true, "также": true, "или": true, "если": true,
	"хотел": true, "хотела": true, "хочется": true, "был": true, "была": true, "было": true, "быть": true, "может": true, "мне бы": true, "пожалуйста": true,
	"скажите": true, "подскажите": true, "здравствуйте": true, "добрый": true, "день": true, "алло": true, "привет": true, "спасибо": true, "сейчас": true, "вот": true, "ну": true, "да": true, "нет": true, "очень": true,
	"просто": true, "только": true, "вопрос": true, "поводу": true, "насчет": true, "насчёт": true, "по поводу": true, "проблема": true, "мен": true, "сен": true, "сіз": true, "біз": true, "сіздер": true,
	"олар": true, "ол": true, "бұл": true, "осы": true, "сол": true, "маған": true, "менің": true, "сізге": true, "сіздің": true, "сіздерде": true, "сіздерге": true, "бізге": true, "бар": true, "жоқ": true,
	"ма": true, "ме": true, "ба": true, "бе": true, "па": true, "пе": true, "деп": true, "де": true, "және": true, "әрі": true, "бірақ": true, "немесе": true, "үшін": true, "туралы": true,
	"бойынша": true, "болса": true, "еді": true, "едім": true, "екен": true, "бір": true, "бірдеңе": true, "нәрсе": true, "сәлеметсіз": true, "сәлем": true,
	"рақмет": true, "иә": true, "осында": true, "мұнда": true, "енді": true, "тағы": true, "әлі": true, "тек": true,
	"өте": true, "жай": true, "сұрайын": true, "айтыңызшы": true, "беріңізші": true,
}

// New builds the index from the catalog and the lexicon.
func New(cat *catalog.Catalog, lex *Lexicon) *Index {
	ix := &Index{lex: lex, df: map[string]int{}, exact: map[string]string{}, prefix: map[string]string{}, phrase: map[string]string{}}
	if lex == nil {
		ix.lex = &Lexicon{}
	}
	for concept, variants := range ix.lex.Concepts {
		for _, v := range variants {
			v = lang.Normalize(v)
			switch {
			case strings.Contains(v, " "):
				ix.phrase[v] = concept
			case utf8.RuneCountInString(v) >= 4:
				ix.prefix[v] = concept
			default:
				ix.exact[v] = concept
			}
		}
	}
	scs, _ := cat.Snapshot()
	total := 0.0
	for _, s := range scs {
		d := doc{id: s.ID, tf: map[string]float64{}}
		for _, l := range []string{"ru", "kk"} {
			for _, ex := range s.Examples[l] {
				for _, t := range ix.terms(ex) {
					d.tf[t]++
					d.len++
				}
			}
		}
		for t := range d.tf {
			ix.df[t]++
		}
		total += d.len
		ix.docs = append(ix.docs, d)
	}
	ix.n = len(ix.docs)
	if ix.n > 0 {
		ix.avgLen = total / float64(ix.n)
	}
	return ix
}

// Concepts returns the lexicon concepts found in a text.
func (ix *Index) Concepts(text string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range ix.terms(text) {
		if strings.HasPrefix(t, "#") && !seen[t] {
			seen[t] = true
			out = append(out, strings.TrimPrefix(t, "#"))
		}
	}
	return out
}

// terms converts text into stems plus "#concept" tokens.
func (ix *Index) terms(text string) []string {
	norm := lang.Normalize(text)
	toks := lang.Tokens(norm)
	out := make([]string, 0, len(toks)+4)
	for _, t := range toks {
		if utf8.RuneCountInString(t) < 2 {
			continue
		}
		if !stopwords[t] {
			out = append(out, lang.Stem(t))
		}
		if c, ok := ix.exact[t]; ok {
			out = append(out, "#"+c)
			continue
		}
		for v, c := range ix.prefix {
			if strings.HasPrefix(t, v) {
				out = append(out, "#"+c)
				break
			}
		}
	}
	padded := " " + norm + " "
	for v, c := range ix.phrase {
		if strings.Contains(padded, " "+v) {
			out = append(out, "#"+c)
		}
	}
	return out
}

func (ix *Index) idf(t string) float64 {
	df := float64(ix.df[t])
	return math.Log(1 + (float64(ix.n)-df+0.5)/(df+0.5))
}

// idfUnknown is the weight of an informative word that no scenario example
// contains: it counts against the match, so "погода" or "кредит" lower every
// candidate's normalized score instead of being ignored.
func (ix *Index) idfUnknown() float64 { return math.Log(1 + (float64(ix.n)+0.5)/0.5) }

func weight(t string) float64 {
	if strings.HasPrefix(t, "#") {
		return conceptWeight
	}
	return 1
}

// Search ranks scenarios for a text and returns the top k candidates.
func (ix *Index) Search(text string, k int) []Candidate {
	qterms := ix.terms(text)
	if len(qterms) == 0 {
		return nil
	}
	// query term multiset
	q := map[string]float64{}
	for _, t := range qterms {
		q[t]++
	}
	denom := 0.0
	for t, qtf := range q {
		if ix.df[t] > 0 {
			denom += ix.idf(t) * weight(t) * qtf
		} else if !strings.HasPrefix(t, "#") {
			denom += ix.idfUnknown() * qtf
		}
	}
	out := make([]Candidate, 0, ix.n)
	for _, d := range ix.docs {
		score := 0.0
		matched := 0.0
		var terms []string
		for t, qtf := range q {
			tf, ok := d.tf[t]
			if !ok {
				continue
			}
			idf := ix.idf(t)
			bm := idf * (tf * (k1 + 1)) / (tf + k1*(1-b+b*d.len/ix.avgLen))
			score += bm * weight(t) * qtf
			matched += idf * weight(t) * qtf
			terms = append(terms, t)
		}
		if score == 0 {
			continue
		}
		norm := 0.0
		if denom > 0 {
			norm = matched / denom
		}
		sort.Strings(terms)
		out = append(out, Candidate{ID: d.id, Score: round3(norm), BM25: round3(score), Evidence: round3(matched), Terms: terms})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].BM25 > out[j].BM25
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > k {
		out = out[:k]
	}
	return out
}

// Segments splits an utterance at multi-intent markers so each part can be
// searched separately (used by the keyless router and as a hint).
func Segments(norm string, markers []string) []string {
	segs := []string{norm}
	for _, m := range markers {
		var next []string
		for _, s := range segs {
			parts := strings.Split(" "+s+" ", " "+m+" ")
			for _, p := range parts {
				if p = strings.TrimSpace(p); p != "" {
					next = append(next, p)
				}
			}
		}
		segs = next
	}
	return segs
}

// OutOfScope reports lexicon out-of-scope markers found in text.
func (ix *Index) OutOfScope(text string) []string {
	norm := lang.Normalize(text)
	var hits []string
	for _, m := range ix.lex.OutOfScope {
		if strings.Contains(norm, lang.Normalize(m)) {
			hits = append(hits, m)
		}
	}
	return hits
}

// ScenarioName returns the localized short name used in clarifying questions.
func (ix *Index) ScenarioName(id, language string) string {
	if names, ok := ix.lex.ScenarioNames[id]; ok {
		if n, ok := names[language]; ok {
			return n
		}
		if n, ok := names["ru"]; ok {
			return n
		}
	}
	return id
}

func round3(f float64) float64 { return math.Round(f*1000) / 1000 }
