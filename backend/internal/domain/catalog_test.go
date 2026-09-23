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
