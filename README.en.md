# Voice Router — Team Plus · HackAlem AI · Halyk Bank case

> Track 09 · Communications · **Voice Router**: a voice robot for an insurance contact center where the scenario is chosen by an **LLM layer that understands the dialogue**, not by an encoder intent classifier. Russian + Kazakh (incl. mixed speech), voice in / voice out, and a supervisor trace panel that shows *what* was chosen, *why*, and *how fast*.
>
> Russian version (primary, per the organizers' README requirements): [README.md](README.md) · Deployed link: `<fill in if deployed>` · Demo access: none needed (keyless mode works out of the box)

## 1. Summary

The client speaks into the microphone (or types). The robot understands the request from the first sentence, keeps context across up to 10 turns, switches topic and comes back, switches between Russian and Kazakh, asks instead of guessing, hands over to a human with a summary when it should, and never runs an irreversible action without an explicit "yes". After every client turn the supervisor sees the selected scenario, the reasoning, the alternatives, the extracted parameters, the executed actions and per-stage timings against the two targets (**scenario decision ≤ 500 ms**, **end of speech → start of the answer ≤ 1.5 s**).

All 40 scenarios + 3 system intents, the slots, the actions, the knowledge base and the mock backend come from the organizer's starter kit in [`data/`](data/) (fictional **Saqta Insurance**; synthetic data). Nothing about the jury's utterances is hardcoded: the router reads `scenarios.json` at start-up (and on `POST /api/catalog/reload`).

## 2. What is implemented

| Requirement | How |
|---|---|
| Voice interaction in the web app | Mic → PCM16 16 kHz over WebSocket → STT (ElevenLabs Scribe v2 **realtime** with partial transcripts, or OpenAI/geko batch) → router → sentence-streamed TTS (ElevenLabs Flash v2.5 / OpenAI / geko Tokay) → gapless playback in the browser. Text input as a fallback. Keyless mode uses the browser's speech APIs. |
| Scenario selection on an LLM layer | One streaming chat completion per turn with the **whole catalog** (descriptions, `not_this_if` boundaries, slots, actions) in a cached system prompt plus the dialogue state; the model returns JSON whose keys are ordered so that the **decision streams before the spoken reply** — the scenario is known ~at first tokens, the reply follows in the same call. No encoder classifier anywhere. |
| Correctness | `python scripts/eval.py` runs the official `data/evaluate.py` over the 104 dev utterances through the live router. Keyless lexical baseline: **90.4 % primary accuracy** (multi-intent recall 0.885, out-of-scope 4/4, unclear 3/3); the LLM router is the production path (see §7). |
| Trace panel | After every utterance: transcript, detected/reply language, scenario(s) with confidence and reason, alternatives, triage signals, retrieval hints, policy verdict, slots, actions (execute / preview / handoff), dialogue state, and a timing waterfall (STT, triage, retrieval, facts, **route**, LLM TTFT, TTS first byte, **first audio**, total) with client-measured end-to-end latency. |
| Russian and Kazakh | Language detection incl. mixed phrases (`ru` / `kk` / `mixed` + Kazakh share), reply in the client's dominant language, STT with RU/KZ code-switching, TTS voice per language. |

Optional items covered: **hybrid fast path** (lexical shortlist answers obvious informational requests in ~1 ms; the LLM verifies in the background and the supervisor sees the agreement rate), **topic stack** (interrupted scenarios are kept and offered back), **clarify instead of guessing** (`SYS_UNCLEAR` with the two most likely options), **handoff with context summary**, **slot extraction** incl. spoken phone numbers ("плюс жеті, жеті жүз бір…" → `+77010000002`), **streaming** (STT partials, LLM tokens, sentence-level TTS), **tone detection** (upset/urgent signal → the reply acknowledges first), **supervisor statistics**, **catalog editing without developers** (`PUT /api/catalog/scenarios/{id}` + reload, `data/` stays read-only), **confirmation gate** for irreversible actions enforced by the server, not by the prompt.

## 3. How it works (main scenario)

```
 browser mic ──PCM16──► WS ──► STT (realtime partials) ──► end of speech (t0)
                                                              │
   ┌──────────────────────────────────────────────────────────┘
   ▼
 1. TRIAGE  (deterministic, <1 ms): language ru/kk/mixed, entities (phone, IIN, plate,
             policy/claim number, spoken numbers → digits), urgency, multi-intent markers,
             yes/no confirmation, goodbye, operator request, out-of-scope hints
 2. RETRIEVAL (lexical BM25 over scenario examples + editable lexicon, ~1 ms): a SHORTLIST with
             scores — a hint for the model and the fast-path trigger, never the decision
 3. FACTS   (mock backend): confirmation gate for the previewed action; identify the client by the
             phone/IIN just said; look up the policy/claim mentioned → injected into the prompt
 4. ROUTE   fast path (informational + clear lexical winner + no multi-intent/urgency) ─► templated reply
            otherwise LLM: system prompt (catalog, rules, slots, actions, few-shot) + state + signals + facts
            → streamed JSON {language, scenarios[{id, confidence, reason}], alternatives, is_continuation,
              slots, actions, handoff, reply}   ← the decision is parsed the moment "reply" starts
 5. POLICY  validate IDs, urgent first, confidence thresholds → proceed | clarify | handoff | out_of_scope
 6. EXECUTE topic stack, slot merge, read-only actions now (+ one follow-up call if the reply needs
            their results), irreversible actions → PREVIEW (pending confirmation), handoff → queue
 7. SPEAK   reply sentences → TTS stream → browser (first sentence plays while the rest is generated)
 8. TRACE   everything above + timings → panel, JSONL store, supervisor stats, debug event stream
```

The server enforces the safety rule: `create_policy`, `cancel_policy`, `create_claim`, `book_appointment`, … are executed **only** when the previous turn previewed exactly that action and the client's current utterance is a confirmation.

## 4. Tech stack

- **Backend:** Go 1.26, `chi` router, `coder/websocket`; no database (in-memory sessions + JSONL trace log in `var/`).
- **Frontend:** Next.js 16 (App Router, TypeScript), Tailwind CSS v4, `lucide-react`; Web Audio worklet capture, energy VAD, PCM streaming playback, Web Speech API fallback.
- **AI:** any OpenAI-compatible chat API for the router (OpenAI `gpt-4.1-mini` default; Groq / OpenRouter / Ollama via `LLM_BASE_URL`); STT: ElevenLabs Scribe v2 realtime + batch, OpenAI `gpt-4o-mini-transcribe`, geko Seta (`seta-kk-ru-v2`, OpenAI-compatible endpoint); TTS: ElevenLabs Flash v2.5 (+ `eleven_v3` for Kazakh), OpenAI `gpt-4o-mini-tts`, geko Tokay (`tokay-kk-v1`).
- Third-party components: [THIRD_PARTY.md](THIRD_PARTY.md). Design/contract docs: [docs/SPEC.md](docs/SPEC.md).

## 5. Architecture

```
frontend (Next.js :3000)  ── REST ──►  backend (Go :8080)
   Call · Supervisor · Eval            ├─ internal/triage      deterministic layer 1
   Catalog · Debug                     ├─ internal/retrieval   BM25 shortlist + config/lexicon.json
   ◄── WS: events + PCM audio ──       ├─ internal/router      prompt, stream parser, policy, mock router
                                       ├─ internal/dialog      engine: state, facts, executor, TTS streaming
                                       ├─ internal/mockbackend actions.json over mock_backend.json + KB
                                       ├─ internal/stt, tts    providers (ElevenLabs, OpenAI-compatible, mock/browser)
                                       ├─ internal/store       sessions + traces.jsonl + supervisor stats
                                       └─ internal/eval        dev-set evaluation (same metrics as evaluate.py)
```

## 6. Requirements

- Docker 24+ with Compose v2 — **or** Go 1.26+, Node.js 20+ and Python 3 (scripts), `ffmpeg` only to regenerate test audio.
- **No API keys are needed to run and review**: without keys the backend uses the deterministic lexical router and the browser's speech recognition/synthesis (Chrome). Keys unlock the LLM router and server-side STT/TTS (see §8).

## 7. Install and run

```bash
git clone https://github.com/BAITC-Hacks/hack-9dd7b5f3-plus.git
cd hack-9dd7b5f3-plus
cp .env.example .env        # works as-is (keyless); add keys for the LLM path
docker compose up --build   # backend http://localhost:8080, frontend http://localhost:3000
```

Without Docker (two terminals, or `make dev`):

```bash
cd backend && go run ./cmd/server          # reads ../.env, serves :8080, data from ../data
```

```bash
cd frontend && npm ci && npm run dev       # http://localhost:3000 (API at <host>:8080 by default)
```

Ports busy? `BACKEND_PORT=8090 FRONTEND_PORT=3001 docker compose up --build` (then open :3001 and set `NEXT_PUBLIC_API_URL=http://localhost:8090` for the frontend build).

### Recommended keys for the full experience
`OPENAI_API_KEY` (router `gpt-4.1-mini`, STT `gpt-4o-mini-transcribe`, TTS `gpt-4o-mini-tts`) **and/or** `ELEVENLABS_API_KEY` (realtime STT with partial transcripts — the lowest-latency path — and Flash v2.5 TTS). With both set the defaults are: LLM OpenAI, STT ElevenLabs realtime, TTS ElevenLabs (the LLM router turns on automatically once `OPENAI_API_KEY` or `LLM_API_KEY` is set).

## 8. Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `LLM_PROVIDER` | auto | empty = `openai` when a key is set, else `mock` (lexical, keyless); can be set explicitly |
| `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` | OpenAI / `OPENAI_API_KEY` / `gpt-4.1-mini` | router model; e.g. Groq `https://api.groq.com/openai/v1` + `llama-3.3-70b-versatile`, Ollama `http://localhost:11434/v1` |
| `STT_PROVIDER` | auto | `browser` · `mock` · `elevenlabs_realtime` · `elevenlabs` · `openai` (also geko Seta / whisper servers via `STT_BASE_URL`) |
| `STT_MODEL`, `STT_LANGUAGE`, `STT_SECONDARY_LANGUAGES` | per provider | model id, language hint (empty = auto), extra languages for realtime |
| `TTS_PROVIDER` | auto | `browser` · `elevenlabs` · `openai` (also geko Tokay via `TTS_BASE_URL`) |
| `TTS_MODEL`, `TTS_VOICE`, `TTS_MODEL_KK`, `TTS_VOICE_KK` | per provider | voice per language (Kazakh override, e.g. `eleven_v3`) |
| `FAST_PATH` | `on` | `on` / `shadow` / `off`; `FAST_PATH_MIN_SCORE=0.75`, `FAST_PATH_MIN_MARGIN=0.35` |
| `POLICY_PROCEED_MIN` / `POLICY_CLARIFY_MIN` | `0.55` / `0.30` | confidence thresholds of the decision policy |
| `DEBUG` | `true` | include prompts and raw model output in traces and the debug stream |
| `DATA_DIR`, `VAR_DIR`, `LEXICON_PATH`, `PORT`, `CORS_ORIGINS` | `../data`, `./var`, `config/lexicon.json`, `8080`, `*` | paths and server |
| `NEXT_PUBLIC_API_URL` | empty | frontend → API base; empty = same host, port 8080 |

Full list with comments: [.env.example](.env.example).

## 9. How to verify

1. **Talk to it.** Open the frontend → *Call*. Press the mic (auto VAD) or hold it (push-to-talk) and say e.g. «Здравствуйте, я вчера оплатил полис, деньги списались, а полиса нет… и ещё адрес поменять надо». Expected: two scenarios `SC30` (payment issue) then `SC29` (contact update), a spoken reply in Russian, the trace panel with reasoning, alternatives, and the timing waterfall. Then say «Сәлеметсіз бе, полисім жарамды ма?» → `SC25`, reply in Kazakh. Then give a phone from the kit («телефон плюс семь семьсот один ноль ноль ноль ноль ноль ноль семь») → the client is identified and the robot uses their data.
2. **Text fallback:** type into the input box; the same pipeline runs.
3. **Accuracy on the official dev set:** with the backend running: `python3 scripts/eval.py` (prints the `data/evaluate.py` table + latency percentiles; also *Eval* page → *Run evaluation*).
4. **Audio tests:** `python3 scripts/audio_test.py` plays [`tests/audio/*.wav`](tests/audio/) (3 recordings: simple RU, RU topic switch, mixed KK/RU) through STT → router → reply and checks the expected scenarios; `--save-replies` stores the spoken answers in `tests/audio/out/`. Keyless: the recording's transcript is passed as a hint so routing still runs for real.
5. **Watch it think in real time:** *Debug* page, or in a terminal: `curl -N http://localhost:8080/api/debug/events` — every step (triage, retrieval candidates, prompt, model tokens, decision, actions, TTS) is an event. `GET /api/prompt` shows the exact system prompt. `python3 scripts/demo_dialog.py` plays seven scripted multi-turn dialogues (identification, topic switch and return, confirmation of an irreversible action, clarification, fraud, mixed language, operator) and prints the state after every turn; `node scripts/ws_smoke.mjs tests/audio/02_ru_topic_switch.wav` streams a recording through the voice WebSocket exactly like the browser does and prints the events with timings (`--text "…"` for a text turn, `--save reply.pcm` to keep the audio).
6. **Supervisor:** *Supervisor* page — accuracy of the fast path vs the LLM shadow check, low-confidence turns, handoffs, latency p50/p90 by stage and by path; drill into any session.
7. **Unit/integration tests:** `cd backend && go test ./...` (language detection, spoken numbers, mock backend prices, stream parser, lexical baseline ≥ 0.85, engine with a fake streaming LLM incl. follow-up actions and the confirmation gate).

### Latency
Measured server-side per stage and shown in the trace; `route` = end of speech → decision parsed, `first_audio` = end of speech → first audio byte sent. The browser additionally measures end of speech → first audio sample played. Keyless routing takes ~1 ms; with an LLM the decision typically lands at TTFT + ~40 tokens, and the first sentence is spoken while the rest of the reply streams. Real numbers depend on the provider and network — the panel shows them honestly, and the fast path shows its Δ against the LLM path in the supervisor statistics.

## 10. Data and integrations

- Official synthetic starter kit in [`data/`](data/) (read-only): 40 scenarios + 3 system intents, slots, actions, knowledge base, mock backend (11 clients, 11 policies, 4 claims, 2 payments), 10 sample dialogs, 104 dev utterances, `evaluate.py`. No real personal data anywhere; nothing personal is sent to external APIs beyond the synthetic utterances.
- Editable routing vocabulary: [`backend/config/lexicon.json`](backend/config/lexicon.json) (RU/KK concept variants + localized scenario names for clarifying questions) — vocabulary normalization, not a scenario mapping.
- External APIs (all optional): OpenAI, ElevenLabs, geko.sh; any OpenAI-compatible LLM/STT/TTS endpoint.

## 11. Limitations (honest)

- The keyless mode is a baseline: the lexical router has no dialogue understanding beyond continuation/confirmation rules, and browser STT (Chrome) recognizes one preset language at a time — mixed phrases are best with a server STT.
- Latency targets are targets: STT finalization and model TTFT dominate; the realtime STT path and the fast path are the two levers, and the numbers are shown, not promised.
- Kazakh TTS: ElevenLabs Flash v2.5 has no Kazakh voice (a Russian voice reads Kazakh Cyrillic); use `TTS_MODEL_KK=eleven_v3`, OpenAI TTS, or geko Tokay for proper Kazakh.
- The mock backend is in-memory; sessions live in memory (traces are persisted to `var/traces.jsonl`). No auth on the supervisor endpoints (hackathon scope).
- Emotion detection and a phone (SIP) channel are not implemented.

## 12. Team Plus
- Tair Kaldybayev — captain, product / AI
- Alikhan — backend
- Ramazan — frontend
