# Deep Research prompt → PRD + SPEC

## How to use (13:00–13:10)
0. The track has several cases — we solve **one**. If undecided, paste all cases into `{{CASE_TEXT}}` and the prompt will pick (step 0 of research).
1. Copy the official task text + criteria from the Tracks tab into `docs/CASE.md`. If the case document includes its own README prompt, save it too.
2. Paste the same text into `{{CASE_TEXT}}` / `{{CASE_CRITERIA}}` below (and `{{TRACK}}` if we switch from Education).
3. Run in **ChatGPT Deep Research** (Pro via hackathon promo) — in parallel run the **Fast prompt** (section B) in a normal chat (GPT / Claude) so we have a draft PRD in ~3 min while DR works (~15–25 min).
4. Save DR output into `docs/PRD.md`, `docs/SPEC.md`, `docs/PITCH.md`. Then run prompt C in Codex / Claude Code to turn SPEC into tasks.
5. If DR asks clarifying questions, answer: "No questions — make reasonable assumptions, mark them, and proceed."

---

## A. Deep Research prompt (copy everything inside the block)

```text
You are a senior product strategist + staff engineer + hackathon judge, helping a 3-person team win HackAlem AI (Astana, Kazakhstan, 23 Sep 2026) — an agentic-AI hackathon organised by Astana Hub with partners incl. OpenAI, Kazakh ministries (MDDIAI / Ministry of Science and Higher Education), Kaspi, Halyk, Kazakhtelecom, Samruk-Kazyna, Beeline, Freedom.

We have ONE task. Research it deeply and produce build-ready PRD and technical SPEC documents for a product we can actually build and demo in the remaining ~4.5 hours.

## Our task
Track: {{TRACK — default: 07 Education}}
Official task text (verbatim):
"""
{{CASE_TEXT}}
"""
Official evaluation criteria / point distribution for this task:
"""
{{CASE_CRITERIA}}
"""

## Hard constraints
- Team: 3 engineers (Tair — AI/LLM & product lead; Alikhan — Go backend; Ramazan — Next.js frontend).
- Build time: 13:00–18:00 (5 hours total, research eats the first ~20 min). Feature freeze 17:15.
- Stack is fixed: Go REST API (chi, pgx, PostgreSQL), Next.js App Router + TypeScript + Tailwind + shadcn/ui + ObsidianUI, deployed on Railway; LLMs via OpenAI API or NVIDIA NIM (OpenAI-compatible), ~$50 credits each. Codex/ChatGPT Pro available for coding.
- The project must run from a clean clone via README (docker compose) and be testable by reviewers WITHOUT our API keys → design a deterministic mock mode for all AI features.
- Every hour we must show a verifiable increment (commits). Each of the 3 members must have a personal contribution.
- The solution must use an AI agent. The case's MANDATORY requirements must be fully met first — nice-to-haves only after.
- Only synthetic or organizer-provided data; no real personal/production data. README must list: what's implemented, data & integrations, known limitations, deployed link.
- Evaluation stage 1: technical experts (+ possibly an AI judge) score against the task's own criteria, and check that it runs. Stage 2 (Demo Day) jury criteria: Value of solution 25, Result & quality 20, Innovation 15, Growth & scale potential 20, Presentation/demo/Q&A 20.
- Users are in Kazakhstan: interfaces should support Russian and Kazakh (English optional). Respect Kazakhstan's personal data law — avoid real personal data; use synthetic seed data.

## What to research (use current, citable sources; prefer Kazakhstan-specific data, then CIS, then global)
0. If more than one case is pasted above: compare them (fit to our stack and skills, buildability in 4.5h, demo-ability, winning odds) and pick ONE. Then research only that one.
1. Decode the task: what the task-setter really wants, explicit vs implicit requirements, what "excellent" looks like against each criterion. Flag any ambiguity and pick the most defensible interpretation.
2. Problem evidence: size of the problem in Kazakhstan with numbers (ministry stats, stat.gov.kz, OECD/PISA, World Bank, UNESCO, news). Who suffers, how often, what it costs.
3. Stakeholders & personas: primary user, buyer/payer (B2G / B2B / B2C), decision-makers in the KZ education system; their current workflow and pain points.
4. Existing solutions: Kazakhstan platforms (e.g. national school / university / e-gov education systems, EdTech startups), CIS and global analogues. For each — what they do, gaps, why our angle is different. Include any government programs or initiatives we should align with.
5. Data & integrations we could realistically use in 5 hours: open datasets (data.egov.kz etc.), public APIs, curricula/standards documents, sample content. Say which are usable now and which to mock.
6. Solution concepts: propose 3–5 distinct concepts. Score each 1–5 on: task-criteria fit, jury criteria (value, quality, innovation, scale, demo-ability), buildability in 4.5h with our stack, "agentic" depth (multi-step reasoning, tool use, autonomy — not just a chat wrapper). Pick ONE winner and justify.
7. Agentic AI design for the winner: agents/steps, tools each agent calls, prompts (draft them), strict JSON output schemas, guardrails, evaluation of AI quality, cost per request estimate with the given models, latency budget, and exact mock-mode behaviour.
8. The "wow" moment: the single 20–30 second moment in the demo that makes judges remember us.
9. Scale & business: path after the hackathon (pilot with whom, pricing model, market size in KZ + Central Asia), key metrics.
10. Risks: technical, data, legal/ethical (minors' data, bias, hallucinations in education), and mitigations. List 8 tough jury questions with strong answers.

## Deliverables — output exactly these three markdown documents, separated by lines `=== PRD.md ===`, `=== SPEC.md ===`, `=== PITCH.md ===`

### PRD.md
1. One-liner and product name (short, memorable, works in RU/KZ).
2. Problem & evidence (with citations).
3. Personas & user stories, prioritised P0 (must demo) / P1 / P2.
4. Core demo scenario — numbered steps, ≤ 3 minutes, with sample inputs.
5. MVP scope IN / OUT (be ruthless: P0 must fit ~3 build hours).
6. Differentiation table vs top 3–5 alternatives.
7. Mapping table: every official task criterion → the feature/evidence that satisfies it; same for the 5 Demo Day criteria.
8. Success metrics & scale path.
9. Assumptions (explicitly marked) and risks.

### SPEC.md
1. Architecture (ASCII diagram) and main data flow.
2. Data model: tables, columns, types, relations; + synthetic seed data plan (what and how much).
3. REST API contract: for each endpoint — method, path, request JSON, response JSON example, errors. Keep to ≤ 10 endpoints.
4. AI layer: provider interface (Go), each agent/step with system prompt draft, input, tools, output JSON Schema, retries/timeouts, mock responses.
5. Frontend: routes, screens, key components (shadcn / ObsidianUI), states (loading/empty/error), i18n RU/KZ approach.
6. Deploy & config: Railway services, env vars, docker-compose outline.
7. Work breakdown: tasks per person per hour (13:20–17:15), with checkpoint deliverables at 14:00, 15:00, 16:00, 17:00, and what gets cut first if we're late.
8. README "How to verify" steps for reviewers (mock mode).

### PITCH.md
1. 3-minute pitch script (Problem → Insight → Demo → Why AI/agentic → Scale → Ask), in Russian.
2. Slide outline (≤ 8 slides) with one key number per slide.
3. 8 hardest jury questions + answers.

## Style
- Be concrete and decisive; no generic advice. Prefer tables and numbered lists.
- Cite sources inline with links; mark anything unverified as ASSUMPTION.
- Optimise for: winning the task's criteria first, Demo Day criteria second, buildability always.
- Do not ask me clarifying questions — make reasonable assumptions and proceed.
```

---

## B. Fast prompt (normal chat, ~3 min) — run in parallel with DR

```text
Hackathon: HackAlem AI (Astana, agentic-AI), track {{TRACK}}. 3 devs (AI lead / Go backend / Next.js frontend), 4.5 hours left, stack Go + Postgres + Next.js (shadcn, ObsidianUI) on Railway, OpenAI/NVIDIA LLMs, must run in mock mode without keys.
Task: """{{CASE_TEXT}}""" Criteria: """{{CASE_CRITERIA}}"""
Jury: value 25, result/quality 20, innovation 15, scale 20, pitch 20.
Give me in 1 page: 3 solution concepts scored against criteria → pick one → P0 feature list (fits 3 build hours) → 3-min demo scenario → ≤8 REST endpoints with JSON shapes → DB tables → AI agent steps with output JSON schemas → hour-by-hour split for 3 people. Be decisive, no fluff.
```

---

## C. Follow-up prompt for Codex / Claude Code (after PRD/SPEC are saved)

```text
Read AGENTS.md, docs/CASE.md, docs/PRD.md, docs/SPEC.md.
1) Point out any contradiction between CASE and SPEC and propose a fix.
2) Produce docs/TASKS.md: ordered tasks per owner (Tair: backend/internal/ai + deploy, Alikhan: backend API/DB, Ramazan: frontend) with acceptance criteria, sized ≤ 30 min, grouped by hourly checkpoint (14/15/16/17).
3) Then implement the first task in your owner's area. Keep README, .env.example and THIRD_PARTY.md in sync. Commit with conventional messages, no AI co-author trailers.
```
