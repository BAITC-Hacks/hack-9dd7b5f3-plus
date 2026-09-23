package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogLoadingAndValidation(t *testing.T) {
	for _, raw := range []string{`[]`, `[{"id":"x","name":"X"},{"id":"x","name":"Y"}]`, `[{"name":"missing id"}]`} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "scenarios.json"), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadCatalog(dir); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "scenarios.json"), []byte(`{"scenarios":[{"scenario_id":"x","title":"Example","purpose":"test"}]}`), 0600)
	c, e := LoadCatalog(dir)
	if e != nil || c.Source != "external" || c.Scenarios[0].ID != "x" {
		t.Fatalf("catalog=%+v err=%v", c, e)
	}
}
func TestPolicyNeverClaimsIrreversibleExecution(t *testing.T) {
	c := Catalog{Scenarios: []Scenario{{ID: "cancel", Name: "Cancel", ResponseRU: "Нужно подтверждение.", ResponseKK: "Растау қажет.", RequiresConfirmation: true}}}
	d := Decision{ScenarioID: "cancel", Status: "route", Confidence: .95, Language: "ru"}
	reply, w := c.Reply(&d)
	if len(w) == 0 || reply != "Нужно подтверждение. В этом демо изменения не выполняются." {
		t.Fatalf("reply=%s warnings=%v", reply, w)
	}
	d.Confidence = .5
	c.Reply(&d)
	if d.Status != "clarify" {
		t.Fatal("low confidence should clarify")
	}
	d.Status = "handoff"
	d.Language = "kk"
	reply, _ = c.Reply(&d)
	if reply == "" {
		t.Fatal("empty Kazakh handoff")
	}
}

func TestOfficialCatalogRetainsFortyAndSystemIntents(t *testing.T) {
	c, err := LoadCatalog("../../../data")
	if err != nil {
		t.Fatal(err)
	}
	if c.BusinessCount != 40 || c.SystemCount != 3 || len(c.Scenarios) != 43 || c.AsOf != "2026-10-01" {
		t.Fatalf("wrong catalog counts/as-of: %+v", c)
	}
	s, ok := c.Find("SC30")
	if !ok || len(s.Examples) < 4 || s.ResponseKK == "" || s.Boundaries == "" {
		t.Fatal("lost official response/examples/boundaries")
	}
	for _, id := range []string{"SYS_UNCLEAR", "SYS_GOODBYE", "SYS_OUT_OF_SCOPE"} {
		if _, ok := c.Find(id); !ok {
			t.Fatalf("missing %s", id)
		}
	}
}
func TestOfficialFactsAreReadAndCited(t *testing.T) {
	c, e := LoadCatalog("../../../data")
	if e != nil {
		t.Fatal(e)
	}
	d := Decision{ScenarioID: "SC33", Status: "route", Language: "ru", Confidence: .95, Slots: []Slot{{Name: "city", Value: "Алматы"}}}
	reply, _, facts := c.Answer(&d, nil)
	if len(facts) != 1 || facts[0].Source != "knowledge_base.json" || reply == "" {
		t.Fatalf("reply=%s facts=%v", reply, facts)
	}
	d = Decision{ScenarioID: "SC17", Status: "route", Language: "kk", Confidence: .95, Slots: []Slot{{Name: "claim_number", Value: "CL-500198"}}}
	reply, _, facts = c.Answer(&d, nil)
	if len(facts) != 1 || facts[0].Source != "mock_backend.json" {
		t.Fatalf("reply=%s facts=%v", reply, facts)
	}
}
func TestInterruptedTopicsAreSeparateFromCurrentMultiIntent(t *testing.T) {
	d := Decision{ScenarioID: "SC33", Status: "route", Pending: []string{}}
	stack := PendingTopics("SC27", nil, d)
	if len(stack) != 1 || stack[0] != "SC27" || len(d.Pending) != 0 {
		t.Fatal("lost interrupted topic or contaminated predictions")
	}
	d.ScenarioID = "SC27"
	stack = PendingTopics("SC33", stack, d)
	if len(stack) != 1 || stack[0] != "SC33" {
		t.Fatalf("return stack=%v", stack)
	}
}
