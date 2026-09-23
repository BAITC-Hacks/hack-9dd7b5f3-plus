# PRD — Bagyt (Бағыт) · Voice Router

> Status: living draft — not final; update as decisions change. docs/CASE.md (official task) wins on conflict, then SPEC, then PRD. Source (RU): docs/research/raw/deep-research-output.ru.md

Translated from the `PRD.md` block of the deep-research output. Product name is a working title.

**Name:** Bagyt (Kazakh "маршрут/направление" — route/direction) — reads naturally in both RU and KZ, a direct metaphor for routing.

**One-liner:** A voice AI robot for a contact center that understands the gist from the very first utterance in both Russian and Kazakh (even with mid-phrase language switching), picks and switches scenarios itself at the LLM layer, and shows the supervisor why and how fast it made that call.

## Personas
- **Contact-center customer (insurance):** speaks in their own words; wants to resolve their issue without an operator; may switch topics and mix languages.
- **Supervisor:** wants to see which scenarios were selected, where the robot hesitated or got it wrong, per-stage latency, and error statistics.

## Priorities

### P0 (mandatory, due 16:00)
- Web microphone + text fallback input.
- Streaming STT (RU/KZ), response-language selection.
- Retrieval shortlist over `scenarios.json` + LLM JSON router (`scenario_id`, `confidence`, `reasoning`, `alternatives`).
- Voice response via TTS.
- Trace panel (transcript, scenario, reasoning, alternatives, per-stage latency).
- `/api/route` + `evaluate.py` integration; one-command `docker compose` startup; `MOCK_MODE`.

### P1 (value, 16:00–17:00)
- Topic stack: `topic_switch`/`return_to_topic`, multi-intent (the payment + address example).
- `action: proceed|clarify|handoff`; operator handoff with context.
- Parameter extraction (`extracted_params`).
- Fast-path (obvious requests) with Δ measurement.
- Supervisor panel with error statistics and an accuracy report.

### P2 (stretch, only if P0+P1 are stable)
- Speculative routing on partial transcripts; 500ms target; prompt caching.
- Escalation to a stronger LLM on low confidence.
- In-UI editing of the scenario catalog.
- Phone channel (Vapi/Twilio/LiveKit).

## Demo script (3 minutes) — as implemented
1. **Simple RU:** «Здравствуйте, что с моим заявлением по каско?» → `SC17` (claim status) with high confidence, spoken reply, trace panel. Then «телефон плюс семь семьсот один ноль ноль ноль ноль ноль ноль семь» → the client (C007) is identified from the kit and the reply uses their claim CL-500330.
2. **Topic switch (official example, insurance form):** «Здравствуйте, я вчера оплатил полис, деньги списались, а полис не оформился… а, и ещё адрес поменять надо» → `SC30` then `SC29`; the second topic goes to the stack and is offered after the first is handled.
3. **Kazakh:** «Сәлеметсіз бе, полисімнің мерзімін ұзартқым келеді» → `SC27`, reply in Kazakh.
4. **Boundary + uncertainty:** «Здравствуйте, у меня проблема с полисом» → `SYS_UNCLEAR`: the robot asks «полис не пришёл или деньги списались, а полис не оформился?» with both alternatives shown in the trace.
5. **Mixed speech:** «Кеше аварияға түстім, но я не виноват, виновник у вас застрахован» → `SC12`; language badge MIXED, reply in the dominant language.
6. **Safety:** «Көлігімді саттым, КАСКО шартын бұзғым келеді» + phone → preview with the refund amount; «Иә, растаймын» → executed only after the yes.
7. Supervisor: dev-set accuracy (Eval page), per-stage latency, fast-path agreement with the LLM, uncertain turns. Debug page: the model's tokens streaming live.

## Mapping to scoring criteria
| Criterion (task, 100) | Score | What we show |
|---|---|---|
| Task fit and functionality | 25 | Live demo of all 5 must-haves, 40 scenarios from the kit |
| Technical implementation | 25 | Retrieval+LLM-JSON, topic stack, streaming, per-stage measurements |
| README and reproducibility | 25 | `docker compose up`, `MOCK_MODE` with no keys, `evaluate.py` report |
| Value and applicability | 15 | Resolution without an operator, honest handoff to an operator, supervisor panel |
| Growth potential and originality | 10 | geko KZ-first, speculative routing, phone stretch, catalog editor |
| Demo Day: value/result/innovation/scale/pitch | 25/20/15/20/20 | Pitch + accuracy/latency metrics |

## Honest limitations
- 500ms to route — a target, not a guarantee; we show real measurements.
- geko cold start (**ASSUMPTION** on magnitude); we warm it up before the demo.
- Speko does not cover KZ/RU — not part of the core.
- KZ quality of the LLM is lower than RU; when in doubt — clarify or hand off to an operator.
