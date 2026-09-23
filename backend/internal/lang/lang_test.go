package lang

import "testing"

func TestDetect(t *testing.T) {
	cases := map[string]string{
		"Скажите, почём сейчас ОГПО на машину в Астане?":                    "ru",
		"Көлікке міндетті сақтандыру бағасы қандай болады, білгім келеді":   "kk",
		"Сәлеметсіз бе, полисім действует ли ещё, тексеріп беріңізші":       "mixed",
		"Кеше аварияға түстім, но я не виноват, виновник у вас застрахован": "mixed",
		"Маған справка керек для посольства, на английском":                 "mixed",
		"Қосымша ашылмай тұр":     "kk",
		"Ну там с машиной вопрос": "ru",
		"Сәлеметсіз бе, Түркияға баруға сақтандыру керек, и ещё скажите, где ваш офис в Астане": "mixed",
	}
	for text, want := range cases {
		got, _ := Detect(text)
		if got != want {
			t.Errorf("Detect(%q) = %s, want %s", text, got, want)
		}
	}
}

func TestStem(t *testing.T) {
	if Stem("страховку") != Stem("страховка") || Stem("сақтандыруды") != Stem("сақтандыру") {
		t.Errorf("stems should fold inflections: %s %s %s %s", Stem("страховку"), Stem("страховка"), Stem("сақтандыруды"), Stem("сақтандыру"))
	}
}
