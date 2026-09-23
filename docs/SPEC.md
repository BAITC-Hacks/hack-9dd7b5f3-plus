# Technical spec — Voice Router (Bagyt)

> Status: living draft — not final; update as decisions change. The API contract here is the source of truth for frontend/backend; change the contract → update this file in the same commit. Source (RU): docs/research/raw/deep-research-output.ru.md

## Implemented frontend ↔ core-llm integration (2026-09-23)

This section describes the implemented path; the Go/DB architecture below remains a target.
`NEXT_PUBLIC_API_MODE=core` enables Python routing with the existing browser dialog executor.
`mock` remains keyless, and `real` retains the separate Go SSE contract in `frontend/src/lib/contract.ts`.

- Python `GET /healthz` → `{ok, model}`.
- Python `POST /api/route` → `Router.route` result. Request:
  `{text: string, history?: [{text: string, scenario: string}], active?: string|null, last_bot?: string}`.
  Text limit: 4000 characters; history limit: 10; body limit: 32 KiB.
- Next.js `POST /api/core-route` proxies the same JSON to server-only `CORE_LLM_URL` (default port 8090).
  Upstream failures return sanitized 502/503 errors; invalid requests return 400/413. No silent mock fallback.
- Result: `{primary, status, intents, alternatives, scenarios, predicted, language, model, route_ms, ...usage}`.
  Entries use `{id, pct, name}`; adapter converts confidence from 0–100 to 0–1. `status` is
  `route|clarify|handoff|out_of_scope|goodbye`. Core status overrides the mock router's confidence threshold.
- Browser passes last four completed turns, active scenario and last bot response. It retains slots,
  confirmation handling, topic stack, synthetic actions and response templates. Failed requests do not commit state.
- The trace explanation describes returned scores/policy; it is not model-generated reasoning.
  Routing timing includes the request to Python. No simulated candidate progression in core mode.
- STT/TTS: browser speech APIs, existing `/api/stt` OpenAI fallback. Core accepts text only.
- Sessions and traces are tab-local and not persisted. Eval in core mode uses the core's `predicted` IDs.
- Compose starts frontend by default; `--profile llm` adds private Python service. No Go/DB dependency.

## ASCII architecture
```
[Browser: mic PCM16 16k / text]  ──WS──┐
        ▲  TTS audio (stream)          │
        │                              ▼
   Next.js (TS/Tailwind/shadcn)     Go (chi) gateway ── pgx ──> Postgres
   TracePanel / Supervisor          │  │  │  │
                                    │  │  │  └─ stage_timings (measurements)
                                    │  │  └─ STT: ElevenLabs Scribe v2 RT (primary)
                                    │  │         / geko Seta (KZ/RU code-switch, fallback)
                                    │  └─ Router: embeddings (shortlist from scenarios.json)
                                    │        → LLM JSON (gpt-4.1-nano) → [escalate gpt-4o]
                                    └─ TTS: geko Tokay (KZ) / ElevenLabs Flash v2.5 (RU)
   VAD: Silero (endpoint + barge-in)      MOCK_MODE=1 → deterministic responses (no keys)
```

## Components and fallbacks
| Layer | Primary | Fallback | Rationale |
|---|---|---|---|
| STT | ElevenLabs Scribe v2 Realtime (~150ms, KZ/RU High Accuracy) | geko Seta seta-kk-ru-v2 (code-switch, 8.71% WER) | latency vs. code-switch quality |
| VAD/endpoint/barge-in | Silero VAD (~32ms frames) | WebRTC VAD | industry standard |
| Shortlist | OpenAI text-embedding-3-small over scenarios.json | lexical match | retrieval, not classification |
| Router LLM | gpt-4.1-nano (JSON schema) | gpt-4o / Gemini 2.5 Flash-Lite; "fast tier" Groq/Cerebras | RU/KZ quality + valid JSON |
| TTS | geko Tokay tokay-kk-v1 (KZ) | ElevenLabs Flash v2.5 (RU) | KZ-first voices, numbers spoken as words |
| (opt.) Voice router | — | Speko (non-KZ/RU languages only) | KZ/RU not covered |

## Router algorithm (step by step)
1. Get the final (or stable partial) transcript + language.
2. Embed the utterance → cosine similarity to precomputed example vectors from scenarios.json → top-K (K=5–8) candidates. This is a SHORTLIST, not a decision.
3. Assemble the prompt: system role + dialogue context (last N turns + topic stack) + K candidates (id, purpose, boundaries) + instruction to return strict JSON.
4. LLM (gpt-4.1-nano) returns a JSON-schema output. If confidence < 0.6 → escalate to gpt-4o (or ask a clarifying question).
5. Update the topic stack (topic_switch/return_to_topic), log route_decisions + stage_timings.
6. Response: action=proceed → generate a reply from knowledge_base/mock_backend + TTS; clarify → clarifying question; handoff → hand off to an operator with context.
7. (P2) Fast-path: if top-1 cosine > 0.85 AND there are no topic-switch markers → a short path without the large LLM, measure Δ; correctness is still validated via the LLM path.
8. (P2) Speculative: run steps 2–4 on a stable partial (>0.85 conf STT), cancel stale calls (−200…400ms P95).

## Router prompt draft
```
You are the routing layer of an insurance company's contact center. You are given:
- dialogue context (turns + active topic stack),
- scenario CANDIDATES (id, purpose, boundaries with neighbors),
- the client's latest utterance (may mix RU/KZ).
Do not guess. Choose a scenario ONLY from the candidates. If the utterance contains
multiple intents — return the primary one + flag the rest in topic_switch.
If confidence is low or the utterance is borderline — action="clarify".
Return JSON STRICTLY per the schema. Reasoning — in the client's language, brief.
```

## JSON decision schema
```json
{
  "scenario_id": "string",
  "confidence": 0.0,
  "reasoning": "string",
  "alternatives": [{"scenario_id":"string","confidence":0.0}],
  "extracted_params": {"key":"value"},
  "topic_switch": true,
  "return_to_topic": "string|null",
  "language": "ru|kk|mixed",
  "action": "proceed|clarify|handoff"
}
```

## Latency budget (target, real numbers to be filled in from measurements)
| Stage | Target | Note |
|---|---|---|
| VAD end → STT final | ~150–300ms | Scribe v2 RT ~150ms finalize |
| Shortlist (embedding+cosine) | ~10–50ms | in memory |
| LLM route (TTFT) | ~300–600ms | gpt-4.1-nano; OpenAI-class TTFT ~450ms; fast tier <150ms warm |
| → scenario selection (sum) | target 500ms | realistically ~500–900ms; speculative hides it |
| LLM answer + first TTS byte | ~400–800ms | streamed by sentence |
| **End of utterance → start of response** | **target 1.5s** | realistically 1.5–2.5s, measured |

## Data model (Postgres)
- `sessions(id, created_at, lang_pref, channel)`
- `turns(id, session_id, idx, role, transcript, lang, created_at)`
- `route_decisions(id, turn_id, scenario_id, confidence, reasoning, alternatives jsonb, extracted_params jsonb, topic_switch, return_to_topic, action, model, escalated bool)`
- `stage_timings(id, turn_id, stage, ms)` — stage ∈ {vad_end, stt_final, shortlist, llm_route, llm_answer, first_tts_byte}

## WebSocket + REST (≤10)
1. `POST /api/session` → `{session_id}`.
2. `WS /ws/voice?session_id=` — in: binary PCM16 frames + `{type:"end"}`; out: `{type:"partial|final",text}`, `{type:"route",decision}`, `{type:"tts",audio_b64|url}`, `{type:"timing",stage,ms}`.
3. `POST /api/route` — `{session_id, text, lang?}` → full decision JSON. **This endpoint calls evaluate.py.** Deterministic when MOCK_MODE=1.
4. `GET /api/session/{id}/trace` → array of turns+route_decisions+timings.
5. `GET /api/supervisor/stats` → `{accuracy, by_scenario, low_confidence_count, avg_timings}`.
6. `POST /api/eval/run` → runs dev_utterances.json through the router → `{accuracy, per_utterance[], latency_p50, latency_p95}` (shown in UI and README).
7. `GET /api/scenarios` → catalog (for UI and the P2 editor).
8. `POST /api/tts` — `{text, lang}` → audio (MOCK: canonical wav).
9. `GET /api/health` → `{ok, mock_mode}`.
10. `GET /api/config` → flags.

Example `/api/route` response:
```json
{ "scenario_id":"payment_not_confirmed", "confidence":0.82,
  "reasoning":"клиент оплатил, заказ не подтверждён — сценарий по статусу оплаты",
  "alternatives":[{"scenario_id":"change_delivery_address","confidence":0.41}],
  "extracted_params":{"topic2":"change_address"}, "topic_switch":true,
  "return_to_topic":"change_delivery_address","language":"ru","action":"proceed" }
```

## evaluate.py / dev_utterances.json integration
- evaluate.py sends `POST /api/route` for each labeled utterance, compares `scenario_id` to the label, computes accuracy. We add `/api/eval/run` as a wrapper — the jury sees accuracy+latency in the UI and README with one click.
- The jury's test utterances are NOT stored in code; the router decides dynamically from scenarios.json.

## Trace panel and supervisor (UI)
- **TracePanel (after each turn):** transcript, chosen scenario + reasoning, alternatives with confidence, a stage_timings table (ms per stage), badges for topic_switch/return_to_topic/language/action.
- **Supervisor:** a table of sessions, accuracy over dev_utterances, top errors and "uncertain" cases (confidence<0.6), average latencies, filters by scenario.

## MOCK_MODE
- `MOCK_MODE=1`: STT/TTS/LLM are replaced by deterministic stubs (STT returns the request text; the router uses rule+embedding matching with no external keys; TTS returns a pre-recorded wav). The jury can test /api/route, /api/eval/run, and the UI without the team's keys.

## Environment variables
`OPENAI_API_KEY, ELEVENLABS_API_KEY, GEKO_API_KEY, SPEKO_API_KEY(opt.), DATABASE_URL, MOCK_MODE, ROUTE_MODEL(gpt-4.1-nano), EMBED_MODEL(text-embedding-3-small), PORT`.

## docker compose
Services: `db` (postgres), `backend` (Go chi), `frontend` (Next.js). `docker compose up` brings everything up; with MOCK_MODE=1 no external keys are needed.

## How we avoid the prohibitions (for README)
- "Not an encoder-only classifier": embeddings only produce a LIST of candidates; the final choice and reasoning are generated by the LLM (Rasa CALM-style command generation) — we show the prompt and the JSON.
- "No hardcoding": there are no test utterances in the code; scenarios.json is read dynamically; embeddings are built from it at startup; changing the catalog requires no code changes.

## Current repo state vs. spec
This spec describes the target architecture. The repo's actual state right now is simpler and not yet aligned with it:

- **Backend:** `backend/cmd/server/main.go`, Go module `hackathon/backend`, go 1.26.5, stdlib `net/http` only, exposing a single `GET /health`. No chi, no pgx, no Postgres wiring yet.
- **Frontend:** a fresh, unmodified Next.js app under `frontend/`.
- **AGENTS.md** proposes a different layout: `backend/cmd/api` + `chi` router + `pgx` for Postgres, with a `GET /healthz` health endpoint.

There is an open decision to align: whether to move/rename `cmd/server` → `cmd/api` (and `/health` → `/healthz`) to match AGENTS.md, or update AGENTS.md to match the existing skeleton. This spec does not pick a side — flag it for Tair/Alikhan to resolve before backend work goes further.

## ElevenLabs TTS integration

The `voice/` service is imported unchanged from `feat/voice-elevenlabs` (`fb7d16c`).
Only TTS is connected: completed frontend response → `POST /api/tts {text, lang: ru|kk}` →
`VOICE_TTS_URL/api/voice/tts {text, lang, format: mp3_22050_32}` → MP3 → browser Audio playback.
The proxy streams bytes; the client buffers the MP3 before playback. `tts_first_audio` includes
synthesis, download and playback startup, measured until `playing`. Muting cancels synthesis/audio.
Text stays in the transcript. Mock uses browser TTS; errors show a notice and use browser TTS.
Limits: 6000 text characters, 30-second upstream deadline; provider errors are sanitized.
Voice/model selection stays in the imported Go module. No changes to browser STT or `/api/stt`.
Compose `--profile voice` starts the service without exposing a host port, alongside profile `llm`.
`ELEVENLABS_API_KEY` is read from local root `.env`; `VOICE_TTS_URL` is server-only.
