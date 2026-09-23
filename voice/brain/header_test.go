package brain

import (
	"slices"
	"strings"
	"testing"
)

// feedAll runs deltas through a headerParser. It returns the decisions, the
// non-empty text pieces and the index of the delta that produced the
// decision (len(deltas) means flush, -1 means none).
func feedAll(deltas ...string) (decs []*Decision, texts []string, at int) {
	var p headerParser
	at = -1
	collect := func(i int, d *Decision, t string) {
		if d != nil {
			decs = append(decs, d)
			if at < 0 {
				at = i
			}
		}
		if t != "" {
			texts = append(texts, t)
		}
	}
	for i, s := range deltas {
		d, t := p.feed(s)
		collect(i, d, t)
	}
	d, t := p.flush()
	collect(len(deltas), d, t)
	return decs, texts, at
}

func TestHeaderParser(t *testing.T) {
	long := strings.Repeat("слово ", 20)
	tests := []struct {
		name   string
		deltas []string
		id     string // "" means no decision
		conf   float64
		at     int
		texts  []string
	}{
		{"one delta", []string{"[[SC12|0.88]]\nЗдравствуйте!"}, "SC12", 0.88, 0, []string{"Здравствуйте!"}},
		{"split across deltas", []string{"[", "[SC", "12|0.8", "8]]\nЗдрав", "ствуйте"}, "SC12", 0.88, 3, []string{"Здрав", "ствуйте"}},
		{"no header", []string{"Здравствуйте", "! Чем помочь?"}, "", 0, -1, []string{"Здравствуйте", "! Чем помочь?"}},
		{"no header, leading space kept", []string{"\n Привет"}, "", 0, -1, []string{"\n Привет"}},
		{"malformed header", []string{"[[не заголовок]] Привет"}, "", 0, -1, []string{"[[не заголовок]] Привет"}},
		{"malformed id", []string{"[[hello|0.9]] Hi"}, "", 0, -1, []string{"[[hello|0.9]] Hi"}},
		{"long text without header", []string{"[" + long}, "", 0, -1, []string{"[" + long}},
		{"long streamed text without header", []string{"[", strings.Repeat("а", 50), " хвост"}, "", 0, -1, []string{"[" + strings.Repeat("а", 50), " хвост"}},
		{"spaces and lowercase", []string{"  [[ sc05 | 0.7 ]]", "  \n", "\n Текст"}, "SC05", 0.7, 0, []string{"Текст"}},
		{"missing confidence", []string{"[[SYS_GOODBYE]]\nСпасибо"}, "SYS_GOODBYE", 0, 0, []string{"Спасибо"}},
		{"percent confidence", []string{"[[SC01|88%]] Да"}, "SC01", 0.88, 0, []string{"Да"}},
		{"comma decimal", []string{"[[SC01|0,75]]\nДа"}, "SC01", 0.75, 0, []string{"Да"}},
		{"single brackets", []string{"[SC17|0.9]\nПроверю"}, "SC17", 0.9, 0, []string{"Проверю"}},
		{"markdown around header", []string{"**[[SC02|0.81]]**\nОформим"}, "SC02", 0.81, 0, []string{"Оформим"}},
		{"second bracket in next delta", []string{"[[SC37|0.99]", "]", "\nСоединяю"}, "SC37", 0.99, 1, []string{"Соединяю"}},
		{"header only at end of stream", []string{"[[SC37|0.99]"}, "SC37", 0.99, 1, nil},
		{"reply later contains brackets", []string{"[[SC40|0.6]]\nТекст ", "[[SC01|0.9]]"}, "SC40", 0.6, 0, []string{"Текст ", "[[SC01|0.9]]"}},
		{"extra header line dropped", []string{"[[SC30|0.86]]\n[[SC29|0.70]]\nВижу платёж"}, "SC30", 0.86, 0, []string{"Вижу платёж"}},
		{"extra header split", []string{"[[SC30|0.86]]\n", "[[SC2", "9|0.7]]", "\nВижу"}, "SC30", 0.86, 0, []string{"Вижу"}},
		{"bad text after header", []string{"[[SC12|0.9]]\n[[bad]] текст"}, "SC12", 0.9, 0, []string{"[[bad]] текст"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decs, texts, at := feedAll(tt.deltas...)
			if tt.id == "" {
				if len(decs) != 0 {
					t.Fatalf("unexpected decision %+v", decs[0])
				}
			} else {
				if len(decs) != 1 {
					t.Fatalf("got %d decisions, want 1", len(decs))
				}
				if decs[0].ScenarioID != tt.id || decs[0].Confidence != tt.conf {
					t.Errorf("decision = %s|%v, want %s|%v", decs[0].ScenarioID, decs[0].Confidence, tt.id, tt.conf)
				}
				if at != tt.at {
					t.Errorf("decision after delta %d, want %d", at, tt.at)
				}
			}
			if !slices.Equal(texts, tt.texts) {
				t.Errorf("texts = %q, want %q", texts, tt.texts)
			}
		})
	}
}

func TestHeaderParserNoBufferingWithoutHeader(t *testing.T) {
	var p headerParser
	if d, s := p.feed("Добрый день"); d != nil || s != "Добрый день" {
		t.Fatalf("feed = %v, %q: plain text must pass through at once", d, s)
	}
}
