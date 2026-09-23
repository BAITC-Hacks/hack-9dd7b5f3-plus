# Requirements checklist — Voice Router

> Status: living draft — not final; update as decisions change. docs/CASE.md (official task) wins on conflict, then SPEC, then PRD. Source (RU): docs/research/raw/deep-research-output.ru.md

Translated from `docs/CASE.md` — requirements checklist for Halyk Bank / Case 2 "Voice Router".

## Must-have (mandatory) → our implementation
- [x] **Voice interaction in the web app** (the jury speaks into a microphone, the robot answers by voice) → browser mic capture (WebAudio, PCM16 16 kHz) over WebSocket to the Go backend; TTS response streamed back to the browser. Owner: Ramazan+Alikhan.
- [x] **Scenario selection happens at the LLM layer, and the layer's inner workings must be shown; an encoder-based intent classifier does NOT count** → retrieval shortlist (embeddings, NOT classification) + generative LLM with JSON output; the UI shows the candidates, the prompt, and the decision. Owner: Tair.
- [x] **Correctness** (10 jury utterances: simple, topic switch, mixed speech, boundary cases) → `/api/route` endpoint + `evaluate.py`; hit rate shown in the UI. Tair.
- [x] **Trace panel after every utterance** (scenario, reasoning, alternatives, per-stage timing) → `TracePanel` component + `stage_timings` table. Ramazan.
- [x] **Russian and Kazakh, including mid-phrase language switching** → geko Seta (KZ/RU single vocabulary) / ElevenLabs Scribe v2 Realtime; response language follows the client's language. Alikhan+Tair.

## Optional → keep / cut
- [x] Hybrid approach (fast-path for obvious cases + LLM for hard ones, measurable gain) → KEEP as P1, show the latency Δ.
- [x] Context retention and returning to an interrupted topic (topic stack) → KEEP P1 (needed for the "and also change the address" example).
- [x] Ask a clarifying question instead of guessing + handoff to an operator with context → KEEP P1 (action: clarify|handoff).
- [x] Extracting parameters from speech → KEEP P1 (`extracted_params` in the JSON).
- [x] Supervisor panel with error statistics → KEEP P1.
- [~] Hitting 500ms / streaming processing / speculative routing → P2 (measure it + speculative on partial transcripts).
- [~] Emotion and tone adaptation → P2, likely cut.
- [~] Editing the catalog without developers → P2, if time remains (read `scenarios.json` from the DB).
- [ ] Phone channel (outbound/inbound) → P2 stretch, only after P0.

## Forbidden → how we comply
- Selecting the scenario with an off-the-shelf intent classifier → we do NOT use a DIET/BERT-style classifier for the decision; embeddings are used only for the shortlist.
- Hardcoding the mapping between the jury's test utterances and scenarios → the router reads `scenarios.json` dynamically; no test utterances live in the code; embeddings are built from `scenarios.json` at startup.
- Real conversation recordings → only synthetic data / organizer-provided data (`voice_router_dataset.zip`).

## Must keep in mind
Synthetic data; irreversible actions require explicit client confirmation; explainability; single-command startup; the LLM must sit at the point of substantive decision-making; AI is not the sole source of truth (we surface uncertainty); mandatory handoff to an operator; not a "black box"; not a demo built around a single scenario.

## Scoring criteria (100)
- Task fit and functionality — 25
- Technical implementation — 25
- README and reproducibility — 25
- Value and applicability — 15
- Growth potential and originality — 10
- Demo Day (later): value 25, result 20, innovation 15, scale 20, pitch 20.
