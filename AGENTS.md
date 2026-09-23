# AGENTS.md — Team Plus @ HackAlem AI

Instructions for every AI coding agent (Codex, Claude Code, Cursor, etc.) working in this repo. Read this file fully before touching code.

## 0. Context in 30 seconds
- Event: HackAlem AI (Astana, 23.09.2026). Build window **13:00–18:00 GMT+5 (5 hours)**. Repo state at **18:00 is final**.
- Team **Plus**: Tair, Alikhan, Ramazan. Track: **07 Education** (confirm against `docs/CASE.md`).
- The case/ТЗ is published at 13:00 → `docs/CASE.md`. Product scope → `docs/PRD.md`. Technical design → `docs/SPEC.md`. Full rules → `docs/HACKATHON_RULES.md`.
- **Read `docs/CASE.md`, `docs/PRD.md`, `docs/SPEC.md` before implementing anything.** If they conflict, CASE (official ТЗ) wins, then SPEC, then PRD.

## 1. Hard rules (violations = disqualification or non-admission)
1. All code lives in this repo (`BAITC-Hacks/hack-9dd7b5f3-plus`). No other repos for main development.
2. Progress must be committed and pushed **every hour** (checkpoints 14:00, 15:00, 16:00, 17:00, 18:00). Aim for a push at least every 30 min. Never leave work uncommitted near :50.
3. The project must run from a clean clone by following `README.md` only. Keep README in sync with reality on every change to setup, env vars, or run commands.
4. Reviewers must be able to test **without our personal accounts or keys**: the app must work with `LLM_PROVIDER=mock` (deterministic canned responses) and ship seed/demo data + demo credentials documented in README.
5. Every third-party dependency, model, dataset, template or UI kit you add → add a line to `THIRD_PARTY.md` (name, license, link, what for).
6. Never commit secrets. Only `.env.example` with placeholder values. Real keys go into local `.env` and Railway variables.
7. Commits are authored by the human team member's own git identity. **Do not add AI co-author / "Generated with" trailers** to commits or PRs.
8. Don't paste code from our other private/NDA projects. Everything in the repo becomes licensed to the organizer (rules §6.1).
9. **Every team member must have a personal, visible contribution** (own commits from own GitHub account) — otherwise their participation doesn't count.
10. We solve **exactly one case** of the track. Implement the case's **mandatory requirements first**; nice-to-haves only after the main scenario works end-to-end.
11. Secrets never go into code, repo, presentations or chat messages. Repo stays private (team + organizers only). Use only synthetic / organizer-provided data — no real personal or production data.
12. README must follow `docs/README_PROMPT.md` (organizers' structure: what's implemented, data & integrations, limitations, deployed link) **plus** env vars and dependencies. If the case document has its own README prompt — it wins.
13. Pushing to the repo is not submission: the captain also presses **«Сдать решение»** on the platform (Tracks → our case) with title + description, before 18:00.

## 2. Stack
| Layer | Choice |
|-------|--------|
| Backend | **Go** (latest stable), `net/http` + `chi` router, `pgx` for Postgres, `sqlc` optional, JSON REST |
| AI | OpenAI-compatible client behind one interface. Providers: `openai` (OpenAI API), `nvidia` (NVIDIA Build, base URL `https://integrate.api.nvidia.com/v1`), `openai_compatible` (any OpenAI-compatible API via `LLM_BASE_URL` — OpenRouter, Groq, Gemini's OpenAI endpoint, Anthropic's OpenAI-compat endpoint, local Ollama), `mock`. Selected by `LLM_PROVIDER`. Any model is allowed by the rules — just list it in THIRD_PARTY.md. Free credits: OpenAI API $50, NVIDIA (Brev GPU credits; Build API keys at build.nvidia.com/settings/api-keys) |
| DB | PostgreSQL (Railway plugin in prod, docker-compose locally). SQLite acceptable only if SPEC says so |
| Frontend | **Next.js** (App Router, TypeScript, Tailwind), `shadcn/ui` + **ObsidianUI** registry (`@obsidian` → `https://www.obsidianui.dev/r/{name}.json`) |
| Deploy | **Railway**: services `backend` (root `/backend`) and `frontend` (root `/frontend`) + Postgres. Each has a Dockerfile |
| Local run | `docker compose up --build` must bring everything up |

## 3. Repo layout
```
/backend          Go service
  cmd/api/main.go entrypoint
  internal/http    handlers, router, middleware (CORS, logging)
  internal/ai      LLM provider interface + openai/nvidia/mock impls + prompts
  internal/store   DB access, migrations (embed SQL files, run on boot)
  internal/domain  core types / business logic
  Dockerfile
/frontend         Next.js app
  src/app          routes
  src/components   ui (shadcn/obsidian) + feature components
  src/lib/api.ts   typed API client (base URL from NEXT_PUBLIC_API_URL)
  Dockerfile
/docs             CASE, PRD, SPEC, rules, plan, progress log, research prompt
docker-compose.yml
.env.example
README.md
THIRD_PARTY.md
```

## 4. Conventions
- **Ownership to avoid merge hell:** Tair — `backend/internal/ai`, agents/prompts, integration, README/docs, deploy. Alikhan — `backend/` (API, DB, domain). Ramazan — `frontend/`. Touch someone else's area → tell them in chat first.
- Branching: trunk-based. Small commits straight to `main` with `git pull --rebase` before push, or short-lived branches merged within 30 min.
- Commit messages: Conventional Commits (`feat(api): ...`, `fix(web): ...`, `docs: ...`, `chore: ...`).
- API contract is the source of truth in `docs/SPEC.md` (endpoints + JSON shapes). Change contract → update SPEC in the same commit.
- Go: `gofmt`, return errors, no panics in handlers, `context` everywhere, config only from env. Health endpoint `GET /healthz`.
- Frontend: server components by default, client components only for interactivity. Loading/empty/error states on every data view. Mobile-friendly.
- LLM calls: timeouts (≤30s), structured JSON output with schema validation, one retry, graceful fallback message. Keep prompts in `internal/ai/prompts/*.md` or Go consts — not scattered.
- Log every LLM call (provider, model, latency, token usage) — useful for the demo and for judges.

## 5. Definition of done (per feature)
- Works end-to-end in the deployed Railway URL **and** via `docker compose up`.
- Works with `LLM_PROVIDER=mock`.
- README "How to verify" section updated if the main scenario changed.
- Committed + pushed.

## 6. Priorities when time is short
Main demo scenario working end-to-end > deploy + README runnable > polish/UI effects > extra features. Feature freeze at **17:15**. From 17:15: only bug fixes, README verification from a clean clone, demo data, screenshots.

## 7. Environment variables (keep `.env.example` in sync)
```
# backend
PORT=8080
DATABASE_URL=postgres://plus:plus@db:5432/plus?sslmode=disable
LLM_PROVIDER=mock            # mock | openai | nvidia
OPENAI_API_KEY=
OPENAI_MODEL=gpt-4.1-mini
NVIDIA_API_KEY=
NVIDIA_MODEL=meta/llama-3.3-70b-instruct
LLM_BASE_URL=                # openai_compatible only, e.g. https://openrouter.ai/api/v1
LLM_API_KEY=
LLM_MODEL=
CORS_ORIGINS=http://localhost:3000
# frontend
NEXT_PUBLIC_API_URL=http://localhost:8080
```
(Model names are defaults — override with whatever the credits cover.)
