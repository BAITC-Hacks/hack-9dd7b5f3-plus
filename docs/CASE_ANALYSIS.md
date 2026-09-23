# Case analysis — Halyk Bank · Voice Router (Track 09 Communications)

> Status: living draft — our working interpretation of the case; update as decisions change. The official task text in docs/CASE.md wins on any conflict. Original (RU): docs/research/raw/case-analysis.ru.md

> Compiled 2026-09-23 ~13:50 from: the astanahub tracks page, the task spec (Google Doc, RU/KZ/EN), the `voice_router_dataset.zip` starter kit (Google Drive), and geko.sh.
> Full task text (RU) — see Appendix A pointer. Dataset README — see Appendix B pointer. Hard dev-set utterances — Appendix C. Test clients — Appendix D.

---

## 0. TL;DR — take it or not

**Take it.** The case fits us perfectly: it's pure LLM engineering (routing, prompts, JSON, latency), the data is ready-made and synthetic, and there's an official evaluation script — we can measure accuracy every 15 minutes and show the numbers in the README. Kazakh and mixed speech are our edge (Geko STT: KK+RU in a single model).

What actually decides the outcome:
1. **Routing accuracy on 10 hidden utterances** (the jury reads them live into a microphone). This is the core: 30 live points plus the whole "Fit to task" block.
2. **README + reproducibility = 25 of 100 points** in the spec — as much as the entire technical implementation. Don't leave it for 17:50.
3. **Voice in the browser is mandatory** (text is only a fallback). STT must handle Kazakh reliably in a noisy EXPO hall.
4. Latency is only a bonus (+2 if median ≤ 1.5 s). Don't trade accuracy for speed.

Main prohibitions: **an encoder-based intent classifier** (no kNN over embeddings for scenario selection), **hardcoding test utterances**, real conversation recordings, irreversible actions without the client's explicit "yes."

---

## 1. All links

| What | Link |
|---|---|
| Tracks page (Track 09 → Task 01 Voice Router → "Choose track" / "Submit solution") | https://edu.astanahub.com/hackathons/df4743f5-c492-415c-b45a-1f13adb78e06?tab=tracks |
| Task spec (Google Doc, RU / KZ / EN tabs) | https://docs.google.com/document/d/1e-F3ahQwPSdRMugpLO1vQ0_gFf5q9hIATUt0GxC3bEM/edit?tab=t.0 |
| Task spec — markdown export (public) | https://docs.google.com/document/d/1e-F3ahQwPSdRMugpLO1vQ0_gFf5q9hIATUt0GxC3bEM/export?format=md |
| Starter kit `voice_router_dataset.zip` (Drive) | https://drive.google.com/file/d/1sHE56gXnzdscHz5lMcNbwd1VsJIVLFUv/view |
| Our repo | https://github.com/BAITC-Hacks/hack-9dd7b5f3-plus |
| Geko (KK/RU STT + KK TTS) | https://geko.sh · console/key https://app.geko.sh · playground https://app.geko.sh/playground |
| Geko docs | https://docs.geko.sh · quickstart https://docs.geko.sh/quickstart · API https://docs.geko.sh/api/reference · OpenAPI https://docs.geko.sh/openapi.json · voice-agents guide https://docs.geko.sh/guides/voice-agents · llms.txt https://geko.sh/llms.txt · models https://geko.sh/models · pricing https://geko.sh/pricing · GitHub https://github.com/gekoai |
| ElevenLabs Kazakh TTS | https://elevenlabs.io/text-to-speech/kazakh |
| NVIDIA Build API keys / NIM endpoint | https://build.nvidia.com/settings/api-keys · base URL `https://integrate.api.nvidia.com/v1` |
| OpenAI Platform ($50 credits) | https://platform.openai.com (Billing → Promotions) |
| Day-of instructions (promo codes, activation) | https://t.me/hackalem/1733 |
| Organizer Q&A chat | https://t.me/+pKzbwN43ot1hYTRi |

The case page also links "Rules · Instructions · OpenAI and NVIDIA · Equipment · Security · ASU course" — general materials already distilled in `docs/HACKATHON_RULES.md`.

---

## 2. The case in 6 lines

- **Client:** Halyk Bank of Kazakhstan JSC. Task "AI for corporate products," Case 2 of 2 (Case 1 — Career Quest, not ours).
- **Problem:** voice bots pick a scenario with an encoder-based intent classifier → they break on topic changes, requests straddling two scenarios, and RU↔KZ code-switching mid-sentence → transfer to an operator / lost customer.
- **Build:** a voice AI robot with a **web simulation interface**: an LLM layer that selects the scenario from dialog context → a voice reply to the customer + a **trace panel** for the supervisor (scenario, why, alternatives, per-stage timing).
- **Data domain:** the fictional insurer **Saqta Insurance** (OGPO, CASCO, VHI, travel, property, accident). 40 scenarios + 3 system intents (`SYS_OUT_OF_SCOPE`, `SYS_UNCLEAR`, `SYS_GOODBYE`). "Today" = **2026-10-01**.
- **Input:** microphone (text is a fallback), RU/KZ/mixed speech, dialogs up to 10 turns.
- **Output:** voice + trace. Targets: scenario selection **500 ms**, end-of-utterance to start-of-reply **1.5 s** (not a pass/fail condition, gives a bonus).

---

## 3. How it's judged (three layers)

### 3.1 Spec criteria (expert technical review, 24–28.09)
| Criterion | Points | What it means for us |
|---|---:|---|
| Fit to task and functionality | 25 | Voice works, LLM-based routing, tracing, RU+KZ. It runs. |
| Technical implementation | 25 | Architecture (triage → router → policy → executor → response), agentic (tool calls against the mock backend), implementation matches the stated logic |
| **README and reproducibility** | **25** | One-command startup, mock mode, accuracy numbers from `evaluate.py`, how to verify |
| Value and applicability | 15 | Fewer operator transfers, explainability for the supervisor |
| Potential and originality | 10 | Hybrid fast path, catalog editable without a developer, portability to bank/telecom |

### 3.2 Jury live test (from the dataset README)
- The jury **reads 10 hidden utterances live** (same for every team): simple ones, topic changes, borderline cases, mixed speech, Kazakh (≥2 in KZ, ≥1 mixed), and requests that **must not** be routed to a business scenario (out-of-scope / unclear).
- **Up to 3 points per utterance:** (1) correct primary scenario; (2) all scenarios for multi-intent, otherwise the correct response language; (3) quality of the data-grounded reply.
- **Latency bonus:** median `latency_ms.total` (end of speech → start of playback): ≤ 1.5 s → +2; ≤ 3 s → +1; > 3 s → 0. Spot-checked with a stopwatch.
- **Personas use clients from `mock_backend.json`** and give a phone number — the robot must identify them and use their data.

### 3.3 Demo Day (29.09, if in the final)
Value 25 · Result/quality 20 · Innovation 15 · Scalability 20 · Presentation 20.

---

## 4. Must-have checklist (from the spec)

- [ ] **Voice in the browser:** the jury speaks into a microphone → the robot recognizes and replies by voice. Text is only a supplement.
- [ ] **Scenario selection at the LLM layer** — show how the layer works. An encoder-based intent classifier does not count.
- [ ] **Correctness:** 10 hidden utterances (simple, topic change, mixed speech, scenario boundaries) — hit rate.
- [ ] **Trace panel after every turn:** scenario, rationale, alternatives, per-stage timing.
- [ ] **RU + KZ**, including code-switching within a sentence.
- [ ] Handoff to an operator where the robot can't cope (with a context summary).
- [ ] Irreversible actions only after an explicit "yes."
- [ ] Low model confidence is visible (confidence, `SYS_UNCLEAR`).
- [ ] One-command startup (`docker compose up`).
- [ ] Works on live data, not a recorded demo.

**Optional (extra points):** a fast-path/LLM hybrid with a measured win; routing ≤ 500 ms; context retention and returning to an interrupted topic (a stack); asking a follow-up instead of guessing; slot extraction from speech; streaming; emotion → tone; a supervisor panel with error stats; catalog editing without a developer.

**Not allowed:** a ready-made intent classifier; hardcoded "utterance → scenario" mappings; real conversation recordings; real PII sent to external APIs.

---

## 5. Starter kit — what's inside

| File | Size | Contents |
|---|---:|---|
| `scenarios.json` | 90 KB | 40 scenarios: `description`, **`not_this_if` (disambiguation rules → use_instead)**, priority, `fast_path_eligible`, `requires_identification`, required/optional slots, actions, `requires_confirmation`, handoff, 4 ru + 3 kk examples, opening/closing replies in ru/kk. Plus 3 system intents |
| `slots.json` | 14 KB | 43 slots: type, pattern/values, question in ru/kk |
| `actions.json` | 11 KB | 31 mock actions (inputs/outputs/errors/irreversible), 6 operator queues, a single error format + error-handling rules |
| `knowledge_base.json` | 14 KB | company, offices, inspection points, products + **pricing formulas**, clinics, claims settlement, documents, payment, cancellation, bonus-malus, app, anti-fraud, complaints |
| `mock_backend.json` | 10 KB | 11 clients, 11 policies, 4 insurance claims, 2 payments |
| `dialogs_sample.json` | 30 KB | 10 annotated dialogs (scenario switch, topic return, mixed language, language switch, clarification, handoff, confirmation) |
| `dev_utterances.json` | 24 KB | 104 utterances: 84 single, 13 multi-intent, 4 out-of-scope, 3 unclear; languages 52 ru / 45 kk / 7 mixed |
| `evaluate.py` | 2 KB | `python evaluate.py predictions.json dev_utterances.json` → primary_accuracy, full_match, intent_recall by language/type |

Key flags:
- **urgent (always first in multi-intent):** SC11 accident happening now, SC15 medical emergency abroad, SC38 fraud.
- **fast_path_eligible:** SC18, SC23, SC24, SC26, SC31, SC33, SC34, SC36, SC37.
- **requires_identification:** SC04, 05, 13–17, 19–22, 24–30, 39.
- **requires_confirmation (irreversible):** SC02, 04, 05, 06, 12, 13, 14, 16, 19, 20, 21, 27, 28, 29.
- Irreversible actions: `create_policy, renew_policy, update_policy, cancel_policy, create_claim, create_dispute, book_inspection, book_appointment, update_contact`.
- Operator queues: `operator_general, claims_team, medical_assistance_24_7, corporate_sales, complaints_team, security_team`.
- Compact catalog (id + priority + name + description + not_this_if + 1 ru + 1 kk example) = **~12,000 characters ≈ 4k tokens** → fits entirely in the system prompt and can be cached. No retrieval needed.

Common traps (straight from the README): "accident" (авария) could be SC11 / SC12 / SC13; "approved but too little" is SC19, not SC17; "paid, money was charged, no policy" is SC30, not SC26.

---

## 6. Tooling: what's available and what to pick

| Layer | Option | Pros | Cons / to check |
|---|---|---|---|
| **STT** | **Geko Seta `seta-kk-ru-v2`** | KK+RU in one model, language switch mid-word; claimed 8.71% WER on Kazakh (Whisper 44.95%); 40× realtime; OpenAI-compatible `/v1/audio/transcriptions`, also `/v1/transcribe` | **Cold start up to tens of seconds** (GPU scale-to-zero) → warm up with a ping on page load and every couple of minutes. Verify the STT base URL in the docs (the site example `https://geko--tokay-serve-web.modal.run/v1` is for TTS) |
| STT | OpenAI `gpt-4o-transcribe` / `gpt-4o-mini-transcribe` | We already have credits, stable | Kazakh is weaker; test on our 3 phrases |
| STT | ElevenLabs Scribe | Fast, multilingual | Paid; check KK quality |
| STT | Browser Web Speech API (Chrome `kk-KZ` / `ru-RU`) | Free, no key, streaming | One language per session → poor with mixed speech. Works as a **keyless fallback** |
| **TTS** | **Geko Tokay `tokay-kk-v1`** | 6 Kazakh voices, reads numbers as words, sentence-level streaming, WAV 24 kHz; OpenAI-compatible `/v1/audio/speech` | Cold start; whether it can do **Russian** — check in the playground |
| TTS | ElevenLabs (has a Kazakh page) | Quality, excellent Russian | Which models support KK (Flash vs Multilingual/v3) — check; paid |
| TTS | OpenAI `gpt-4o-mini-tts` | Good Russian, we have credits | Kazakh comes out accented |
| TTS | Browser `speechSynthesis` | Keyless fallback | Quality |
| **Router LLM** | OpenAI (`gpt-4.1-mini` / `-nano` or whatever's available) | Structured outputs, prompt caching, logprobs | Measure TTFT on-site |
| LLM | NVIDIA NIM (`integrate.api.nvidia.com/v1`) | $50 in credits, OpenAI-compatible | Free-tier latency is usually worse — check |
| LLM | Groq / Cerebras (openai_compatible) | Very fast TTFT → fast tier | Separate key, rate limits |

**Default decision:** STT = Geko Seta; TTS = Geko Tokay for `kk`, OpenAI/ElevenLabs for `ru`; LLM = OpenAI (router + response), provider swappable via `LLM_PROVIDER`. Everything sits behind `STT`, `TTS`, `LLM` interfaces → switchable by env variable.

---

## 7. Quick solution sketch (ahead of finishing research)

### 7.1 Name
**Saqta Voice Router** / working name "**Plus Router**." One "Call" screen and one "Supervisor" screen.

### 7.2 Architecture
```
Browser (Next.js)
  [Push-to-talk 🎙] ─ MediaRecorder (webm/opus) ──► POST /api/turn (audio | text, session_id)
  Trace panel ◄── JSON trace ──┐                     │
  <audio> ◄── TTS stream ──────┤                     ▼
                               │   Go backend
                               │   1. STT (Geko Seta)                       t_stt
                               │   2. Router LLM — ONE call = triage+routing  t_router
                               │        input: compact catalog (cached) + dialog state + utterance
                               │        output: {language, urgent, parts[], scenarios[], alternatives[], slots, is_continuation}
                               │   3. Decision policy (deterministic Go)
                               │        ≥0.75 run · 0.45–0.75 SYS_UNCLEAR (top-2) · <0.45×2 → handoff
                               │        urgent first · scenario stack · continuation → slots only
                               │   4. Scenario executor (Go FSM, driven by scenarios.json fields)
                               │        identify → ask missing slot (one at a time) → actions (mock) →
                               │        preview + "yes?" for irreversible → execute → close/pop stack
                               │   5. Response LLM (streaming, 1–2 sentences, grounded in data only) t_resp
                               │   6. TTS (Geko Tokay kk / OpenAI ru) → first chunk               t_tts_first
                               └── trace → Postgres (for the supervisor panel)
```
Dialog state: `language, client_id, active_scenario, stack[], slots{}, awaiting_slot, pending_confirmation, low_conf_streak, turns[-3:]`.

### 7.3 Why this isn't an "intent classifier" (say this in the README and at defense)
- The scenario is chosen by a **generative LLM** that reads scenario descriptions and the **`not_this_if` disambiguation rules** plus dialog state. No training on utterances, no embeddings/kNN for scenario selection.
- The catalog is data: add a scenario to `scenarios.json` → the router knows it with no retraining (this is the "catalog editing without a developer" bullet).
- `dev_utterances.json` is used **only for evaluation**, never as few-shot examples (keeps us honest and shields against a hardcoding accusation).

### 7.4 Router — system prompt draft
```text
You are the scenario router of a voice contact center for Saqta Insurance (Kazakhstan). Today is 2026-10-01.
Clients speak Russian, Kazakh, or both mixed inside one sentence. Transcripts come from speech recognition and may contain errors.

Task: for the CURRENT client utterance, given the dialog state, choose which scenario(s) from the catalog the client needs now.
Rules:
- Read each scenario's description and NOT-rules carefully; NOT-rules override surface keywords.
- If the dialog state has an active scenario awaiting a slot or a yes/no confirmation and the utterance answers it, set is_continuation=true and return the active scenario.
- One utterance may contain several requests: return ALL of them, urgent ones first (SC11, SC15, SC38), then in spoken order.
- Not about Saqta services (loans, life insurance, weather, jobs) -> SYS_OUT_OF_SCOPE. Too vague to choose -> SYS_UNCLEAR. Client ends the call -> SYS_GOODBYE.
- confidence: your honest probability in [0,1]. Put the 1–2 closest competing scenarios in alternatives.
- language: dominant language of the utterance: "ru", "kk" or "mixed" (answer language = dominant one).
- slots: extract only values explicitly stated; normalize phones to +77XXXXXXXXX, dates to YYYY-MM-DD relative to today, numbers to digits.
- reason: max 12 words, English.
Return JSON only.

CATALOG:
{{compact catalog generated from scenarios.json at boot: ID [priority] name: description | NOT: condition -> use_instead | ex: ru / kk}}
SYSTEM INTENTS: SYS_OUT_OF_SCOPE, SYS_UNCLEAR, SYS_GOODBYE
```
User message: `STATE: {...}\nUTTERANCE: "..."`

Output JSON schema (strict, structured outputs; matches the dataset README's contract):
```json
{
  "language": "ru|kk|mixed",
  "urgent": false,
  "is_continuation": false,
  "scenarios": [{"scenario_id": "SC30", "confidence": 0.86, "reason": "money charged, policy not issued"}],
  "alternatives": [{"scenario_id": "SC26", "confidence": 0.31}],
  "slots": {"payment_date": "2026-09-30"}
}
```
Settings: temperature 0, max_tokens ~200, static prefix (rules + catalog) first → prompt caching; 5 s timeout, 1 retry, fallback → `SYS_UNCLEAR`.

Upgrade if time allows: derive confidence not from the model's text but from the **logprobs** of the scenario-ID token (top_logprobs) → honest calibration + free alternatives.

### 7.5 Hybrid / fast path (optional, but earns points and a good pitch story)
1. **Continuation fast path (no LLM):** if `awaiting_slot` = phone/IIN/policy/date and the utterance parses via regex/normalizer → fill the slot without calling the router. This is slot filling, not scenario selection — allowed.
2. **LLM cascade:** fast tier (nano / Groq) → if 1 scenario, confidence ≥ 0.9, and no continuation conflict → go with it; otherwise fall back to the big tier. Show `tier: fast|full` in the trace and the time saved in ms.
3. **Pre-synthesized opening lines:** at server startup, synthesize TTS for `responses.opening` of all 40 scenarios × ru/kk (no placeholders) → as soon as the router picks a scenario, **play the cached opening immediately** while the LLM generates the continuation. Time to first sound ≈ STT + router only.
4. For `fast_path_eligible` scenarios (offices, clinics, payment methods…) — answer from a KB template, no second LLM call.

### 7.6 Latency budget (target — median ≤ 1.5 s)
| Stage | Target |
|---|---:|
| STT (3–4 s utterance, Geko, warm) | ~250–400 ms |
| Router (cached prompt, short JSON) | ~300–600 ms |
| Cached opening / first response sentence | 0 / ~300–500 ms |
| TTS first chunk | ~200–400 ms |
| **Total** | **~0.8–1.5 s** |
Measurement: client marks `t0` on button release, `t_audio` on `audio.onplaying`; the server reports per-stage timings. We show both.

**Push-to-talk, not VAD** — in a noisy hall, VAD will false-trigger; releasing the button gives a clean "end of utterance" mark for measurement.

### 7.7 Response (LLM #2)
Input: scenario, language, slots, action results, the relevant KB excerpt, `responses.opening/closing` as a style reference, the next missing slot (`slots.json` prompt ru/kk). Rules from the README: 1–2 sentences, one question at a time, empathy first, **numbers spelled out** ("thirty-eight thousand tenge"), mask PII (`r***@mail.example`), be honest about being a robot, read back details and ask "yes?" before any irreversible action. Stream → split by sentence → TTS.

### 7.8 Mock / keyless mode (rule 5.6.6 — testing without our keys)
- A deployed URL on Railway with server-side keys (experts just open the link) — the primary path.
- `LLM_PROVIDER=mock`, `STT_PROVIDER=browser`, `TTS_PROVIDER=browser`: browser STT/TTS plus a deterministic mock router, **clearly labeled as mock** in the UI, replaying the 10 dialogs from `dialogs_sample.json`. The README states honestly that real routing requires a key for some OpenAI-compatible provider.

### 7.9 Eval harness (mandatory — our main argument)
- `go run ./cmd/eval` → runs the router over the 104 utterances in `dev_utterances.json` → `predictions.json` → `python data/voice_router_dataset/evaluate.py predictions.json` → a table in the README (primary_accuracy, full_match, intent_recall, by ru/kk/mixed) + router latency p50/p95.
- Run after every prompt change. Target: primary ≥ 0.9, mixed/kk no worse than ru.

### 7.10 Screens
- **/call**: a large 🎙 push-to-talk button, a text field (fallback), no client-persona selector needed (identification is by voice via phone number), a turn feed, a language badge, **Trace panel**: scenario + confidence bar, reason, alternatives, slots, actions (preview/execute), scenario stack, a **latency waterfall** for STT / router / response / TTS / total, fast/full tier.
- **/supervisor**: every turn from Postgres: scenario distribution, low confidence, SYS_UNCLEAR, handoffs with summaries, median latency. A "run the dev set" button (shows accuracy live — a good wow moment).

### 7.11 Demo wow moment
The jury says: "Sәlemetsiz be, keshe avariyaga tüstim, no ya ne vinovat… a, i eshchyo polis na pochtu ne prishёl" ("Hello, I had an accident yesterday, but I'm not at fault… oh, and the policy never arrived by mail" — RU/KZ mixed). The robot replies in Kazakh in ~1 s; the trace shows: `SC12 0.88 (victim, culprit insured) → SC26 0.8`, alternative `SC11 0.2 — accident not now`, stack `[SC26]`, the latency waterfall. Then "eight seven-oh-one, zero…" ("восемь семьсот один, ноль…") → identified as Arman Tulegenov → data pulled from his policy.

---

## 8. Split across the three of us (now ~13:50, freeze 17:15, submit by 18:00)

| Checkpoint | Tair (AI, prompts, providers, README) | Alikhan (Go API, data, FSM) | Ramazan (Next.js) |
|---|---|---|---|
| **14:00** | `docs/CASE.md` (this file) + dataset in `data/`, update the track in AGENTS.md | `backend/` skeleton + `/healthz` + load the starter-kit JSON | `frontend/` `/call` skeleton |
| **15:00** | router (catalog from JSON, prompt, structured output) + `cmd/eval` + first numbers | `POST /api/turn` (text), dialog state, decision policy, mock actions: find_client, get_policies/policy, get_claim, check_payment, kb_lookup, get_offices, list_clinics | UI: text input, feed, Trace panel (on mock JSON) |
| **16:00** | STT (Geko) + TTS (Geko kk / OpenAI ru) + warmup; response LLM | executor FSM: identify → slots → preview/confirm → execute; stack; handoff with summary; Postgres trace | push-to-talk + send audio + playback, latency waterfall; deploy to Railway |
| **17:00** | fast path + cached opening audio; prompt iteration on eval errors | remaining actions (calc_*, create_* via preview), error handling per `error_handling` | `/supervisor` stats, language badge, polishing |
| **17:15–18:00** | README (sections + eval table + limits + deploy link), THIRD_PARTY.md, **submit the solution** | verify `docker compose up` from a clean clone | screenshots, live run of 10 dialogs |

Everyone commits from their own account at least once every 30 minutes. No AI co-author trailers.

---

## 9. Check in the first 15 minutes (risks)

1. **Geko key + credits**: balance remaining, base URL for STT and TTS, whether Tokay speaks Russian, real latency and cold start from the venue (network is wired EXPO only).
2. **STT on three spoken phrases** in the hall: clean RU, clean KZ, mixed (U083/U093 from Appendix C). Geko vs. OpenAI transcribe.
3. **Router TTFT** on OpenAI vs. NIM (same utterance ×10, median).
4. Ask the organizers (Q&A chat) if unclear: will the jury test on our laptop or via the link → prepare for both just in case; the browser microphone requires **HTTPS** (Railway is fine, localhost is fine).
5. Keys only in `.env` / Railway. Never show them on screen during the demo.

---

## Appendix A — official task text

See `docs/CASE.md` for the official task text (RU verbatim + EN translation).

## Appendix B — dataset README

See `data/README.md` (EN) / `data/README.ru.md` / `data/README.kz.md`.

## Appendix C — hard dev-set utterances (kk / mixed / multi / out-of-scope / unclear)

Use ONLY for evaluation and manual voice test runs, never as few-shot examples.

| ID | lang | type | expected | text | gloss (EN) |
|---|---|---|---|---|---|
| U002 | kk | single | SC01 | Көлікке міндетті сақтандыру бағасы қандай болады, білгім келеді | I want to know how much mandatory car insurance costs |
| U004 | mixed | single | SC02 | Сәлеметсіз бе, ОГПО оформить етейін деп едім | Hello, I wanted to get OGPO issued |
| U006 | kk | single | SC03 | Жаңа көлікке КАСКО есептеп берсеңіз | Please calculate CASCO for my new car |
| U010 | kk | single | SC05 | Көлікті жаңасына ауыстырдым, полис ескісінде қалды | I replaced my car, the policy is still on the old one |
| U014 | kk | single | SC07 | Жеке үйімді сақтандыру шарттарын білгім келеді | I want to know the terms for insuring my house |
| U018 | kk | single | SC09 | Өзіме жеке ДМС сатып алғым келеді, бағасы қандай? | I want to buy individual VHI for myself, what's the price? |
| U022 | kk | single | SC11 | Қазір ғана соқтығысып қалдық, жолдың ортасында тұрмын | We just collided, I'm standing in the middle of the road |
| U026 | kk | single | SC13 | КАСКО бойынша өтініш бергім келеді, көлікке бұршақ түсті | I want to file a CASCO claim, hail damaged my car |
| U030 | kk | single | SC15 | Шетелде аяғымды сындырып алдым, сақтандыруым бар | I broke my leg abroad, I have insurance |
| U034 | kk | single | SC17 | Төлемім қашан түседі, білгім келеді | I want to know when my payout will arrive |
| U038 | kk | single | SC19 | Маған төлемнен негізсіз бас тартты | They denied my payout without grounds |
| U042 | kk | single | SC21 | ДМС бойынша кардиологқа жазылғым келеді | I want to book a cardiologist under VHI |
| U046 | kk | single | SC23 | Қарағандыда қай емханалар сақтандыру бойынша қабылдайды? | Which clinics in Karaganda accept the insurance? |
| U050 | kk | single | SC25 | Полисімнің мерзімі қашан бітеді? | When does my policy expire? |
| U054 | kk | single | SC27 | Полисті тағы бір жылға ұзарту керек | I need to renew the policy for another year |
| U058 | kk | single | SC29 | Жаңа мекенжайыма көштім, деректерді өзгертіңізші | I moved to a new address, please update my details |
| U062 | kk | single | SC31 | Сақтандыруды бөліп төлеуге бола ма? | Can I pay for insurance in installments? |
| U066 | kk | single | SC33 | Астанадағы кеңсе сағат нешеде ашылады? | What time does the Astana office open? |
| U070 | kk | single | SC35 | Бір аптадан бері ешкім хабарласпады, шағым қалдырамын | Nobody has contacted me in a week, I'm filing a complaint |
| U074 | kk | single | SC37 | Маған операторды қосыңызшы | Please connect me to an operator |
| U078 | kk | single | SC39 | Сақтандыру ақысын төлегенім туралы анықтама керек | I need a certificate that I paid the insurance premium |
| U081 | ru | multi_intent | SC27, SC04 | Хочу продлить ОГПО и заодно добавить в него сына | I want to renew OGPO and also add my son to it |
| U082 | kk | multi_intent | SC25, SC29 | Полисімнің жарамдылығын тексеріп беріңізші, әрі поштам өзгерді | Please check if my policy is valid, and my email has changed |
| U083 | mixed | multi_intent | SC13, SC20 | Кеше аулада көлігімді біреу соғып кетіпті, КАСКО бар, и ещё подскажите, где у вас осмотр делают | Someone hit my car in the yard yesterday, I have CASCO, and also tell me where you do inspections |
| U084 | ru | multi_intent | SC03, SC01 | Посчитайте каско и обязательную страховку на одну машину | Calculate CASCO and mandatory insurance for one car |
| U085 | ru | multi_intent | SC26, SC29 | Не пришёл полис на почту, и ещё хочу поменять почту на новую | The policy didn't arrive by email, and I also want to change my email |
| U086 | kk | multi_intent | SC06, SC31 | Шетелге шығуға сақтандыру керек, қалай төлеуге болады? | I need travel insurance, how can I pay? |
| U087 | ru | multi_intent | SC19, SC35 | Отказали в выплате, и ваш оператор ещё и нагрубил | They denied my payout, and your operator was also rude |
| U088 | ru | multi_intent | SC21, SC22 | Запишите к лору и скажите, покрывает ли полис лекарства | Book me with an ENT doctor and tell me if the policy covers medication |
| U089 | ru | multi_intent | SC26, SC25 | Полис на почту не пришёл, и заодно скажите, до какого числа он действует | The policy didn't arrive by email, and also tell me until what date it's valid |
| U090 | ru | multi_intent | SC14, SC18 | Соседи затопили квартиру, что делать и какие документы собирать? | Neighbors flooded my apartment, what do I do and what documents do I need? |
| U091 | ru | multi_intent | SC07, SC08 | Хочу застраховать квартиру и себя от несчастного случая | I want to insure my apartment and myself against accidents |
| U092 | kk | multi_intent | SC28, SC29 | Көлігімді саттым, полисті бұзайын, әрі телефон нөмірім өзгерді | I sold my car, I want to cancel the policy, and my phone number has changed |
| U093 | mixed | multi_intent | SC06, SC33 | Сәлеметсіз бе, Түркияға баруға сақтандыру керек, и ещё скажите, где ваш офис в Астане | Hello, I need insurance to travel to Turkey, and also tell me where your Astana office is |
| U094 | mixed | single | SC25 | Сәлеметсіз бе, полисім действует ли ещё, тексеріп беріңізші | Hello, please check whether my policy is still valid |
| U095 | mixed | single | SC12 | Кеше аварияға түстім, но я не виноват, виновник у вас застрахован | I had an accident yesterday, but I'm not at fault, the at-fault party is insured with you |
| U096 | mixed | single | SC39 | Маған справка керек для посольства, на английском | I need a certificate for the embassy, in English |
| U097 | mixed | single | SC34 | Қосымшаға кіре алмай жатырмын, код не приходит | I can't log into the app, the code isn't arriving |
| U098 | ru | out_of_scope | SYS_OUT_OF_SCOPE | Можно у вас взять кредит на машину? | Can I take out a car loan with you? |
| U099 | kk | out_of_scope | SYS_OUT_OF_SCOPE | Сіздерде өмірді сақтандыру бар ма? | Do you have life insurance? |
| U100 | ru | out_of_scope | SYS_OUT_OF_SCOPE | Какая погода завтра в Алматы? | What's the weather tomorrow in Almaty? |
| U101 | kk | out_of_scope | SYS_OUT_OF_SCOPE | Сіздерге жұмысқа орналасуға бола ма? | Can I get a job with you? |
| U102 | ru | unclear | SYS_UNCLEAR | Алло, я по поводу страховки | Hello, I'm calling about insurance |
| U103 | kk | unclear | SYS_UNCLEAR | Мен бір нәрсе сұрайын деп едім | I wanted to ask something |
| U104 | ru | unclear | SYS_UNCLEAR | Ну там с машиной вопрос | Well, it's a question about the car |

## Appendix D — test clients (mock_backend.json), for phone-based demo identification

| client_id | Name | Phone | City | BM | Policies | Claims |
|---|---|---|---|---|---|---|
| C001 | Arman Tulegenov | +77010000001 | Almaty | 7 | SQ-OGPO-104501 (until 2027-03-14), SQ-CASCO-204118 (until 2027-03-14) | CL-500198 paid |
| C002 | Aigerim Bekova | +77010000002 | Astana | 3 | SQ-DMS-604220 (until 2026-12-31) | — |
| C003 | Yerlan Omarov | +77010000003 | Shymkent | 1 | SQ-OGPO-102850 (until 2026-09-29) | — |
| C004 | Natalia Smirnova | +77010000004 | Almaty | 3 | SQ-PROP-404077 (until 2027-01-31) | CL-500311 documents_requested |
| C005 | Daniyar Kaliyev | +77010000005 | Karaganda | 4 | — | CL-500287 approved |
| C006 | Madina Akhmetova | +77010000006 | Astana | 3 | SQ-TRVL-304552 (until 2026-10-05) | — |
| C007 | Sergey Popov | +77010000007 | Pavlodar | 6 | SQ-CASCO-204300 (until 2026-10-20) | CL-500330 under_review |
| C008 | Alibek Sarsenbayev | +77010000008 | Almaty | 4 | SQ-OGPO-103990 (until 2027-02-14) | — |
| C009 | Rustem Ismailov | +77010000009 | Almaty | 10 | SQ-OGPO-104777 (until 2027-05-31) | — |
| C010 | Kamila Utepova | +77010000010 | Atyrau | 3 | SQ-CASCO-204350 (until 2027-04-30) | — |
| C011 | Nurlan Zhumabekov | +77010000011 | Astana | 5 | SQ-OGPO-105120 (until 2027-04-09) | — |

Payments:
- `{"payment_id": "P-2950", "client_id": "C009", "date": "2026-06-01", "amount": 22800, "product": "ogpo", "status": "success", "policy_number": "SQ-OGPO-104777"}`
- `{"payment_id": "P-3001", "client_id": "C003", "date": "2026-09-30", "amount": 31200, "product": "ogpo", "status": "charged_policy_not_issued", "policy_number": null, "note": "Renewal for vehicle 222ABC17. Bank charged the card, policy issuance failed."}`

Sample dialogs (dialogs_sample.json):
- D01 — OGPO quote turns into purchase (scenario_switch, confirmation), client None
- D02 — Victim claim under culprit's OGPO (Kazakh) (kazakh, confirmation), client None
- D03 — Claim status, detour to renewal, return to claim (topic_switch, context_return, multi_intent), client C007
- D04 — DMS appointment with mixed Kazakh-Russian speech (mixed_language, multi_intent), client C002
- D05 — Low confidence, clarification, then resend (clarification), client C009
- D06 — Complaint escalated to a human (handoff, emotional), client C004
- D07 — CASCO termination with explicit confirmation (Kazakh) (kazakh, irreversible_action), client C010
- D08 — Property claim status and missing document (context_carry), client C004
- D09 — Fraud report followed by policy check (security, scenario_switch), client C008
- D10 — Travel purchase, client switches from Kazakh to Russian (language_switch, confirmation), client None
