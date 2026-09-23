# Bagyt — Voice Router · Team Plus · HackAlem AI

> Track 09 · Communications · Case: **Halyk Bank — Voice Router** (working product name: *Bagyt / Бағыт*, "route")
> Deployment: local Docker Compose · Demo access: no login required
>
> Planning docs: [docs/README.md](docs/README.md).

## 1. Summary
A web simulator of a voice robot for an insurance contact center (fictional **Saqta Insurance**, from the organizer's dataset). The client speaks Russian, Kazakh, or both in one phrase; an **LLM layer** — not an encoder intent classifier — picks one of 40 scenarios from the whole dialog context, the robot answers by voice, and a **trace panel** shows the supervisor which scenario was chosen, why, the alternatives, and per-stage timings.

## 2. What is implemented
- Frontend call simulator, microphone/text input, browser voice response and supervisor trace.
- Python `core-llm` router via OpenRouter: 40 scenarios, multiple intents, dialog context, RU/KZ.
- Connected **LLM** mode: Next.js proxy → Python router → existing dialog executor on synthetic data.
- **Mock** mode without keys, and dev-set evaluation for the selected router.

## 3. How it works (main scenario)
Browser STT or text → Next.js `/api/core-route` → Python `Router.route` → scenario percentages and policy →
frontend slot collection/demo actions → catalog response + browser TTS → supervisor trace.
The explanation is generated from the returned IDs/scores and policy, not LLM chain-of-thought.

## 4. Tech stack
- Router service: Python standard library; Go API and PostgreSQL remain a separate planned integration
- Frontend: Next.js (App Router, TypeScript), Tailwind CSS, shadcn/ui, ObsidianUI
- AI: OpenRouter LLM routing, browser STT/TTS, optional OpenAI STT; **mock mode for review without keys**
- Local deployment: Docker Compose; Railway deployment pending
- Third-party components: [THIRD_PARTY.md](./THIRD_PARTY.md)

## 5. Architecture
```
Browser (dialog state, demo actions, STT/TTS)
  → Next.js /api/core-route → Python core-llm → OpenRouter
```
The Go backend is independent and is not required for this integration. Sessions and traces live
in the browser tab. The **Бэкенд** selector retains the separate Go SSE contract.

## 6. Requirements
- Node.js 22+ and Python 3.10+, or Docker Compose v2.
- Mock review needs no keys or login. LLM mode needs `OPENROUTER_API_KEY`.

## 7. Install and run
From a clean clone, keyless demo:
```bash
docker compose up --build
```
Open http://localhost:3000/call and use **Мок**. No demo credentials are needed.

For the real LLM, copy `core-llm/.env.example` to `core-llm/.env` and set `OPENROUTER_API_KEY` there.
```bash
docker compose --profile llm up --build
```
Select **LLM** in the page header. Python is private to the Docker network. Set `FRONTEND_PORT=3200`
if port 3000 is busy. To default to LLM at build time, set `NEXT_PUBLIC_API_MODE=core`.

Manual run, terminal 1 (repository root):
```bash
python3 core-llm/server.py
```
Terminal 2:
```bash
cd frontend
cp .env.local.example .env.local
npm ci
npm run dev
```
`CORE_LLM_URL` in `frontend/.env.local` defaults to `http://127.0.0.1:8090`.
The OpenRouter key stays in Python. For keyless manual use, skip Python and select **Мок**.

## 8. Environment variables
| Variable | Default | Purpose |
|-----|---------|---------|
| `CORE_LLM_URL` | http://127.0.0.1:8090 | Next.js server → Python router |
| `OPENROUTER_API_KEY` | — | Python router key (`core-llm/.env`) |
| `ROUTER_MODEL` | google/gemini-2.5-flash-lite | OpenRouter model |
| `NEXT_PUBLIC_API_MODE` | mock | mock / core / real; set at build time |
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
1. Open `/call`, select **LLM**, enter «Хочу продлить ОГПО и добавить сына».
2. Expected: SC27 + SC04; bot acknowledges the second request and asks for identification.
3. Open **Консоль** without reloading: chosen scenarios, model, scores, alternatives, policy and timings appear.
4. Try «А какие документы нужны при ДТП?» to switch topics, and «Полисімнің мерзімін ұзартқым келеді» for Kazakh.
5. In Chrome/Edge allow the microphone and speak; the transcript goes through the same router.
   Browser TTS uses installed voices. Server STT requires a separate OpenAI key.
6. Run the dev set from the quality panel; in **LLM** mode it calls the real core (104 paid requests).
7. `cd frontend && npm test && npm run lint && npm run build`; from root:
   `python3 -m unittest discover -s core-llm -p 'test_*.py'`.

## 10. Data and integrations
Official synthetic starter kit in `data/` (unchanged), with frontend copies in `frontend/src/data`.
OpenRouter routes to `google/gemini-2.5-flash-lite` by default. Browser STT/TTS; optional OpenAI server STT.
No real customer data or production insurance operations are used.

## 11. Limitations
- Slots/actions/responses use the existing deterministic demo executor; only routing calls the LLM.
- Sessions and supervisor statistics are local to one browser tab and disappear on reload.
- Speech recognition depends on browser support/network; Kazakh synthesis depends on installed voices.
- The Go backend, persistence, geko/ElevenLabs and Railway deployment are outside this frontend/core integration.
- Latencies depend on the network/provider. No silent mock fallback when the LLM service fails.

## 12. Team Plus
- Tair Kaldybayev — captain, product / AI
- Alikhan — backend
- Ramazan — frontend
