# Bagyt — frontend

Next.js 16 (App Router, TypeScript, Tailwind v4) · coss ui + ObsidianUI · Speko design system.

## Run

```bash
cd frontend
npm ci
npm run dev        # http://localhost:3000
```

No keys needed: `NEXT_PUBLIC_API_MODE=mock` (default) runs the whole pipeline in the browser — STT via the Web Speech API (Chrome; `ru-RU` / `kk-KZ`), a lexical mock router + dialog engine over the starter kit in `src/data/`, TTS via `speechSynthesis`. Switch to the Go backend with `NEXT_PUBLIC_API_MODE=real` and `NEXT_PUBLIC_API_URL=http://localhost:8080` (contract: [`docs/API_CONTRACT.md`](../docs/API_CONTRACT.md), types: `src/lib/contract.ts`).

## Voice without Google

Chrome's Web Speech API sends audio to Google. If that fails (Arc/Brave/Yandex browsers, restricted venue network) the app shows «Нет связи с сервисом распознавания» and switches to server-side recognition through `POST /api/stt` (Next route → OpenAI transcription). Enable it once:

```bash
cp .env.local.example .env.local   # put your OPENAI_API_KEY there, restart npm run dev
```
The toggle «Распознавание: Chrome / Сервер» is in the top bar. TTS stays in the browser.

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

## Python core-llm integration

From the repository root, start `python3 core-llm/server.py` with `core-llm/.env` configured.
Copy `frontend/.env.local.example` to `frontend/.env.local`, then run `npm ci && npm run dev`
from `frontend`. Open `/call` and select **LLM**; `/admin` shows the same conversation's trace.

- `mock`: existing keyless lexical router and demo actions, entirely in the browser.
- `core`: real Python LLM router through `/api/core-route`, browser dialog state and demo actions.
- `real`: existing Go SSE contract (separate integration, not needed for core-llm).

`CORE_LLM_URL` is server-only, defaults to `http://127.0.0.1:8090`.
`NEXT_PUBLIC_API_MODE=core` selects LLM initially; absent this variable, mock remains the default.
Chrome/Edge speech recognition and browser speech synthesis work in both mock and core modes.
Server STT still requires `OPENAI_API_KEY` in `frontend/.env.local`; Kazakh voice availability
in browser synthesis depends on the installed voices. Sessions/traces do not survive reloads.


TTS update: **LLM** mode now uses ElevenLabs through `/api/tts`; set `VOICE_TTS_URL` to the
voice gateway (default `http://127.0.0.1:8091`). Browser STT is unchanged. Root README documents
`docker compose --profile llm --profile voice up --build` and the server-only ElevenLabs key.
The speaker switch stops playback and cancels pending synthesis. If voice is unavailable,
a visible notice explains the fallback to browser synthesis. Text remains in the transcript.
