# Design — direction and references

> Status: living draft. Nothing here is locked; pick, adapt, and record decisions in the "Decisions" section below.

## What we are designing

Two screens (see `docs/SPEC.md` for the contract and `docs/PRD.md` for scope):

1. **Call** — the client-facing voice simulator: push-to-talk / mic button, live transcript, the robot's spoken reply, language badge (RU / KZ / mixed).
2. **Trace / Supervisor** — shown after every client turn: selected scenario, reasoning, alternatives with scores, extracted parameters, per-stage timings (STT → route → reply → TTS), plus aggregate error stats.

The trace panel is scored by the jury, so it is a data-dense operator console, not a marketing page.

## References in this repo

| Reference | File | What it is | Use it for |
|---|---|---|---|
| **Console — components & blocks** | [references/speko-console-components.pdf](references/speko-console-components.pdf) | Dark-first operator console for a voice-provider router: real sizes of buttons, inputs, status badges, metric cards, data tables, empty/error states | **Primary reference for the Trace / Supervisor screen** — latency metric cards, per-stage timing tables, status badges |
| **Brand — landing** | [references/speko-brand-landing.pdf](references/speko-brand-landing.pdf) | Light-only "technical paper" brand: off-white paper, green-cast ink, one blue | Landing / README screenshots / pitch slides, if we want a light variant |
| **1609SAT design system** | [/DESIGN.md](../../DESIGN.md) | Full Tailwind v4 design system (tokens, type, the "+" motif, layout, controls) — dark navy canvas, single electric-blue accent | Base token set and component conventions for the Next.js frontend; the "+" motif fits team **Plus** |

## Key tokens at a glance

### Console (from `speko-console-components.pdf`)
- Dark-first; the dark state is the default, not a theme.
- Spacing: 4px base, stock Tailwind scale (1·4, 2·8, 3·12, 4·16, 6·24, 8·32, 12·48, 16·64, 24·96). No custom steps.
- Page gutter 32px, content max width 1248px; section gap 32px; card inset 16px (20px on wide cards); table row padding 10px (`p-2.5`) with a hairline below.
- Buttons (desktop heights): xs 24 · sm 28 · default 32 · lg 36 · xl 40; variants default / secondary / outline / ghost / destructive / link; disabled = 64% opacity; pending shows a spinner.
- Inputs: 30px desktop height, 10px radius, 1px input border; focus ring in brand blue; invalid state in red.
- Status badges: Routable (green), Degraded (amber), Failed (red), Scheduled (blue), plus Outline / Secondary / BETA / mono `provider:model` chips. Fill at the 500 shade, text lightened to 400 — never the same value.
- Blocks: page headers use noun titles with no description line; **metric cards** are the one place a description survives (e.g. "FIRST BYTE P50 · 312 ms · across 9 providers"); data tables use tabular numerals and hairline rows; empty states keep a description + CTA; error states use prose + Retry.

### Brand (from `speko-brand-landing.pdf`)
- Brand blue `#2f6ad1` = primary, accent and focus ring; `--brand-hi #2a5ebb`; alpha steps a20 (ring, selection), a12, a06 (accent fill).
- Neutrals (green-cast, not slate): paper `#faf9f6`, paper-2 `#f2f2ec`, card `#ffffff`, body `#4e5b54`, ink `#0e1512`.
- Sequential scale s0–s7 in blue (one measurement scale, not a rainbow); semantic up `#2c5498`, down `#a8443c`, warn `#7c5d27`, ok `#3f7a52`.
- Type: Hanken Grotesk 600, tracking −0.022em, `text-wrap: balance` for H1–H3; mono 11px, tracking .14em, tabular-nums for section markers (`[ 03 / 09 ] ROUTING`) and numbers (`p50 536`, right-aligned).
- Radius: xs 4 · sm 8 · md 12 (base) · lg 18.

### 1609SAT (from `/DESIGN.md`)
- Dark by default: canvas `#0a0d15`, text `#c7d3e0`, single accent `#3755ed`; surfaces are the text color mixed over the canvas at 3–16%.
- Geist (`ss08`, −0.02em, weight 450) on brand surfaces, Inter on utility surfaces; numbers are the hero (`tabular-nums`).
- Hairline borders, no shadows on brand panels; the custom "+" glyph as brand motif.
- Stack assumptions there (Tailwind v4 via `@theme inline`, COSS UI on Base UI, lucide, framer-motion, recharts) must be reconciled with the frontend stack in `AGENTS.md` (shadcn/ui + ObsidianUI).

## Open decisions

- [ ] Which token set is the base for `frontend/`: 1609SAT (`DESIGN.md`) or the console palette? Suggested: 1609SAT tokens + console component sizing/patterns for the trace panel.
- [ ] Dark-only, or dark + light toggle?
- [ ] Component library: shadcn/ui + ObsidianUI (AGENTS.md) vs COSS UI / Base UI (DESIGN.md).
- [ ] Charts for the supervisor view (recharts?) — add to `THIRD_PARTY.md` when chosen.

## Decisions

_Record decisions here with date/time and owner._
