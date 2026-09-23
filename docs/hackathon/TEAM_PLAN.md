# Team Plus — 5-hour battle plan (23.09, GMT+5)

Roles (proposed — adjust in the first 5 min):
- **Tair** — captain, product/AI lead: case analysis, Deep Research, PRD/SPEC, `backend/internal/ai`, Railway deploy, README, pitch.
- **Alikhan** — backend: Go API, DB schema, domain logic, seed data.
- **Ramazan** — frontend: Next.js screens, ObsidianUI/shadcn, API client, demo polish.

## Before 13:00
- [ ] All three checked in (badge QR scanned); laptops + chargers (laptop AND phone) + USB/USB-C→RJ-45 adapters with drivers.
- [ ] **Sit together in the same sector BEFORE 13:00** — after the start no seat changes at all. Be back in the zone before the start is announced.
- [ ] Ethernet: IP + DNS = automatic (DHCP). `ipconfig` must not show 169.254.x.x. **No phone hotspots / tethering** — forbidden.
- [ ] 12:30 — grab promo codes (hackathon page / Telegram bot). ChatGPT Pro: confirm **only if "Due today: $0"**, note next billing date. OpenAI API credits → Billing → Promotions. NVIDIA Brev → Billing → Redeem Code. NVIDIA Build API key → build.nvidia.com/settings/api-keys. Details: docs/hackathon/RULES.md §12.3.
- [ ] Everyone has push access to `BAITC-Hacks/hack-9dd7b5f3-plus` from their **own** GitHub account; `git config user.name/email` = personal.
- [ ] Railway: project created, team invited, Postgres added. Toolchains ready: Go, Node 20+, Docker.

## Timeline
| Time | Tair | Alikhan | Ramazan | Checkpoint pushed |
|------|------|---------|---------|-------------------|
| 13:00–13:15 | Read ALL cases of the track → pick **one** case → paste into `docs/CASE.md`; launch Deep Research (`docs/research/DEEP_RESEARCH_PROMPT.md`) | Go skeleton: `/healthz`, config, DB conn, Dockerfile | Next.js skeleton + Tailwind + shadcn + ObsidianUI registry | |
| 13:15–14:00 | Draft PRD-lite from ТЗ while DR runs; `docker-compose.yml`; first Railway deploy (hello world both services) | Schema v0 + migrations + seed | Layout, landing/main screen shell, `lib/api.ts` | **14:00** docs + scaffolds + deploy |
| 14:00–15:00 | DR done → finalize PRD/SPEC (API contract!). `internal/ai` provider interface + mock + openai/nvidia | Core CRUD endpoints per SPEC | Main flow screens against mock JSON | **15:00** core API + screens |
| 15:00–16:00 | AI feature #1 end-to-end (prompts, structured output) | Wire AI into domain endpoints | Wire screens to real API | **16:00** main scenario E2E |
| 16:00–17:00 | AI feature #2 / "wow" moment; logging; deploy | Edge cases, validation, demo data | Polish, loading/error states, mobile | **17:00** feature-complete |
| 17:00–17:15 | last small feature | fixes | fixes | **17:15 FEATURE FREEZE** |
| 17:15–17:50 | README via `docs/hackathon/README_PROMPT.md` (or the case's own prompt) + fresh-clone test (mock mode), final deploy, THIRD_PARTY, **press «Сдать решение» on the platform (title + description) — do it by 17:40, can be updated until deadline** | Fix what the fresh-clone test finds | Screenshots / short screen recording for Demo Day | |
| 17:50 | **Final push.** Verify on GitHub that `main` has everything | | | **18:00 hard stop** |

## Rules of engagement
- **Breaks:** 60 min total per person, tell the steward before leaving and after returning, badge scanned both ways. Take breaks before 17:00 — **17:00–18:00 nobody leaves the zone** (emergency ≤ 5 min with steward).
- Everyone must have personal commits (personal contribution is checked).
- Tech questions → raise your hand, a mentor comes.
- Push at least every 30 min. Nobody sits on uncommitted work at :45.
- API contract lives in `docs/SPEC.md`; frontend codes against it with mock JSON until backend is ready.
- Blocked > 10 min → say it out loud.
- Log each hour in `docs/PROGRESS.md` (2–3 bullets) — cheap proof of hourly progress.
