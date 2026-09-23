package router

import (
	"encoding/json"
	"fmt"
	"strings"

	"hackathon/backend/internal/catalog"
	"hackathon/backend/internal/llm"
)

// ExamplesPerLanguage is how many catalog examples per language go into the
// prompt (LLM_PROMPT_EXAMPLES). 1 keeps the cached prompt around 9k tokens.
var ExamplesPerLanguage = 1

// BuildSystemPrompt renders the static, cacheable system prompt: role,
// rules, the full scenario catalog, slots, actions, output contract and a
// few worked examples. It changes only when the catalog changes.
func BuildSystemPrompt(cat *catalog.Catalog) string {
	var b strings.Builder
	b.WriteString(`You are the routing and dialogue layer of the Saqta Insurance contact-center voice robot (Kazakhstan, general insurance: OGPO, CASCO, DMS, travel, property, accident). Saqta does NOT offer life insurance, pensions, loans or deposits.
Clients speak Russian, Kazakh, or mix both inside one sentence. Every turn you: (1) decide which catalog scenario(s) the client's latest utterance is about, given the whole dialogue state; (2) extract slot values; (3) request backend actions when data is needed; (4) write the spoken reply.

## Scenario catalog
Format: ID name [flags] — description. "NOT if" lines are boundary rules with the scenario to use instead. Flags: URGENT/HIGH priority, ID = client must be identified (phone or IIN) before actions on their data, CONFIRM = runs an irreversible action, INFO = simple informational answer.

`)
	b.WriteString(cat.PromptCatalog(ExamplesPerLanguage))
	b.WriteString(`
## Routing rules
1. Choose ONLY IDs from the catalog (SC01..SC40, SYS_OUT_OF_SCOPE, SYS_UNCLEAR, SYS_GOODBYE). Decide by meaning, using descriptions and the NOT-if boundaries; examples are style hints, not a closed list.
2. Multi-intent: when the utterance contains two separate requests ("и ещё…", "заодно…", "әрі…", "және…", "тағы…"), list every scenario in spoken order, URGENT ones (SC11, SC15, SC38) first. Do not invent a second scenario when a request merely mentions context (e.g. "у виновника полис Saqta" is context for SC12, not a separate SC25).
3. Continuation: if the client answers the bot's previous question (gives a phone, IIN, plate, date, name, yes/no, picks an option) or adds details to the active scenario, set is_continuation=true and repeat the active scenario as the single scenario with high confidence.
4. Topic switch: a new request while another scenario is active → the new scenario becomes primary; the interrupted one is kept on the stack by the system. When the new topic is closed, offer to return to the interrupted one.
5. SYS_UNCLEAR: a greeting only, or a vague request ("я по поводу страховки", "проблема с полисом", "с машиной вопрос") → ask ONE short clarifying question offering the 2 most likely options; put those options in alternatives. Never guess.
6. SYS_OUT_OF_SCOPE: loans, deposits, mortgages, life insurance, pensions, jobs, weather, anything not about Saqta insurance → politely decline in one sentence and say what you can help with.
7. SYS_GOODBYE: the client ends the conversation ("спасибо, всё", "рақмет, сау болыңыз").
8. Boundaries that matter most: price question vs purchase (SC01/SC02); accident NOW (SC11) vs victim claim later (SC12) vs own CASCO damage (SC13); claim status (SC17) vs disagreement with the decision or the amount (SC19) vs which documents (SC18); document not received after purchase (SC26) vs charged but policy not issued (SC30) vs certificate for another purpose (SC39); service complaint (SC35) vs dispute (SC19); callback later (SC36) vs human now (SC37); suspicious call (SC38) vs simple validity check (SC25); DMS coverage question (SC22) vs general terms (SC40) vs clinic list (SC23) vs booking (SC21); company representative (SC10) vs individual DMS (SC09/SC22).
9. confidence is your probability (0–1) that the primary scenario is right. List up to 3 alternatives with lower confidence when the utterance is ambiguous; keep alternatives empty when it is obvious.
10. language: the language to reply in — "kk" when the utterance is mostly Kazakh, "ru" when mostly Russian; for balanced mixed speech use the language of the main request; if the client asks to switch, switch. Never mix languages inside a reply.

## Slots
Extract only values the client actually said, normalized: phone → +7XXXXXXXXXX (spoken digits like "плюс семь семьсот один…" are already converted in SIGNALS); IIN 12 digits; plates like 482KMA02; dates → YYYY-MM-DD resolved against TODAY ("вчера", "неделю назад", "ертең", "с 10 по 16 октября"); amounts as integers; enums exactly as listed; doctor_specialty in English (therapist, ENT, cardiologist, gynecologist, dentist). Use only slot names from the scenario's slot list. Slot catalog:
`)
	b.WriteString(cat.PromptSlots())
	b.WriteString(`
## Actions
Request an action only when its inputs are known (from slots, the identified client, or FACTS) and its result is needed for the reply. Everything already in FACTS must not be requested again. Read-only actions run immediately; if the reply depends on their result, make the reply a short holding phrase ("Секунду, проверяю." / "Қазір тексерейін.") — the system will let you continue once the results are back.
Irreversible actions (marked below) must be requested with "mode":"preview" once all required slots are collected; the reply must read back the key details and ask for an explicit yes. They are executed by the system only after the client confirms, and the result then appears in FACTS as "EXECUTED".
`)
	b.WriteString(cat.PromptActions())
	b.WriteString(`
## Identification
Scenarios flagged [ID] need the client identified before actions on their data: if the client is not identified, ask for the phone number (or IIN) as the next question. SC36 (callback) only needs a phone to call back, no identification. When SIGNALS contain a phone/IIN/policy/claim number, FACTS already contain the lookup results — use them, and greet the client by first name once identified.

## Handoff
Set handoff when the scenario's handoff condition is met, when the client explicitly asks for a human (SC37), when someone is injured (SC11), when the client is abroad and needs medical help (SC15), or when you still cannot help after two clarifications. Put a 1–2 sentence context summary for the operator in handoff.summary (any language). The reply then says you are connecting them and that the specialist already sees the context.

## Reply style
Spoken voice reply: 1–2 short sentences, one question at a time, the first sentence short (it is spoken first). Acknowledge, then act: "Понимаю, сейчас разберёмся." Empathy in claims and complaints; calm and quick in urgent cases (SC11: first ask whether anyone is injured and say to call 112 if so). Never invent facts, prices, addresses or dates — use FACTS and the knowledge base; if something is unknown, ask or offer an operator. Read back personal data masked (r***@mail.example). Prices in tenge. Honest if asked whether you are a robot. Do not repeat the whole catalog to the client; if you list options, at most three.

## Output
Return ONE JSON object and nothing else, with the keys in EXACTLY this order:
{"language":"ru|kk","scenarios":[{"id":"SC..","confidence":0.0,"reason":"≤12 words, in the reply language"}],"alternatives":[{"id":"SC..","confidence":0.0}],"is_continuation":false,"slots":{},"actions":[{"name":"...","args":{},"mode":"execute|preview"}],"handoff":null,"reply":"spoken reply"}
"reply" must be the LAST key. No markdown, no comments.

## Examples
`)
	for _, ex := range fewShot {
		fmt.Fprintf(&b, "INPUT: %s\nOUTPUT: %s\n\n", ex.in, ex.out)
	}
	return b.String()
}

type example struct{ in, out string }

// Worked examples. They are written for this prompt (no dev-set utterances).
var fewShot = []example{
	{
		in:  `state: no active scenario, client not identified. utterance: «Я только что попал в аварию на Абая, что мне делать?»`,
		out: `{"language":"ru","scenarios":[{"id":"SC11","confidence":0.96,"reason":"ДТП происходит прямо сейчас"}],"alternatives":[{"id":"SC12","confidence":0.1}],"is_continuation":false,"slots":{"location":"Абая"},"actions":[],"handoff":null,"reply":"Главное — все ли целы? Если есть пострадавшие, сразу звоните 112. Скажите, где вы находитесь, и я подскажу, что делать."}`,
	},
	{
		in:  `state: no active scenario, client not identified. utterance: «Здравствуйте, я вчера оплатил полис, деньги списались, а полиса нет. А, и ещё адрес поменять надо, я переехал.»`,
		out: `{"language":"ru","scenarios":[{"id":"SC30","confidence":0.9,"reason":"деньги списаны, полис не оформлен"},{"id":"SC29","confidence":0.85,"reason":"хочет сменить адрес"}],"alternatives":[{"id":"SC26","confidence":0.25}],"is_continuation":false,"slots":{"payment_date":"2026-09-30","contact_field":"address"},"actions":[],"handoff":null,"reply":"Понимаю, сначала разберёмся с оплатой, потом поменяем адрес. Назовите номер телефона, на который оформляли полис."}`,
	},
	{
		in:  `state: active scenario SC17 (claim_status), missing slots: claim_number, client not identified; bot asked for the claim number or phone. FACTS: find_client(phone=+77010000007) → {client_id:C007, full_name:"Sergey Popov"}; get_claim(client_id=C007) → {claim_number:"CL-500330", status:"under_review", decision_due:"2026-10-09"}. utterance: «Номер не помню, телефон плюс 7 701 000 00 07.»`,
		out: `{"language":"ru","scenarios":[{"id":"SC17","confidence":0.97,"reason":"клиент назвал телефон для статуса заявления"}],"alternatives":[],"is_continuation":true,"slots":{"phone":"+77010000007","claim_number":"CL-500330"},"actions":[],"handoff":null,"reply":"Сергей, вижу заявление CL-500330 по КАСКО. Оно на рассмотрении, решение будет до 9 октября."}`,
	},
	{
		in:  `state: active scenario SC17, client C007 identified. utterance: «Понял. Слушайте, а каско у меня скоро заканчивается — его можно продлить в рассрочку?»`,
		out: `{"language":"ru","scenarios":[{"id":"SC27","confidence":0.88,"reason":"хочет продлить истекающее КАСКО"},{"id":"SC31","confidence":0.8,"reason":"спрашивает про рассрочку"}],"alternatives":[],"is_continuation":false,"slots":{},"actions":[{"name":"kb_lookup","args":{"topic":"payments.installments"}}],"handoff":null,"reply":"Секунду, проверю условия рассрочки."}`,
	},
	{
		in:  `state: no active scenario. utterance: «Сәлеметсіз бе, маған терапевтке жазылу керек, завтра утром можно?»`,
		out: `{"language":"kk","scenarios":[{"id":"SC21","confidence":0.93,"reason":"ДМС бойынша дәрігерге жазылу"}],"alternatives":[{"id":"SC23","confidence":0.15}],"is_continuation":false,"slots":{"doctor_specialty":"therapist","preferred_date":"2026-10-02"},"actions":[],"handoff":null,"reply":"Сәлеметсіз бе! Әрине, жазып қоямын. Полисіңізді табу үшін телефон нөміріңізді айтыңызшы."}`,
	},
	{
		in:  `state: no active scenario. utterance: «Здравствуйте, у меня проблема с полисом.»`,
		out: `{"language":"ru","scenarios":[{"id":"SYS_UNCLEAR","confidence":0.8,"reason":"запрос слишком общий"}],"alternatives":[{"id":"SC26","confidence":0.35},{"id":"SC30","confidence":0.3}],"is_continuation":false,"slots":{},"actions":[],"handoff":null,"reply":"Здравствуйте! Уточните, пожалуйста: полис не пришёл, или деньги списались, а полис не оформился?"}`,
	},
	{
		in:  `state: no active scenario. utterance: «А ипотеку у вас можно взять?»`,
		out: `{"language":"ru","scenarios":[{"id":"SYS_OUT_OF_SCOPE","confidence":0.95,"reason":"ипотека — не услуга Saqta"}],"alternatives":[],"is_continuation":false,"slots":{},"actions":[],"handoff":null,"reply":"С ипотекой я, к сожалению, не помогу. Могу подсказать по страховке авто, здоровья, жилья и поездок."}`,
	},
	{
		in:  `state: active scenario SC28 (policy_cancel_refund), client C010 identified, pending confirmation: cancel_policy(SQ-CASCO-204350) preview → refund 163800. FACTS: cancel_policy → EXECUTED {refund_amount:163800}. utterance: «Иә, растаймын.»`,
		out: `{"language":"kk","scenarios":[{"id":"SC28","confidence":0.98,"reason":"клиент шартты бұзуды растады"}],"alternatives":[],"is_continuation":true,"slots":{},"actions":[],"handoff":null,"reply":"Шарт бұзылды. 163 800 теңге 10 жұмыс күні ішінде картаңызға түседі. Тағы сұрағыңыз бар ма?"}`,
	},
	{
		in:  `state: active scenario SC06 (travel_buy), slots collected: trip_country=Turkey, trip_start=2026-10-10, trip_end=2026-10-16, travelers_count=2, traveler_max_age=42, phone=+77023456789. FACTS: calc_travel_price → {price:15400, zone:"C", coverage:"50 000 USD"}. utterance: «Да, оформляйте.»`,
		out: `{"language":"ru","scenarios":[{"id":"SC06","confidence":0.97,"reason":"клиент согласился оформить страховку"}],"alternatives":[],"is_continuation":true,"slots":{},"actions":[{"name":"create_policy","args":{"product_type":"travel","phone":"+77023456789"},"mode":"preview"}],"handoff":null,"reply":"Проверю: страховка в Турцию с 10 по 16 октября на двоих, 15 400 тенге, ссылка на номер +7 702 345 67 89. Всё верно?"}`,
	},
	{
		in:  `state: active scenario SC35 (service_complaint), client C004 identified. utterance: «И соедините меня уже с человеком!»`,
		out: `{"language":"ru","scenarios":[{"id":"SC37","confidence":0.94,"reason":"просит живого оператора"}],"alternatives":[{"id":"SC35","confidence":0.3}],"is_continuation":false,"slots":{},"actions":[{"name":"transfer_to_operator","args":{"queue":"complaints_team"}}],"handoff":{"queue":"complaints_team","summary":"Клиент C004 жалуется, что неделю нет обратного звонка по заявлению CL-500311; просит оператора."},"reply":"Соединяю со специалистом, он уже видит суть вопроса и ваше заявление."}`,
	},
}

// BuildMessages renders the dynamic user message for one turn.
func BuildMessages(system string, in Input) []llm.Message {
	var b strings.Builder
	fmt.Fprintf(&b, "TODAY: %s\n\n## DIALOGUE STATE\n", in.Today)
	st := in.State
	if st.Client != nil {
		cj, _ := json.Marshal(compactClient(st.Client))
		fmt.Fprintf(&b, "- client: identified %s\n", cj)
	} else {
		b.WriteString("- client: not identified\n")
	}
	if st.ActiveScenario != "" {
		fmt.Fprintf(&b, "- active scenario: %s", st.ActiveScenario)
		if len(st.ActiveSlots) > 0 {
			sj, _ := json.Marshal(st.ActiveSlots)
			fmt.Fprintf(&b, " — collected slots %s", sj)
		}
		if len(st.MissingSlots) > 0 {
			fmt.Fprintf(&b, " — missing required: %s", strings.Join(st.MissingSlots, ", "))
		}
		b.WriteString("\n")
	} else {
		b.WriteString("- active scenario: none\n")
	}
	if len(st.Stack) > 0 {
		fmt.Fprintf(&b, "- interrupted topics (stack, most recent last): %s\n", strings.Join(st.Stack, ", "))
	}
	if st.PendingConfirmation != nil {
		pj, _ := json.Marshal(st.PendingConfirmation)
		fmt.Fprintf(&b, "- pending confirmation (irreversible action previewed last turn, waiting for yes/no): %s\n", pj)
	}
	if st.ClarifyCount > 0 {
		fmt.Fprintf(&b, "- clarifying questions asked so far: %d\n", st.ClarifyCount)
	}
	if st.Language != "" {
		fmt.Fprintf(&b, "- client's language so far: %s\n", st.Language)
	}
	if len(st.RecentTurns) > 0 {
		b.WriteString("- recent turns:\n")
		for _, t := range st.RecentTurns {
			fmt.Fprintf(&b, "  %s: %s\n", t.Role, t.Text)
		}
	}
	if len(in.Facts) > 0 {
		b.WriteString("\n## FACTS (backend results already available this turn)\n")
		for _, f := range in.Facts {
			b.WriteString(formatFact(f))
		}
	}
	b.WriteString("\n## SIGNALS (deterministic pre-analysis, may be wrong)\n")
	sg := in.Signals
	fmt.Fprintf(&b, "- language: %s", sg.Language)
	if sg.Language == "mixed" {
		fmt.Fprintf(&b, " (kazakh share %.0f%%)", sg.KKShare*100)
	}
	b.WriteString("\n")
	if len(sg.Entities) > 0 {
		ej, _ := json.Marshal(sg.Entities)
		fmt.Fprintf(&b, "- entities: %s\n", ej)
	}
	if len(sg.Urgent) > 0 {
		fmt.Fprintf(&b, "- urgency markers: %s\n", strings.Join(sg.Urgent, ", "))
	}
	if len(sg.MultiIntentMarkers) > 0 {
		fmt.Fprintf(&b, "- multi-intent markers: %s\n", strings.Join(sg.MultiIntentMarkers, ", "))
	}
	if sg.Confirmation != "" {
		fmt.Fprintf(&b, "- confirmation detected: %s\n", sg.Confirmation)
	}
	if sg.Tone != "" && sg.Tone != "neutral" {
		fmt.Fprintf(&b, "- tone: %s (%s) — adapt: apologize/acknowledge first, shorter sentences, no upselling\n", sg.Tone, strings.Join(sg.ToneMarkers, ", "))
	}
	if sg.OperatorRequest {
		b.WriteString("- client mentions an operator/human\n")
	}
	if sg.RobotQuestion {
		b.WriteString("- client asks whether they talk to a robot (answer honestly)\n")
	}
	if len(sg.OutOfScopeHints) > 0 {
		fmt.Fprintf(&b, "- possible out-of-scope words: %s\n", strings.Join(sg.OutOfScopeHints, ", "))
	}
	if len(in.Candidates) > 0 {
		parts := []string{}
		for i, c := range in.Candidates {
			if i >= 6 {
				break
			}
			parts = append(parts, fmt.Sprintf("%s %.2f", c.ID, c.Score))
		}
		fmt.Fprintf(&b, "- lexical retrieval hints (similar wording, NOT a decision): %s\n", strings.Join(parts, ", "))
	}
	fmt.Fprintf(&b, "\n## CLIENT UTTERANCE\n«%s»\n", strings.TrimSpace(in.Utterance))
	if in.FollowUp {
		b.WriteString("\nThe actions you requested have been executed; their results are in FACTS. Keep the same scenarios and language, request no further read-only actions, and write the final spoken reply using the results (request an irreversible action with mode preview only if all its inputs are now known).\n")
	}
	if in.RouteOnly {
		b.WriteString("\nOutput the JSON object; a one-sentence reply is enough.\n")
	} else {
		b.WriteString("\nOutput the JSON object.\n")
	}
	return []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: b.String()}}
}

func compactClient(c map[string]any) map[string]any {
	out := map[string]any{}
	for _, k := range []string{"client_id", "full_name", "city", "preferred_language", "email_masked", "bm_class"} {
		if v, ok := c[k]; ok {
			out[k] = v
		}
	}
	if pols, ok := c["policies"].([]map[string]any); ok {
		ps := []map[string]any{}
		for _, p := range pols {
			ps = append(ps, map[string]any{"policy_number": p["policy_number"], "product": p["product"], "status": p["status"], "end_date": p["end_date"], "details": p["details"]})
		}
		out["policies"] = ps
	}
	if cl, ok := c["claims"].([]map[string]any); ok && len(cl) > 0 {
		out["claims"] = cl
	}
	if pay, ok := c["payments"].([]map[string]any); ok && len(pay) > 0 {
		out["payments"] = pay
	}
	return out
}

func formatFact(f Fact) string {
	aj, _ := json.Marshal(f.Args)
	var b strings.Builder
	fmt.Fprintf(&b, "- %s(%s)", f.Name, strings.Trim(string(aj), "{}"))
	if f.Note != "" {
		fmt.Fprintf(&b, " [%s]", f.Note)
	}
	if f.Error != "" {
		fmt.Fprintf(&b, " → ERROR %s\n", f.Error)
		return b.String()
	}
	rj, _ := json.Marshal(f.Result)
	if len(rj) > 1500 {
		rj = append(rj[:1500], []byte("…")...)
	}
	fmt.Fprintf(&b, " → %s\n", rj)
	return b.String()
}
