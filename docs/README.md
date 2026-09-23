# Docs — index

> All docs are **living drafts**: research is done, decisions are not locked. Edit freely; keep the precedence rule and the API contract in sync.
>
> **Precedence on conflict:** `CASE.md` (official task) → `SPEC.md` → `PRD.md` → everything else.

## Core (read before coding)
| File | What | Status |
|---|---|---|
| [CASE.md](CASE.md) | Official Halyk Bank "Voice Router" task, verbatim (RU + EN) | Official — do not edit the task text |
| [CASE_ANALYSIS.md](CASE_ANALYSIS.md) | Our reading of the case: scoring, must-haves, starter kit, tools, quick-solution sketch, team split, risks | Draft |
| [REQUIREMENTS.md](REQUIREMENTS.md) | Must-have / optional / forbidden checklist mapped to our implementation and owners | Draft — tick boxes as we ship |
| [PRD.md](PRD.md) | Product: personas, P0/P1/P2 scope, demo script, criteria mapping, limitations | Draft |
| [SPEC.md](SPEC.md) | Architecture, router algorithm + prompt, JSON schema, latency budget, data model, **API contract**, env vars | Draft — contract changes go here in the same commit |
| [TASKS.md](TASKS.md) | Hour-by-hour build plan with owners, cut list, submission steps | Draft |
| [PITCH.md](PITCH.md) | 3-minute pitch + hard jury Q&A | Draft |
| [PROGRESS.md](PROGRESS.md) | Hourly progress log (proof of hourly progress, rules §5.4.8) | Update every hour |

## Data
- [`/data`](../data/) — official starter kit (read-only). Start with [`data/README.md`](../data/README.md) (EN) · [RU](../data/README.ru.md) · [KZ](../data/README.kz.md).

## Research — [`research/`](research/)
- [DEEP_RESEARCH_REPORT.md](research/DEEP_RESEARCH_REPORT.md) — findings on STT/TTS (geko, ElevenLabs), LLM choice, routing approach, VAD, speculative routing, telephony, proposed third-party list.
- [DEEP_RESEARCH_PROMPT.md](research/DEEP_RESEARCH_PROMPT.md) — prompt used to generate the research.
- [`raw/`](research/raw/) — original Russian sources: `case-analysis.ru.md`, `deep-research-output.ru.md` (English docs above are translations of these).

## Design — [`design/`](design/)
- [design/README.md](design/README.md) — screens, token summary, open design decisions.
- [`/DESIGN.md`](../DESIGN.md) — 1609SAT design system (full reference).
- [`design/references/`](design/references/) — brand and console PDFs.

## Hackathon — [`hackathon/`](hackathon/)
- [RULES.md](hackathon/RULES.md) — rules and logistics digest (EN); original RU in [`raw/RULES.ru.md`](hackathon/raw/RULES.ru.md).
- [README_PROMPT.md](hackathon/README_PROMPT.md) — organizers' README structure.
- [TEAM_PLAN.md](hackathon/TEAM_PLAN.md) — pre-start checklist and 5-hour timeline.
- [`organizer/`](hackathon/organizer/) — official organizer PDFs (RU).

## Open decisions
- Product name: *Bagyt* (research) vs *Saqta Voice Router / Plus Router* (case analysis).
- Backend layout: existing `backend/cmd/server` + stdlib `/health` vs AGENTS.md `cmd/api` + chi + pgx + `/healthz` (see end of SPEC.md).
- Keyless mode flag: `LLM_PROVIDER=mock` (AGENTS.md) vs `MOCK_MODE` (SPEC.md) — pick one and use it everywhere.
- UI base: see open decisions in [design/README.md](design/README.md).
