// Package mockbackend implements the actions from actions.json over
// mock_backend.json and knowledge_base.json. Every action returns either a
// result map or an error in the kit's format {"error":{"code","message"}}.
// Irreversible actions are executed only by the dialogue engine after an
// explicit client confirmation; this package just performs them.
package mockbackend

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"hackathon/backend/internal/catalog"
)

type Client struct {
	ClientID          string `json:"client_id"`
	FullName          string `json:"full_name"`
	Phone             string `json:"phone"`
	IIN               string `json:"iin"`
	City              string `json:"city"`
	Email             string `json:"email"`
	Address           string `json:"address"`
	BMClass           string `json:"bm_class"`
	PreferredLanguage string `json:"preferred_language"`
}

type Policy struct {
	PolicyNumber string         `json:"policy_number"`
	ClientID     string         `json:"client_id"`
	Product      string         `json:"product"`
	StartDate    string         `json:"start_date"`
	EndDate      string         `json:"end_date"`
	Premium      *float64       `json:"premium"`
	Details      map[string]any `json:"details"`
	Status       string         `json:"status,omitempty"`
	Cancelled    bool           `json:"-"`
}

type Claim struct {
	ClaimNumber      string   `json:"claim_number"`
	ClientID         string   `json:"client_id"`
	PolicyNumber     string   `json:"policy_number"`
	ClaimType        string   `json:"claim_type"`
	IncidentDate     string   `json:"incident_date"`
	Status           string   `json:"status"`
	ApprovedAmount   *float64 `json:"approved_amount,omitempty"`
	AssessorEstimate *float64 `json:"assessor_estimate,omitempty"`
	DecisionDate     string   `json:"decision_date,omitempty"`
	DecisionDue      string   `json:"decision_due,omitempty"`
	InspectionDate   string   `json:"inspection_date,omitempty"`
	MissingDocuments []string `json:"missing_documents,omitempty"`
	NextStep         string   `json:"next_step"`
}

type Payment struct {
	PaymentID    string  `json:"payment_id"`
	ClientID     string  `json:"client_id"`
	Date         string  `json:"date"`
	Amount       float64 `json:"amount"`
	Product      string  `json:"product"`
	Status       string  `json:"status"`
	PolicyNumber *string `json:"policy_number"`
	Note         string  `json:"note,omitempty"`
}

type data struct {
	Defaults struct {
		UnknownIINBMClass string `json:"unknown_iin_bm_class"`
	} `json:"defaults"`
	Clients  []Client  `json:"clients"`
	Policies []Policy  `json:"policies"`
	Claims   []Claim   `json:"claims"`
	Payments []Payment `json:"payments"`
}

// Backend is the in-memory mock backend.
type Backend struct {
	mu       sync.Mutex
	cat      *catalog.Catalog
	d        data
	today    time.Time
	counters map[string]int
	log      []ActionLog
}

// ActionLog records every executed action (for the supervisor panel).
type ActionLog struct {
	At     time.Time      `json:"at"`
	Name   string         `json:"name"`
	Args   map[string]any `json:"args"`
	Result map[string]any `json:"result"`
	Err    *ActionError   `json:"error,omitempty"`
}

// ActionError is the kit's error format.
type ActionError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ActionError) Error() string { return e.Code + ": " + e.Message }

func errf(code, format string, a ...any) *ActionError {
	return &ActionError{Code: code, Message: fmt.Sprintf(format, a...)}
}

// New builds the backend from the catalog's raw mock_backend.json.
func New(cat *catalog.Catalog) (*Backend, error) {
	b := &Backend{cat: cat, counters: map[string]int{}}
	raw, err := json.Marshal(cat.Backend)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &b.d); err != nil {
		return nil, fmt.Errorf("mock backend: %w", err)
	}
	b.today, err = time.Parse("2006-01-02", cat.AsOfDate)
	if err != nil {
		b.today = time.Now()
	}
	b.counters["claim"] = 500400
	b.counters["policy"] = 105200
	b.counters["ticket"] = 700200
	b.counters["fraud"] = 900100
	return b, nil
}

// Today returns the dataset snapshot date.
func (b *Backend) Today() time.Time { return b.today }

// Log returns a copy of the action log.
func (b *Backend) Log() []ActionLog {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]ActionLog, len(b.log))
	copy(out, b.log)
	return out
}

// IsIrreversible reports whether the action needs confirmation.
func (b *Backend) IsIrreversible(name string) bool {
	if a := b.cat.Action(name); a != nil {
		return a.Irreversible
	}
	return false
}

// Execute runs an action by name. The result is a map (possibly empty) or an
// ActionError.
func (b *Backend) Execute(name string, args map[string]any) (map[string]any, *ActionError) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cat.Action(name) == nil {
		return nil, errf("invalid_input", "unknown action %s", name)
	}
	res, err := b.run(name, args)
	b.log = append(b.log, ActionLog{At: time.Now(), Name: name, Args: args, Result: res, Err: err})
	if len(b.log) > 1000 {
		b.log = b.log[len(b.log)-1000:]
	}
	return res, err
}

func str(args map[string]any, key string) string {
	if v, ok := args[key]; ok && v != nil {
		switch t := v.(type) {
		case string:
			return strings.TrimSpace(t)
		case float64:
			if t == math.Trunc(t) {
				return strconv.FormatInt(int64(t), 10)
			}
			return strconv.FormatFloat(t, 'f', -1, 64)
		case int:
			return strconv.Itoa(t)
		case bool:
			return strconv.FormatBool(t)
		default:
			return fmt.Sprint(t)
		}
	}
	return ""
}

func num(args map[string]any, key string) (float64, bool) {
	v := str(args, key)
	v = strings.ReplaceAll(v, " ", "")
	if v == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	return f, err == nil
}

func list(args map[string]any, key string) []string {
	switch t := args[key].(type) {
	case []any:
		out := []string{}
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return out
	case []string:
		return t
	case string:
		if t == "" {
			return nil
		}
		parts := strings.FieldsFunc(t, func(r rune) bool { return r == ',' || r == ';' || r == ' ' })
		return parts
	}
	return nil
}

func normPhone(p string) string {
	d := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, p)
	if len(d) == 11 && (d[0] == '7' || d[0] == '8') {
		return "+7" + d[1:]
	}
	if len(d) == 10 {
		return "+7" + d
	}
	return p
}

func (b *Backend) findClient(phone, iin string) *Client {
	for i := range b.d.Clients {
		c := &b.d.Clients[i]
		if phone != "" && c.Phone == phone {
			return c
		}
		if iin != "" && c.IIN == iin {
			return c
		}
	}
	return nil
}

func (b *Backend) clientByID(id string) *Client {
	for i := range b.d.Clients {
		if b.d.Clients[i].ClientID == id {
			return &b.d.Clients[i]
		}
	}
	return nil
}

func (b *Backend) policyStatus(p *Policy) string {
	if p.Cancelled {
		return "cancelled"
	}
	start, _ := time.Parse("2006-01-02", p.StartDate)
	end, _ := time.Parse("2006-01-02", p.EndDate)
	switch {
	case b.today.Before(start):
		return "not_yet_active"
	case b.today.After(end):
		return "expired"
	default:
		return "active"
	}
}

func (b *Backend) policyView(p *Policy) map[string]any {
	m := map[string]any{
		"policy_number": p.PolicyNumber, "product": p.Product, "status": b.policyStatus(p),
		"start_date": p.StartDate, "end_date": p.EndDate, "details": p.Details, "client_id": p.ClientID,
	}
	if p.Premium != nil {
		m["premium"] = *p.Premium
	}
	return m
}

func (b *Backend) claimView(c *Claim) map[string]any {
	raw, _ := json.Marshal(c)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}

func (b *Backend) nextID(kind, prefix string) string {
	b.counters[kind]++
	return fmt.Sprintf("%s%d", prefix, b.counters[kind])
}

// ClientProfile returns a redacted profile plus policies, claims and payments
// for prompt facts (used by the engine after identification).
func (b *Backend) ClientProfile(clientID string) map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.clientByID(clientID)
	if c == nil {
		return nil
	}
	pols := []map[string]any{}
	for i := range b.d.Policies {
		if b.d.Policies[i].ClientID == clientID {
			pols = append(pols, b.policyView(&b.d.Policies[i]))
		}
	}
	claims := []map[string]any{}
	for i := range b.d.Claims {
		if b.d.Claims[i].ClientID == clientID {
			claims = append(claims, b.claimView(&b.d.Claims[i]))
		}
	}
	pays := []map[string]any{}
	for _, p := range b.d.Payments {
		if p.ClientID == clientID {
			raw, _ := json.Marshal(p)
			var m map[string]any
			_ = json.Unmarshal(raw, &m)
			pays = append(pays, m)
		}
	}
	return map[string]any{
		"client_id": c.ClientID, "full_name": c.FullName, "phone": c.Phone, "city": c.City,
		"email_masked": maskEmail(c.Email), "bm_class": c.BMClass, "preferred_language": c.PreferredLanguage,
		"policies": pols, "claims": claims, "payments": pays,
	}
}

func maskEmail(e string) string {
	at := strings.Index(e, "@")
	if at <= 1 {
		return e
	}
	return e[:1] + "***" + e[at:]
}

func (b *Backend) run(name string, args map[string]any) (map[string]any, *ActionError) {
	switch name {
	case "find_client":
		phone := normPhone(str(args, "phone"))
		iin := str(args, "iin")
		if phone == "" && iin == "" {
			return nil, errf("invalid_input", "phone or iin is required")
		}
		c := b.findClient(phone, iin)
		if c == nil {
			id := phone
			if id == "" {
				id = "IIN " + iin
			}
			return nil, errf("not_found", "Client with %s not found", id)
		}
		return map[string]any{"client_id": c.ClientID, "full_name": c.FullName, "city": c.City, "preferred_language": c.PreferredLanguage, "email_masked": maskEmail(c.Email), "bm_class": c.BMClass}, nil

	case "get_policies":
		cid := str(args, "client_id")
		if b.clientByID(cid) == nil {
			return nil, errf("not_found", "client %s not found", cid)
		}
		out := []map[string]any{}
		for i := range b.d.Policies {
			if b.d.Policies[i].ClientID == cid {
				out = append(out, b.policyView(&b.d.Policies[i]))
			}
		}
		return map[string]any{"policies": out}, nil

	case "get_policy":
		pn := strings.ToUpper(str(args, "policy_number"))
		plate := strings.ToUpper(strings.ReplaceAll(str(args, "vehicle_plate"), " ", ""))
		if pn == "" && plate == "" {
			return nil, errf("invalid_input", "policy_number or vehicle_plate is required")
		}
		for i := range b.d.Policies {
			p := &b.d.Policies[i]
			if (pn != "" && p.PolicyNumber == pn) || (plate != "" && fmt.Sprint(p.Details["vehicle_plate"]) == plate) {
				v := b.policyView(p)
				if c := b.clientByID(p.ClientID); c != nil {
					v["holder_name"] = c.FullName
				}
				return v, nil
			}
		}
		return nil, errf("not_found", "policy %s%s not found", pn, plate)

	case "get_bm_class":
		iin := str(args, "iin")
		if len(iin) != 12 {
			return nil, errf("invalid_input", "iin must be 12 digits")
		}
		if c := b.findClient("", iin); c != nil {
			return map[string]any{"bm_class": c.BMClass, "iin": iin}, nil
		}
		return map[string]any{"bm_class": b.d.Defaults.UnknownIINBMClass, "iin": iin, "note": "unknown IIN, start class"}, nil

	case "calc_ogpo_price":
		return b.calcOGPO(args)
	case "calc_casco_price":
		return b.calcCASCO(args)
	case "calc_travel_price":
		return b.calcTravel(args)
	case "calc_property_price":
		return b.calcProperty(args)
	case "calc_accident_price":
		return b.calcAccident(args)

	case "create_policy":
		prod := strings.ToLower(str(args, "product_type"))
		phone := normPhone(str(args, "phone"))
		if prod == "" {
			return nil, errf("invalid_input", "product_type is required")
		}
		prefix := map[string]string{"ogpo": "SQ-OGPO-", "casco": "SQ-CASCO-", "travel": "SQ-TRVL-", "property": "SQ-PROP-", "accident": "SQ-NS-", "dms": "SQ-DMS-"}[prod]
		if prefix == "" {
			return nil, errf("invalid_input", "unknown product_type %s", prod)
		}
		pn := b.nextID("policy", prefix)
		cid := ""
		if c := b.findClient(phone, ""); c != nil {
			cid = c.ClientID
		}
		details := map[string]any{}
		for k, v := range args {
			if k != "product_type" && k != "phone" {
				details[k] = v
			}
		}
		b.d.Policies = append(b.d.Policies, Policy{PolicyNumber: pn, ClientID: cid, Product: prod, StartDate: b.today.Format("2006-01-02"), EndDate: b.today.AddDate(1, 0, -1).Format("2006-01-02"), Details: details})
		return map[string]any{"policy_number": pn, "payment_link_sent_to": phone, "status": "awaiting_payment"}, nil

	case "renew_policy":
		pn := strings.ToUpper(str(args, "policy_number"))
		for i := range b.d.Policies {
			p := &b.d.Policies[i]
			if p.PolicyNumber == pn {
				price := 0.0
				if p.Premium != nil {
					price = *p.Premium
				}
				newPN := b.nextID("policy", pn[:strings.LastIndex(pn, "-")+1])
				end, _ := time.Parse("2006-01-02", p.EndDate)
				start := end.AddDate(0, 0, 1)
				if start.Before(b.today) {
					start = b.today
				}
				b.d.Policies = append(b.d.Policies, Policy{PolicyNumber: newPN, ClientID: p.ClientID, Product: p.Product, StartDate: start.Format("2006-01-02"), EndDate: start.AddDate(1, 0, -1).Format("2006-01-02"), Premium: &price, Details: p.Details})
				return map[string]any{"policy_number": newPN, "price": price, "start_date": start.Format("2006-01-02"), "renewed_from": pn}, nil
			}
		}
		return nil, errf("not_found", "policy %s not found", pn)

	case "update_policy":
		pn := strings.ToUpper(str(args, "policy_number"))
		for i := range b.d.Policies {
			p := &b.d.Policies[i]
			if p.PolicyNumber == pn {
				if b.policyStatus(p) != "active" {
					return nil, errf("policy_inactive", "policy %s is %s", pn, b.policyStatus(p))
				}
				extra := 0.0
				if d := str(args, "new_driver_iin"); d != "" {
					drivers, _ := p.Details["drivers_iin"].([]any)
					p.Details["drivers_iin"] = append(drivers, d)
					if p.Premium != nil {
						extra = math.Round(*p.Premium * 0.1)
					}
				}
				if plate := strings.ToUpper(str(args, "vehicle_plate")); plate != "" {
					p.Details["vehicle_plate"] = plate
					extra = 0
				}
				return map[string]any{"policy_number": pn, "extra_premium": extra, "details": p.Details}, nil
			}
		}
		return nil, errf("not_found", "policy %s not found", pn)

	case "cancel_policy":
		pn := strings.ToUpper(str(args, "policy_number"))
		for i := range b.d.Policies {
			p := &b.d.Policies[i]
			if p.PolicyNumber != pn {
				continue
			}
			if p.Cancelled {
				return nil, errf("already_done", "policy %s is already cancelled", pn)
			}
			if b.policyStatus(p) != "active" {
				return nil, errf("policy_inactive", "policy %s is %s", pn, b.policyStatus(p))
			}
			for _, c := range b.d.Claims {
				if c.PolicyNumber == pn && c.Status == "paid" {
					return nil, errf("not_eligible", "a claim was paid under policy %s, no refund", pn)
				}
			}
			premium := 0.0
			if p.Premium != nil {
				premium = *p.Premium
			}
			end, _ := time.Parse("2006-01-02", p.EndDate)
			months := 0
			for d := b.today.AddDate(0, 1, 0); !d.After(end.AddDate(0, 0, 1)); d = d.AddDate(0, 1, 0) {
				months++
			}
			refund := math.Round(premium * float64(months) / 12 * 0.9)
			p.Cancelled = true
			return map[string]any{"policy_number": pn, "refund_amount": refund, "unused_full_months": months, "refund_time": "10 working days"}, nil
		}
		return nil, errf("not_found", "policy %s not found", pn)

	case "create_claim":
		prod := strings.ToLower(str(args, "product_type"))
		if prod == "" {
			prod = "general"
		}
		cn := b.nextID("claim", "CL-")
		cl := Claim{ClaimNumber: cn, ClientID: str(args, "client_id"), PolicyNumber: strings.ToUpper(str(args, "policy_number")), ClaimType: prod, IncidentDate: str(args, "incident_date"), Status: "registered", NextStep: "Upload the documents in the app or send them to claims@saqta-insurance.example"}
		b.d.Claims = append(b.d.Claims, cl)
		docs := b.documentsFor(prod)
		return map[string]any{"claim_number": cn, "status": "registered", "documents_needed": docs}, nil

	case "get_claim":
		cn := strings.ToUpper(str(args, "claim_number"))
		cid := str(args, "client_id")
		if cn == "" && cid == "" {
			return nil, errf("invalid_input", "claim_number or client_id is required")
		}
		found := []map[string]any{}
		for i := range b.d.Claims {
			c := &b.d.Claims[i]
			if (cn != "" && c.ClaimNumber == cn) || (cn == "" && c.ClientID == cid) {
				found = append(found, b.claimView(c))
			}
		}
		if len(found) == 0 {
			return nil, errf("not_found", "claim %s%s not found", cn, cid)
		}
		if cn != "" || len(found) == 1 {
			return found[0], nil
		}
		return map[string]any{"claims": found}, nil

	case "create_dispute":
		cn := strings.ToUpper(str(args, "claim_number"))
		exists := false
		for _, c := range b.d.Claims {
			if c.ClaimNumber == cn {
				exists = true
			}
		}
		if !exists {
			return nil, errf("not_found", "claim %s not found", cn)
		}
		return map[string]any{"ticket_id": b.nextID("ticket", "D-"), "claim_number": cn, "review_time": "15 working days"}, nil

	case "book_inspection":
		city := strings.Title(strings.ToLower(str(args, "city")))
		date := str(args, "preferred_date")
		if date == "" {
			date = b.today.AddDate(0, 0, 1).Format("2006-01-02")
		}
		addr, hours := "City office parking, by appointment", "Mon-Fri 10:00-16:00"
		if pts, ok := b.cat.KBSection("inspection_points"); ok {
			for _, p := range pts.([]any) {
				m := p.(map[string]any)
				if strings.EqualFold(fmt.Sprint(m["city"]), city) {
					addr, hours = fmt.Sprint(m["address"]), fmt.Sprint(m["hours"])
				}
			}
		}
		return map[string]any{"slot_datetime": date + " 11:00", "address": addr, "hours": hours, "city": city}, nil

	case "book_appointment":
		spec := strings.ToLower(str(args, "doctor_specialty"))
		city := str(args, "city")
		date := str(args, "preferred_date")
		if date == "" {
			date = b.today.AddDate(0, 0, 1).Format("2006-01-02")
		}
		clinics, _ := b.cat.KBSection("clinics")
		var pick map[string]any
		for _, c := range clinics.([]any) {
			m := c.(map[string]any)
			if city != "" && !strings.EqualFold(fmt.Sprint(m["city"]), city) {
				continue
			}
			if spec != "" {
				ok := false
				for _, s := range m["specialties"].([]any) {
					if strings.Contains(strings.ToLower(fmt.Sprint(s)), specialtyKey(spec)) {
						ok = true
					}
				}
				if !ok {
					continue
				}
			}
			pick = m
			break
		}
		if pick == nil {
			return nil, errf("no_availability", "no partner clinic in %s for %s; nearest alternatives: therapist in any partner clinic", city, spec)
		}
		return map[string]any{"clinic_name": pick["name"], "address": pick["address"], "slot_datetime": date + " 09:30", "doctor_specialty": spec}, nil

	case "check_coverage":
		pn := strings.ToUpper(str(args, "policy_number"))
		service := strings.ToLower(str(args, "service_name"))
		pkg := "Comfort"
		for _, p := range b.d.Policies {
			if p.PolicyNumber == pn && p.Product == "dms" {
				pkg = fmt.Sprint(p.Details["package"])
			}
		}
		dms, _ := b.cat.KBSection("products.dms.packages." + pkg)
		covered, notCovered := []string{}, []string{}
		if m, ok := dms.(map[string]any); ok {
			for _, x := range m["covered"].([]any) {
				covered = append(covered, fmt.Sprint(x))
			}
			for _, x := range m["not_covered"].([]any) {
				notCovered = append(notCovered, fmt.Sprint(x))
			}
		}
		key := serviceKey(service)
		for _, n := range notCovered {
			if key != "" && strings.Contains(strings.ToLower(n), key) {
				return map[string]any{"covered": false, "package": pkg, "note": n}, nil
			}
		}
		for _, c := range covered {
			if key != "" && strings.Contains(strings.ToLower(c), key) {
				return map[string]any{"covered": true, "package": pkg, "note": c}, nil
			}
		}
		return map[string]any{"covered": nil, "package": pkg, "note": "not listed explicitly", "covered_list": covered, "not_covered_list": notCovered}, nil

	case "list_clinics":
		city := str(args, "city")
		clinics, _ := b.cat.KBSection("clinics")
		out := []any{}
		for _, c := range clinics.([]any) {
			m := c.(map[string]any)
			if city == "" || strings.EqualFold(fmt.Sprint(m["city"]), city) {
				out = append(out, m)
			}
		}
		if len(out) == 0 {
			return nil, errf("not_found", "no partner clinics in %s", city)
		}
		return map[string]any{"clinics": out}, nil

	case "resend_documents":
		pn := strings.ToUpper(str(args, "policy_number"))
		for _, p := range b.d.Policies {
			if p.PolicyNumber == pn {
				if b.policyStatus(&p) == "expired" {
					return nil, errf("policy_inactive", "policy %s is expired", pn)
				}
				email := ""
				if c := b.clientByID(p.ClientID); c != nil {
					email = c.Email
				}
				return map[string]any{"sent_to": email, "sent_to_masked": maskEmail(email), "policy_number": pn}, nil
			}
		}
		return nil, errf("not_found", "policy %s not found", pn)

	case "check_payment":
		cid := str(args, "client_id")
		date := str(args, "payment_date")
		for _, p := range b.d.Payments {
			if p.ClientID == cid && (date == "" || p.Date == date) {
				return map[string]any{"payment_id": p.PaymentID, "payment_status": p.Status, "amount": p.Amount, "date": p.Date, "product": p.Product, "note": p.Note}, nil
			}
		}
		return nil, errf("not_found", "no payment found for client %s on %s", cid, date)

	case "update_contact":
		cid := str(args, "client_id")
		field := str(args, "contact_field")
		val := str(args, "new_value")
		c := b.clientByID(cid)
		if c == nil {
			return nil, errf("not_found", "client %s not found", cid)
		}
		switch field {
		case "phone":
			c.Phone = normPhone(val)
		case "email":
			c.Email = val
		case "address":
			c.Address = val
		default:
			return nil, errf("invalid_input", "contact_field must be phone|email|address")
		}
		return map[string]any{"updated": field, "client_id": cid}, nil

	case "request_document":
		doc := str(args, "document_type")
		email := str(args, "email")
		avail, _ := b.cat.KBSection("documents_available")
		when := "email, within 1 working day"
		if m, ok := avail.(map[string]any); ok {
			if v, ok := m[doc]; ok {
				when = fmt.Sprint(v)
			}
		}
		return map[string]any{"sent_to": email, "document_type": doc, "delivery": when}, nil

	case "get_offices":
		city := str(args, "city")
		offices, _ := b.cat.KBSection("offices")
		for _, o := range offices.([]any) {
			m := o.(map[string]any)
			if strings.EqualFold(fmt.Sprint(m["city"]), city) {
				return map[string]any{"city": m["city"], "address": m["address"], "hours": m["hours"]}, nil
			}
		}
		all := []any{}
		for _, o := range offices.([]any) {
			all = append(all, o)
		}
		return map[string]any{"offices": all, "note": "no office in " + city}, nil

	case "kb_lookup":
		topic := str(args, "topic")
		if v, ok := b.cat.KBSection(topic); ok {
			return map[string]any{"topic": topic, "answer": v}, nil
		}
		// fuzzy: find the first top-level key containing the topic word
		for _, k := range b.cat.KBTopics() {
			if strings.Contains(strings.ToLower(topic), k) || strings.Contains(k, strings.ToLower(topic)) {
				v, _ := b.cat.KBSection(k)
				return map[string]any{"topic": k, "answer": v}, nil
			}
		}
		return nil, errf("not_found", "no knowledge base topic %q (available: %s)", topic, strings.Join(b.cat.KBTopics(), ", "))

	case "send_sms":
		phone := normPhone(str(args, "phone"))
		if phone == "" {
			return nil, errf("invalid_input", "phone is required")
		}
		return map[string]any{"sent_to": phone}, nil

	case "create_callback":
		return map[string]any{"scheduled": true, "phone": normPhone(str(args, "phone")), "callback_time": str(args, "callback_time")}, nil

	case "create_complaint":
		return map[string]any{"ticket_id": b.nextID("ticket", "T-"), "review": "Shift supervisor calls back the same day; written answer within 15 working days"}, nil

	case "report_fraud":
		return map[string]any{"ticket_id": b.nextID("fraud", "F-"), "forwarded_to": "security_team"}, nil

	case "transfer_to_operator":
		q := str(args, "queue")
		if q == "" {
			q = "operator_general"
		}
		return map[string]any{"queue": q, "transferred": true}, nil
	}
	return nil, errf("invalid_input", "action %s not implemented", name)
}

func specialtyKey(spec string) string {
	m := map[string]string{"терапевт": "therapist", "лор": "ent", "кардиолог": "cardiolog", "гинеколог": "gynecolog", "стоматолог": "dentist", "зубн": "dentist", "педиатр": "pediatric", "узи": "ultrasound", "анализ": "lab", "лаборатор": "lab",
		"therapist": "therapist", "ent": "ent", "cardiologist": "cardiolog", "gynecologist": "gynecolog", "dentist": "dentist", "pediatrician": "pediatric", "ultrasound": "ultrasound", "lab": "lab"}
	for k, v := range m {
		if strings.Contains(spec, k) {
			return v
		}
	}
	return spec
}

func serviceKey(service string) string {
	m := map[string]string{"мрт": "mri", "mri": "mri", "кт": "ct", "ct": "ct", "узи": "ultrasound", "ultrasound": "ultrasound", "анализ": "lab", "талдау": "lab", "lab": "lab", "стомат": "dental", "зуб": "dental", "тіс": "dental", "dent": "dental",
		"лекарств": "medication", "дәрі": "medication", "medic": "medication", "госпитал": "hospital", "hospital": "hospital", "терапевт": "therapist", "therapist": "therapist", "лор": "ent", "ent": "ent", "кардиолог": "cardiolog", "гинеколог": "gynecolog",
		"протез": "prosthet", "имплант": "implant", "косметолог": "cosmetolog", "cosmet": "cosmetolog", "скорая": "emergency", "emergency": "emergency", "specialist": "specialist", "специалист": "specialist"}
	for k, v := range m {
		if strings.Contains(service, k) {
			return v
		}
	}
	return ""
}

func (b *Backend) documentsFor(prod string) []string {
	key := map[string]string{"ogpo": "ogpo_victim", "ogpo_victim": "ogpo_victim", "casco": "casco", "property": "property", "accident": "accident", "travel": "travel"}[prod]
	if key == "" {
		return nil
	}
	v, ok := b.cat.KBSection("claims.documents." + key)
	if !ok {
		return nil
	}
	out := []string{}
	for _, x := range v.([]any) {
		out = append(out, fmt.Sprint(x))
	}
	return out
}

func (b *Backend) kbFloat(path string) (float64, bool) {
	v, ok := b.cat.KBSection(path)
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}

func (b *Backend) calcOGPO(args map[string]any) (map[string]any, *ActionError) {
	region := strings.ToLower(str(args, "region"))
	if region == "" {
		if plate := str(args, "vehicle_plate"); len(plate) >= 2 {
			code := plate[len(plate)-2:]
			if v, ok := b.cat.KBSection("products.ogpo.pricing.region_by_plate_code." + code); ok {
				region = fmt.Sprint(v)
			} else {
				region = "other"
			}
		}
	}
	if region != "almaty" && region != "astana" {
		if region == "" {
			return nil, errf("invalid_input", "region is required (almaty|astana|other)")
		}
		region = "other"
	}
	vt := strings.ToLower(str(args, "vehicle_type"))
	if vt == "" {
		vt = "car"
	}
	base, ok := b.kbFloat("products.ogpo.pricing.base_by_region_kzt." + region)
	if !ok {
		return nil, errf("invalid_input", "unknown region %s", region)
	}
	vcoef, ok := b.kbFloat("products.ogpo.pricing.vehicle_type_coef." + vt)
	if !ok {
		return nil, errf("invalid_input", "unknown vehicle_type %s", vt)
	}
	worst := 1.0
	classes := []string{}
	drivers := list(args, "drivers_iin")
	if len(drivers) == 0 {
		drivers = []string{""}
	}
	for _, iin := range drivers {
		class := b.d.Defaults.UnknownIINBMClass
		if c := b.findClient("", iin); c != nil {
			class = c.BMClass
		}
		classes = append(classes, class)
		if coef, ok := b.kbFloat("products.ogpo.pricing.bm_coef." + class); ok && coef > worst {
			worst = coef
		}
	}
	term := str(args, "term_months")
	if term == "" {
		term = "12"
	}
	tcoef, ok := b.kbFloat("products.ogpo.pricing.term_coef." + term)
	if !ok {
		tcoef = 1
	}
	price := math.Round(base * vcoef * worst * tcoef)
	return map[string]any{"price": price, "region": region, "vehicle_type": vt, "bm_classes": classes, "term_months": term, "currency": "KZT"}, nil
}

func (b *Backend) calcCASCO(args map[string]any) (map[string]any, *ActionError) {
	value, ok := num(args, "car_value")
	year, ok2 := num(args, "car_year")
	if !ok || !ok2 {
		return nil, errf("invalid_input", "car_value and car_year are required")
	}
	pkg := str(args, "package")
	if pkg == "" {
		pkg = "Standard"
	}
	age := float64(b.today.Year()) - year
	maxAge, _ := b.kbFloat("products.casco.pricing.max_car_age." + pkg)
	if maxAge > 0 && age > maxAge {
		return nil, errf("not_eligible", "car age %d exceeds the maximum %d for package %s", int(age), int(maxAge), pkg)
	}
	rate := 0.0
	switch {
	case age <= 3:
		rate, _ = b.kbFloat("products.casco.pricing.rate_by_car_age.0-3")
	case age <= 7:
		rate, _ = b.kbFloat("products.casco.pricing.rate_by_car_age.4-7")
	default:
		rate, _ = b.kbFloat("products.casco.pricing.rate_by_car_age.8-10")
	}
	fr := str(args, "franchise")
	if fr == "" {
		fr = "0"
	}
	fcoef, ok := b.kbFloat("products.casco.pricing.franchise_coef." + fr)
	if !ok {
		fcoef = 1
	}
	pcoef, _ := b.kbFloat("products.casco.pricing.package_coef." + pkg)
	price := math.Round(value * rate * fcoef * pcoef)
	return map[string]any{"price": price, "package": pkg, "franchise": fr, "car_age": age, "currency": "KZT"}, nil
}

var zoneByCountry = map[string]string{
	"россия": "A", "ресей": "A", "узбекистан": "A", "өзбекстан": "A", "кыргызстан": "A", "қырғызстан": "A", "грузия": "A", "грузи": "A", "беларусь": "A", "армения": "A", "азербайджан": "A", "таджикистан": "A", "молдова": "A", "georgia": "A", "russia": "A", "uzbekistan": "A", "kyrgyzstan": "A",
	"германия": "B", "франция": "B", "италия": "B", "испания": "B", "польша": "B", "чехия": "B", "австрия": "B", "нидерланды": "B", "греция": "B", "португалия": "B", "великобритания": "B", "англия": "B", "лондон": "B", "шенген": "B", "европа": "B", "европ": "B", "финляндия": "B", "швеция": "B", "норвегия": "B", "швейцария": "B", "венгрия": "B", "латвия": "B", "литва": "B", "эстония": "B", "germany": "B", "france": "B", "italy": "B", "spain": "B", "schengen": "B", "uk": "B", "europe": "B",
	"сша": "D", "америка": "D", "канада": "D", "usa": "D", "united states": "D", "canada": "D", "ақш": "D",
}

func (b *Backend) calcTravel(args map[string]any) (map[string]any, *ActionError) {
	country := strings.ToLower(str(args, "trip_country"))
	start, err1 := time.Parse("2006-01-02", str(args, "trip_start"))
	end, err2 := time.Parse("2006-01-02", str(args, "trip_end"))
	travelers, ok := num(args, "travelers_count")
	age, ok2 := num(args, "traveler_max_age")
	if err1 != nil || err2 != nil || !ok {
		return nil, errf("invalid_input", "trip_start, trip_end (YYYY-MM-DD) and travelers_count are required")
	}
	if !ok2 {
		age = 30
	}
	if age > 75 {
		return nil, errf("not_eligible", "travelers over 75 are insured only via an operator")
	}
	zone := "C"
	for k, z := range zoneByCountry {
		if strings.Contains(country, k) {
			zone = z
			break
		}
	}
	days := int(end.Sub(start).Hours()/24) + 1
	if days <= 0 {
		return nil, errf("invalid_input", "trip_end must be after trip_start")
	}
	rate, _ := b.kbFloat("products.travel.zones." + zone + ".rate_per_day_kzt")
	cov, _ := b.cat.KBSection("products.travel.zones." + zone + ".coverage")
	ageCoef := 1.0
	if age >= 65 {
		ageCoef = 2.0
	}
	price := math.Round(rate * float64(days) * travelers * ageCoef)
	return map[string]any{"price": price, "zone": zone, "coverage": cov, "days": days, "travelers": travelers, "currency": "KZT"}, nil
}

func (b *Backend) calcProperty(args map[string]any) (map[string]any, *ActionError) {
	pt := strings.ToLower(str(args, "property_type"))
	sum := str(args, "sum_insured")
	base, ok := b.kbFloat("products.property.price_per_year_kzt." + sum)
	if !ok {
		return nil, errf("invalid_input", "sum_insured must be one of 5000000|10000000|20000000")
	}
	if pt == "house" {
		coef, _ := b.kbFloat("products.property.house_coef")
		base *= coef
	}
	return map[string]any{"price": math.Round(base), "property_type": pt, "sum_insured": sum, "currency": "KZT"}, nil
}

func (b *Backend) calcAccident(args map[string]any) (map[string]any, *ActionError) {
	sum := str(args, "sum_insured")
	price, ok := b.kbFloat("products.accident.price_per_year_kzt." + sum)
	if !ok {
		return nil, errf("invalid_input", "sum_insured must be one of 1000000|3000000|5000000")
	}
	return map[string]any{"price": price, "sum_insured": sum, "currency": "KZT"}, nil
}

// Offices returns the office list sorted by city (for templates).
func (b *Backend) Offices() []map[string]any {
	v, _ := b.cat.KBSection("offices")
	out := []map[string]any{}
	for _, o := range v.([]any) {
		out = append(out, o.(map[string]any))
	}
	sort.Slice(out, func(i, j int) bool { return fmt.Sprint(out[i]["city"]) < fmt.Sprint(out[j]["city"]) })
	return out
}

// Preview computes what an irreversible action would do without doing it
// (refund amount, renewal price, echo of the details to read back).
func (b *Backend) Preview(name string, args map[string]any) (map[string]any, *ActionError) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch name {
	case "cancel_policy":
		pn := strings.ToUpper(str(args, "policy_number"))
		for i := range b.d.Policies {
			p := &b.d.Policies[i]
			if p.PolicyNumber != pn {
				continue
			}
			if b.policyStatus(p) != "active" {
				return nil, errf("policy_inactive", "policy %s is %s", pn, b.policyStatus(p))
			}
			premium := 0.0
			if p.Premium != nil {
				premium = *p.Premium
			}
			end, _ := time.Parse("2006-01-02", p.EndDate)
			months := 0
			for d := b.today.AddDate(0, 1, 0); !d.After(end.AddDate(0, 0, 1)); d = d.AddDate(0, 1, 0) {
				months++
			}
			return map[string]any{"policy_number": pn, "refund_amount": math.Round(premium * float64(months) / 12 * 0.9), "unused_full_months": months, "preview": true}, nil
		}
		return nil, errf("not_found", "policy %s not found", pn)
	case "renew_policy":
		pn := strings.ToUpper(str(args, "policy_number"))
		for _, p := range b.d.Policies {
			if p.PolicyNumber == pn {
				price := 0.0
				if p.Premium != nil {
					price = *p.Premium
				}
				return map[string]any{"policy_number": pn, "price": price, "new_end_date": mustDate(p.EndDate).AddDate(1, 0, 0).Format("2006-01-02"), "preview": true}, nil
			}
		}
		return nil, errf("not_found", "policy %s not found", pn)
	default:
		out := map[string]any{"preview": true}
		for k, v := range args {
			out[k] = v
		}
		return out, nil
	}
}

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Now()
	}
	return t
}
