// Package catalog loads the official starter kit (scenarios, slots, actions,
// knowledge base, mock backend) and renders the compact catalog text that the
// LLM router sees. Nothing here is scenario-specific code: change the JSON
// files and the router follows.
package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Boundary struct {
	Condition  string `json:"condition"`
	UseInstead string `json:"use_instead"`
}

type Handoff struct {
	When  string `json:"when"`
	Queue string `json:"queue"`
}

type Scenario struct {
	ID                     string     `json:"scenario_id"`
	Slug                   string     `json:"slug"`
	Name                   string     `json:"name"`
	Domain                 string     `json:"domain"`
	Category               string     `json:"category"`
	Description            string     `json:"description"`
	NotThisIf              []Boundary `json:"not_this_if"`
	Priority               string     `json:"priority"`
	FastPathEligible       bool       `json:"fast_path_eligible"`
	RequiresIdentification bool       `json:"requires_identification"`
	Slots                  struct {
		Required []string `json:"required"`
		Optional []string `json:"optional"`
	} `json:"slots"`
	Actions              []string            `json:"actions"`
	RequiresConfirmation bool                `json:"requires_confirmation"`
	Handoff              *Handoff            `json:"handoff"`
	Examples             map[string][]string `json:"examples"`
	Responses            map[string]struct {
		Opening string `json:"opening"`
		Closing string `json:"closing"`
	} `json:"responses"`
}

type SystemIntent struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Behavior    string            `json:"behavior"`
	Response    map[string]string `json:"response"`
}

type Slot struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Pattern     string            `json:"pattern"`
	Values      []any             `json:"values"`
	Prompt      map[string]string `json:"prompt"`
}

type Action struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Inputs       []string `json:"inputs"`
	Outputs      []string `json:"outputs"`
	Errors       []string `json:"errors"`
	Irreversible bool     `json:"irreversible"`
}

type scenariosFile struct {
	Meta struct {
		Dataset  string `json:"dataset"`
		Version  string `json:"version"`
		AsOfDate string `json:"as_of_date"`
	} `json:"meta"`
	Scenarios     []Scenario     `json:"scenarios"`
	SystemIntents []SystemIntent `json:"system_intents"`
}

type slotsFile struct {
	Slots []Slot `json:"slots"`
}

type actionsFile struct {
	Queues        []string          `json:"queues"`
	ErrorCodes    map[string]string `json:"error_codes"`
	ErrorHandling []string          `json:"error_handling"`
	Actions       []Action          `json:"actions"`
}

// Catalog is the loaded starter kit.
type Catalog struct {
	mu            sync.RWMutex
	dir           string
	AsOfDate      string
	Scenarios     []Scenario
	SystemIntents []SystemIntent
	Slots         []Slot
	Actions       []Action
	Queues        []string
	ErrorCodes    map[string]string
	KB            map[string]any // knowledge_base.json, raw
	Backend       map[string]any // mock_backend.json, raw

	byID     map[string]*Scenario
	slotByNm map[string]*Slot
	actByNm  map[string]*Action
	sysByID  map[string]*SystemIntent
}

// Load reads every kit file from dir.
func Load(dir string) (*Catalog, error) {
	c := &Catalog{dir: dir}
	if err := c.Reload(); err != nil {
		return nil, err
	}
	return c, nil
}

// Reload re-reads the JSON files (used by POST /api/catalog/reload so the
// catalog can be edited without a restart).
func (c *Catalog) Reload() error {
	var sf scenariosFile
	if err := readJSON(filepath.Join(c.dir, "scenarios.json"), &sf); err != nil {
		return err
	}
	var slf slotsFile
	if err := readJSON(filepath.Join(c.dir, "slots.json"), &slf); err != nil {
		return err
	}
	var af actionsFile
	if err := readJSON(filepath.Join(c.dir, "actions.json"), &af); err != nil {
		return err
	}
	var kb, be map[string]any
	if err := readJSON(filepath.Join(c.dir, "knowledge_base.json"), &kb); err != nil {
		return err
	}
	if err := readJSON(filepath.Join(c.dir, "mock_backend.json"), &be); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.AsOfDate = sf.Meta.AsOfDate
	c.Scenarios = sf.Scenarios
	c.SystemIntents = sf.SystemIntents
	c.Slots = slf.Slots
	c.Actions = af.Actions
	c.Queues = af.Queues
	c.ErrorCodes = af.ErrorCodes
	c.KB = kb
	c.Backend = be
	c.byID = map[string]*Scenario{}
	for i := range c.Scenarios {
		c.byID[c.Scenarios[i].ID] = &c.Scenarios[i]
	}
	c.sysByID = map[string]*SystemIntent{}
	for i := range c.SystemIntents {
		c.sysByID[c.SystemIntents[i].ID] = &c.SystemIntents[i]
	}
	c.slotByNm = map[string]*Slot{}
	for i := range c.Slots {
		c.slotByNm[c.Slots[i].Name] = &c.Slots[i]
	}
	c.actByNm = map[string]*Action{}
	for i := range c.Actions {
		c.actByNm[c.Actions[i].Name] = &c.Actions[i]
	}
	return nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func (c *Catalog) Dir() string { return c.dir }

// Scenario returns a scenario by ID (SC01..SC40) or nil.
func (c *Catalog) Scenario(id string) *Scenario {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.byID[id]
}

// System returns a system intent by ID or nil.
func (c *Catalog) System(id string) *SystemIntent {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sysByID[id]
}

// IsKnown reports whether id is a scenario or a system intent.
func (c *Catalog) IsKnown(id string) bool {
	return c.Scenario(id) != nil || c.System(id) != nil
}

func (c *Catalog) Slot(name string) *Slot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.slotByNm[name]
}

func (c *Catalog) Action(name string) *Action {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actByNm[name]
}

// Priority rank: urgent=0, high=1, normal=2 (lower goes first).
func PriorityRank(p string) int {
	switch p {
	case "urgent":
		return 0
	case "high":
		return 1
	default:
		return 2
	}
}

// PromptCatalog renders the compact scenario catalog for the LLM system prompt.
// It is deterministic so the provider's prompt cache stays warm.
func (c *Catalog) PromptCatalog() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var b strings.Builder
	for _, s := range c.Scenarios {
		flags := []string{}
		if s.Priority != "normal" {
			flags = append(flags, strings.ToUpper(s.Priority))
		}
		if s.RequiresIdentification {
			flags = append(flags, "ID")
		}
		if s.RequiresConfirmation {
			flags = append(flags, "CONFIRM")
		}
		if s.FastPathEligible {
			flags = append(flags, "INFO")
		}
		fmt.Fprintf(&b, "%s %s", s.ID, s.Name)
		if len(flags) > 0 {
			fmt.Fprintf(&b, " [%s]", strings.Join(flags, ","))
		}
		fmt.Fprintf(&b, " — %s\n", s.Description)
		for _, nt := range s.NotThisIf {
			fmt.Fprintf(&b, "  NOT if: %s → %s\n", nt.Condition, nt.UseInstead)
		}
		if len(s.Slots.Required) > 0 {
			fmt.Fprintf(&b, "  slots: %s", strings.Join(s.Slots.Required, ", "))
			if len(s.Slots.Optional) > 0 {
				fmt.Fprintf(&b, " (optional: %s)", strings.Join(s.Slots.Optional, ", "))
			}
			b.WriteString("\n")
		} else if len(s.Slots.Optional) > 0 {
			fmt.Fprintf(&b, "  slots: none required (optional: %s)\n", strings.Join(s.Slots.Optional, ", "))
		}
		if len(s.Actions) > 0 {
			fmt.Fprintf(&b, "  actions: %s\n", strings.Join(s.Actions, ", "))
		}
		if s.Handoff != nil {
			fmt.Fprintf(&b, "  handoff → %s when: %s\n", s.Handoff.Queue, s.Handoff.When)
		}
		ex := []string{}
		for _, l := range []string{"ru", "kk"} {
			for i, e := range s.Examples[l] {
				if i >= 2 {
					break
				}
				ex = append(ex, "«"+e+"»")
			}
		}
		if len(ex) > 0 {
			fmt.Fprintf(&b, "  e.g. %s\n", strings.Join(ex, " / "))
		}
	}
	b.WriteString("\nSYSTEM INTENTS:\n")
	for _, s := range c.SystemIntents {
		fmt.Fprintf(&b, "%s — %s Behavior: %s\n", s.ID, s.Description, s.Behavior)
	}
	return b.String()
}

// PromptSlots renders the slot catalog (types, formats, enum values).
func (c *Catalog) PromptSlots() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var b strings.Builder
	for _, s := range c.Slots {
		fmt.Fprintf(&b, "- %s (%s): %s", s.Name, s.Type, s.Description)
		if len(s.Values) > 0 {
			vals := make([]string, 0, len(s.Values))
			for _, v := range s.Values {
				vals = append(vals, fmt.Sprint(v))
			}
			fmt.Fprintf(&b, " — values: %s", strings.Join(vals, "|"))
		} else if s.Pattern != "" {
			fmt.Fprintf(&b, " — format %s", s.Pattern)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// PromptActions renders the action catalog with inputs.
func (c *Catalog) PromptActions() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var b strings.Builder
	for _, a := range c.Actions {
		irr := ""
		if a.Irreversible {
			irr = " [IRREVERSIBLE → preview first]"
		}
		fmt.Fprintf(&b, "- %s(%s) → %s%s: %s\n", a.Name, strings.Join(a.Inputs, ", "), strings.Join(a.Outputs, ", "), irr, a.Description)
	}
	fmt.Fprintf(&b, "Handoff queues: %s\n", strings.Join(c.Queues, ", "))
	return b.String()
}

// ScenarioIDs returns all scenario IDs in catalog order.
func (c *Catalog) ScenarioIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ids := make([]string, 0, len(c.Scenarios))
	for _, s := range c.Scenarios {
		ids = append(ids, s.ID)
	}
	return ids
}

// Snapshot returns copies of the scenario and system intent lists for the API.
func (c *Catalog) Snapshot() ([]Scenario, []SystemIntent) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	sc := make([]Scenario, len(c.Scenarios))
	copy(sc, c.Scenarios)
	si := make([]SystemIntent, len(c.SystemIntents))
	copy(si, c.SystemIntents)
	return sc, si
}

// KBSection returns a top-level knowledge base section by dotted path
// (e.g. "products.dms" or "claims.documents").
func (c *Catalog) KBSection(path string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var cur any = c.KB
	for _, p := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// KBTopics lists the top-level knowledge base keys (for kb_lookup hints).
func (c *Catalog) KBTopics() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]string, 0, len(c.KB))
	for k := range c.KB {
		if k == "meta" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
