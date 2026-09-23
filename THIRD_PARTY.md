# Third-party components (disclosure per rules §5.4.4)

Every dependency, model, API, dataset, template or UI kit we use must be listed here. When something moves from "Planned" to actually used in code, move its row to "In use".

## In use

| Component | License | Link | Used for |
|-----------|---------|------|----------|
| Voice Router starter kit (`data/`: scenarios, slots, actions, knowledge base, mock backend, dialogs, dev utterances, `evaluate.py`) | Provided by the organizer / Halyk Bank for HackAlem AI | https://drive.google.com/file/d/1sHE56gXnzdscHz5lMcNbwd1VsJIVLFUv | Synthetic scenarios, dialogs, facts; official evaluation script |
| Next.js | MIT | https://nextjs.org | Frontend framework |
| Tailwind CSS | MIT | https://tailwindcss.com | Styling |
| Go (stdlib `net/http`) | BSD-3-Clause | https://go.dev | Backend service |

## Planned (from `docs/SPEC.md` / `docs/research/DEEP_RESEARCH_REPORT.md` — confirm when integrated)

| Component | License / terms | Link | Used for |
|-----------|-----------------|------|----------|
| shadcn/ui | MIT | https://ui.shadcn.com | UI primitives |
| ObsidianUI | MIT | https://www.obsidianui.dev | UI components / blocks |
| Motion | MIT | https://motion.dev | Animations (ObsidianUI dependency) |
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
