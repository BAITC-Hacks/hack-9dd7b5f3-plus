package brain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// realData loads the official starter kit, skipping when it is absent.
func realData(t *testing.T) *Dataset {
	t.Helper()
	dir := filepath.Join("..", "..", "data")
	if _, err := os.Stat(filepath.Join(dir, "scenarios.json")); err != nil {
		t.Skipf("starter kit not found: %v", err)
	}
	d, err := LoadDataset(dir)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestLoadDataset(t *testing.T) {
	d := realData(t)
	if d.AsOf != "2026-10-01" {
		t.Errorf("AsOf = %q", d.AsOf)
	}
	if len(d.Scenarios) != 40 || len(d.System) != 3 || len(d.Clients) != 11 {
		t.Errorf("got %d scenarios, %d system intents, %d clients", len(d.Scenarios), len(d.System), len(d.Clients))
	}
	if len(d.Policies) == 0 || len(d.Claims) == 0 || len(d.Payments) == 0 {
		t.Errorf("records missing: %d policies, %d claims, %d payments", len(d.Policies), len(d.Claims), len(d.Payments))
	}
	if !json.Valid([]byte(d.KnowledgeJSON)) || strings.Contains(d.KnowledgeJSON, "\n") {
		t.Error("KnowledgeJSON is not minified JSON")
	}
	if s := d.ScenarioByID("SC12"); s == nil || s.Priority != "high" || len(s.NotThisIf) == 0 {
		t.Errorf("SC12 = %+v", s)
	}

	p := d.StaticPrompt()
	for _, want := range []string{
		"OUTPUT FORMAT", "[[SCENARIO_ID|CONFIDENCE]]", "Today is 2026-10-01",
		"SC12 [high] Claim as victim under culprit's OGPO:", "-> SC11", "SC40",
		"SYS_UNCLEAR", "SYS_OUT_OF_SCOPE", "SYS_GOODBYE",
		"opening ru: \"", "| kk: \"", "handoff: always -> operator_general",
		"KNOWLEDGE", "Abai Ave 150",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("static prompt lacks %q", want)
		}
	}
	if p != d.StaticPrompt() {
		t.Error("static prompt is not deterministic")
	}
	t.Logf("static prompt: %d bytes, %d runes", len(p), utf8.RuneCountInString(p))
}

func TestClientLookup(t *testing.T) {
	d := realData(t)
	for _, phone := range []string{"+77010000001", "87010000001", "8 (701) 000-00-01"} {
		if c := d.ClientByPhone(phone); c == nil || c.FullName != "Arman Tulegenov" {
			t.Errorf("ClientByPhone(%q) = %+v", phone, c)
		}
	}
	if c := d.ClientByIIN("850314 300121"); c == nil || c.ID != "C001" {
		t.Errorf("ClientByIIN = %+v", c)
	}
	if c := d.ClientByPhone("+77019999999"); c != nil {
		t.Errorf("unknown phone matched %s", c.ID)
	}
	if c := d.ClientByPhone(""); c != nil {
		t.Errorf("empty phone matched %s", c.ID)
	}

	card := d.Card(d.ClientByPhone("+77010000001"))
	if !json.Valid([]byte(card)) {
		t.Fatalf("card is not valid JSON: %s", card)
	}
	for _, want := range []string{`"client":{"client_id":"C001"`, "SQ-OGPO-104501", "SQ-CASCO-204118", "CL-500198", `"payments":[]`} {
		if !strings.Contains(card, want) {
			t.Errorf("card lacks %q: %s", want, card)
		}
	}
	if strings.Contains(card, "SQ-DMS-604220") {
		t.Error("card contains another client's policy")
	}
	if d.Card(nil) != "" {
		t.Error("Card(nil) should be empty")
	}
}

func TestGenericPrompt(t *testing.T) {
	var d *Dataset
	p := d.StaticPrompt()
	for _, want := range []string{"OUTPUT FORMAT", "SYS_UNCLEAR", "No scenario catalog is loaded"} {
		if !strings.Contains(p, want) {
			t.Errorf("generic prompt lacks %q", want)
		}
	}
	if d.ClientByPhone("+77010000001") != nil || d.ClientByIIN("850314300121") != nil {
		t.Error("nil dataset matched a client")
	}
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"+77010000001":      "+77010000001",
		"87010000001":       "+77010000001",
		"8 (701) 000-00-01": "+77010000001",
		"77010000001":       "+77010000001",
		"7010000001":        "+77010000001",
		"+7 727 000 7575":   "+77270007575",
		"901 000 00 01":     "",
		"6010000001":        "",
		"12345":             "",
		"":                  "",
	}
	for in, want := range cases {
		if got := NormalizePhone(in); got != want {
			t.Errorf("NormalizePhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindPhone(t *testing.T) {
	cases := map[string]string{
		"мой номер 8 701 000 00 01, спасибо":       "+77010000001",
		"Телефон +7 (701) 000-00-01.":              "+77010000001",
		"плюс 7 701 000 00 07":                     "+77010000007",
		"номер 701 000 00 02":                      "+77010000002",
		"+7 707, 123, 45, 67":                      "+77071234567",
		"мой ИИН 850314300121 телефон 87010000001": "+77010000001",
		"87010000003 850314300121":                 "+77010000003",
		"полис 104501, 8 701 000 00 05":            "+77010000005",
		"ИИН 850314300121":                         "",
		"полис SQ-OGPO-104501 от 2026-03-15":       "",
		"8 916 123 45 67":                          "", // not a Kazakhstan number
		"без цифр":                                 "",
	}
	for in, want := range cases {
		if got := FindPhone(in); got != want {
			t.Errorf("FindPhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindIIN(t *testing.T) {
	cases := map[string]string{
		"мой ИИН 850314300121":                "850314300121",
		"ИИН 850314 300121, спасибо":          "850314300121",
		"8503 1430 0121":                      "850314300121",
		"Мой 910512300456, жены 930824400789": "910512300456",
		"87010000001 850314300121":            "850314300121",
		"87010000001":                         "",
		"1234567890123":                       "",
		"":                                    "",
	}
	for in, want := range cases {
		if got := FindIIN(in); got != want {
			t.Errorf("FindIIN(%q) = %q, want %q", in, got, want)
		}
	}
}
