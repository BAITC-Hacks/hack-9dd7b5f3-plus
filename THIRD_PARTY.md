# Third-party components (disclosure per rules §5.4.4)

Every dependency, model, API, dataset, template or UI kit we use must be listed here. When something moves from "Planned" to actually used in code, move its row to "In use".

## In use

| Component | License | Link | Used for |
|-----------|---------|------|----------|
| Voice Router starter kit (`data/`: scenarios, slots, actions, knowledge base, mock backend, dialogs, dev utterances, `evaluate.py`) | Provided by the organizer / Halyk Bank for HackAlem AI | https://drive.google.com/file/d/1sHE56gXnzdscHz5lMcNbwd1VsJIVLFUv | Synthetic scenarios, dialogs, facts; official evaluation script |
| Next.js | MIT | https://nextjs.org | Frontend framework |
| Tailwind CSS | MIT | https://tailwindcss.com | Styling |
| coss ui (53 primitives, `@coss/style` preset) | MIT | https://coss.com/ui | UI components (Button, Badge, Card, Table, Tabs, Switch, Input, …) — installed via shadcn CLI into `frontend/src/components/ui` |
| Base UI (`@base-ui/react`) | MIT | https://base-ui.com | Accessible primitives under coss ui |
| ObsidianUI (`dotted-grid`, `text-stream` blocks) | MIT | https://www.obsidianui.dev | Landing page background / text stream — `frontend/src/components/block` |
| GSAP | Standard "no charge" license (free for use incl. commercial) | https://gsap.com | Dependency of ObsidianUI text-stream |
| lucide-react | ISC | https://lucide.dev | Icons |
| class-variance-authority, clsx, tailwind-merge | Apache-2.0 / MIT / MIT | npm | Class utilities used by coss ui |
| Hanken Grotesk, Geist Mono (Google Fonts via `next/font`) | SIL OFL 1.1 | https://fonts.google.com | Typography (Speko type system) |
| Speko design system (brand + console PDFs) | Provided by Speko's founder for this project | `docs/design/references/` | Visual design tokens, component sizes |
| Web Speech API / speechSynthesis (browser) | Browser built-in | — | Keyless STT/TTS in mock mode (Chrome) |
| ElevenLabs STT/TTS — Scribe v2 / Flash v2.5 / v3 conversational | Commercial API terms | https://elevenlabs.io | Russian/Kazakh microphone transcription and voice replies via imported voice module |
| github.com/coder/websocket | ISC | https://github.com/coder/websocket | Imported voice module WebSocket dependency (frontend uses HTTP TTS only) |
| Python standard library | PSF License | https://www.python.org | core-llm HTTP adapter and router |
| OpenRouter / Google Gemini 2.5 Flash Lite | Commercial API terms | https://openrouter.ai/google/gemini-2.5-flash-lite | LLM scenario routing |
| OpenAI transcription API | Commercial API terms | https://platform.openai.com | Existing server STT fallback |
| Node.js and official Node/Python Docker images | MIT / PSF and bundled OS licenses | https://hub.docker.com/_/node · https://hub.docker.com/_/python | Reproducible frontend/core runtime |
| Go (stdlib `net/http`) | BSD-3-Clause | https://go.dev | Backend service |

## Planned (from `docs/SPEC.md` / `docs/research/DEEP_RESEARCH_REPORT.md` — confirm when integrated)

| Component | License / terms | Link | Used for |
|-----------|-----------------|------|----------|
| chi | MIT | https://github.com/go-chi/chi | Go HTTP router |
| pgx | MIT | https://github.com/jackc/pgx | Postgres driver |
| PostgreSQL | PostgreSQL License | https://www.postgresql.org | Database |
| geko.sh — Seta-1.0 (`seta-kk-ru-v2`) STT, Tokay-1.0 (`tokay-kk-v1`) TTS | Commercial API (key not in repo) | https://geko.sh | KZ/RU code-switching STT; Kazakh TTS |
| ElevenLabs — Scribe v2 Realtime (STT), Flash v2.5 (TTS) | Commercial API (key not in repo) | https://elevenlabs.io | Streaming STT / TTS, fallback to geko |
| OpenAI API — `gpt-4.1-nano` / `gpt-4.1-mini` / `gpt-4o`, `text-embedding-3-small` | Commercial API | https://platform.openai.com | Router LLM (JSON), reply generation, low-confidence escalation, shortlist embeddings |
| NVIDIA Build (NIM) API | Commercial API | https://build.nvidia.com | Alternative LLM provider |
| Silero VAD | MIT | https://github.com/snakers4/silero-vad | End-of-utterance detection / barge-in |
| Speko (optional, not on the KZ/RU path) | Commercial API | https://speko.ai | Voice provider routing — optional only |
| Design references (Speko brand + console, 1609SAT `DESIGN.md`) | Provided by their authors for this project | `docs/design/` | UI styling reference |
| Railway | Vendor platform | https://railway.app | Deployment |
| Docker / Docker Compose | Apache-2.0 | https://www.docker.com | Local orchestration |
| Vapi / Twilio / LiveKit SIP (stretch) | Vendor platforms | https://vapi.ai · https://twilio.com · https://livekit.io | Phone channel, only if implemented |

AI coding assistants used during development: OpenAI Codex, Claude.

| Official Go, Alpine and PostgreSQL Docker images | BSD-3-Clause / Alpine package licenses / PostgreSQL License | https://hub.docker.com/_/golang · https://hub.docker.com/_/alpine · https://hub.docker.com/_/postgres | Backend multi-stage build, runtime, and local database |
