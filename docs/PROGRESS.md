# Progress log (hourly proof — rules §5.4.8)

## 13:00–14:00
- Read all track cases; picked Halyk Bank **Voice Router** (Track 09 Communications). Official task saved to `docs/CASE.md`, starter kit unpacked into `data/`.
- Backend (Go, `GET /health`) and frontend (Next.js) skeletons pushed (Alikhan / Ramazan).
- Deep research launched on providers (STT/TTS for KZ/RU), routing approach and latency.

## 14:00–15:00
- Research merged into English living docs: `CASE_ANALYSIS`, `REQUIREMENTS`, `PRD`, `SPEC`, `TASKS`, `PITCH`, `research/DEEP_RESEARCH_REPORT`; rules digest translated.
- Docs reorganized (`docs/README.md` index; `research/`, `design/`, `hackathon/`); AGENTS.md, README, THIRD_PARTY, `.env.example` updated for Voice Router.

## 15:00–16:00
- Backend implemented end to end (Go): triage → lexical retrieval → facts → fast path / LLM router (streaming JSON, decision before reply) → policy → executor (topic stack, confirmation gate) → sentence-streamed TTS → trace. Providers: OpenAI-compatible LLM/STT/TTS, ElevenLabs realtime + batch STT and streaming TTS, keyless mock/browser mode.
- REST + WebSocket + SSE API (`docs/SPEC.md`), `scripts/eval.py` (official `evaluate.py` wrapper), `scripts/audio_test.py` + 3 test recordings, `scripts/ws_smoke.mjs`.
- Keyless lexical baseline on the dev set: 90.4 % primary accuracy (from a first 77.9 %), fast path 5/5 precise. Go tests: parser, triage, backend prices, engine with a fake streaming LLM, provider clients against fake servers.
- Docker image for the backend builds and runs; README/THIRD_PARTY/SPEC rewritten to match the implementation.

## 16:00–17:00
- Frontend (Next.js 16): Call simulator with mic/VAD/push-to-talk, trace panel with timing waterfall, Supervisor, Eval, Catalog, Debug pages.
- Docker Compose end-to-end check, README verification from a clean clone.

## 17:00–18:00
- 
