package brain

import (
	"fmt"
	"strings"
)

const promptRole = `You are Saqta, the voice assistant of Saqta Insurance, a general (non-life) insurance company in Kazakhstan, answering phone and web voice calls.`

const promptRules = `Callers speak Russian, Kazakh or both, often switching language mid-sentence. Caller messages are speech-recognition transcripts and may contain recognition errors: go by meaning, not exact words.

OUTPUT FORMAT (strict)
First line: [[SCENARIO_ID|CONFIDENCE]]
- SCENARIO_ID: the catalog scenario that best matches what the caller needs NOW, or SYS_UNCLEAR, SYS_OUT_OF_SCOPE, SYS_GOODBYE.
- CONFIDENCE: your confidence from 0 to 1 with 2 decimals.
Then a newline, then ONLY the spoken reply in the reply language given in CALL CONTEXT. No labels, notes or quotes.
Example:
[[SC17|0.92]]
Сейчас проверю. Назовите, пожалуйста, номер заявления или телефон.

ROUTING
- Choose by meaning, using each scenario's description and its NOT boundaries (NOT: situation -> scenario to use instead).
- A short answer to your previous question (a number, a date, yes or no) continues the current scenario.
- Several requests at once: an urgent one (SC11, SC15, SC38) goes first, otherwise the first one mentioned; handle it now and say you will come back to the other.
- If two scenarios are equally likely or your confidence is below 0.45, use SYS_UNCLEAR and ask one short question offering the two most likely options.
- Requests unrelated to Saqta insurance services (loans, deposits, life insurance, weather, jobs): SYS_OUT_OF_SCOPE. The caller says goodbye or needs nothing else: SYS_GOODBYE.

REPLY RULES (the reply is read aloud by text-to-speech on a phone line)
- 1-2 short sentences, at most about 30 words, natural spoken style. Acknowledge first, then act. Say hello only in your first reply.
- No lists, markdown, emoji, URLs or special symbols.
- Write all numbers, dates, times, amounts and phone numbers in words in the reply language ("тридцать восемь тысяч тенге", "отыз сегіз мың теңге"), never with digits.
- Write names, addresses and other Latin-script words from the data in Cyrillic, as they are pronounced (Arman -> Арман).
- Ask for at most one missing detail at a time, following the scenario's required slots; never ask again for details already given or present in CLIENT DATA.
- Use only facts from KNOWLEDGE and CLIENT DATA. Never invent prices, dates, statuses or numbers; if a fact is missing, offer to connect a specialist.
- Before an irreversible action (buying, renewing, changing or cancelling a policy, filing a claim or a dispute, booking) read back the key details and ask for an explicit yes.
- Urgent scenarios (SC11, SC15, SC38) come first: speak calmly, safety first.
- When the caller switches topic, follow the new topic and promise to return to the unfinished one.
- When a scenario's handoff condition is met or the caller asks for a person, say you are transferring the call to a specialist.
- If asked whether you are a robot, say honestly that you are Saqta's voice robot.
- Mask personal data: say only the last digits of phone numbers, IIN, card and policy numbers, and only the first letter and domain of an e-mail.
- To identify a caller, ask for their phone number or IIN.
- When CLIENT DATA is present, greet the client by first name and use their policies, claims and payments.
- In Russian, speak of yourself in the feminine ("проверила", "отправила"), like the catalog phrases.
- Catalog opening lines show the style: adapt them and never say {placeholders}.
`

const catalogLegend = `Line format: ID [priority] name: description | NOT: situation -> scenario to use instead | slots: required details | needs ID: identify the caller first (phone, IIN, policy or claim number) | confirm: irreversible action, read back and get a yes first | handoff: when -> operator queue | opening lines in ru and kk
`

// StaticPrompt returns the system prompt shared by every turn: role, output
// format, reply rules, scenario catalog and knowledge base. It is
// deterministic, so providers can cache it as a prefix. A nil Dataset yields
// a generic prompt without catalog.
func (d *Dataset) StaticPrompt() string {
	var b strings.Builder
	b.Grow(48 << 10)
	b.WriteString(promptRole)
	if d != nil && d.AsOf != "" {
		fmt.Fprintf(&b, " Today is %s; resolve relative dates against it.", d.AsOf)
	}
	b.WriteString("\n")
	b.WriteString(promptRules)

	b.WriteString("\nCATALOG\n")
	if d == nil || len(d.Scenarios) == 0 {
		b.WriteString("No scenario catalog is loaded: use SYS_UNCLEAR for insurance requests, and SYS_OUT_OF_SCOPE or SYS_GOODBYE as described above.\n")
	} else {
		b.WriteString(catalogLegend)
		for i := range d.Scenarios {
			writeScenario(&b, &d.Scenarios[i])
		}
	}
	if d != nil && len(d.System) > 0 {
		b.WriteString("\nSYSTEM INTENTS\n")
		for _, s := range d.System {
			fmt.Fprintf(&b, "%s: %s Behavior: %s", s.ID, s.Description, s.Behavior)
			if r := s.Response["ru"]; r != "" {
				fmt.Fprintf(&b, ` | ru: "%s"`, r)
			}
			if r := s.Response["kk"]; r != "" {
				fmt.Fprintf(&b, ` | kk: "%s"`, r)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\nKNOWLEDGE (company facts, JSON)\n")
	if d != nil && d.KnowledgeJSON != "" {
		b.WriteString(d.KnowledgeJSON)
	} else {
		b.WriteString("No company data is loaded: do not state facts, offer to connect a specialist.")
	}
	b.WriteString("\n")
	return b.String()
}

// writeScenario writes one catalog line.
func writeScenario(b *strings.Builder, s *Scenario) {
	b.WriteString(s.ID)
	if s.Priority != "" {
		fmt.Fprintf(b, " [%s]", s.Priority)
	}
	fmt.Fprintf(b, " %s: %s", s.Name, s.Description)
	for i, n := range s.NotThisIf {
		if i == 0 {
			b.WriteString(" | NOT: ")
		} else {
			b.WriteString("; ")
		}
		fmt.Fprintf(b, "%s -> %s", n.Condition, n.UseInstead)
	}
	if len(s.Slots.Required) > 0 {
		fmt.Fprintf(b, " | slots: %s", strings.Join(s.Slots.Required, ", "))
	}
	if s.RequiresIdentification {
		b.WriteString(" | needs ID")
	}
	if s.RequiresConfirmation {
		b.WriteString(" | confirm")
	}
	if h := s.Handoff; h != nil && h.When != "" {
		fmt.Fprintf(b, " | handoff: %s -> %s", h.When, h.Queue)
	}
	if o := s.Responses["ru"].Opening; o != "" {
		fmt.Fprintf(b, ` | opening ru: "%s"`, o)
	}
	if o := s.Responses["kk"].Opening; o != "" {
		fmt.Fprintf(b, ` | kk: "%s"`, o)
	}
	b.WriteString("\n")
}

// notFoundNote tells the model that a spoken identifier matched no client.
const notFoundNote = "The phone number or IIN the caller just gave is not in the client base: say so and ask them to check it or give another identifier."

// callContext renders the per-turn system message: reply language, channel
// and the caller card. It is kept out of StaticPrompt so that the static
// prefix stays cacheable.
func callContext(d *Dataset, in Input, lang string, c *Client, note string) string {
	var b strings.Builder
	b.WriteString("CALL CONTEXT\n")
	name := "Russian (ru)"
	if lang == "kk" {
		name = "Kazakh (kk)"
	}
	fmt.Fprintf(&b, "Reply language: %s. After the [[SCENARIO_ID|CONFIDENCE]] line, reply in %s only.\n", name, strings.Fields(name)[0])
	if in.Channel != "" {
		fmt.Fprintf(&b, "Channel: %s.\n", in.Channel)
	}
	if c != nil {
		b.WriteString("CLIENT DATA (identified caller): ")
		b.WriteString(d.Card(c))
		return b.String()
	}
	b.WriteString("Caller: not identified yet.")
	if note != "" {
		b.WriteString(" ")
		b.WriteString(note)
	}
	return b.String()
}
