# Bagyt — Voice Router · Team Plus · HackAlem AI

> Track 09 · Communications · Case: **Halyk Bank — Voice Router** (working product name: *Bagyt / Бағыт*, "route")
> Deployed: `<https://...up.railway.app>` · Demo access: `<none / demo credentials>`
>
> ⚠️ Work in progress — sections marked `<...>` are filled in as features land. Planning docs: [docs/README.md](docs/README.md).

## 1. Summary
A web simulator of a voice robot for an insurance contact center (fictional **Saqta Insurance**, from the organizer's dataset). The client speaks Russian, Kazakh, or both in one phrase; an **LLM layer** — not an encoder intent classifier — picks one of 40 scenarios from the whole dialog context, the robot answers by voice, and a **trace panel** shows the supervisor which scenario was chosen, why, the alternatives, and per-stage timings.

## 2. What is implemented
- <feature 1>
- <feature 2>
- <AI agent: what it does>

## 3. How it works (main scenario)
Mic (or text fallback) → STT (RU/KZ) → retrieval shortlist over `data/scenarios.json` → LLM router (JSON: scenario, confidence, reasoning, alternatives) → reply → TTS → trace panel. Details: [docs/SPEC.md](docs/SPEC.md).

## 4. Tech stack
- Backend: Go (`net/http`; chi + pgx planned), PostgreSQL
- Frontend: Next.js (App Router, TypeScript), Tailwind CSS, shadcn/ui, ObsidianUI
- AI: LLM router via OpenAI / NVIDIA / any OpenAI-compatible API; STT/TTS via geko.sh (KZ/RU) with ElevenLabs fallback; **mock mode for review without keys**
- Deploy: Railway; locally — Docker Compose
- Third-party components: [THIRD_PARTY.md](./THIRD_PARTY.md)

## 5. Architecture
```
[Next.js frontend] --REST/WS--> [Go API] --> [PostgreSQL]
       mic / audio                  |
                                    +--> [STT/TTS: geko | ElevenLabs | mock]
                                    +--> [LLM router: OpenAI | NVIDIA | OpenAI-compatible | mock]
```
Full design: [docs/SPEC.md](docs/SPEC.md).

## 6. Requirements
- Docker 24+ and Docker Compose v2 (recommended), **or**
- Go 1.26+, Node.js 20+, PostgreSQL 15+ for a manual run
- **No API keys needed for review**: `LLM_PROVIDER=mock` by default

## 7. Install and run
```bash
git clone https://github.com/BAITC-Hacks/hack-9dd7b5f3-plus.git
cd hack-9dd7b5f3-plus
cp .env.example .env          # works as-is in mock mode
docker compose up --build     # <docker-compose.yml: TODO>
```
- Frontend: http://localhost:3000
- API: http://localhost:8080/health

Without Docker:
```bash
cd backend && go run ./cmd/server
cd frontend && npm ci && npm run dev
```

## 8. Environment variables
| Variable | Default | Purpose |
|-----|---------|---------|
| `PORT` | 8080 | API port |
| `DATABASE_URL` | Postgres from compose | DB connection |
| `LLM_PROVIDER` | `mock` | `mock` \| `openai` \| `nvidia` \| `openai_compatible` |
| `OPENAI_API_KEY` / `OPENAI_MODEL` | — | for `openai` |
| `NVIDIA_API_KEY` / `NVIDIA_MODEL` | — | for `nvidia` |
| `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` | — | for `openai_compatible` |
| `GEKO_API_KEY` | — | STT/TTS (KZ/RU) |
| `ELEVENLABS_API_KEY` | — | STT/TTS fallback |
| `CORS_ORIGINS` | http://localhost:3000 | allowed origins |
| `NEXT_PUBLIC_API_URL` | http://localhost:8080 | API URL for the frontend |

## 9. How to verify
1. Open http://localhost:3000 (or the deployed URL).
2. <step>
3. Routing accuracy on the official dev set: `python data/evaluate.py <predictions>` — see [data/README.md](data/README.md).
4. Expected result: <what the reviewer sees>

## 10. Data and integrations
- Official synthetic starter kit in [`data/`](data/) (40 scenarios + 3 system ones, slots, actions, knowledge base, mock backend, dev utterances, `evaluate.py`). No real personal data.
- <external APIs used in the final build>

## 11. Limitations
- <not implemented / known limitations>

## 12. Team Plus
- Tair Kaldybayev — captain, product / AI
- Alikhan — backend
- Ramazan — frontend
