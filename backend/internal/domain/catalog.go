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

type Catalog struct {
	Scenarios []Scenario      `json:"scenarios"`
	Source    string          `json:"source"`
	Hash      string          `json:"hash"`
	Raw       json.RawMessage `json:"-"`
	Knowledge json.RawMessage `json:"-"`
	Customers json.RawMessage `json:"-"`
}

func LoadCatalog(dir string) (*Catalog, error) {
	b, err := os.ReadFile(filepath.Join(dir, "scenarios.json"))
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err = json.Unmarshal(b, &rows); err != nil {
		var envelope struct {
			Scenarios []map[string]any `json:"scenarios"`
		}
		if err = json.Unmarshal(b, &envelope); err != nil {
			return nil, err
		}
		rows = envelope.Scenarios
	}
	if len(rows) == 0 || len(rows) > 200 {
		return nil, fmt.Errorf("catalog must contain 1..200 scenarios")
	}
	c := &Catalog{Raw: b, Source: "external", Scenarios: []Scenario{}}
	if _, err := os.Stat(filepath.Join(dir, "SYNTHETIC_DEMO")); err == nil {
		c.Source = "synthetic_demo"
	}
	sum := sha256.Sum256(b)
	c.Hash = hex.EncodeToString(sum[:])[:16]
	seen := map[string]bool{}
	for _, row := range rows {
		encoded, _ := json.Marshal(row)
		var s Scenario
		_ = json.Unmarshal(encoded, &s)
		s.ID = firstString(row, "id", "scenario_id", "code")
		s.Name = firstString(row, "name", "title", "name_ru")
		s.Description = firstString(row, "description", "purpose", "description_ru")
		if s.ID == "" || s.Name == "" || seen[s.ID] {
			return nil, fmt.Errorf("each scenario needs a unique string id/scenario_id and name/title; invalid %q", s.ID)
		}
		seen[s.ID] = true
		s.Raw = row
		if s.ResponseRU == "" {
			s.ResponseRU = "Я помогу с вопросом «" + s.Name + "». Уточните, пожалуйста, детали обращения."
		}
		if s.ResponseKK == "" {
			s.ResponseKK = "Осы мәселе бойынша көмектесемін. Өтінішіңіздің мәліметтерін нақтылаңыз."
		}
		c.Scenarios = append(c.Scenarios, s)
	}
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
	return c, nil
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
