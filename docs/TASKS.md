# Tasks — build plan 13:50–18:00 (GMT+5), feature freeze 17:15

> Status: living draft — not final; update as decisions change. Source (RU): docs/research/raw/deep-research-output.ru.md

## General rules
Commit every hour (14/15/16/17/18), each person their own commits; no secrets; THIRD_PARTY.md; runnable from README; MOCK_MODE for the jury; synthetic data only; official wired network.

## Checkpoint 14:00
- **Tair (AI/lead):** [13:50–14:00] unpack voice_router_dataset.zip, study scenarios.json/dialogs_sample.json/evaluate.py; lock down the router's JSON schema. AC: schema in SPEC, repo created, README skeleton.
- **Alikhan (Go):** chi + pgx skeleton + migrations (sessions/turns/route_decisions/stage_timings) + /api/health + docker compose. AC: `docker compose up` brings up db+backend, /api/health=ok.
- **Ramazan (Next.js):** UI skeleton (Speko design system), "Simulator" and "Supervisor" pages, empty TracePanel. AC: `npm run dev` renders, pings /api/health.

## Checkpoint 15:00 (routing core)
- **Tair:** [14:00–14:30] embedding shortlist over scenarios.json (in memory at startup); [14:30–15:00] LLM-JSON route (gpt-4.1-nano) + MOCK router. AC: POST /api/route returns valid JSON for 5 manual utterances.
- **Alikhan:** [14:00–14:30] /api/session, /api/route (proxies to Tair's router), writes to DB; [14:30–15:00] stage_timings instrumentation. AC: decision and timings land in the DB.
- **Ramazan:** [14:00–15:00] TracePanel renders the decision from /api/route; text input. AC: type text → trace is visible.

## Checkpoint 16:00 (voice + P0 finish)
- **Alikhan:** [15:00–15:30] WS /ws/voice: receives PCM, emits events; [15:30–16:00] STT integration (Scribe v2 RT; geko fallback), TTS (geko Tokay/EL Flash). AC: voice in browser → transcript → route → voice back.
- **Ramazan:** [15:00–15:30] mic capture (WebAudio PCM16) + TTS player; [15:30–16:00] Silero VAD (endpoint), recording indicator. AC: the "Speak" button works end-to-end.
- **Tair:** [15:00–15:30] /api/eval/run + wire up evaluate.py/dev_utterances.json; [15:30–16:00] accuracy+latency report. AC: an accuracy number shows in UI and README.

## Checkpoint 17:00 (P1 value)
- **Tair:** topic stack (topic_switch/return_to_topic), action=clarify|handoff, extracted_params; the official example passes. AC: demo utterance #2 works.
- **Alikhan:** /api/supervisor/stats + fast-path with Δ measurement. AC: stats returns accuracy/timings; Δ is visible.
- **Ramazan:** supervisor panel (accuracy, errors, "uncertain" cases, latencies), language/topic_switch badges. AC: panel is populated.

## 17:00–17:15 (freeze prep)
- Everyone: remove secrets (`git log -p | grep -iE "key|secret|token"`), update THIRD_PARTY.md, final commits (17:00). Feature freeze 17:15.

## What to cut first (if behind)
1. Phone (P2) — cut immediately. 2. Speculative/500ms — keep only the measurement. 3. Catalog editor. 4. Fast-path (keep only the LLM path). 5. Supervisor stats → minimal table. **NEVER cut:** the 5 must-haves (voice, LLM route, correctness/evaluate, trace, RU+KZ).

## 17:15–18:00 (submission)
- [ ] Fresh clone into a clean folder → `docker compose up` → works per README.
- [ ] MOCK_MODE=1: jury can test without keys; /api/eval/run gives accuracy.
- [ ] README: all sections from the organizer's prompt; link to deploy (Railway).
- [ ] Deploy to Railway, verify the live URL.
- [ ] README contains only verifiable facts.
- [ ] **Captain clicks "Submit solution" on the platform (push ≠ submission!).**

## Phone stretch slot (only if P0 done by 16:00)
- 30–45 min: Vapi + a Twilio number (available country) OR LiveKit SIP inbound; our /api/route as the "brain". KZ +7 KYC won't make it in time (**ASSUMPTION**) — demo on a non-KZ number or present it as an architectural plan in the README.
