package lang

import "testing"

func TestWordLang(t *testing.T) {
	cases := []struct {
		word string
		want Lang
	}{
		{"сәлеметсіз", KK},
		{"полисімнің", KK},
		{"керек", KK},
		{"здравствуйте", RU},
		{"полис", RU},
		{"не", RU},
		{"да", RU},
		{"hello", Unknown},
		{"123", Unknown},
		{"төледім", KK},
		{"картамен", KK},
		{"экзамен", RU},
		{"жатырмын", KK},
	}
	for _, c := range cases {
		t.Run(c.word, func(t *testing.T) {
			if got := WordLang(c.word); got != c.want {
				t.Errorf("WordLang(%q) = %q, want %q", c.word, got, c.want)
			}
		})
	}
}

func TestAnalyzeDominantRussian(t *testing.T) {
	m := Analyze("Здравствуйте, хочу продлить полис")
	if m.Dominant != RU {
		t.Errorf("Dominant = %q, want %q", m.Dominant, RU)
	}
	if got := m.Language(); got != RU {
		t.Errorf("Language() = %q, want %q", got, RU)
	}
}

func TestAnalyzeDominantKazakhStrong(t *testing.T) {
	m := Analyze("Сәлеметсіз бе, полисімнің мерзімін ұзартқым келеді")
	if m.Dominant != KK {
		t.Errorf("Dominant = %q, want %q", m.Dominant, KK)
	}
	if !m.Strong {
		t.Errorf("Strong = false, want true")
	}
}

func TestAnalyzeMixed(t *testing.T) {
	m := Analyze("Сәлеметсіз бе, кеше аварияға түстім, но я не виноват, а ещё полис на почту не пришёл")
	if got := m.Language(); got != Mixed {
		t.Errorf("Language() = %q, want %q", got, Mixed)
	}
	if m.Dominant != RU {
		t.Errorf("Dominant = %q, want %q", m.Dominant, RU)
	}
}

func TestAnalyzeIgnoresSingleLettersAndUnknown(t *testing.T) {
	// "я", "и", "в" are ignored single letters; "hello" and "123" tag
	// Unknown and are ignored too.
	m := Analyze("я и в полис hello 123")
	if m.KK != 0 || m.RU != 1 {
		t.Errorf("got KK=%d RU=%d, want KK=0 RU=1", m.KK, m.RU)
	}
	if len(m.Words) != 1 || m.Words[0].Word != "полис" {
		t.Errorf("Words = %+v, want a single полис entry", m.Words)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	m := Analyze("   ")
	if m.Dominant != Unknown {
		t.Errorf("Dominant = %q, want Unknown", m.Dominant)
	}
	if m.Share != 0 {
		t.Errorf("Share = %v, want 0", m.Share)
	}
	if got := m.Language(); got != Unknown {
		t.Errorf("Language() = %q, want Unknown", got)
	}
}

func TestExplicitRequest(t *testing.T) {
	cases := []struct {
		text string
		want Lang
	}{
		{"Можете говорить по-казахски?", KK},
		{"Орысша сөйлеңізші", RU},
		{"говорите на русском, нет, қазақша", KK},
		{"просто текст", Unknown},
	}
	for _, c := range cases {
		t.Run(c.text, func(t *testing.T) {
			if got := ExplicitRequest(c.text); got != c.want {
				t.Errorf("ExplicitRequest(%q) = %q, want %q", c.text, got, c.want)
			}
		})
	}
}

func TestFromSTT(t *testing.T) {
	cases := []struct {
		code string
		want Lang
	}{
		{"kk", KK}, {"KAZ", KK}, {"Kazakh", KK},
		{"ru", RU}, {"RUS", RU}, {"Russian", RU},
		{"en", Unknown}, {"", Unknown},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			if got := FromSTT(c.code); got != c.want {
				t.Errorf("FromSTT(%q) = %q, want %q", c.code, got, c.want)
			}
		})
	}
}

func TestPolicySequence(t *testing.T) {
	p := &Policy{}

	c1 := p.Choose("Здравствуйте, хочу продлить полис на машину", "")
	if c1.Reply != RU || c1.Rule != "dominant" {
		t.Fatalf("turn1: got Reply=%q Rule=%q, want RU/dominant", c1.Reply, c1.Rule)
	}

	// A single Kazakh word is not enough tagged words to flip a session
	// that already has a language (MinWords defaults to 2): stays sticky.
	c2 := p.Choose("иә", "")
	if c2.Reply != RU || c2.Rule != "sticky" {
		t.Fatalf("turn2: got Reply=%q Rule=%q, want RU/sticky", c2.Reply, c2.Rule)
	}

	c3 := p.Choose("Сәлеметсіз бе, полисімнің мерзімін ұзартқым келеді", "")
	if c3.Reply != KK || c3.Rule != "dominant" {
		t.Fatalf("turn3: got Reply=%q Rule=%q, want KK/dominant", c3.Reply, c3.Rule)
	}

	// Symmetric case: one Russian word is not enough to flip back.
	c4 := p.Choose("да", "")
	if c4.Reply != KK || c4.Rule != "sticky" {
		t.Fatalf("turn4: got Reply=%q Rule=%q, want KK/sticky", c4.Reply, c4.Rule)
	}

	// An explicit request always wins immediately, regardless of Current.
	c5 := p.Choose("говорите по-русски пожалуйста", "")
	if c5.Reply != RU || c5.Rule != "explicit" {
		t.Fatalf("turn5: got Reply=%q Rule=%q, want RU/explicit", c5.Reply, c5.Rule)
	}

	// Peek must never mutate Current.
	before := p.Current
	peeked := p.Peek("қазақша", "")
	if p.Current != before {
		t.Fatalf("Peek mutated Current: before=%q after=%q", before, p.Current)
	}
	if peeked.Reply != KK || peeked.Rule != "explicit" {
		t.Fatalf("peek: got Reply=%q Rule=%q, want KK/explicit", peeked.Reply, peeked.Rule)
	}

	fresh := Policy{}
	stt := fresh.Peek("", "kaz")
	if stt.Reply != KK || stt.Rule != "stt" {
		t.Fatalf("stt: got Reply=%q Rule=%q, want KK/stt", stt.Reply, stt.Rule)
	}

	def := fresh.Peek("", "")
	if def.Reply != RU || def.Rule != "default" {
		t.Fatalf("default: got Reply=%q Rule=%q, want RU/default", def.Reply, def.Rule)
	}
}
