package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SlotDefinition struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Pattern string            `json:"pattern"`
	Values  []any             `json:"values"`
	Prompt  map[string]string `json:"prompt"`
}
type Catalog struct {
	Scenarios     []Scenario                `json:"scenarios"`
	Source        string                    `json:"source"`
	Hash          string                    `json:"hash"`
	BusinessCount int                       `json:"business_count"`
	SystemCount   int                       `json:"system_count"`
	Raw           json.RawMessage           `json:"-"`
	Knowledge     json.RawMessage           `json:"-"`
	Customers     json.RawMessage           `json:"-"`
	Slots         map[string]SlotDefinition `json:"-"`
	AsOf          string                    `json:"as_of_date"`
}

func LoadCatalog(dir string) (*Catalog, error) {
	b, err := os.ReadFile(filepath.Join(dir, "scenarios.json"))
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	var envelope struct {
		Scenarios []map[string]any `json:"scenarios"`
		System    []map[string]any `json:"system_intents"`
		Meta      struct {
			Dataset string `json:"dataset"`
			AsOf    string `json:"as_of_date"`
		} `json:"meta"`
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		if err = json.Unmarshal(b, &envelope); err != nil {
			return nil, err
		}
		rows = envelope.Scenarios
	}
	if len(rows) == 0 || len(rows) > 200 {
		return nil, fmt.Errorf("catalog must contain 1..200 business scenarios")
	}
	c := &Catalog{Source: "external", Scenarios: []Scenario{}, BusinessCount: len(rows), SystemCount: len(envelope.System), Slots: map[string]SlotDefinition{}, AsOf: envelope.Meta.AsOf}
	if envelope.Meta.Dataset == "Voice Router - Saqta Insurance" && len(rows) == 40 {
		c.Source = "organizer_saqta"
	}
	if _, err := os.Stat(filepath.Join(dir, "SYNTHETIC_DEMO")); err == nil {
		c.Source = "synthetic_demo"
	}
	sum := sha256.Sum256(b)
	c.Hash = hex.EncodeToString(sum[:])[:16]
	seen := map[string]bool{}
	businessCount := len(rows)
	rows = append(rows, envelope.System...)
	compact := []map[string]any{}
	for i, row := range rows {
		s := Scenario{ID: firstString(row, "id", "scenario_id", "code"), Name: firstString(row, "name", "title", "name_ru"), Description: firstString(row, "description", "purpose", "description_ru"), Raw: row, System: i >= businessCount, Slug: firstString(row, "slug"), Priority: firstString(row, "priority"), Boundaries: firstString(row, "boundaries"), ResponseRU: firstString(row, "response_ru"), ResponseKK: firstString(row, "response_kk")}
		if s.System {
			s.Name = s.ID
		}
		if s.ID == "" || s.Name == "" || seen[s.ID] {
			return nil, fmt.Errorf("each scenario needs a unique string id/scenario_id and name/title; invalid %q", s.ID)
		}
		seen[s.ID] = true
		s.RequiresConfirmation, _ = row["requires_confirmation"].(bool)
		s.Actions = stringsArray(row["actions"])
		s.RequiredSlots = stringsArray(row["required_slots"])
		if slots, ok := row["slots"].(map[string]any); ok {
			s.RequiredSlots = stringsArray(slots["required"])
		}
		if examples, ok := row["examples"].(map[string]any); ok {
			s.Examples = append(stringsArray(examples["ru"]), stringsArray(examples["kk"])...)
		} else {
			s.Examples = stringsArray(row["examples"])
		}
		if boundaries, ok := row["not_this_if"].([]any); ok {
			parts := []string{}
			for _, entry := range boundaries {
				if m, ok := entry.(map[string]any); ok {
					parts = append(parts, firstString(m, "condition")+" → "+firstString(m, "use_instead"))
				}
			}
			s.Boundaries = strings.Join(parts, "; ")
		}
		if responses, ok := row["responses"].(map[string]any); ok {
			for lang, target := range map[string]*string{"ru": &s.ResponseRU, "kk": &s.ResponseKK} {
				if local, ok := responses[lang].(map[string]any); ok {
					*target = firstString(local, "opening")
				}
			}
		}
		if responses, ok := row["response"].(map[string]any); ok {
			s.ResponseRU = firstString(responses, "ru")
			s.ResponseKK = firstString(responses, "kk")
		}
		if s.ResponseRU == "" {
			s.ResponseRU = "Я помогу с вопросом «" + s.Name + "». Уточните, пожалуйста, детали обращения."
		}
		if s.ResponseKK == "" {
			s.ResponseKK = "Осы мәселе бойынша көмектесемін. Өтінішіңіздің мәліметтерін нақтылаңыз."
		}
		c.Scenarios = append(c.Scenarios, s)
		compact = append(compact, map[string]any{"id": s.ID, "name": s.Name, "description": s.Description, "boundaries": s.Boundaries, "priority": s.Priority, "slots": row["slots"], "examples": s.Examples, "system_behavior": row["behavior"]})
	}
	c.Raw, _ = json.Marshal(map[string]any{"as_of_date": c.AsOf, "scenarios": compact})
	for name, target := range map[string]*json.RawMessage{"knowledge_base.json": &c.Knowledge, "mock_backend.json": &c.Customers} {
		data, e := os.ReadFile(filepath.Join(dir, name))
		if os.IsNotExist(e) {
			*target = json.RawMessage(`{}`)
			continue
		}
		if e != nil {
			return nil, e
		}
		if !json.Valid(data) {
			return nil, fmt.Errorf("invalid %s", name)
		}
		*target = data
	}
	if data, e := os.ReadFile(filepath.Join(dir, "slots.json")); e == nil {
		var defs struct {
			Slots []SlotDefinition `json:"slots"`
		}
		if e = json.Unmarshal(data, &defs); e != nil {
			return nil, e
		}
		for _, d := range defs.Slots {
			c.Slots[d.Name] = d
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	return c, nil
}
func stringsArray(v any) []string {
	out := []string{}
	if a, ok := v.([]any); ok {
		for _, x := range a {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
func (c *Catalog) Find(id string) (Scenario, bool) {
	for _, s := range c.Scenarios {
		if s.ID == id {
			return s, true
		}
	}
	return Scenario{}, false
}
func (c *Catalog) Validate(d Decision) error {
	if d.Status != "route" && d.Status != "clarify" && d.Status != "handoff" {
		return fmt.Errorf("invalid status")
	}
	if d.Language != "ru" && d.Language != "kk" && d.Language != "mixed" {
		return fmt.Errorf("invalid language")
	}
	if d.Confidence < 0 || d.Confidence > 1 || strings.TrimSpace(d.Reason) == "" {
		return fmt.Errorf("invalid confidence/reason")
	}
	if d.ScenarioID != "" {
		if _, ok := c.Find(d.ScenarioID); !ok {
			return fmt.Errorf("unknown scenario")
		}
	} else if d.Status == "route" {
		return fmt.Errorf("route requires scenario")
	}
	if len(d.Alternatives) > 3 || len(d.Pending) > 4 || len(d.Slots) > 12 {
		return fmt.Errorf("too many alternatives/pending/slots")
	}
	for _, a := range d.Alternatives {
		if _, ok := c.Find(a.ScenarioID); !ok || a.Reason == "" {
			return fmt.Errorf("invalid alternative")
		}
	}
	for _, id := range d.Pending {
		if _, ok := c.Find(id); !ok {
			return fmt.Errorf("invalid pending scenario")
		}
	}
	for _, s := range d.Slots {
		if s.Name == "" || len(s.Value) > 200 {
			return fmt.Errorf("invalid slot")
		}
	}
	return nil
}

// Reply is grounded in curated catalog data. No LLM can claim an executed operation.
func (c *Catalog) Reply(d *Decision) (string, []string) {
	warnings := []string{}
	if d.ScenarioID == "SYS_UNCLEAR" {
		d.Status = "clarify"
	}
	if d.ScenarioID == "SC37" && c.Source == "organizer_saqta" {
		d.Status = "handoff"
	}
	kk := d.Language == "kk"
	if d.Status == "route" && d.Confidence < 0.70 {
		d.Status = "clarify"
		warnings = append(warnings, "Низкая уверенность модели: нужен переспрос. Это самооценка, не вероятность правильности.")
	}
	if d.Status == "handoff" {
		if kk {
			return "Оператор қажет. Әңгіме контексті дайын; осы демода нақты операторға қосылу жоқ.", warnings
		}
		return "Нужна помощь оператора. Контекст обращения подготовлен; в этом демо реального подключения к оператору нет.", warnings
	}
	if d.Status == "clarify" {
		if kk {
			return "Өтінішіңізді нақтылаңыз: қандай мәселені бірінші шешеміз?", warnings
		}
		r := "Уточните, пожалуйста, какой вопрос решаем первым?"
		if len(d.Alternatives) >= 2 {
			a, _ := c.Find(d.Alternatives[0].ScenarioID)
			b, _ := c.Find(d.Alternatives[1].ScenarioID)
			r = "Уточните, пожалуйста: «" + a.Name + "» или «" + b.Name + "»?"
		}
		return r, warnings
	}
	s, ok := c.Find(d.ScenarioID)
	if !ok {
		return "Нужна помощь оператора.", []string{"Сценарий отсутствует в каталоге."}
	}
	reply := s.ResponseRU
	if kk {
		reply = s.ResponseKK
	}
	if s.RequiresConfirmation {
		warnings = append(warnings, "Требуется отдельное подтверждение клиента. Операции в демо не выполняются.")
		if kk {
			reply += " Бұл демода өзгерістер енгізілмейді."
		} else {
			reply += " В этом демо изменения не выполняются."
		}
	}
	return reply, warnings
}
