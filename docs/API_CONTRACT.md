# API contract — frontend ⇄ Go backend (Mock-Driven Development)

> Source of truth: `frontend/src/lib/contract.ts`. The frontend already runs end-to-end against a browser mock that emits exactly these shapes (`frontend/src/lib/mock/engine.ts`). The backend replaces the mock without touching the UI: set `NEXT_PUBLIC_API_MODE=real` and `NEXT_PUBLIC_API_URL=http://localhost:8080`.

## Endpoints

| Method | Path | Body → Response |
|---|---|---|
| `POST` | `/api/session` | — → `{ session_id, state: DialogState }` |
| `POST` | `/api/turn` | `TurnRequest` → `text/event-stream` of `TurnEvent` (one JSON per `data:` line, frames separated by a blank line) |
| `GET` | `/api/session/{id}/trace` | → `Trace[]` |
| `GET` | `/api/supervisor/stats` | → `SupervisorStats` |
| `GET` | `/api/scenarios` | → `scenarios.json` (catalog; later editable) |
| `POST` | `/api/eval/run` | — → `EvalResult`: the `data/evaluate.py` metrics computed server-side on `dev_utterances.json` — `{ groups:[{key,n,primary,full}], intent_recall, errors:[{id,text,expected,got,lang,type}], mean_ms, model }` (shown on /admin → «Качество маршрутизации») |
| `GET` | `/health` | → `{ status: "ok", mode: "mock" \| "real" }` |

CORS: allow `NEXT_PUBLIC_API_URL`'s origin (`CORS_ORIGINS`). SSE headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, flush after every event.

## `TurnRequest`

```json
{ "session_id": "s_ab12", "text": "Хочу продлить ОГПО", "lang_hint": "ru", "client_t0": 1758630000000 }
```
or with audio from the browser recorder:
```json
{ "session_id": "s_ab12", "audio_base64": "...", "audio_mime": "audio/webm;codecs=opus", "client_t0": 1758630000000 }
```
`client_t0` = epoch ms when the client stopped speaking; the frontend measures end-to-end latency from it.

## Event order for one turn (`TurnEvent`)

```
turn.start        { turn, session_id, t0 }
stt.partial*      { text }                                  (0..n, only for audio input)
stt.final         { text, language: ru|kk|mixed, ms }
triage            { language, urgent, parts[], normalized, ms }
router.candidates*{ candidates:[{scenario_id, confidence}], partial:true }   (0..n — live confidence; speculative/streamed)
router.decision   { decision: RouterDecision, ms }
policy            { verdict: PolicyVerdict, ms }
action*           { call: ActionCall }                      (0..n; mode read|preview|execute)
state             { state: DialogState }
response.delta*   { text }                                  (streamed text)
response.final    { text, language: ru|kk, ms }
tts.audio         { audio_base64, mime, ms_first_audio }    (or { browser_tts:true } to let the browser speak)
turn.done         { trace: Trace, latency_ms }
error             { message }                               (any time; ends the turn)
```

### `RouterDecision` (README "Уровень 2")
```json
{
  "scenarios": [{ "scenario_id": "SC30", "confidence": 0.86, "reason": "money charged, policy not issued" }],
  "alternatives": [{ "scenario_id": "SC26", "confidence": 0.31 }],
  "language": "ru",
  "slots": { "payment_date": "2026-09-30" },
  "is_continuation": false,
  "reason": "…one sentence for the supervisor…",
  "model": "gpt-4.1-mini",
  "tier": "full"
}
```
Order of `scenarios`: `urgent` first (SC11, SC15, SC38), then mention order. System intents: `SYS_OUT_OF_SCOPE`, `SYS_UNCLEAR`, `SYS_GOODBYE`.

### `PolicyVerdict`
```json
{ "action": "run|continue|clarify|handoff|out_of_scope|goodbye", "scenario_id": "SC30", "queue": "operator_general", "reason": "confidence 0.86 ≥ 0.75 → run", "stack": ["SC29"], "low_conf_streak": 0 }
```
Thresholds: `≥ 0.75` run · `0.45–0.75` clarify (SYS_UNCLEAR with two options) · `< 0.45` twice → handoff with context.

### `ActionCall`
```json
{ "name": "create_claim", "mode": "preview", "input": { "product_type": "ogpo", "incident_date": "2026-09-28" }, "result": { "claim_number": "CL-500342" }, "ms": 12 }
{ "name": "find_client", "mode": "read", "input": { "phone": "+77010000099" }, "error": { "code": "not_found", "message": "…" }, "ms": 3 }
```
Irreversible actions (`create_policy, renew_policy, update_policy, cancel_policy, create_claim, create_dispute, book_inspection, book_appointment, update_contact`) are emitted first with `mode: "preview"`; `execute` only after the client's explicit «да».

### `DialogState`
```json
{ "session_id": "s_ab12", "language": "ru", "client_id": "C007", "client_name": "Sergey Popov", "active_scenario": "SC17", "stack": ["SC27"], "slots": { "phone": "+77010000007" }, "awaiting": { "kind": "slot", "slot": "claim_number" }, "low_conf_streak": 0, "turn": 3, "ended": false }
```
`awaiting` is `null`, `{kind:"slot", slot}` or `{kind:"confirmation", action}`.

### `Trace` (README "Трассировка" + extras)
```json
{
  "turn": 3, "transcript": "…", "language": "mixed",
  "scenarios": [{ "scenario_id": "SC21", "confidence": 0.9 }],
  "alternatives": [{ "scenario_id": "SC23", "confidence": 0.4 }],
  "reason": "…", "slots": { "doctor_specialty": "therapist" },
  "actions": ["find_client", "book_appointment:preview"],
  "latency_ms": { "stt": 0, "triage": 0, "router": 0, "policy": 0, "executor": 0, "response": 0, "tts_first_audio": 0, "total": 0 },
  "policy": { "...PolicyVerdict" }, "response_text": "…", "response_lang": "kk", "model": "…", "tier": "full",
  "candidates_history": [[{ "scenario_id": "SC21", "confidence": 0.4 }], [{ "scenario_id": "SC21", "confidence": 0.9 }]]
}
```

### `SupervisorStats`
`{ sessions, turns, avg_confidence, low_confidence_turns, clarifications, handoffs, out_of_scope, median_total_ms, p95_total_ms, by_scenario:[{scenario_id,count,avg_confidence}], by_language:[{language,count}], recent: Trace[] }`

## Mock mode
`LLM_PROVIDER=mock` on the backend must produce the same stream deterministically (no keys). The frontend's own mock (`NEXT_PUBLIC_API_MODE=mock`, default) is a lexical router + rule executor over the starter kit and is labelled "mock" in the UI; it is NOT the product router.
