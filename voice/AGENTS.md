# AGENTS.md — voice module (for Claude Code / Codex integrating it)

Read `voice/README.md` first. This file tells an AI agent how to plug the voice layer into the rest of the repo **without editing it**.

## What this module is

`voice/` is a self-contained Go module (`hackathon/voice`) owned by Tair: ElevenLabs speech-to-text and text-to-speech (RU/KZ/mixed), a real-time conversation engine, a browser voice gateway, and phone transports (Asterisk AudioSocket for a Kazakhstan number, Twilio). It never imports `backend/` or `frontend/`, and they don't need to import it.

## Integration contract (preferred: run it as a service)

```
browser mic ──WS /ws/voice──► voice gateway ──POST /api/turn (SSE)──► backend (Ramazan: routing + reply)
phone (KZ SIP → Asterisk) ──AudioSocket──┘      │
                                                ├─ Scribe v2 Realtime (STT, language_code=kk)
                                                └─ ElevenLabs TTS (Flash v2.5 ru / v3 conversational kk)
```

1. **Backend (Ramazan)**: implement the frontend contract `frontend/src/lib/contract.ts`:
   - `POST /api/session` → `{"session_id": "..."}`
   - `POST /api/turn` with `{"session_id","text","lang_hint":"ru|kk","client_t0":<epoch ms>,"tts":false}` → `text/event-stream`, one JSON `TurnEvent` per `data:` line.
   - The gateway speaks `response.delta` events as they arrive, so **stream the reply**: emit `router.decision` as soon as routing is done, then `response.delta` chunks, then `response.final` and `turn.done`.
   - `tts:false` means the gateway does the speech: don't synthesize audio in the backend for these requests.
   - Set `BACKEND_URL=http://<backend>:8080` for the gateway (`VOICE_BRAIN=backend` forces it; `auto` falls back to the built-in OpenRouter brain when the backend is down).
   - Compatibility: `/api/turns/stream` NDJSON (`routing_complete`, `policy.reply`) is also understood.
2. **Frontend (Alikhan)**: use the gateway WebSocket for real voice. `voice/web/index.html` is the reference client (mic capture at 16 kHz, gapless playback, barge-in via `tts.clear`).
   - Client sends `{"type":"start","mode":"ptt"|"vad","greeting":bool,"lang":""}`, binary PCM16 16 kHz frames, and `{"type":"commit"}` on push-to-talk release.
   - Server sends the same `TurnEvent` names as the contract (`stt.partial`, `stt.final`, `router.decision`, `response.delta`, `response.final`, `turn.done`), plus `tts.start`, binary PCM16 audio, `tts.clear` and `voice.metrics` (stt / response / tts_first_audio / total ms).
   - When the brain is the backend, the backend's own events are relayed unchanged, so the supervisor console keeps working.
3. **Admin/supervisor**: `GET /api/voice/events` (server-sent events: every stage of every call, web and phone), `GET /api/voice/stats` (p50/p95 latency, share under 1.5 s), `GET /api/voice/sessions`.
4. **Simple HTTP helpers** (no WebSocket): `POST /api/voice/tts {"text","lang","format"}` streams audio; `POST /api/voice/stt` (a whole recording as the body, e.g. webm/opus) returns `{"text","language"}`.

## Embedding as a Go library (only if you really need it)

`backend/go.mod`: `require hackathon/voice v0.0.0` + `replace hackathon/voice => ../voice` (or `go work init ./backend ./voice`). Build the Docker image with the repo root as context. Use `speech.ElevenSTT`, `speech.ElevenTTS`, `speech.NewChunker`, `lang.Policy`; see the README snippet.

## Rules for agents

- Don't edit `voice/` from another owner's task. Tell Tair if the contract has to change, and keep `README.md`/`AGENTS.md` in sync with any change.
- Never commit keys. `config` reads the repo-root `.env` (canonical `ELEVENLABS_API_KEY`, `OPENROUTER_API_KEY`; aliases `eleven-labs`, `openrouter-api` also work).
- Keep `ELEVENLABS_STT_LANGUAGE=kk`: auto-detect transcribes Kazakh as Turkish (measured, see `demos/RESULTS.md`).
- Tests: `cd voice && go test ./...` (offline). Live: `VOICE_LIVE=1 go test ./elevenlabs/ -run Live -v`, `go run ./cmd/voicedemo tts|stt`.
- Run the gateway: `cd voice && go run ./cmd/voice` → http://localhost:8090 (voice test page).
