# Third-party components (disclosure per rules §5.4.4)

Every dependency, model, API, dataset, template or UI kit we use is listed here.

## In use

| Component | License | Link | Used for |
|-----------|---------|------|----------|
| Voice Router starter kit (`data/`: scenarios, slots, actions, knowledge base, mock backend, dialogs, dev utterances, `evaluate.py`) | Provided by the organizer / Halyk Bank for HackAlem AI | https://drive.google.com/file/d/1sHE56gXnzdscHz5lMcNbwd1VsJIVLFUv | Synthetic scenarios, dialogs, facts; official evaluation script |
| Go (stdlib) | BSD-3-Clause | https://go.dev | Backend service |
| go-chi/chi v5 | MIT | https://github.com/go-chi/chi | HTTP router |
| coder/websocket | ISC | https://github.com/coder/websocket | WebSocket server (browser audio) and client (ElevenLabs realtime STT) |
| Next.js | MIT | https://nextjs.org | Frontend framework |
| React | MIT | https://react.dev | UI |
| Tailwind CSS v4 | MIT | https://tailwindcss.com | Styling |
| lucide-react | ISC | https://lucide.dev | Icons |
| Geist / Geist Mono (via next/font) | SIL OFL 1.1 | https://vercel.com/font | Typography |
| OpenAI API — chat completions (`gpt-4.1-mini` default), `gpt-4o-mini-transcribe`, `gpt-4o-mini-tts` | Commercial API (key not in repo) | https://platform.openai.com | LLM router / dialogue, STT, TTS (optional) |
| ElevenLabs — Scribe v2 (batch) and Scribe v2 Realtime (STT), Flash v2.5 / v3 (TTS) | Commercial API (key not in repo) | https://elevenlabs.io | Streaming STT with partial transcripts, TTS (optional) |
| geko.sh — Seta-1.0 (`seta-kk-ru-v2`) STT, Tokay-1.0 (`tokay-kk-v1`) TTS | Commercial API (key not in repo) | https://geko.sh | Kazakh/Russian code-switching STT and Kazakh TTS through their OpenAI-compatible endpoints (optional) |
| Any OpenAI-compatible endpoint (Groq, OpenRouter, Ollama, speaches/whisper, …) | Vendor terms | — | Alternative LLM / STT / TTS providers via `*_BASE_URL` (optional) |
| Web Speech API (`webkitSpeechRecognition`, `speechSynthesis`) | Browser built-in | https://developer.mozilla.org/docs/Web/API/Web_Speech_API | Keyless STT/TTS fallback in the browser |
| macOS `say` + ffmpeg | Apple / LGPL-2.1+ | https://ffmpeg.org | Generating the synthetic test recordings in `tests/audio/` |
| Docker / Docker Compose | Apache-2.0 | https://www.docker.com | One-command local run |
| Design references (Speko brand + console, 1609SAT `DESIGN.md`) | Provided by their authors for this project | `docs/design/` | UI styling reference |

## Not used (evaluated during research)

shadcn/ui, ObsidianUI, Motion, pgx/PostgreSQL, Silero VAD (an energy VAD in the browser is used instead), Speko, Railway, Vapi/Twilio/LiveKit.

AI coding assistants used during development: OpenAI Codex, Claude.
