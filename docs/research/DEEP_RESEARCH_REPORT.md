# Deep research report — Voice Router

> Status: research snapshot (23 Sep 2026). Findings marked ASSUMPTION must be verified against live docs before claiming them to the jury. Source (RU): docs/research/raw/deep-research-output.ru.md. Derived docs: docs/REQUIREMENTS.md, docs/PRD.md, docs/SPEC.md, docs/TASKS.md, docs/PITCH.md.

## TL;DR
- **We're building "Bagyt" (Бағыт)** — a web simulator of a voice robot where the choice among 40 scenarios is made by an **LLM layer** (retrieval shortlist + a generative JSON router in the style of Rasa CALM command generation), **NOT** an encoder-based intent classifier. RU/KZ voice with mid-phrase language switching is handled by **geko.sh** (Seta STT / Tokay TTS), with **ElevenLabs Scribe v2 Realtime** as fallback.
- **Speko is cut from the core**: it benchmarks and routes ~10 languages, and **neither Kazakh nor Russian is among them** — so it's useless for the KZ/RU path (kept only as optional / for the design system). A "custom routing model" is also cut: per the spec it gives no advantage, and encoder-based classifiers are prohibited.
- **The 500ms scenario-selection target** is not reliably achievable (a single cloud LLM's TTFT is already ~0.45–0.6s) — we measure honestly by stage, show it in the UI, and claw back speed with speculative routing on partial transcripts. **Telephony is a stretch goal only, after P0.**

## Key findings

### STT/TTS providers
- **geko.sh is the deciding provider for KZ/RU.** A single STT model, **Seta-1.0 (seta-kk-ru-v2)**, understands Kazakh and Russian "in one vocabulary," with switching "mid-sentence, and inside a word"; claimed **WER 8.71%** vs. **Whisper 44.95%** on the same Kazakh audio, "40× faster than real time." TTS **Tokay-1.0 (tokay-kk-v1)** — WAV 24kHz 16-bit PCM mono, sentence-by-sentence streaming, speaks numbers as words (important for insurance amounts). Endpoints are **OpenAI-compatible** (`/v1/audio/speech`, `/v1/audio/transcriptions`, `/v1/transcribe`, `/v1/tts/stream`) — "just change the base URL and model." Pilots are already live at Kcell, Beeline, Oneshott Coffee. Downsides: GPU backends scale-to-zero → possible cold start (magnitude is an **ASSUMPTION**, verify); the number of Kazakh Tokay voices shown in the demo needs confirming against the data sheet (**ASSUMPTION** — not explicitly confirmed on the homepage).
- **ElevenLabs Scribe v2 Realtime** — streaming STT with a claimed latency of ~150ms; Kazakh and Russian fall in the High Accuracy category (>5%…≤10% WER); a good primary for latency and a fallback to geko.

### Routing approach
- **A router with no encoder.** Retrieval (embeddings of examples from scenarios.json) only **shortlists** candidates — this is not classification. The final choice and its justification are made by a generative LLM that outputs a strict JSON/commands — exactly the Rasa **CALM CommandGenerator** approach ("intentless," driven by full dialogue context). This is the required "LLM layer."

### LLM choice
- **LLM for routing.** **gpt-4.1-nano** — in an independent eval (MemX, 2026), 100% valid JSON-schema output, P50 ~1.25s, P99 3.4s, cheaper than gpt-4o-mini; supports structured output via json_schema/function-calling. OpenAI-class TTFT is ~0.45–0.6s (EdenAI 2026: GPT-4o TTFT p50 ~450ms). For a "fast tier" — **Groq/Cerebras** (TTFT ~80–150ms on warmed endpoints, but with risk to KZ/RU quality and no guarantee on cold starts). Alternative — **Gemini 2.5 Flash-Lite** ($0.10/$0.40 per 1M tokens, optimized for classification/translation/low latency).

### Speko
- **Speko** (YC S26, "OpenRouter for Voice," founder Bek): base `router.speko.dev` (the open-source gateway also uses `relay.speko.dev` — **inconsistency, verify**); contract `POST /v1/stt/transcriptions`, `/v1/tts/speech`, `/v1/llm/responses`; intent set via `optimizeFor=balanced|accuracy|latency|cost` (in the gateway — `objective=quality|latency|cost|balanced`), `language`, `region`; `provider:model` pinning; response in headers `Speko-Provider/Model/Region/Attempt-ID`; official SDKs `@spekoai/sdk` and `spekoai` (PyPI); adapters for LiveKit/Pipecat/Vapi/Retell. **But KZ/RU are not among its benchmarked languages** (published set: EN, AR, FR, DE, HI, NB, ES, TA, TE + one unlisted) → not usable for our core.

### VAD/barge-in
- **VAD/barge-in.** The standard is **Silero VAD** (~32ms frames) for endpoint detection and barge-in; gate barge-in on VAD, not on STT partials (STT is too slow to serve as a trigger). WebRTC VAD is a lightweight fallback.

### Speculative routing
- **Speculative routing.** Kicking off the route pipeline on a stable partial transcript (STT confidence >0.85) with cancellation of stale calls yields −200…400ms P95 (Pipecat/Futureagi 2026); this effectively hides scenario-selection latency by the time the utterance ends.

### Telephony
- **Telephony.** The fastest path today is **Vapi + a Twilio number** (porting/buying a number in an available country) or **LiveKit SIP inbound** with our `/api/route` as the "brain." A KZ **+7** number with KYC can't be arranged within the build window (**ASSUMPTION**) — for the demo, use a non-KZ number, or leave it as an architectural plan in the README.

## Recommendations
1. **Right now (13:50–14:00):** lock the router's JSON schema and stand up the skeleton (three services + `/api/health` + docker compose). Don't argue about the stack — the default Go/chi + Next.js is fine; a separate Python service for voice is **not needed** (geko/ElevenLabs are HTTP/WS APIs, callable directly from Go).
2. **By 15:00:** a working `POST /api/route` (retrieval shortlist + gpt-4.1-nano JSON) + a MOCK router. This is the core of the evaluation (25+25 points) — top priority.
3. **By 16:00:** close all 5 must-haves (end-to-end voice, trace, RU+KZ, evaluate.py accuracy). After that — freeze P0 and don't touch it.
4. **16:00–17:00:** P1 value (topic stack for the official example, clarify/handoff, supervisor panel). This is what distinguishes it from a "single-scenario demo."
5. **Telephony — only if P0+P1 are stable by 17:00** and there's 30–45 minutes: Vapi+Twilio number as the fastest path. Otherwise — an architectural plan in the README, without implementation.

**Thresholds that change the plan:**
- If there's no valid JSON from the router by 15:00 → drop fast-path/speculative entirely, run only the LLM path.
- If geko shows a cold start >5s during the demo → switch primary to ElevenLabs Scribe v2 RT for Russian, keep geko only for pure Kazakh.
- If Groq/Cerebras give poor KZ/RU quality → don't use the fast tier for the decision, only gpt-4.1-nano/gpt-4o.
- If Speko does turn out to support KZ/RU in live docs (currently not the case) → it could be added as an STT/TTS router, but not on the critical path before freeze.

## Caveats
- **500ms for scenario selection** — a marketing target from the brief; not stably achievable with a cloud LLM (TTFT alone is ~0.45–0.6s). Don't promise the jury "we hit 500ms" without a real measurement; show measured numbers and speculative routing as a way to hide the latency.
- **geko cold start** — GPU scale-to-zero is confirmed on the site ("A first call can take tens of seconds"); the exact figure for our traffic is an **ASSUMPTION**. Keep a warmup ping and warm it up before the demo.
- **Number of Kazakh Tokay voices (6)** — not explicitly confirmed on geko's homepage (matches the open KazakhTTS2 dataset); verify against the data sheet before stating it in the pitch — **ASSUMPTION**.
- **Speko host** — `router.speko.dev` (website) vs. `relay.speko.dev` (open-source gateway) diverge; and `optimizeFor` (SDK) vs. `objective`/`quality` (gateway). Clarify in live docs if you decide to integrate it. Speko is **not** OpenAI-compatible as a drop-in (native `/v1/llm/responses`).
- **KZ +7 number with KYC** within the build window — **ASSUMPTION** that it can't be done in time; hence telephony only on a non-KZ number or as a plan.
- **LLM/Groq/Cerebras latency figures** are taken from third-party 2026 benchmarks (EdenAI, ArtificialAnalysis, MemX) and depend on endpoint warmth and region — verify with your own measurements rather than quoting others' numbers in the README as your own.
- **ElevenLabs Scribe** for KZ/RU — "High Accuracy" category (not top-tier); for pure Kazakh, geko Seta is stronger by its claimed WER — so the primary/fallback order may invert depending on what matters more for the demo (latency vs. KZ accuracy).

## Proposed third-party components

| Component | Type | License/terms | Link | Used for |
|---|---|---|---|---|
| voice_router_dataset.zip (organizer starter kit: scenarios.json, dialogs_sample.json, knowledge_base.json, mock_backend.json, dev_utterances.json, evaluate.py) | Data | Hackathon organizer's license | — | Synthetic scenarios/dialogues/facts |
| geko.sh — Seta-1.0 (seta-kk-ru-v2) | STT API | External key (GEKO_API_KEY), not in repo | geko.sh | KZ/RU code-switching speech-to-text |
| geko.sh — Tokay-1.0 (tokay-kk-v1) | TTS API | External key (GEKO_API_KEY), not in repo | geko.sh | KZ (and RU) text-to-speech |
| ElevenLabs — Scribe v2 Realtime | STT API | External key (ELEVENLABS_API_KEY), not in repo | elevenlabs.io | Streaming speech-to-text (primary/fallback) |
| ElevenLabs — Flash v2.5 | TTS API | External key (ELEVENLABS_API_KEY), not in repo | elevenlabs.io | Text-to-speech |
| OpenAI — gpt-4.1-nano | LLM API | External key (OPENAI_API_KEY), not in repo | openai.com | Route decision (JSON output) |
| OpenAI — text-embedding-3-small | Embeddings API | External key (OPENAI_API_KEY), not in repo | openai.com | Shortlist retrieval |
| OpenAI — gpt-4o | LLM API | External key (OPENAI_API_KEY), not in repo | openai.com | Escalation on low confidence |
| Speko | Voice model router (optional, not on the KZ/RU path) | External key (SPEKO_API_KEY), not in repo | router.speko.dev / relay.speko.dev | Not used for KZ/RU; optional/design-system only |
| Silero VAD | Voice activity detection | Open source, MIT | — | End-of-utterance detection / barge-in |
| Go, chi | Backend language/router library | Open source | — | Backend service |
| pgx | Postgres driver | Open source | — | Database access |
| Next.js, TypeScript, Tailwind, shadcn/ui | Frontend framework/libraries | Open source | — | Frontend UI |
| Speko design system | Design system | Provided by Speko's founder | — | UI styling |
| Docker, docker compose | Infra tooling | Open source | — | Local orchestration |
| Postgres | Database | Open source | — | Persistence |
| Railway | Deployment platform | Vendor platform | — | Deploy |
| Vapi / Twilio / LiveKit SIP | Telephony (P2/stretch) | Vendor platform | — | Inbound/outbound phone channel, if implemented (record number/provider here) |

Note: secrets are kept only in `.env` (gitignored); no keys are committed to the repository.
