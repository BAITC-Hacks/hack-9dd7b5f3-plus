# Bagyt — frontend

Next.js 16 (App Router, TypeScript, Tailwind v4) · coss ui + ObsidianUI · Speko design system.

## Run

```bash
cd frontend
npm ci
npm run dev        # http://localhost:3000
```

No keys needed: `NEXT_PUBLIC_API_MODE=mock` (default) runs the whole pipeline in the browser — STT via the Web Speech API (Chrome; `ru-RU` / `kk-KZ`), a lexical mock router + dialog engine over the starter kit in `src/data/`, TTS via `speechSynthesis`. Switch to the Go backend with `NEXT_PUBLIC_API_MODE=real` and `NEXT_PUBLIC_API_URL=http://localhost:8080` (contract: [`docs/API_CONTRACT.md`](../docs/API_CONTRACT.md), types: `src/lib/contract.ts`).

## Routes

| Route | Surface | What |
|---|---|---|
| `/` | landing, light ("technical paper") | product story, pipeline, example trace |
| `/call` | console, dark | the client's view: push-to-talk, transcript, voice reply |
| `/admin` | console, dark | supervisor: live candidate confidences, decision + `not_this_if` rules, policy verdict, slots, actions (preview/execute), latency waterfall, turn journal, raw README trace |

## Layout

```
src/lib/contract.ts      event + trace contract (frontend ⇄ backend)
src/lib/store.ts         conversation store (useConversation, sendText, startVoice…)
src/lib/api.ts           mock ⇄ real switch, SSE reader, stats
src/lib/voice.ts         browser STT / TTS / recorder
src/lib/mock/            router.ts (lexical mock), engine.ts (policy + slot FSM), actions.ts (mock backend)
src/lib/catalog.ts       typed starter kit (scenarios, slots, KB, mock_backend)
src/data/                copies of ../data/*.json (re-copy if the kit changes)
src/components/ui/       coss ui primitives (generated, edit freely)
src/components/block/    ObsidianUI blocks
src/components/app/      conversation panel, console shell, admin widgets
src/components/landing/  landing sections
```

## Mock router accuracy (official script)

```bash
npx tsx scripts/eval-mock.ts && python ../data/evaluate.py ../data/predictions_mock.json ../data/dev_utterances.json
```
The mock is a keyword/cue router (no LLM) — a floor for the UI demo, not the product's number.
