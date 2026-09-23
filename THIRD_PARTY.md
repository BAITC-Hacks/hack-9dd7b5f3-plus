# Third-party components and data

| Компонент | Лицензия / условия | Ссылка | Применение |
|---|---|---|---|
| Go 1.26.5 | BSD-3-Clause | https://go.dev/LICENSE | Backend runtime / standard library |
| chi v5 | MIT | https://github.com/go-chi/chi | HTTP router / recovery |
| pgx v5, pgpassfile, pgservicefile, puddle | MIT | https://github.com/jackc/pgx | PostgreSQL client and supporting packages |
| golang.org/x/sync, golang.org/x/text | BSD-3-Clause | https://pkg.go.dev/golang.org/x | pgx transitive dependencies |
| PostgreSQL 17 | PostgreSQL License | https://www.postgresql.org/about/licence/ | Session persistence |
| Next.js 16.3.6 | MIT | https://nextjs.org | Frontend App Router |
| React / React DOM 19 | MIT | https://react.dev | UI runtime |
| TypeScript, @types/node, @types/react, @types/react-dom | Apache-2.0 / MIT (types) | https://www.typescriptlang.org | Static typing |
| Tailwind CSS 4, @tailwindcss/postcss | MIT | https://tailwindcss.com | CSS build pipeline |
| ESLint, eslint-config-next | MIT | https://eslint.org | Frontend checks |
| Lucide React | ISC | https://lucide.dev/license | Interface icons |
| Node.js 24 | MIT and bundled component licenses | https://nodejs.org | Frontend build/runtime |
| Alpine Linux container base | Multiple free software licenses | https://alpinelinux.org | Minimal Docker runtime |
| OpenAI API | Commercial API terms | https://platform.openai.com | Optional LLM, STT, TTS, Realtime |
| OpenRouter API | OpenRouter service terms | https://openrouter.ai/terms | Optional OpenAI-compatible routing gateway; locally verified with openai/gpt-4.1-mini |
| GPT-4.1 mini | OpenAI API terms | https://developers.openai.com/api/docs/models/gpt-4.1-mini | Configurable initial router model; no bundled weights |
| gpt-4o-mini-transcribe, gpt-4o-mini-tts, gpt-live-transcribe | OpenAI API terms | https://developers.openai.com/api/docs/guides/audio | Configurable speech models; no bundled weights |
| NVIDIA Build / NIM | NVIDIA service terms | https://build.nvidia.com | Optional OpenAI-compatible routing |
| Llama 3.3 70B Instruct | Llama 3.3 Community License | https://www.llama.com/llama3_3/license/ | Optional NVIDIA default model; no bundled weights |
| Web Speech / WebRTC / Web Audio | Browser/platform implementation terms | https://developer.mozilla.org/en-US/docs/Web/API | Browser speech fallback, transport and playback; not vendored |
| macOS Milena synthetic voice | Apple macOS software license; generated sample speech | https://www.apple.com/legal/sla/ | Three artificial Russian audio test fixtures; no voice model distributed |
| FFmpeg | LGPL-2.1-or-later / GPL depending on local build | https://ffmpeg.org/legal.html | Optional local conversion of generated samples to PCM WAV; not linked/distributed |
| Python 3 standard library | PSF License | https://docs.python.org/3/license.html | Evaluation and fixture generation scripts |
| Synthetic catalog, text cases and audio scripts | Original Team Plus work under hackathon repo terms | backend/data/demo, samples, scripts | Demonstration data; not organizer data |

Versions and all transitives are pinned by frontend/package-lock.json and backend/go.sum. UI is original CSS/components using Lucide; no shadcn/ObsidianUI or external templates were added. Organizer datasets are present under data/ and unchanged; source files were integrated after synchronizing main. Any additional local/hosted LLM selected later must be disclosed here with its license/terms.

AI coding assistance during this implementation: OpenAI Codex. No AI authorship trailers added to git commits.

| Organizer Voice Router / Saqta starter kit | HackAlem case usage terms (synthetic, supplied by organizers) | data/README.md | Original 40 scenarios, 3 system intents, facts, synthetic clients and reference evaluator; runtime normalization, grounded answers and official evaluation export |

Development formatting tool: Prettier 3.6.2 (MIT, https://prettier.io), run via npm exec; not a runtime dependency.
