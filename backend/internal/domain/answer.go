package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type Evidence struct {
	Source string `json:"source"`
	Path   string `json:"path"`
	Value  any    `json:"value"`
}

// Answer only reads organizer synthetic records. Mutation actions are never dispatched.
func (c *Catalog) Answer(d *Decision, history []Turn) (string, []string, []Evidence) {
	reply, warnings := c.Reply(d)
	evidence := []Evidence{}
	if d.Status != "route" {
		return reply, warnings, evidence
	}
	scenario, ok := c.Find(d.ScenarioID)
	if !ok {
		return reply, warnings, evidence
	}
	lang := "ru"
	if d.Language == "kk" {
		lang = "kk"
	}
	slots := map[string]string{}
	for _, t := range history {
		if t.Decision.ScenarioID == d.ScenarioID {
			for _, slot := range t.Decision.Slots {
				slots[slot.Name] = slot.Value
			}
		}
	}
	filtered := []Slot{}
	for _, slot := range d.Slots {
		def, exists := c.Slots[slot.Name]
		if len(c.Slots) > 0 && !exists {
			warnings = append(warnings, "Неизвестный параметр отброшен: "+slot.Name)
			continue
		}
		if def.Pattern != "" && def.Type != "list" {
			if matched, e := regexp.MatchString(def.Pattern, slot.Value); e != nil || !matched {
				warnings = append(warnings, "Неверный формат параметра: "+slot.Name)
				continue
			}
		}
		slots[slot.Name] = slot.Value
		filtered = append(filtered, slot)
	}
	d.Slots = filtered
	localized := func(ru, kk string) string {
		if lang == "kk" {
			return kk
		}
		return ru
	}
	var kb map[string]any
	_ = json.Unmarshal(c.Knowledge, &kb)
	var records map[string][]map[string]any
	// Decode only the read-only collections; meta/defaults aren't arrays.
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(c.Customers, &raw)
	records = map[string][]map[string]any{}
	for _, key := range []string{"policies", "claims", "clients", "payments"} {
		var rows []map[string]any
		_ = json.Unmarshal(raw[key], &rows)
		records[key] = rows
	}
	switch scenario.Slug {
	case "offices", "dms_clinics":
		city := normalizeCity(slots["city"])
		key := "offices"
		if scenario.Slug == "dms_clinics" {
			key = "clinics"
		}
		if city != "" {
			list, _ := kb[key].([]any)
			answers := []string{}
			for i, v := range list {
				entry, ok := v.(map[string]any)
				if !ok || !strings.EqualFold(firstString(entry, "city"), city) {
					continue
				}
				answer := firstString(entry, "address")
				if key == "clinics" {
					answer = firstString(entry, "name") + ": " + answer
				} else {
					answer += ". " + translateHours(firstString(entry, "hours"), lang)
				}
				answers = append(answers, answer)
				evidence = append(evidence, Evidence{"knowledge_base.json", fmt.Sprintf("%s[%d]", key, i), entry})
			}
			if len(answers) > 0 {
				return localized("По данным демо-компании: ", "Демо-компания деректері бойынша: ") + strings.Join(answers, "; "), warnings, evidence
			}
			return localized("В каталоге нет офиса или клиники в этом городе. Нужна помощь оператора.", "Бұл қаладағы кеңсе не емхана каталогта жоқ. Оператор көмегі қажет."), warnings, evidence
		}
	case "claim_status":
		if number := slots["claim_number"]; number != "" {
			for i, claim := range records["claims"] {
				if claim["claim_number"] == number {
					status := firstString(claim, "status")
					labels := map[string][2]string{"paid": {"выплачено", "төленді"}, "under_review": {"на рассмотрении", "қарастырылуда"}, "awaiting_documents": {"ожидаются документы", "құжаттар күтілуде"}, "approved": {"одобрено", "мақұлданды"}, "rejected": {"отказ", "бас тартылды"}}
					label := status
					if v, ok := labels[status]; ok {
						label = v[0]
						if lang == "kk" {
							label = v[1]
						}
					}
					evidence = append(evidence, Evidence{"mock_backend.json", fmt.Sprintf("claims[%d]", i), claim})
					return localized("Демонстрационное заявление ", "Демонстрациялық өтініш ") + number + ": " + label + ".", warnings, evidence
				}
			}
			return localized("Такого номера в тестовых данных нет. Проверьте номер или обратитесь к оператору.", "Тест деректерінде бұл нөмір жоқ. Нөмірді тексеріңіз немесе операторға жүгініңіз."), warnings, evidence
		}
	case "policy_validity":
		if number := slots["policy_number"]; number != "" {
			for i, policy := range records["policies"] {
				if policy["policy_number"] == number {
					start := firstString(policy, "start_date")
					end := firstString(policy, "end_date")
					valid := c.AsOf != "" && start <= c.AsOf && end >= c.AsOf
					status := localized("не действует на дату среза", "деректер күніне жарамсыз")
					if valid {
						status = localized("действует на дату среза", "деректер күніне жарамды")
					}
					evidence = append(evidence, Evidence{"mock_backend.json", fmt.Sprintf("policies[%d]", i), policy})
					return localized("Демо-полис ", "Демо-полис ") + number + ": " + status + " " + c.AsOf + ". " + localized("Срок: ", "Мерзімі: ") + start + " — " + end + ".", warnings, evidence
				}
			}
			return localized("Полис не найден в тестовых данных. Проверьте номер.", "Полис тест деректерінде табылмады. Нөмірді тексеріңіз."), warnings, evidence
		}
	case "payment_methods":
		if payments, ok := kb["payments"].(map[string]any); ok {
			methods := stringsArray(payments["methods"])
			for i, m := range methods {
				methods[i] = translatePayment(m, lang)
			}
			evidence = append(evidence, Evidence{"knowledge_base.json", "payments.methods", payments["methods"]})
			return localized("Доступные способы оплаты: ", "Қолжетімді төлем тәсілдері: ") + strings.Join(methods, "; ") + ".", warnings, evidence
		}
	}
	// Keep urgent opening safety guidance ahead of administrative slot collection.
	if scenario.Priority != "urgent" && !scenario.System {
		for _, name := range scenario.RequiredSlots {
			if slots[name] == "" {
				if def, ok := c.Slots[name]; ok && def.Prompt[lang] != "" {
					reply = def.Prompt[lang]
					if scenario.RequiresConfirmation {
						reply += " " + localized("Изменения в демо не выполняются.", "Демода өзгерістер орындалмайды.")
					}
					return reply, warnings, evidence
				}
			}
		}
	}
	if scenario.RequiresConfirmation && len(scenario.RequiredSlots) > 0 {
		all := true
		for _, name := range scenario.RequiredSlots {
			if slots[name] == "" {
				all = false
			}
		}
		if all {
			reply = localized("Параметры собраны. Для операции нужны проверка и отдельное подтверждение клиента; в этом стенде изменения не выполняются.", "Параметрлер жиналды. Операция үшін тексеру және клиенттің жеке растауы қажет; бұл стендте өзгерістер орындалмайды.")
		}
	}
	return reply, warnings, evidence
}
func normalizeCity(s string) string {
	aliases := map[string]string{"алматы": "Almaty", "алмата": "Almaty", "астана": "Astana", "шымкент": "Shymkent", "қарағанды": "Karaganda", "караганда": "Karaganda", "актобе": "Aktobe", "ақтөбе": "Aktobe", "атырау": "Atyrau", "павлодар": "Pavlodar", "өскемен": "Oskemen", "усть-каменогорск": "Oskemen"}
	if v, ok := aliases[strings.ToLower(strings.TrimSpace(s))]; ok {
		return v
	}
	return s
}
func translateHours(s, lang string) string {
	if lang == "kk" {
		return strings.NewReplacer("Mon-Fri", "дс–жм", "Mon-Sat", "дс–сб", "Sat", "сб").Replace(s)
	}
	return strings.NewReplacer("Mon-Fri", "пн–пт", "Mon-Sat", "пн–сб", "Sat", "сб").Replace(s)
}
func translatePayment(s, lang string) string {
	m := map[string][2]string{"Bank card in the app or on the website": {"карта в приложении или на сайте", "қосымшада немесе сайтта банк картасы"}, "Payment link by SMS": {"ссылка на оплату в SMS", "SMS арқылы төлем сілтемесі"}, "Bank transfer (companies)": {"банковский перевод для компаний", "компаниялар үшін банк аударымы"}, "Card terminal in any office": {"терминал в офисе", "кеңседегі терминал"}}
	if v, ok := m[s]; ok {
		if lang == "kk" {
			return v[1]
		}
		return v[0]
	}
	return s
}

// The interrupted topic stack is distinct from additional current-utterance intents
// (Decision.Pending), so the evaluator never sees stale historical topics as predictions.
func PendingTopics(active string, previous []string, d Decision) []string {
	result := []string{}
	seen := map[string]bool{}
	add := func(id string) {
		if id != "" && id != d.ScenarioID && !strings.HasPrefix(id, "SYS_") && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	for _, p := range previous {
		add(p)
	}
	if d.Status == "route" && active != d.ScenarioID && !strings.HasPrefix(d.ScenarioID, "SYS_") {
		add(active)
	}
	for _, p := range d.Pending {
		add(p)
	}
	if len(result) > 4 {
		result = result[len(result)-4:]
	}
	return result
}
