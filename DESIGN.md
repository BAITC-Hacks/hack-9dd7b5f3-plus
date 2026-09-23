# 1609SAT — Design System

The single source of truth for how 1609SAT looks. Everything here is taken from
the shipped code (`src/index.css`, `src/components/**`, `src/views/**`,
`src/app/**`), with class strings quoted verbatim, so a new screen built from
this file should be indistinguishable from an existing one.

Stack: Next.js 16 (App Router, `basePath: /app`) · Tailwind CSS v4 (no
`tailwind.config`, everything in `src/index.css` via `@theme inline`) · COSS UI
primitives on **Base UI** (`@base-ui/react`, not Radix) · `cva` + `cn`
(`twMerge(clsx())`) · lucide-react icons · framer-motion · recharts.

---

## 0. The identity in ten lines

1. **Dark by default.** Deep navy canvas `#0a0d15`, cool blue-grey text `#c7d3e0`, one electric accent `#3755ed`.
2. **Surfaces are the text colour laid over the canvas** at 3–16% (`color-mix`), never neutral greys.
3. **One accent.** Blue is the only brand colour. Green/orange/red are *status only*.
4. **The plus is the brand.** A custom 20×20 "+" glyph (not lucide `Plus`) marks the active nav item, bullets feature lists, fills empty space in fields, and draws progress as a rail of plusses.
5. **Geist with `ss08`, tight tracking (-0.02em), light weights (`font-[450]`)** on brand surfaces; Inter on utility/kit surfaces.
6. **Hairline + no shadow** on brand panels (`rounded-[15px] border bg-card`); kit cards get a whisper `shadow-xs`.
7. **Numbers are the hero**: big `tabular-nums`, negative tracking, weight 450, units small and muted beside them.
8. **Muted text by alpha, not by new colours**: `text-muted-foreground/55`, `/60`, `/70`, `text-foreground/85`.
9. **Sentence-case microcopy labels** (`text-[10.5px] text-muted-foreground/55`) on brand surfaces; UPPERCASE tracked eyebrows only in the kit layer and marketing.
10. **Brand-blue macOS cursors** site-wide, press-scale (`active:scale-[0.97]`) on everything clickable.

No purple, no rainbow gradients, no glassmorphism cards, no stock pill badges for brand marks.

---

## 1. Color

### 1.1 Semantic tokens (`src/index.css`)

Theme switches on the `.dark` class on `<html>` (`@custom-variant dark (&:is(.dark *))`).
Default is **dark**; persisted in `localStorage["sat1609.theme"]`; `?t=light|dark`
query overrides; a no-FOUC inline script in `app/layout.tsx` applies it before paint.

| Token | Light | Dark (default) |
|---|---|---|
| `--background` | `#ffffff` | `#0a0d15` |
| `--foreground` | `#1a1a1a` | `#c7d3e0` |
| `--card` | `#ffffff` | `color-mix(in srgb, #c7d3e0 5%, #0a0d15)` |
| `--popover` | `#ffffff` | mix 8% |
| `--primary` | `#3755ed` | `#3755ed` |
| `--primary-foreground` | `#ffffff` | `#ffffff` |
| `--secondary` | `#f6f5f4` | mix 6% |
| `--secondary-foreground` | `#37352f` | `#c7d3e0` |
| `--muted` | `#f6f5f4` | mix 5% |
| `--muted-foreground` | `#5d5b54` | mix 55% |
| `--accent` (hover fill) | `#f0eeec` | mix 9% |
| `--accent-foreground` | `#1a1a1a` | `#c7d3e0` |
| `--border` | `#e5e3df` | mix 12% |
| `--input` | `#c8c4be` | mix 16% |
| `--ring` | `#3755ed` | `#3755ed` |
| `--destructive` / fg | `#e03131` / `#fff` | `#ff6b6b` / `#1a0b0b` |
| `--success` / fg | `#1aae39` / `#fff` | `#37c964` / `#04140a` |
| `--warning` / fg | `#dd5b00` / `#fff` | `#ff9f4a` / `#1a0f04` |
| `--info` / fg | `#0075de` / `#fff` | `#6b9bff` / `#061323` |
| `--sidebar` | `#fafaf9` | mix 3% |
| `--sidebar-foreground` | `#37352f` | mix 62% |
| `--sidebar-accent` / fg | `#e7ecfe` / `#2740c9` | `color-mix(in srgb, #3755ed 20%, #0a0d15)` / `#a7b7f7` |
| `--sidebar-border` | `#ede9e4` | mix 8% |

"mix N%" = `color-mix(in srgb, #c7d3e0 N%, #0a0d15)`.

Brand extras (Tailwind: `bg-brand-navy`, `text-link`, `text-charcoal`…):

| Token | Light | Dark |
|---|---|---|
| `--brand-navy` | `#0a1530` | `#0a1530` |
| `--brand-navy-deep` | `#070f24` | `#060d1f` |
| `--link-blue` | `#0075de` | `#6b9bff` |
| `--charcoal` | `#37352f` | `#dbe4ee` |
| `--slate` | `#5d5b54` | mix 78% |
| `--steel` | `#787671` | mix 60% |
| `--stone` | `#a4a097` | mix 45% |

Tints (`--tint-peach/rose/mint/lavender/sky/yellow/yellow-bold/cream`) exist in
both themes but are **not used** anywhere in product UI — don't reach for them.

Chart palette: `--chart-1..5` = `#3755ed`, `#0075de`/`#6b9bff`, `#2a9d99`/`#35c9c2`,
`#dd5b00`/`#ff9f4a`, `#1aae39`/`#37c964` (light/dark).

### 1.2 How color is actually applied

- **Accent tints by alpha**, always from `primary`:
  - hover-ish wash `bg-primary/[0.06]`–`/[0.08]`
  - selected fill `bg-primary/[0.07]` + `border-primary/30…/40`
  - active chip `bg-primary/[0.12] text-primary border-primary/40`
  - count badge `bg-primary/[0.14] text-primary`
- **Neutral hover** on brand surfaces: `hover:bg-foreground/[0.045]` (nav), `hover:bg-foreground/[0.025]` (rows). Kit surfaces use `hover:bg-accent`.
- **Status pill recipe** (any status X): `border-X/30 bg-X/10 text-X`.
- **Muted text ladder**: `/85` body-in-list → `text-muted-foreground` → `/70` sublines → `/60` meta → `/55` labels → `/45` section labels & weekday heads → `/30` empty values `—`.
- **Inverted "Next" button**: `bg-foreground text-background border-foreground` — the solving UI's primary-forward action is ink, not blue.

### 1.3 Hard-coded colors that are part of the design

| Where | Values |
|---|---|
| Marketing canvas / ink | `#F4F6F9` canvas, `#0a0d15` ink at opacity steps `/40…/80`, hairlines `/[0.04]…/15` |
| Marketing CTA | `#3755ed`, hover `#2c46cf` |
| Pro pricing card gradient | `linear-gradient(170deg, #3755ed 0%, #2740c9 46%, #131f5c 100%)` |
| Upgrade button (app header) | `bg-amber-400 hover:bg-amber-300 text-amber-950` |
| Streak flame | `text-orange-500` active today, `amber-600/500` at risk |
| Mastery scale (`lib/insights.ts`) | untested `#ffffff`, needs work `#f43f5e`, developing `#d97706`, proficient `#3b82f6`, mastered `#10b981` |
| Time-by-difficulty donut | easy `#bfdbfe`, medium `#60a5fa`, hard `#2563eb` |
| Domain dots (`lib/taxonomy.ts`) | `#3b82f6`, `#16a34a`, `#0891b2`, `#d97706` |
| Chart grid / ticks | `rgba(100,116,139,0.055)` / `rgba(100,116,139,0.35)` |
| Bluebook replica | blue `#324DC7`, ink `#1e1e1e`, highlight `#F5E960` |
| Passage highlights | see §9.6 |

---

## 2. Typography

### 2.1 Families (`app/layout.tsx`, `next/font/google`)

| Var | Font | Role |
|---|---|---|
| `--font-sans` (= `font-sans`, `font-heading`) | **Inter** | default body; kit pages; question content |
| `--font-geist` (= `font-geist`) | **Geist** | brand surfaces: shell/sidebar, dashboard, leaderboard, practice tests, paywall, score report, curator, banners, marketing |
| `--font-mono` | **Geist Mono** | grid-in answers, code |

**Brand register** — declare once on the surface root and let it inherit:

```tsx
const SS08: CSSProperties = { fontFeatureSettings: '"ss08" 1' };
<div className="font-geist tracking-[-0.02em]" style={SS08}>…</div>
```

Marketing pages set it inline instead: `style={{ fontFamily: "var(--font-geist), var(--font-sans), sans-serif" }}`.
Product mockups on the landing switch back to Inter so they read as real screenshots.

Body is always `antialiased`. Weights: `font-[450]` for brand headings & big numbers,
`font-medium` (500) for labels/buttons, `font-semibold` for kit headings,
`font-bold` only for kit stat values, number badges, ProTag and tiny uppercase stamps.

### 2.2 Type scale (what the code actually uses)

Most-used arbitrary sizes, in order: `13px` › `11px` › `12.5px` › `12px` › `11.5px` › `15px` › `13.5px` › `14px` › `10px` › `10.5px`. Half-pixel sizes are intentional.

**Brand register (Geist, ss08)**

| Role | Classes |
|---|---|
| Page h1 (dashboard) | `text-[24px] sm:text-[28px] font-[450] leading-tight tracking-[-0.035em]` (+ 12px PlusMark at `mt-[9px]`; first name in `text-muted-foreground/55`) |
| Page subline | `mt-0.5 text-[12.5px] text-muted-foreground/70` |
| Panel h2 | `text-[14.5px] font-[450] tracking-[-0.02em]` |
| Stat value | `text-[32px] font-[450] leading-none tracking-[-0.035em] tabular-nums` |
| Stat label | `text-[10.5px] text-muted-foreground/55` (sentence case) |
| Stat unit | `text-[11px] text-muted-foreground/55` |
| Footnote | `text-[11.5px] tabular-nums text-muted-foreground/65` |
| Rank / hero figure | `text-[38px] sm:text-[44px] font-[450] tracking-[-0.04em] tabular-nums` (`#` at `text-[0.42em] text-muted-foreground/45`) |
| Practice card figure | `text-[34px] font-[450] tracking-[-0.04em]` |
| Price | `text-[24px] font-[450] leading-none tracking-[-0.04em] tabular-nums` |
| Modal title | `text-[19px] font-[450] tracking-[-0.03em]` |
| Notice title | `text-[17px] font-[450] tracking-[-0.03em]` |
| Row text | `text-[13px]`–`text-[13.5px]`, list body `text-foreground/85` |
| Sidebar section label | `text-[11px] text-muted-foreground/45` (sentence case) |

**Kit register (Inter, `font-heading`)** — `src/components/ui/kit.tsx`

| Role | Classes |
|---|---|
| `PageTitle` h1 | `font-heading text-2xl font-semibold tracking-tight text-foreground sm:text-3xl` |
| `PageTitle` sub | `mt-1.5 text-sm text-muted-foreground` |
| `Eyebrow` | `text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground` |
| `SectionHeader` h2 | `font-heading text-base font-semibold tracking-tight text-foreground sm:text-lg` |
| `SectionHeader` sub | `text-[11.5px] text-muted-foreground sm:text-[12px]` |
| `StatTile` value | `font-heading text-[28px] font-bold leading-none tracking-tight tabular-nums` |
| `StatTile` label | `text-[12px] font-medium text-muted-foreground` |
| `BigNumber` | `font-heading font-semibold tabular-nums tracking-tight` `text-2xl / text-4xl / text-5xl sm:text-6xl` |
| `Pill` | `text-[10.5px] font-semibold uppercase tracking-[0.06em]` |

**Question content** (all solving surfaces)

| Role | Classes |
|---|---|
| Passage | `text-[16px] leading-[1.38] tracking-[-0.04em] text-black dark:text-foreground`, `max-w-2xl` |
| Stem | `text-[16px] font-medium leading-snug tracking-[-0.04em]` |
| Choice text | `text-[16px] font-medium leading-snug tracking-[-0.04em] text-black dark:text-foreground` |

**Score report** (`ExamResults.tsx`, Geist)

| Role | Classes |
|---|---|
| Total score | `font-geist text-[64px] font-medium leading-[0.8] tracking-[-0.026em] tabular-nums` (count-up) |
| Section score | `text-4xl font-medium leading-[1.4] tracking-[-0.046em]` + `200-800` at `text-[13px]` `/50` |
| Verdict | `text-[12px] font-medium leading-[1.8] tracking-[-0.032em]` in `rounded-[10px] bg-[#c7d3e0]/40 p-2` |

**Marketing** (see §11 for full table): h1 `text-[40px] sm:text-[48px] font-medium leading-[1.05] tracking-tight`; h2 `text-[36px] sm:text-[44px] font-medium leading-tight tracking-tight`; eyebrow `text-[13px] font-medium uppercase tracking-[0.14em] text-[#3755ed]`. Headings are **never bold**.

Tracking vocabulary: `-0.02em` (brand body), `-0.03em` (titles), `-0.035em` (big titles/figures), `-0.04em` (question text, huge figures), `tracking-tight`; positive `0.06em` (pills), `0.08em` (eyebrows), `0.12–0.16em` (tiny caps, ProTag).

---

## 3. Radius, borders, elevation

### 3.1 Radius

`--radius: 0.75rem` (12px). Tailwind steps are **one notch larger than stock shadcn**:

| Class | px | Used for |
|---|---|---|
| `rounded-sm` | 8 | menu/select items, skeletons |
| `rounded-md` | 10 | icon buttons, xs buttons, tooltips, kbd |
| `rounded-lg` | 12 | buttons, inputs, chips, choice rows, popovers |
| `rounded-xl` | 16 | work card, footer bar, alerts, tooltips-as-cards |
| `rounded-2xl` | **24** | cards (kit + COSS), dialogs, link tiles |
| `rounded-3xl` | 32 | — |

Brand surfaces use explicit pixel radii instead — keep these exact:

| Radius | Element |
|---|---|
| `rounded-[15px]` | brand PANEL (dashboard, leaderboard, practice, curator, locked result) |
| `rounded-[12px]` | plan rows, leaderboard rows, brand CTAs |
| `rounded-[10px]` | nav items, weak-skill rows, date chips, stepper |
| `rounded-[9px]` | sidebar small buttons, avatar square, banner CTAs, mobile nav items |
| `rounded-[7px]` / `[6px]` | ProTag outer / inner, tiny stepper buttons |
| `rounded-[3px]` / `[2px]` | heatmap cells / meter segments |
| `rounded-[18px]` | paywall modal |
| `rounded-[24px]`, `[28px]`, `[32px]`, `[35px]` | marketing mock frame / pricing card / hero card / auth image |
| `rounded-full` | pills, status chips, bullets, marketing CTAs & fields |

### 3.2 Borders

Everything gets `border-border` by default (`* { @apply border-border outline-ring/50 }`).
Dividers: `divide-y divide-border`, `border-b border-border`. Section splits in solving
UI use `border-dashed`. Hairline on marketing: `border-[#0a0d15]/[0.06]…/[0.08]`.

### 3.3 Elevation

Three levels only:

1. **Flat** — brand PANEL: border, **no shadow**.
2. **Whisper** — kit card: `shadow-xs`; COSS surfaces: `shadow-xs/5` + inner 1px highlight (below).
3. **Floating** — popovers/menus `shadow-lg/5`; navigator & slide-over `shadow-2xl shadow-black/30…/50`; confirm dialog `shadow-2xl shadow-black/50`.

**COSS inner-highlight trick** (buttons, inputs, cards, popups):

```
border bg-X not-dark:bg-clip-padding shadow-xs/5
before:pointer-events-none before:absolute before:inset-0
before:rounded-[calc(var(--radius-*)-1px)]
before:shadow-[0_1px_--theme(--color-black/4%)]
dark:before:shadow-[0_-1px_--theme(--color-white/6%)]
```

Light: 1px darker lip at the bottom. Dark: 1px white catch-light at the top. Removed on `:disabled`, `:active`, `[data-pressed]`.

Coloured glow is used exactly twice: primary button `shadow-xs shadow-primary/24` and ProTag `shadow-sm shadow-primary/25` (plus marketing hero `shadow-2xl shadow-[#3755ed]/25`).

---

## 4. The Plus — brand motif

The only decorative texture the identity has. **Never substitute lucide `Plus`.**
Source: `src/components/brand/PlusField.tsx`, `src/index.css`.

### 4.1 `PlusMark` glyph

```svg
<svg viewBox="0 0 20 20" fill="currentColor">
  <rect x="7.813" y="0" width="4.373" height="20" rx="0.733"/>
  <rect x="0" y="7.813" width="20" height="4.373" rx="0.733"/>
</svg>
```

Bar thickness 4.373 / 20, corner radius 0.733 — don't round these. Colour = `currentColor`, almost always `text-primary`.

Sizes in use: 6 (mobile nav active), 8 (sidebar active marker), 9 (feature bullets, sidebar field), 11 (banner lead, avatar fallback), 12 (page title / modal masthead), 13 (leaderboard fields), 20 (slider handle).

### 4.2 `PlusField` — patterned field

Authored as strings, one char per cell: `B` = bright (`text-primary`), `F` = faint (`text-primary/15`), `.` = empty.
Column gap = `cell × 1.375`, row gap = `cell × 1.125` (pitch 27.5 × 22.5 at cell 20).

```tsx
<PlusField pattern={["B.F.", ".F.B", "F..F", ".B.."]} cell={9} className="mt-auto px-2.5 pt-8" /> // sidebar foot
<PlusField pattern={["B.F..F.B", ".F.B..F.", "F..F.B.B"]} cell={9} />                            // paywall
<PlusField pattern={["B.F.", ".F.B", "F..F"]} cell={13} />                                      // empty-state "icon"
```

Options:
- `twinkle` — bright marks breathe: `.plus-twinkle` 7s ease-in-out infinite, opacity 1 → 0.45 (never lower). Delay deterministic: `((r*7 + i*3) % 11) * 0.55s`.
- `interactive` — click a mark and it falls: `.plus-fall` 750ms `cubic-bezier(0.5,0,0.75,0.4)` to `translateY(90px) rotate(26deg); opacity:0`. The easter egg.

### 4.3 `.plus-rail` — progress drawn in plusses

For the *remaining* part of a progress track (leaderboard, dashboard points, curator):

```css
.plus-rail {
  background-color: color-mix(in srgb, var(--primary) 32%, transparent);
  mask-image: var(--plus-mask);          /* 47.5×20 tile, plus centred */
  mask-size: 21.4px 9px;
  mask-repeat: space no-repeat;          /* only whole marks */
  mask-position: 0 center;
}
```

Composition: solid `bg-primary rounded-full` fill of width W%, then a `.plus-rail` span from `left: calc(W% + 7px)` to the right edge. **No grey track behind it.**

### 4.4 Where the plus replaces conventional UI

| Conventional | 1609SAT |
|---|---|
| Active nav pill | 8px PlusMark hanging in the left padding (`absolute -left-1 top-1/2 -translate-y-1/2 text-primary`), no background |
| Check-mark feature list | 9px PlusMark bullet `mt-[5px] text-primary` + `text-[13px] leading-[1.35] text-foreground/85` |
| Empty-state icon | small PlusField |
| Ellipsis between leaderboard rows | vertical stack of three plusses, cell 6, `opacity-40` |
| Avatar fallback | PlusMark 11px (bright for top-3 & viewer, `/25` otherwise) |
| Slider thumb | PlusMark 20px, `group-hover:scale-110`, `scale-125` while dragging |
| Grey progress track | `.plus-rail` |
| Section decoration | twinkling PlusFields in page gutters at `2xl` |

---

## 5. Signature marks

### 5.1 ProTag — gradient-rim frame

Not a pill. A 1px gradient rim catching the light on one corner, tracked "PRO" inside.

```tsx
<span className="inline-flex shrink-0 rounded-[7px] p-px shadow-sm shadow-primary/25"
  style={{ background: "linear-gradient(135deg, var(--primary), color-mix(in srgb, var(--primary) 30%, transparent) 55%, color-mix(in srgb, var(--primary) 12%, transparent))" }}>
  <span className="rounded-[6px] px-1.5 py-[3px] text-[9px] font-bold uppercase leading-none tracking-[0.16em] text-primary bg-background">Pro</span>
</span>
```

The inner `bg-*` **must equal the surface underneath** (`bg-sidebar`, `bg-card`…), otherwise it reads as a filled chip.
The same rim idea: Pro avatar = 1px conic-gradient rim of primary around `rounded-[8px] bg-sidebar`; marketing `MockFrame` = 1px `linear-gradient(150deg, rgba(255,255,255,0.95), rgba(10,13,21,0.10))` rim.

### 5.2 Wordmark

`src/assets/1609sat.svg` (white, for dark) / `1609sat-dark.svg` (black, for light), 3826×1757. Swap with `hidden dark:block` / `block dark:hidden`. Heights: `h-7` sidebar, `h-8` marketing header, `h-10` footer. Import it (bundled) — a root-absolute `src` skips the `/app` basePath.

### 5.3 Cursors

macOS cursor set recoloured to `#3755ed` outline / white fill, in `public/cursors/*.svg`, exposed as CSS vars and remapped Tailwind `cursor-*` utilities:

```css
--cursor-default: url("/app/cursors/default.svg") 8 5, default;
--cursor-pointer: url("/app/cursors/pointer.svg") 12 8, pointer;
--cursor-not-allowed: url("/app/cursors/not-allowed.svg") 4 2, not-allowed;
--cursor-grab / --cursor-grabbing: … 16 16;  --cursor-move / --cursor-ew-resize: … 9 9;
--cursor-text: text;  /* no custom I-beam */
```

`html` → default, `a[href]` → pointer, buttons/selects → **default arrow** (not pointer), text fields → system I-beam.

### 5.4 Scrollbars

Thin, foreground-tinted: `scrollbar-color: color-mix(in srgb, var(--foreground) 18%, transparent) transparent`; WebKit 10px, thumb `/16%` (hover `/30%`), `border-radius: 8px`, `border: 2px solid transparent; background-clip: padding-box`.

---

## 6. Layout

### 6.1 App shell (`components/layout/AppShell.tsx`)

```
┌──────────┬──────────────────────────────────────────┐
│ Sidebar  │ Header h-14 (sticky, blurred)            │
│ 248px    ├──────────────────────────────────────────┤
│ (68px    │ <main> overflow-y-auto                   │
│ collapsed│   page wrapper p-4 sm:p-6 lg:p-10        │
│ )        │     max-w-6xl / 5xl / 3xl mx-auto        │
└──────────┴──────────────────────────────────────────┘
mobile: sidebar hidden → fixed bottom nav (5 items)
```

Root: `flex h-dvh max-h-dvh overflow-hidden bg-background`.

**Sidebar**
- aside: `hidden shrink-0 flex-col border-r border-sidebar-border bg-sidebar font-geist tracking-[-0.02em] transition-[width] duration-300 ease-in-out md:flex` + `w-[248px]` / `w-[68px]`, `style={SS08}`.
- Logo row: `flex h-14 items-center justify-between px-4`; collapse button `grid size-8 place-items-center rounded-[9px] text-muted-foreground/70 hover:bg-foreground/[0.06] hover:text-foreground` (`PanelLeftClose`/`PanelLeft` size-4).
- Nav: `flex min-h-0 flex-1 flex-col overflow-y-auto px-3 pb-2`. Sections **Prep** (Dashboard, Leaderboard, Question Bank, AI Study Plan) · **Simulate** (Practice Tests, Blitz, Score Calculator) · **Insights** (Analytics, Saved, Review Queue).
- Section label: `mb-1 mt-4 px-2.5 text-[11px] text-muted-foreground/45`; collapsed → `mx-2.5 my-2.5 border-t border-sidebar-border`.
- Item stack `flex flex-col gap-0.5`. **NavItem**:
  ```
  group relative flex select-none items-center rounded-[10px] transition-[background-color,color,transform] duration-150 active:scale-[0.98]
  expanded: h-9 gap-2.5 px-2.5     collapsed: mx-auto size-9 justify-center (+ bg-primary/[0.12] when active)
  inactive: hover:bg-foreground/[0.045]
  icon: strokeWidth 1.75, size-[18px]; active text-primary, else text-muted-foreground/70 group-hover:text-foreground/80
  label: flex-1 truncate text-sm tracking-[-0.02em]; active font-medium text-foreground, else text-muted-foreground
  active marker: <PlusMark size={8} className="absolute -left-1 top-1/2 -translate-y-1/2 text-primary"/>
  badge: rounded-full bg-primary/[0.14] px-1.5 py-px text-[10.5px] font-medium tabular-nums text-primary
  ```
- PlusField fills leftover height at the foot of the nav (§4.2).
- Bottom links (Support, Settings): `px-3 pb-2 pt-1`.
- User block: `border-t border-sidebar-border p-3` → link `-m-1 flex gap-2.5 rounded-[10px] p-1 hover:bg-foreground/[0.045]`; avatar **rounded square** `size-7 rounded-[9px] bg-primary/[0.12] text-[11px] font-bold text-primary`; email `truncate text-[13px] font-[450]` + ProTag; streak line `text-[11px] tabular-nums text-muted-foreground` with `Flame size-3`, `·` separator `text-muted-foreground/50`, "Best Nd" `/70`; settings cog appears on hover. Sign out: `mt-2 w-full rounded-[9px] px-2.5 py-1.5 text-left text-[12px] text-muted-foreground/70 hover:bg-foreground/[0.045]`.

**Header**: `sticky top-0 z-30 flex h-14 items-center justify-between gap-4 border-b border-border bg-background/85 px-4 backdrop-blur-md sm:px-6`. Right cluster `ml-auto flex items-center gap-2`: Upgrade (free) `inline-flex h-8 items-center gap-1.5 rounded-lg bg-amber-400 px-3 text-[12.5px] font-semibold text-amber-950 hover:bg-amber-300 active:scale-[0.97]` with `Zap h-3.5`; or ProTag (pro); theme toggle `grid size-9 place-items-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground` (`Sun`/`Moon` size-[18px]). Wordmark shows in header only below `md`.

**Mobile bottom nav**: `fixed inset-x-0 bottom-0 z-40 flex items-center justify-around border-t border-border bg-background/90 py-2 font-geist tracking-[-0.02em] backdrop-blur-md md:hidden`. Item `flex-col gap-0.5 rounded-[9px] px-3 py-1 active:scale-[0.94]`, icon `size-5 strokeWidth 1.75`, label `text-[9px]` (first word), active `text-primary font-medium` + `PlusMark size={6}` at `absolute right-1.5 top-0`.

**Page wrapper**: `min-h-full p-4 pb-24 sm:p-6 md:pb-10 lg:p-10` → `mx-auto` + max width:

| Max width | Pages |
|---|---|
| `max-w-6xl` | Dashboard, Practice Tests, Study Plan, curator |
| `max-w-5xl` | Analytics, Leaderboard, Saved, Review Queue, Question Bank hub, Formulas, locked result |
| `max-w-3xl` | Settings |

Vertical rhythm: `space-y-4` on brand pages, `space-y-6` on kit pages.

### 6.2 Grids

- Stat row: `grid grid-cols-2 gap-3 lg:grid-cols-4` (kit) or 4 cells inside one PANEL `grid grid-cols-2 lg:grid-cols-4 divide-x divide-y divide-border lg:divide-y-0` (brand).
- Dashboard working row: `grid items-start gap-4 lg:grid-cols-3`.
- Cards: `grid gap-4 sm:grid-cols-2 xl:grid-cols-3` (papers), `grid-cols-1 sm:grid-cols-2 gap-4 sm:gap-5` (hub), `grid gap-4 lg:grid-cols-2` (analytics).

### 6.3 Breakpoints

`md` sidebar ↔ bottom nav · `sm` padding step, leaderboard columns · `lg` 4-up stats, `p-10` · `2xl` gutter decorations. COSS controls are **one step taller on mobile** (`h-9 sm:h-8`, `text-base sm:text-sm`) and have 44px coarse-pointer hit areas.

---

## 7. Surfaces & cards

There are two card languages. Pick by page register (§2).

### 7.1 Brand PANEL (Geist pages)

```tsx
const PANEL = "relative rounded-[15px] border border-border bg-card";   // no shadow
```

- Header: `flex items-baseline justify-between border-b border-border px-4 py-3`; h2 `text-[14.5px] font-[450] tracking-[-0.02em]`; right note `text-[11.5px] tabular-nums text-muted-foreground/60`.
- Body: `p-4`.
- Footer link row: `border-t border-border px-4 py-3 text-[12.5px] font-medium text-primary` + `ArrowRight` `transition-transform group-hover:translate-x-0.5`.
- Hover (clickable tiles/cards): `hover:-translate-y-px hover:border-primary/30`.
- Error: `PANEL + border-destructive/25 bg-destructive/[0.04] p-6 text-center`.

**Stat cell inside a PANEL**:

```tsx
<div className="min-w-0 px-4 py-4 sm:px-5">
  <p className="truncate text-[10.5px] text-muted-foreground/55">Predicted score</p>
  <p className="mt-1.5 flex items-baseline gap-1.5">
    <span className="text-[32px] font-[450] leading-none tracking-[-0.035em] tabular-nums text-foreground">1340</span>
    <span className="text-[11px] text-muted-foreground/55">/1600</span>
  </p>
  <p className="mt-1.5 text-[11.5px] tabular-nums text-muted-foreground/65">+40 since last mock</p>
</div>
```

Empty value: `—` in `text-muted-foreground/30`, never a fabricated `0`.

### 7.2 Kit Card (Inter pages)

```tsx
"relative flex flex-col rounded-2xl border border-border bg-card text-card-foreground shadow-xs"  // p-5 sm:p-6 typical
```

- **StatTile**: Card `rise relative overflow-hidden p-4`; label/value per §2.2; ghost icon `pointer-events-none absolute -bottom-2 -right-1 h-16 w-16 text-foreground/[0.04]` bleeding off the corner.
- **LinkTile**: `group relative flex h-full flex-col rounded-2xl border border-border bg-card p-5 text-left shadow-xs hover:border-primary/40 hover:bg-accent/50` + PRESSABLE; ends with **ArrowCta** `mt-auto inline-flex items-center gap-1.5 pt-3 text-[13px] font-medium text-primary` + `ArrowRight h-3.5 w-3.5 group-hover:translate-x-0.5`.
- **Source card** (Question Bank hub): `min-h-[176px] rounded-2xl border bg-card p-5 sm:p-6 shadow-xs hover:border-primary/40 hover:bg-accent/40 active:scale-[0.99]` with a line illustration in `text-primary` bleeding off the right (`-right-3 w-[46%]`), detail by stroke/fill opacity.
- **IconChip**: `flex shrink-0 items-center justify-center border border-border bg-accent text-muted-foreground` — sm `h-8 w-8 rounded-lg`, md `h-10 w-10 rounded-lg`, lg `h-12 w-12 rounded-xl`. Neutral always.
- **Danger zone**: `rounded-2xl border border-destructive/25 bg-destructive/[0.03] p-5 sm:p-6`.

### 7.3 COSS primitives (`components/ui/*`)

Available but mostly unused by pages (kit.tsx dominates). When used:
`Card` = `rounded-2xl border bg-card shadow-xs/5` + inner highlight; `CardHeader p-6 gap-1.5`; `CardTitle font-heading font-semibold text-lg leading-none`; `CardPanel p-6`; `CardFooter p-6`. `CardFrame` = same shell with `before:bg-muted/72` tray holding nested cards. `Frame` = `rounded-2xl bg-muted/72 p-1` tray around `rounded-xl border bg-background p-5` panels.

---

## 8. Controls

### 8.1 Buttons

Shared press feel (`kit.tsx`):

```ts
const PRESSABLE = "select-none transition-[transform,background-color,border-color,color] duration-150 ease-out-expo active:scale-[0.97]";
```

| Button | Classes |
|---|---|
| **GlowButton** (primary CTA) | `group inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-primary px-5 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-40` + PRESSABLE |
| **AccentButton** | `rounded-lg border border-primary/25 bg-primary/10 px-4 py-2 text-sm font-medium text-primary hover:bg-primary/15` |
| **GhostButton** | `rounded-lg border border-border bg-transparent px-4 py-2 text-sm text-foreground/70 hover:bg-accent hover:text-foreground` |
| Brand CTA | `rounded-[12px] bg-primary px-4 py-2.5 text-[13.5px] font-medium text-primary-foreground hover:opacity-90 active:scale-[0.97]` |
| Brand quiet CTA | `rounded-[12px] border border-border px-4 py-2.5 text-[13.5px] font-medium text-muted-foreground hover:bg-accent hover:text-foreground active:scale-[0.97]` |
| Confirm (dialog) | `h-11 rounded-xl px-5 text-sm font-medium active:scale-[0.97]` `bg-primary` / `bg-destructive` |
| Text link | `text-[12.5px] font-medium text-primary` (+ nudging arrow) |

**COSS `Button`** (`ui/button.tsx`): base `rounded-lg border font-medium text-base sm:text-sm`, focus `ring-2 ring-ring ring-offset-1 ring-offset-background`, `disabled:opacity-64`, loading = absolute spinner + `text-transparent`.

| Size | mobile / sm+ | pad-x |
|---|---|---|
| xs | `h-7` / `h-6`, `rounded-md text-sm sm:text-xs` | 7px |
| sm | `h-8` / `h-7` | 9px |
| default | `h-9` / `h-8` | 11px |
| lg | `h-10` / `h-9` | 13px |
| xl | `h-11` / `h-10`, `text-lg sm:text-base` | 15px |
| icon(-xs/-sm/-lg/-xl) | `size-7/8/9/10/11` → one smaller at sm | — |

Variants: **default** `border-primary bg-primary shadow-xs shadow-primary/24` + `inset-shadow-[0_1px_--theme(--color-white/16%)]` (pressed → `inset-shadow black/8%`) · **outline** `border-input bg-popover shadow-xs/5 hover:bg-accent/50 dark:bg-input/32 dark:hover:bg-input/64` + inner highlight · **secondary** `bg-secondary hover:bg-secondary/90` · **ghost** `border-transparent hover:bg-accent` · **destructive / destructive-outline** · **link**.

### 8.2 Chips, pills, tabs

| Element | Classes |
|---|---|
| Kit **Pill** (status) | `inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10.5px] font-semibold uppercase tracking-[0.06em]` + tone `border-X/30 bg-X/10 text-X` (X = info/success/warning/destructive/primary; neutral `border-border bg-accent text-muted-foreground`) |
| Card chip (sentence case) | `rounded-full border px-[9px] py-[3px] text-[10px] font-medium` — Free `border-primary/40 bg-primary/[0.12] text-primary`, In progress `border-warning bg-warning/10 text-warning`, Completed `border-success bg-success/10 text-success` |
| Stamp "New" | `bg-primary text-primary-foreground text-[10px] font-bold uppercase tracking-[0.08em]` |
| "You" pill | `rounded-full border border-primary/30 bg-primary/10 px-1.5 py-px text-[10px] font-medium text-primary` |
| Filter pill | `rounded-full border px-3 py-1 text-[12.5px] font-medium`; on `border-primary/40 bg-primary/[0.12] text-primary`; off `border-border text-muted-foreground hover:bg-accent` |
| Filter chip (`ui/filters.tsx`) | `rounded-lg border px-2.5 py-1 text-[11.5px] font-medium`; on `border-primary/50 bg-primary/[0.12] text-primary`; off `border-border bg-background text-muted-foreground`; dead `text-muted-foreground/35` |
| Status tabs | `rounded-lg px-3 py-1.5 text-[13px] font-medium`; active `bg-primary/[0.12] text-primary`; count bubble `h-[18px] min-w-[18px] rounded-full text-[10.5px] font-semibold` (`bg-primary/20` / `bg-muted`) |
| Segmented control | container `inline-flex rounded-lg border border-border bg-muted p-0.5 text-[12px]`; item `rounded-md px-2.5 py-1 font-medium`; active `bg-accent text-foreground shadow-xs` (admin: `bg-card`) |
| Underline tabs (report) | `-mb-px border-b-2 pb-2.5 text-[13px]`, active `border-primary font-medium` |
| Stepper | `rounded-[10px] border p-0.5` with two `size-7 rounded-[7px]` buttons and `h-4 w-px bg-border` divider |
| Kbd | `inline-flex h-6 min-w-6 items-center justify-center rounded-md border border-border bg-muted px-1.5 text-[12px] font-medium` |

### 8.3 Form fields

- COSS Input: wrapper `inline-flex w-full rounded-lg border border-input bg-background shadow-xs/5 text-base sm:text-sm` + inner highlight; focus `border-ring ring-[3px] ring-ring/24`; invalid `border-destructive/36`; inner `h-8.5 sm:h-7.5 px-[calc(--spacing(3)-1px)] placeholder:text-muted-foreground/72` (total 36/32px).
- Hand-rolled field (support widget etc.): `h-9 rounded-lg border border-input bg-background px-3 text-sm focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring`; label `text-[11px] font-semibold uppercase tracking-[0.06em] text-muted-foreground`.
- Checkbox `size-4.5 sm:size-4 rounded-[.25rem] border-input`, checked fills `bg-primary`. Brand round check: `size-4 rounded-full` → done `bg-success/15 text-success`, todo `border border-input`.
- Radio (plan picker): `size-4 rounded-full border` (`border-primary` / `border-input`) with `size-[7px] rounded-full bg-primary` dot.
- Switch: thumb `size-5 / sm:size-4`, track `rounded-full p-px data-checked:bg-primary data-unchecked:bg-input`, thumb stretches `scale-x-110` while pressed.
- Marketing/auth field: `flex w-full items-center gap-3 rounded-full border border-[#0a0d15]/30 px-6 py-[15px] focus-within:border-[#3755ed]/60`.

### 8.4 Progress & meters

| Meter | Recipe |
|---|---|
| Kit `Bar` | track `h-1.5 overflow-hidden rounded-full bg-muted`, fill `bg-primary rounded-full transition-[width] duration-700`, animates from 0 on reveal |
| Score rail | `h-2.5` track `bg-primary/[0.15]`, fill `bg-primary`; goal tick `absolute -top-1 h-[18px] w-[2px] rounded-full bg-foreground/45` |
| Plus-rail | §4.3; leaderboard fades rows: `strength = 0.34 + (W/100)·0.66`, rail `0.55 + strength·0.45`, viewer row = 1 |
| Thin task meter | `h-1 rounded-full bg-foreground/10` |
| Weak-skill meter | `h-1 bg-primary/[0.12]` filled by band: ≥70 success, ≥50 warning, else destructive |
| 7-segment domain meter | `flex gap-[3px]`, segments `h-1.5 flex-1 rounded-[2px]`, filled `bg-primary`, empty `bg-foreground/10`, 45ms stagger |
| Module track | four `h-2.5 flex-1 rounded-[3px]` cells, `ml-2` before 3rd (section break): done `bg-primary`, current `bg-primary/30 ring-1 ring-primary/60`, pending `bg-primary/[0.15]` |
| Round tracker (Blitz) | 10 × `h-1.5 flex-1 rounded-full`: `bg-success`/`bg-destructive`, current `animate-pulse bg-primary`, rest `bg-muted` |
| Donut (kit) | SVG r=78 / viewBox 200, stroke 12, track `var(--border)`, butt caps, dash 1.4s `cubic-bezier(0.22,1,0.36,1)` |

### 8.5 Lists & rows

- Brand row: `rounded-[12px] border px-3.5 py-3`, gap `1.5` between rows; self `border-primary/30 bg-primary/[0.07]`; others `border-border hover:bg-foreground/[0.025]`.
- Divided list row (Saved / Review): `-mx-2 flex gap-3 sm:gap-4 rounded-lg px-2 py-3 hover:bg-accent` in `divide-y`; index `w-6 text-[14px] font-semibold tabular-nums text-muted-foreground/70` zero-padded (`01`); meta `text-[11px] text-muted-foreground` with ` · `; title `text-[14px] font-medium truncate`; remove `size-8 text-muted-foreground/40 hover:text-destructive`.
- Task row: `flex gap-3 py-2.5` in `ul.divide-y px-4`; label `text-[13px] text-foreground/85`, done `line-through text-muted-foreground`; count `w-11 text-right text-[11px] tabular-nums`.
- Definition rows (settings): `flex gap-3 border-b py-3 first:pt-0 last:border-b-0`, dt `w-28 sm:w-32 text-[13px] text-muted-foreground`, dd `text-[13.5px] font-medium`.
- Meta separator is always ` · ` in `text-muted-foreground/50`.

### 8.6 Empty, loading, locked

- **EmptyState** (kit): `flex flex-col items-center justify-center gap-3 py-10 text-center`; icon disc `h-11 w-11 rounded-full bg-primary/10 text-primary ring-1 ring-primary/20`; title `text-sm font-semibold sm:text-base`; desc `mx-auto mt-1 max-w-sm text-xs text-muted-foreground sm:text-sm`.
- Brand empty: one sentence `text-[12.5px] leading-relaxed text-muted-foreground` + primary text link; or `Notice` = PlusField + `text-[17px] font-[450]` title, `py-12`.
- Skeletons: layout-shaped `animate-pulse rounded bg-muted` blocks; bars `bg-foreground/[0.06]`. (Shimmer `ui/skeleton.tsx` exists: 120° gradient, white/64% light, white/4% dark, `skeleton 2s -1s infinite linear`.)
- Pending link: overlay `absolute inset-0 z-10 rounded-[inherit] bg-background/55 backdrop-blur-[1px]` + `Loader2 h-4 w-4 animate-spin text-primary`.
- **LockedSection**: content `opacity-40 blur-[2px] inert`; overlay `rounded-2xl bg-gradient-to-b from-transparent via-background/30 to-background/60`; lock tile `h-11 w-11 rounded-xl border bg-card shadow-xs` (18px lock at `/70`); title `font-heading text-[16px] font-semibold tracking-tight`; desc `max-w-[280px] text-[12.5px] leading-relaxed`.

---

## 9. Solving surfaces (question bank, exam, blitz)

Full-screen routes (`app/(full)`) have no app shell.

### 9.1 Session frame

- Outer: `h-dvh overflow-hidden bg-muted dark:bg-background`; inner `style={{ zoom: 1.2, height: "100%" }}` — **everything renders at 120%**.
- Header: `grid h-14 grid-cols-[1fr_auto_1fr] items-center px-1`, transparent. Timer `font-heading text-[17px] font-semibold tabular-nums tracking-tight` (exam countdown 19px; ≤5 min → `text-destructive`, cannot be hidden; hidden → `--:--`).
- Top tools: text `inline-flex h-9 items-center gap-1.5 rounded-lg border px-3 text-[13px] font-medium`, icon `h-9 w-9 rounded-lg border`; neutral `border-border text-muted-foreground hover:bg-accent hover:text-foreground/80`; active calculator/saved `border-primary/40 bg-primary/[0.12] text-primary`; active Highlights & Notes `border-warning/50 bg-warning/[0.14] text-warning`.
- **Work card**: `mx-auto max-w-[1200px] flex-1 overflow-hidden rounded-xl border border-border bg-card`.
- **Footer bar** (floating card): `mb-2 mt-0.5 flex h-14 items-center justify-between rounded-xl border border-border bg-card px-2.5 sm:px-3`. Buttons `h-10 rounded-xl border px-3.5 text-[13px] font-medium disabled:opacity-45`. Centre: navigator trigger `h-10 rounded-xl border px-3.5` with `LayoutGrid` + `{i}/{total}` (slash `/50`). Next = `border-foreground bg-foreground text-background`; Submit = `border-primary/50 bg-primary text-primary-foreground`.
- Run strip: `mb-1.5 rounded-xl border px-3 py-1.5`, `text-[12px]`, progress `h-1 rounded-full bg-foreground/10`.

### 9.2 Question header strip

`flex items-center gap-3 border-b border-dashed border-border px-5 py-2.5 sm:px-6`
- Number badge: `grid h-7 w-7 place-items-center rounded-full bg-foreground text-[13px] font-bold tabular-nums text-background` (inverts with theme).
- Classification: `text-[12px] text-muted-foreground`, separators `/50`.
- Difficulty pill: `rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.06em]` — EASY success, MEDIUM warning, HARD destructive (`border-X/30 bg-X/10 text-X`). Hidden during exams.
- Mark for review: `h-7 rounded-lg border px-2.5 text-[12px]`; flagged `border-warning/50 bg-warning/[0.14] text-warning` + `Flag fill-current`.

### 9.3 Split layout

- Passage | question, default 50%, draggable 30–70% (persisted `sessionStorage["sat1609.qbank.split"]`), question pane `md:min-w-[300px]`, stacked below `md`. Math = single centred column.
- Passage pane `px-5 py-6 sm:px-8`.
- Divider: hit area `w-3 cursor-col-resize`, rule `w-px bg-border`, grip `h-9 w-[5px] rounded-full bg-border group-hover:bg-muted-foreground/60`; double-click resets.

### 9.4 Answer choices (`components/question/choice.tsx`)

Row: `group/opt relative flex min-h-[48px] items-center gap-3 rounded-lg border px-3.5 py-2 text-left active:scale-[0.99]`

| State | Row | Bullet (`h-6 w-6 rounded-full border text-[12.5px] font-semibold`) |
|---|---|---|
| idle | `border-border bg-card hover:border-primary/30 hover:bg-accent` | `border-border text-muted-foreground group-hover/opt:border-primary/40` |
| picked | `border-primary bg-primary/[0.07] ring-1 ring-primary/40` | `bg-primary text-primary-foreground` |
| correct | `border-success/50 bg-success/[0.10]` | `bg-success text-success-foreground` |
| wrong | `border-destructive/50 bg-destructive/[0.10]` | `bg-destructive text-destructive-foreground` |
| correctSoft / wrongSoft (Blitz) | `border-X/70 bg-X/[0.1] ring-1 ring-X/40` | `bg-X/15 text-X` |
| eliminated | `opacity-45` + 1px rule `absolute inset-x-3 top-1/2 h-px -translate-y-1/2 bg-muted-foreground` (not `line-through`) | |

- ABC eliminate toggle: `rounded-md border px-2 py-1 text-[11px] font-bold tracking-wide`, on `border-foreground bg-foreground text-background`, strike = `-rotate-[10deg]` 1px line.
- Per-choice cross-out column `w-10`; crossed → "Undo" `text-[12px] font-semibold text-primary underline`.
- Graded: right-side pill "Correct answer"/"Your answer" `rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.06em]`; rationale `border-t border-border/60 pt-2 text-[13px]`.

### 9.5 Navigator & explanation

- Navigator panel (bottom-anchored over `bg-black/40`): `max-w-[440px] max-h-[520px] rounded-2xl border bg-popover p-3 shadow-2xl shadow-black/30`, enters +8px rise + fade.
- Grid `grid grid-cols-6 gap-1.5`, cell `relative flex h-10 items-center justify-center overflow-hidden rounded-lg border`, font shrinks by digit count (12 → 10 → 8.5 → 7px).

  | Cell | Classes |
  |---|---|
  | correct | `border-success/25 bg-success/15 text-success` |
  | incorrect | `border-destructive/25 bg-destructive/15 text-destructive` |
  | corrected | `border-warning/30 bg-warning/20 text-warning` |
  | answered | `border-transparent bg-foreground/10 text-foreground/80` |
  | unsolved | `border-border/60 text-muted-foreground/70` |
  | current | + `ring-2 ring-foreground ring-offset-1 ring-offset-popover font-bold` |

  Difficulty dot `absolute right-1 top-1 h-1.5 w-1.5 rounded-full` (`bg-success/warning/destructive`); saved = top-left fold `border-r-[9px] border-t-[9px] border-r-transparent border-t-primary`.
- Exam navigator: `grid-cols-8 gap-x-2 gap-y-3`, cells `h-9 w-9 rounded-lg`; answered `border-primary/40 bg-primary/[0.12] text-primary`; unanswered `border-dashed border-border`; current `ring-2 ring-foreground/70` + `MapPin` above; flagged `Flag fill-warning` at corner.
- Slide-over explanation: docked `w-[400px] border-l`, overlay `fixed right-0 w-[min(460px,94vw)] shadow-2xl shadow-black/50`; header `h-13 border-b px-4`, title `text-[13.5px] font-semibold`. Motion `cubic-bezier(0.22,1,0.36,1)` 280ms in / 200ms out.
- Verdict line: `rounded-lg border px-3 py-2`, `text-[13px] font-semibold` — "Correct" `border-success/30 bg-success/10`, "Not quite" destructive, none `bg-muted`.
- Explanation card: `relative rounded-lg border bg-card p-3.5 pl-4` with a `w-[3px] bg-primary` left bar; eyebrow `text-[10px] uppercase tracking-[0.14em]` + `Lightbulb text-primary`.

### 9.6 Highlights & notes (Bluebook parity)

Painted via CSS Custom Highlight API; text under marks stays `#1e1e1e` in both themes.

| Colour | Swatch | Resting `::highlight(hl-*)` | Active |
|---|---|---|---|
| Yellow | `#fdf3bd` | `#fdf1a9` | `#ffde00` |
| Blue | `#e7f2fc` | `#d8ecfb` | `#93cdf6` |
| Pink | `#fbe5f3` | `#fbdcee` | `#f4a3d3` |

Underlines: `underline solid|dashed|dotted 2px`. Toolbar: `rounded-full border border-[#d6d6d6] bg-white px-2 py-1.5 shadow-[0_4px_18px_rgba(0,0,0,0.18)]`, tool circles `size-[34px] rounded-full`, swatch border `1.5px #8f8f8f` → selected `2.5px #4f4f4f`. Notes column `w-[184px] bg-[#f0f0f0] dark:bg-white/[0.035]`, note cards `rounded-md border-[1.5px] border-[#3a3a3a] bg-white` (active `border-[#1e1e1e] shadow-[0_2px_8px_rgba(0,0,0,0.12)]`), header tinted with the mark colour. Drawer `duration-300 ease-[cubic-bezier(0.32,0.72,0,1)]`.

### 9.7 Question HTML (`.rich-text`)

Restores what Preflight strips: lists `margin: 1em 0; padding-left: 1.5em` (disc/decimal, `li` 0.2em), tables `border-collapse` with `1px solid currentColor` cells `padding: .45em .7em`, `th` bold, `figure.table { display:block; overflow-x:auto }`, `h1` 1.1em bold, blockquote `border-left: 2px solid currentColor`, images `max-width:100%` centred in block context, paragraphs `margin: 0 0 1em` (block only).

### 9.8 Bluebook replica (fidelity reference, `BluebookRunner.tsx`)

Always light, hard-coded: page `#fff`, ink `#1e1e1e`, blue `#324DC7`, serif content `'Minion Pro', Georgia, 'Times New Roman', serif` at `leading-[1.7]` (15px R&W / 16px Math), system sans chrome. Header `h-16 border-b border-gray-300`, timer `text-[22px] font-semibold`. Question strip `border-b-2 border-dashed border-gray-400 bg-[#f4f4f4]`, **square** black number badge. Choices `rounded-lg border border-gray-500 px-4 py-3`, selected `border-[#324DC7] shadow-[inset_0_0_0_1px_#324DC7]`, bullet 26px `border-2 border-gray-800` → filled blue. Footer `h-[72px]`, centre black pill "Question N of M ▲", Back/Next `rounded-full px-7 py-2.5 bg-[#324DC7]`. Divider solid `w-[3px] bg-gray-300`. The live branded runner (`Sat1609Runner`) uses §9.1–9.5 instead.

### 9.9 Blitz

Header `border-b px-4 py-3 max-w-5xl`; logo tile `h-6 w-6 rounded-md bg-warning/15 text-warning` + `Zap`. Hearts `h-[18px] w-[18px]` `fill-destructive text-destructive` / empty `text-muted-foreground/40`; lost heart `.blitz-heart-fall`. Streak `Flame` + `x{n}` `text-[13px] font-bold`, `text-warning` at ≥3. Timer is a digit-less `h-[3px]` bar: primary > 50%, warning > 20%, destructive below, `width 120ms linear`. Wrong answer → `.blitz-shake` on `<main>`. Stat tiles `rounded-xl border bg-muted px-4 py-4`, labels `text-[11px] uppercase tracking-[0.12em]`.

---

## 10. Overlays

| Overlay | Recipe |
|---|---|
| COSS Dialog | backdrop `fixed inset-0 z-50 bg-black/32 backdrop-blur-sm`; viewport `grid grid-rows-[1fr_auto_3fr] p-4` (upper third; pass `sm:grid-rows-[1fr_auto_1fr]` to centre); popup `max-w-lg rounded-2xl border bg-popover shadow-lg/5` + highlight, enters `scale-98` + fade, 200ms; mobile = bottom sheet `max-sm:rounded-none` sliding `translate-y-4`; header `p-6 gap-2`, title `font-heading text-xl font-semibold leading-none`; footer tray `border-t bg-muted/72 px-6 py-4 sm:justify-end` |
| ConfirmDialog | overlay `z-[100] bg-black/60 backdrop-blur-sm`; panel `rise max-w-md rounded-2xl border border-border bg-popover p-6 shadow-2xl shadow-black/50`; title `text-lg font-semibold`; message `mt-2.5 text-[14px] leading-relaxed text-muted-foreground`; GhostButton + confirm |
| Popover / Menu | `rounded-lg border bg-popover shadow-lg/5` + highlight, `p-1`; item `min-h-8 sm:min-h-7 gap-2 rounded-sm px-2 py-1 text-base sm:text-sm data-highlighted:bg-accent`; label `px-2 py-1.5 text-xs font-medium text-muted-foreground`; separator `mx-2 my-1 h-px bg-border` |
| Tooltip | `rounded-md border bg-popover text-xs shadow-md/5 px-2 py-1 text-balance` |
| Chart tooltip | `tip-pop min-w-[168px] rounded-xl border border-border bg-popover px-3 py-2.5 text-[12px] shadow-xl`, footer `border-t pt-1.5 text-[10.5px]` |
| Sheet | side `w-[calc(100%-3rem)] max-w-md border-s bg-popover`, slides `translate-x-8` |
| Shortcuts modal | `max-w-sm rounded-2xl border bg-background p-5 shadow-2xl shadow-black/40` over `bg-black/40`, `z-[80]` |
| Support widget | launcher `fixed bottom-20 right-4 md:bottom-5 md:right-5 z-40 h-12 rounded-full bg-primary pl-3.5 pr-4 shadow-lg hover:scale-105 active:scale-95` (`LifeBuoy size-6`, `font-heading text-sm font-semibold`); panel = kit Card `w-[calc(100vw-2rem)] max-w-md overflow-hidden shadow-xl` |

### 10.1 Paywall (brand register)

- `PaywallModal`: `DialogPopup max-w-[780px] rounded-[18px] shadow-none before:hidden`, centred; body `font-geist tracking-[-0.02em]` + ss08.
- Masthead: 12px PlusMark + h2 `text-[19px] font-[450] tracking-[-0.03em]` + context `text-[12.5px] text-muted-foreground/70`.
- Layout `flex-col sm:flex-row-reverse gap-5 sm:gap-7 px-6 pb-6`; benefits column `sm:w-[236px]`: eyebrow `text-[10.5px] text-muted-foreground/55` (sentence case), plus-bulleted list `space-y-2`, trust lines `border-t pt-3.5 mt-4 text-[11.5px] text-muted-foreground/70`, decorative PlusField.
- **PlanPicker** — plans are radio rows, not tall cards: `w-full rounded-[12px] border p-3.5 text-left transition-colors duration-150`; selected `border-primary/40 bg-primary/[0.07]`; else `border-border hover:bg-foreground/[0.025]`. Name `text-[13.5px] font-medium`, sub `text-[11.5px] text-muted-foreground/70`, price per §2.2 with unit `text-[11px] text-muted-foreground/60`, old price struck `line-through decoration-muted-foreground/45`, saving as a line of type `text-[11.5px] font-medium tabular-nums text-success` (the only green on the surface). Date chips `rounded-[10px] border px-1 py-[9px]` (on `border-primary/50 bg-primary/[0.16] text-primary`), discount tag `absolute -right-1 -top-2 rounded-full bg-primary px-[5px] py-px text-[9.5px] font-bold`.
- CTA: `w-full rounded-[12px] bg-primary px-5 py-2.5 text-[14px] font-medium text-primary-foreground hover:opacity-90 active:scale-[0.98]` + nudging `ArrowRight`, pinned with `mt-auto pt-4`.
- "Not now": `mt-2.5 w-full text-center text-[12.5px] text-muted-foreground hover:text-foreground`.

### 10.2 Page-top banners

Replace or sit under the header; full-bleed, no radius.

| Banner | Recipe |
|---|---|
| Paper release | `relative flex h-14 items-center justify-center gap-x-3 border-b border-primary/25 bg-primary/[0.08] px-4 sm:px-6`, lead `PlusMark size={11}`, text `text-[13px]` + `tabular-nums` countdown, CTA `h-7 rounded-[9px] bg-primary px-3 text-[12px] font-medium`, secondary `h-7 rounded-[9px] border border-primary/35 text-primary hover:bg-primary/10` |
| New paper | `h-11 … border-b border-primary/20 bg-primary/[0.06]`, `Sparkles size-3.5`, `text-[12.5px]`, CTA `h-[26px] rounded-[8px] bg-primary px-2.5 text-[11.5px]`, dismiss `absolute right-3 size-6 rounded-[7px] hover:bg-foreground/5` |
| Ready | same shape, `border-border bg-accent/60` |
| Dismissal | `.banner-fly-out` — 400ms flies toward the Settings corner |

---

## 11. Charts & calendars

**Recharts stacked bar** (`analytics/ActivityTrend.tsx`): `margin {top:8,right:4,bottom:0,left:-18}`, `barCategoryGap="26%"`, `maxBarSize={26}`, height `h-[240px]`; `CartesianGrid vertical={false} stroke="rgba(100,116,139,0.055)"`; ticks `fill rgba(100,116,139,0.35) fontSize 11`, no tick lines, Y axis no line `tickCount 4 width 30`; cursor `rgba(100,116,139,0.05)`. Series: medium = `var(--success)`/`var(--destructive)`, easy = `color-mix(in oklab, X 45%, white)`, hard = `color-mix(in oklab, X 65%, black)`. Only the top segment of each column is rounded (radius 5). 550ms ease-out, disabled for reduced motion.
Admin charts: grid `var(--border)` dashed `4 4` horizontal only, ticks 11px `var(--muted-foreground)`.

Legend pill: `rounded-full bg-muted px-3 py-1.5 ring-1 ring-border` + dot + `text-xl font-bold tabular-nums` + `text-[12.5px]` label.

**Activity heatmap** (`lib/calendarGrid.ts`): cells `aspect-square rounded-[3px]` in `grid grid-cols-7 gap-[3px]`:

| Questions | Class |
|---|---|
| 0 | `bg-foreground/[0.07]` |
| 1–2 | `bg-primary/25` |
| 3–5 | `bg-primary/45` |
| 6–9 | `bg-primary/70` |
| 10+ | `bg-primary` |

Today `ring-1 ring-primary/70`; weekday heads `text-[9px] text-muted-foreground/45` (first column `text-primary/60`); 7-day strip `h-2.5 flex-1 rounded-[2px]`.

**Study plan week**: `grid-cols-7 gap-2.5` (min 980px, h-scroll); day `min-h-[88px] rounded-lg p-1.5`, today `bg-primary/[0.05] ring-1 ring-primary/20`, else `bg-muted/40`; task card `rounded-lg border p-2.5`; checkbox `h-4 w-4 rounded-[5px] border`. Task-type ink: Practice `text-primary`, Review `text-warning`, Lesson `text-info`, Mock `text-destructive`.

**Leaderboard row geometry**: rank `w-7 text-right text-[12.5px] text-muted-foreground/60`, mark `size-[22px]`, name `flex-1 sm:w-[26%] lg:w-[29%] text-[13.5px]`, points `w-12 sm:w-14 text-[15px] font-medium`, factor columns `w-14…w-16 text-[13px] text-muted-foreground` (hidden < sm, collapse to one `text-[11px]` line). Captions `text-[10.5px] text-muted-foreground/50`, `px-[15px]`. Rows enter from 10px right, 500ms, 45ms stagger.

---

## 12. Marketing surfaces (`/`, `/auth`, `/sat-score-calculator`)

**Always light, independent of the theme tokens** (hex only; `--primary` happens to match).

- Canvas `#F4F6F9`, ink `#0a0d15` at opacity steps. Container `mx-auto max-w-[1152px] px-6 sm:px-8` (auth `max-w-[1040px]`).
- Section rhythm: hero `py-20 lg:py-28`, stat strip `py-10`, others `py-16`/`py-20`; heading→content `mt-12`.
- **Header**: `sticky top-0 z-50 border-b border-[#0a0d15]/[0.06] bg-[#F4F6F9]/80 backdrop-blur-md`, inner `flex h-16`, wordmark `h-8`, links `rounded-lg px-3 py-1.5 text-[14px] font-medium text-[#0a0d15]/70 hover:bg-[#0a0d15]/[0.04] hover:text-[#0a0d15]`.
- **Footer**: `border-t border-[#0a0d15]/[0.08]`, `py-12`, wordmark `h-10`, `© 2026 1609plus` `text-[13px] uppercase tracking-wide /55`, 4 link columns (`text-[14px] font-medium` titles, `text-[13px] /60` links).

| Element | Classes |
|---|---|
| Hero h1 | `text-[40px] font-medium leading-[1.05] tracking-tight sm:text-[48px]` |
| Hero body | `mt-6 max-w-[486px] text-[16px] leading-relaxed text-[#0a0d15]/65` |
| Eyebrow | `text-[13px] font-medium uppercase tracking-[0.14em] text-[#3755ed]` |
| Section h2 | `text-[36px] font-medium leading-tight tracking-tight sm:text-[44px]` |
| Lede | `mx-auto mt-4 max-w-[520px] text-[16px] leading-relaxed /60 sm:text-[18px]` |
| Step h3 | `mt-5 text-[26px] font-medium leading-snug tracking-tight sm:text-[32px]` |
| Stat | `text-[34px] font-medium tabular-nums tracking-tight sm:text-[40px]` + label `text-[13px] /55` |
| Price | `text-[52px] font-medium leading-none tracking-tight tabular-nums` |
| Auth h1 | `text-[36px] font-medium leading-[1] tracking-[-0.04em] sm:text-[44px]` |

Buttons: **Start now** pill `group inline-flex select-none items-center gap-2 rounded-full bg-[#3755ed] font-medium text-white hover:bg-[#2c46cf] active:scale-[0.98]` — lg `h-14 px-6 text-[18px]`, sm `h-9 px-4 text-[14px]`, `ArrowRight` nudges `group-hover:translate-x-0.5`. Outline pill `h-12 rounded-full border border-[#0a0d15]/15 text-[15px]`. White-on-blue pill `h-12 rounded-full bg-white text-[#0a0d15]`. Auth submit `w-full rounded-2xl bg-[#3755ed] py-2 text-[16px]`.

Cards:
- Hero score card: `aspect-[388/224] rounded-[32px] shadow-2xl shadow-[#3755ed]/25`, photo under `bg-[#0a0d15]/55` wash, score `text-[64px] sm:text-[74px] font-medium` + `/1600` `text-[22px] text-white/60`, delta pill `rounded-full bg-white/15 backdrop-blur-sm`. Ghost copies peek from the viewport edges at `xl`.
- MockFrame: 1px gradient rim `rounded-[24px] p-px shadow-[0_30px_60px_-24px_rgba(10,13,21,0.22)]` → inner `rounded-[23px] bg-white p-4 sm:p-5`; contents are **live React mockups** in the light app skin + Inter, floating `y: [0,-8,0]` 6s.
- Pricing: `rounded-[28px] p-7 sm:p-9`, Free `bg-[#f6f5f4]`, Pro = gradient §1.3, check bubbles `h-[22px] w-[22px] rounded-full bg-[#0a0d15]/[0.07]` (`bg-white/15` on blue).
- Range card: `rounded-[24px] bg-gradient-to-b from-white to-[#eef2f7] p-6 sm:p-10 shadow-xl shadow-[#0a0d15]/[0.04]`.
- Tool card: `rounded-2xl border border-[#0a0d15]/[0.08] bg-white p-5 sm:p-6`; insight panel `rounded-2xl border border-[#3755ed]/25 bg-[#3755ed]/[0.06] p-5`.

Motion: framer-motion `EASE = [0.22, 1, 0.36, 1]`; `Reveal` = fade + 24px rise, 0.6s, `viewport once, margin "-64px"`; hero stagger 0/0.1/0.2s; float `y: [0,-10,0]` 7s; count-ups 1.1–1.4s; all behind `useReducedMotion`.

---

## 13. Motion

Tokens (`index.css`, `transitions.css`):

| Token | Value |
|---|---|
| `--ease-out` | `cubic-bezier(0.23, 1, 0.32, 1)` |
| `--ease-in-out` | `cubic-bezier(0.77, 0, 0.175, 1)` |
| `.t-*` ease | `cubic-bezier(0.22, 1, 0.36, 1)` |
| modal | open 250ms / close 150ms, scale 0.96 |
| dropdown | open 250ms / close 150ms, pre-scale 0.97, origin-aware (`data-origin`) |
| number pop | 320ms (`.t-number-pop`) |
| page enter | 320ms, 8px rise (`.t-page`) |

Named animations:

| Class | Effect |
|---|---|
| `.rise` | 300ms fade + 8px rise, `--rise-delay` for stagger — default entrance for cards |
| `.count-swap` | 240ms 4px rise; key the node by value |
| `.tip-pop` | 150ms scale 0.96→1 (tooltips) |
| `.banner-fly-out` | 400ms to `translate(-10%,80%) scale(.85)` |
| `.plus-twinkle` / `.plus-fall` | §4.2 |
| `.blitz-shake` / `.blitz-heart-fall` | 500ms shake / 700ms fall |
| `.plan-check-draw` | 600ms stroke draw, 250ms delay |
| `.plan-gen-mark` | 4.5s looping pop of calendar marks |
| `animate-skeleton` | 2s linear shimmer |

Micro-interactions: press `active:scale-[0.97]` (buttons), `0.98` (nav, marketing CTA), `0.99` (rows/large cards), `0.94` (mobile nav); arrows `group-hover:translate-x-0.5`; card hover lift `hover:-translate-y-px`; meters animate width from 0 over 700ms; numbers count up (`useCountUp`).
Everything collapses under `prefers-reduced-motion` (`.rise` → plain 200ms fade).

---

## 14. Iconography

lucide-react only. `strokeWidth={1.75}` in navigation; default elsewhere. Sizes: `size-[18px]` nav, `size-5` mobile nav, `size-4`/`h-3.5` inside buttons (`[&_svg]:size-4.5 sm:size-4`, `opacity-80`), 12px inside verdict dots. Icons inherit text colour; tint to `text-primary` only for active/brand meaning. For the brand "+", use `PlusMark`, never lucide `Plus`.

---

## 15. Rules of thumb for new screens

1. Pick the register: **brand** (Geist + ss08, `rounded-[15px]` PANEL, `font-[450]`, sentence-case labels, plus motif) for anything student-facing and emotional (dashboard, results, leaderboard, paywall); **kit** (Inter, `kit.tsx` Card/StatTile/PageTitle) for utility lists and settings. Don't mix inside one page.
2. Colour = one blue + status. Status colours only mean status.
3. Hierarchy through size, weight 450 and alpha — not through new colours, boxes or shadows.
4. Numbers: `tabular-nums`, negative tracking, unit small and muted, `—` for no data.
5. Dark first; check light. Surfaces via tokens only (`bg-card`, `bg-muted`, `border-border`, `bg-foreground/[0.0x]`), never raw greys.
6. Every clickable thing presses (`active:scale-*`) and every entrance uses `.rise` or `EASE [0.22,1,0.36,1]`.
7. Use the plus where other products use checkmarks, active pills, ellipses and grey tracks.
8. Solving UIs replicate College Board conventions (Bluebook layout, highlight colours, navigator) — familiarity beats novelty there.

---

## Appendix — known inconsistencies in code

Documented so they are copied knowingly, not accidentally:

- `ease-out-expo` and `hover-hover:` classes are used but not defined in CSS — they compile to nothing (default Tailwind easing applies).
- Landing `PlusGrid` and range marker use lucide `Plus`, not the brand `PlusMark`.
- Heatmap legend empty swatch is `bg-muted`, grid empty cell is `bg-foreground/[0.07]`.
- Auth headings track `-0.04em`, landing `tracking-tight`.
- `tint-*` tokens and most COSS primitives (`ui/card`, `ui/badge`, `ui/toast`, `ui/tabs`…) are unused by pages.
