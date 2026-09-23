# Technical spec — Voice Router

> Status: describes the implemented system (source of truth for the API contract). Product scope: [PRD.md](PRD.md). Official task: [CASE.md](CASE.md).

## Architecture

```
[Browser]  mic PCM16 16k ──WS binary──►  [Go backend]                      [Providers]
           JSON control  ──WS text───►   httpapi/ws.go                     STT: elevenlabs_realtime | elevenlabs | openai(-compatible) | mock
           ◄── events JSON / PCM24k ──   │                                 TTS: elevenlabs | openai(-compatible) | browser
           REST /api/*  ◄──────────────  │                                 LLM: openai(-compatible) | mock
                                         ▼
                        dialog.Engine.RunTurn  (one turn = one utterance)
                        ├─ triage.Analyze        language, entities, signals               (~µs)
                        ├─ retrieval.Search      BM25 shortlist + lexicon                  (~1 ms)
                        ├─ prefetch facts        confirmation gate, find_client, get_policy/claim
                        ├─ policy.FastPath ? mock.Route : llm.Route (streaming JSON, decision before reply)
                        ├─ policy.Apply          validate, urgent-first, thresholds
                        ├─ execute               topic stack, slots, actions (read-only now / irreversible → preview), handoff
                        ├─ follow-up llm call    only if the model asked for data it needs for the reply
                        ├─ speaker               sentence → TTS stream → WS binary frames (or `speak` events for browser TTS)
                        └─ trace                 events bus → WS/SSE; store (JSONL) → supervisor stats
```

No database: `internal/store` keeps sessions in memory and appends every trace to `var/traces.jsonl` (reloaded on start). Swapping in Postgres means implementing `Append/Sessions/Session/Stats`.

## Router contract (LLM output)

```json
{"language":"ru|kk",
 "scenarios":[{"id":"SC30","confidence":0.9,"reason":"деньги списаны, полис не оформлен"},{"id":"SC29","confidence":0.85,"reason":"…"}],
 "alternatives":[{"id":"SC26","confidence":0.25}],
 "is_continuation":false,
 "slots":{"payment_date":"2026-09-30"},
 "actions":[{"name":"find_client","args":{"phone":"+77010000003"},"mode":"execute|preview"}],
 "handoff":null,
 "reply":"Понимаю, сначала разберёмся с оплатой…"}
```

Keys are generated in this order; `internal/router/parser.go` parses the object as soon as `"reply":"` appears, so the routing latency (`timings.route`) is measured before the reply is generated. Truncated / fenced / slightly broken JSON is repaired (`closeJSON`), and a routing failure falls back to the lexical router (`path=fallback`).

Prompt = static system part (role, rules, the full catalog rendered from `scenarios.json`, slots, actions, output contract, 10 worked examples — `GET /api/prompt`) + dynamic user part (TODAY, dialogue state, FACTS, SIGNALS, retrieval hints, the utterance). The static part is identical across turns so provider prompt caching applies.

## Decision policy (`internal/router/policy.go`)

- unknown IDs dropped; urgent scenarios (SC11, SC15, SC38) moved first; duplicates removed
- `confidence < POLICY_CLARIFY_MIN (0.30)` → `SYS_UNCLEAR` (the model's reply is replaced by the clarifying template with the top-2 options)
- `POLICY_CLARIFY_MIN ≤ confidence < POLICY_PROCEED_MIN (0.55)` → keep the scenario, verdict `clarify`
- third `SYS_UNCLEAR` in a row → `handoff` to `operator_general`
- `handoff.queue` set (by the model, by `transfer_to_operator`, or by the scenario's handoff rule) → `handoff`

Fast path (`FAST_PATH=on`): top lexical candidate is `fast_path_eligible`, score ≥ 0.75 and margin ≥ 0.35 over the runner-up, no multi-intent / urgency / out-of-scope markers, no pending confirmation, no open slots, ≤ 14 tokens → templated answer without the LLM; the LLM then verifies in the background (`shadow` event, `fast_path_agree` in stats). `shadow` = always LLM, log what the fast path would have done; `off` = disabled.

## Safety

Irreversible actions (`actions.json: irreversible=true`) requested by the model are turned into a **preview** (`dialog.Engine.execute`) and stored as `pending_confirmation`. On the next turn the deterministic triage detects an explicit yes/no; only a yes executes the action (`prefetch`), and the result is fed to the model as `FACTS … EXECUTED` so the reply reports it. The model cannot execute them directly.

## REST API

| Method & path | Body / query | Returns |
|---|---|---|
| `GET /health`, `/healthz`, `/api/health` | — | `{ok, mock_mode, router, stt, tts, uptime_s}` |
| `GET /api/config` | — | redacted providers, policy thresholds, `router`, `tts_sample_rate`, `as_of_date` |
| `GET /api/prompt` | — | text/plain system prompt |
| `POST /api/sessions` | `{channel}` | `{session_id, session}` |
| `GET /api/sessions?limit=` | — | `{sessions:[…]}` (store view) |
| `GET /api/sessions/{id}` | — | `{session, turns:[Record], state?}` |
| `POST /api/sessions/{id}/turn` | `{text, voice?, collect_audio?, source?}` | `{session_id, turn, reply, language, scenarios[], confidence, path, timings, trace, audio_wav_base64?}` |
| `POST /api/sessions/{id}/turn/audio` | multipart `file` (WAV PCM16), `transcript_hint?`, `voice?`, `collect_audio?` | same as above (STT first) |
| `POST /api/turn` | `{text, session_id?}` | same; creates a session when absent |
| `POST /api/route` | `{text}` | stateless routing: `{scenarios[], confidence, path, route_ms, decision, verdict, candidates, signals, fast_path, llm?}` — used by `scripts/eval.py` |
| `POST /api/eval/run` | `{concurrency?, limit?}` | evaluation report (same metrics as `data/evaluate.py` + latency) |
| `GET /api/eval/last` | — | `{running, progress, report}` |
| `GET /api/supervisor/stats` | — | aggregates: by scenario/language/path/policy, low confidence, handoffs, fast-path agreement, timing percentiles |
| `GET /api/supervisor/actions` | — | mock backend action log |
| `GET /api/catalog` · `POST /api/catalog/reload` | — | catalog JSON · reload from disk |
| `PUT /api/catalog/scenarios/{id}` | `{description?, not_this_if?, examples?, priority?, fast_path_eligible?}` | edits a scenario without a restart: persisted to `VAR_DIR/catalog/scenarios.json` (an override of `data/scenarios.json`, which stays untouched), index and prompt rebuilt |
| `GET /api/lexicon` | — | the retrieval lexicon in use |
| `GET /api/debug/events?session=&replay=1` | SSE | every pipeline event of every session |
| `POST /api/tts` | `{text, lang}` | WAV |

## WebSocket `/ws?session_id=`

Client → server: binary frames = PCM16 LE mono 16 kHz; JSON `{type:"speech_start"|"speech_end"|"text"|"config"|"cancel"|"ping", text?, source?, voice?, t?, transcript_hint?}`.
Server → client: JSON `Event {type, session_id, turn, t, data}` for `session, stt_partial, stt_start, stt_final, stt_error, stt_realtime, turn_start, triage, retrieval, facts, fast_path, llm_start, llm_delta, llm_error, route, reply_delta, action, handoff, reply_done, speak, tts_first_byte, tts_sentence, tts_error, audio_end, shadow, turn_done (data = full Trace), turn_error, error`; binary frames = PCM16 mono reply audio at `audio_out.sample_rate` (24 kHz).

## Trace (`turn_done` payload / stored record)

`input{source, transcript, stt_provider, audio_ms}`, `language{detected, kk_share, reply}`, `triage`, `retrieval[]`, `path` (`llm|fast|mock|fallback`), `fast_path{eligible, reason}`, `decision`, `policy{action, confidence, notes}`, `facts[]`, `actions[{name, args, mode, result, error, ms}]`, `state{client, active_scenario, stack, slots, pending_confirmation, handoff, closed}`, `reply{text, lang, sentences, follow_up}`, `timings{stt, triage, retrieval, facts, route, llm_route, llm_ttft, llm_total, followup_llm, tts_first_byte, first_audio, total}` (ms), `llm{provider, model, usage{prompt, completion, cached}, raw, messages}` (with `DEBUG=true`), `shadow`, `errors[]`.

Targets shown in the UI: `route ≤ 500 ms`, `first_audio ≤ 1500 ms` (both from end of speech). Keyless routing is ~1 ms; with an LLM `route ≈ TTFT + ~40 output tokens`.

## Evaluation

`scripts/eval.py` → `POST /api/route` for each of the 104 dev utterances → `var/predictions.json` → `data/evaluate.py`. The jury's hidden utterances are not in the code; the dev set is used only for measurement (it is not in the prompt).

## Environment

See `.env.example`. Precedence: specific `LLM_/STT_/TTS_API_KEY` → `OPENAI_API_KEY` / `ELEVENLABS_API_KEY`. Provider auto-selection when `STT_PROVIDER`/`TTS_PROVIDER` are empty: ElevenLabs if its key is set, else OpenAI if its key is set, else browser.
