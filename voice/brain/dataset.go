package brain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// Dataset is the official starter kit: scenario catalog, knowledge base and
// mock client records.
type Dataset struct {
	AsOf          string            // snapshot date, "today" for the assistant
	Scenarios     []Scenario        // catalog, in file order
	System        []SystemIntent    // SYS_OUT_OF_SCOPE, SYS_UNCLEAR, SYS_GOODBYE
	Clients       []Client          // mock clients
	Policies      []json.RawMessage // mock policies, compact JSON
	Claims        []json.RawMessage // mock claims, compact JSON
	Payments      []json.RawMessage // mock payments, compact JSON
	KnowledgeJSON string            // minified knowledge_base.json
}

// Scenario is one entry of scenarios.json.
type Scenario struct {
	ID                     string              `json:"scenario_id"`
	Slug                   string              `json:"slug"`
	Name                   string              `json:"name"`
	Domain                 string              `json:"domain"`
	Category               string              `json:"category"`
	Description            string              `json:"description"`
	NotThisIf              []Boundary          `json:"not_this_if"`
	Priority               string              `json:"priority"` // normal | high | urgent
	FastPath               bool                `json:"fast_path_eligible"`
	RequiresIdentification bool                `json:"requires_identification"`
	Slots                  Slots               `json:"slots"`
	Actions                []string            `json:"actions"`
	RequiresConfirmation   bool                `json:"requires_confirmation"`
	Handoff                *Handoff            `json:"handoff"`
	Examples               map[string][]string `json:"examples"`  // by language
	Responses              map[string]Lines    `json:"responses"` // by language
}

// Boundary names a situation where another scenario fits better.
type Boundary struct {
	Condition  string `json:"condition"`
	UseInstead string `json:"use_instead"`
}

// Slots lists the details a scenario collects.
type Slots struct {
	Required []string `json:"required"`
	Optional []string `json:"optional"`
}

// Handoff says when a scenario is passed to a human queue.
type Handoff struct {
	When  string `json:"when"`
	Queue string `json:"queue"`
}

// Lines are a scenario's reference replies in one language.
type Lines struct {
	Opening string `json:"opening"`
	Closing string `json:"closing"`
}

// SystemIntent is a non-business intent such as SYS_UNCLEAR.
type SystemIntent struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Behavior    string            `json:"behavior"`
	Response    map[string]string `json:"response"` // by language
}

// Client is a mock_backend.json client.
type Client struct {
	ID                string `json:"client_id"`
	FullName          string `json:"full_name"`
	Phone             string `json:"phone"`
	IIN               string `json:"iin"`
	City              string `json:"city"`
	Email             string `json:"email"`
	Address           string `json:"address"`
	BMClass           string `json:"bm_class"`
	PreferredLanguage string `json:"preferred_language"`

	raw json.RawMessage // original record, compact
}

// LoadDataset reads scenarios.json, knowledge_base.json and mock_backend.json
// from dir.
func LoadDataset(dir string) (*Dataset, error) {
	var sc struct {
		Meta struct {
			AsOf string `json:"as_of_date"`
		} `json:"meta"`
		Scenarios []Scenario     `json:"scenarios"`
		System    []SystemIntent `json:"system_intents"`
	}
	if err := readJSON(filepath.Join(dir, "scenarios.json"), &sc); err != nil {
		return nil, err
	}
	if len(sc.Scenarios) == 0 {
		return nil, fmt.Errorf("brain: %s: no scenarios", filepath.Join(dir, "scenarios.json"))
	}

	var kb json.RawMessage
	if err := readJSON(filepath.Join(dir, "knowledge_base.json"), &kb); err != nil {
		return nil, err
	}

	var mb struct {
		Clients  []json.RawMessage `json:"clients"`
		Policies []json.RawMessage `json:"policies"`
		Claims   []json.RawMessage `json:"claims"`
		Payments []json.RawMessage `json:"payments"`
	}
	if err := readJSON(filepath.Join(dir, "mock_backend.json"), &mb); err != nil {
		return nil, err
	}

	d := &Dataset{
		AsOf:          sc.Meta.AsOf,
		Scenarios:     sc.Scenarios,
		System:        sc.System,
		Policies:      compactAll(mb.Policies),
		Claims:        compactAll(mb.Claims),
		Payments:      compactAll(mb.Payments),
		KnowledgeJSON: string(compact(kb)),
	}
	if d.AsOf == "" {
		var meta struct {
			Meta struct {
				AsOf string `json:"as_of_date"`
			} `json:"meta"`
		}
		_ = json.Unmarshal(kb, &meta)
		d.AsOf = meta.Meta.AsOf
	}
	for _, raw := range mb.Clients {
		var c Client
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("brain: mock_backend.json client: %w", err)
		}
		c.raw = compact(raw)
		d.Clients = append(d.Clients, c)
	}
	return d, nil
}

// readJSON decodes the JSON file at path into v.
func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("brain: %w", err)
	}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")) // UTF-8 BOM
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("brain: %s: %w", path, err)
	}
	return nil
}

// compact returns raw without insignificant whitespace.
func compact(raw []byte) []byte {
	var b bytes.Buffer
	if json.Compact(&b, raw) != nil {
		return raw
	}
	return b.Bytes()
}

func compactAll(raws []json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, len(raws))
	for i, r := range raws {
		out[i] = compact(r)
	}
	return out
}

// ScenarioByID returns the catalog scenario with the given id, or nil.
func (d *Dataset) ScenarioByID(id string) *Scenario {
	if d == nil {
		return nil
	}
	for i := range d.Scenarios {
		if d.Scenarios[i].ID == id {
			return &d.Scenarios[i]
		}
	}
	return nil
}

// systemIntent returns the system intent with the given id, or nil.
func (d *Dataset) systemIntent(id string) *SystemIntent {
	if d == nil {
		return nil
	}
	for i := range d.System {
		if d.System[i].ID == id {
			return &d.System[i]
		}
	}
	return nil
}

// canonicalID maps near-miss ids onto catalog ids: "SC7" -> "SC07",
// "UNCLEAR" -> "SYS_UNCLEAR".
func (d *Dataset) canonicalID(id string) string {
	known := func(id string) bool { return d.ScenarioByID(id) != nil || d.systemIntent(id) != nil }
	if d == nil || known(id) {
		return id
	}
	if n, err := strconv.Atoi(strings.TrimPrefix(id, "SC")); err == nil && strings.HasPrefix(id, "SC") {
		if c := fmt.Sprintf("SC%02d", n); known(c) {
			return c
		}
	}
	if c := "SYS_" + id; known(c) {
		return c
	}
	return id
}

// describe returns the catalog definition of id. It serves as the decision
// reason: the model writes no free-text reasoning, which keeps TTFT low.
func (d *Dataset) describe(id string) string {
	if s := d.ScenarioByID(id); s != nil {
		return s.Name + ": " + s.Description
	}
	if s := d.systemIntent(id); s != nil {
		return s.Description
	}
	return ""
}

// status classifies a scenario id for Decision.Status: route, clarify,
// out_of_scope, goodbye, or handoff for scenarios that always hand off.
func status(d *Dataset, id string) string {
	switch id {
	case "SYS_UNCLEAR":
		return "clarify"
	case "SYS_OUT_OF_SCOPE":
		return "out_of_scope"
	case "SYS_GOODBYE":
		return "goodbye"
	}
	if s := d.ScenarioByID(id); s != nil && s.Handoff != nil && s.Handoff.When == "always" {
		return "handoff"
	}
	return "route"
}

// ClientByPhone finds a client by phone number in any common spelling.
func (d *Dataset) ClientByPhone(phone string) *Client {
	p := NormalizePhone(phone)
	if d == nil || p == "" {
		return nil
	}
	for i := range d.Clients {
		if NormalizePhone(d.Clients[i].Phone) == p {
			return &d.Clients[i]
		}
	}
	return nil
}

// ClientByIIN finds a client by the 12-digit IIN.
func (d *Dataset) ClientByIIN(iin string) *Client {
	id := digitsOf(iin)
	if d == nil || len(id) != 12 {
		return nil
	}
	for i := range d.Clients {
		if digitsOf(d.Clients[i].IIN) == id {
			return &d.Clients[i]
		}
	}
	return nil
}

// Card returns compact JSON with the client and their policies, claims and
// payments: {"client":{...},"policies":[...],"claims":[...],"payments":[...]}.
func (d *Dataset) Card(c *Client) string {
	if d == nil || c == nil {
		return ""
	}
	var b bytes.Buffer
	b.WriteString(`{"client":`)
	if len(c.raw) > 0 {
		b.Write(c.raw)
	} else {
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(c)
		b.Truncate(b.Len() - 1) // Encode adds a newline
	}
	for _, part := range []struct {
		name string
		recs []json.RawMessage
	}{{"policies", d.Policies}, {"claims", d.Claims}, {"payments", d.Payments}} {
		fmt.Fprintf(&b, `,%q:[`, part.name)
		n := 0
		for _, r := range part.recs {
			var owner struct {
				ClientID string `json:"client_id"`
			}
			if c.ID == "" || json.Unmarshal(r, &owner) != nil || owner.ClientID != c.ID {
				continue
			}
			if n > 0 {
				b.WriteByte(',')
			}
			b.Write(r)
			n++
		}
		b.WriteByte(']')
	}
	b.WriteByte('}')
	return b.String()
}

// NormalizePhone keeps the digits of s and returns a +7 number:
// 8XXXXXXXXXX and 7XXXXXXXXXX become +7XXXXXXXXXX, ten digits starting with
// 7 get +7 in front. Anything else yields "".
func NormalizePhone(s string) string {
	d := digitsOf(s)
	switch {
	case len(d) == 11 && (d[0] == '8' || d[0] == '7'):
		return "+7" + d[1:]
	case len(d) == 10 && d[0] == '7':
		return "+7" + d
	}
	return ""
}

// FindPhone returns the first Kazakhstan phone number in free text
// ("8 701 000 00 01", "+7 (701) 000-00-01"), normalized, or "".
func FindPhone(text string) string {
	for _, run := range digitRuns(text, isPhoneSep) {
		for i := range run {
			s := ""
			for _, g := range run[i:] {
				s += g
				if len(s) > 11 {
					break
				}
				// Kazakhstan numbers are +7 7xx / +7 6xx.
				if p := NormalizePhone(s); strings.HasPrefix(p, "+77") || strings.HasPrefix(p, "+76") {
					return p
				}
			}
		}
	}
	return ""
}

// FindIIN returns the first 12-digit number in text (digit groups may be
// separated by spaces), or "".
func FindIIN(text string) string {
	for _, run := range digitRuns(text, unicode.IsSpace) {
		for i := range run {
			s := ""
			for _, g := range run[i:] {
				s += g
				if len(s) >= 12 {
					if len(s) == 12 {
						return s
					}
					break
				}
			}
		}
	}
	return ""
}

// digitRuns splits text into runs of digit groups; groups joined only by
// separator runes belong to the same run.
func digitRuns(text string, sep func(rune) bool) [][]string {
	var runs [][]string
	var run []string
	start := -1
	for i, r := range text {
		if r >= '0' && r <= '9' {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			run = append(run, text[start:i])
			start = -1
		}
		if !sep(r) && len(run) > 0 {
			runs = append(runs, run)
			run = nil
		}
	}
	if start >= 0 {
		run = append(run, text[start:])
	}
	if len(run) > 0 {
		runs = append(runs, run)
	}
	return runs
}

// isPhoneSep reports runes that may appear inside a spoken or written phone number.
func isPhoneSep(r rune) bool {
	switch r {
	case '+', '-', '(', ')', '.', ',', '‐', '‑', '‒', '–', '—':
		return true
	}
	return unicode.IsSpace(r)
}

// digitsOf returns the ASCII digits of s.
func digitsOf(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
