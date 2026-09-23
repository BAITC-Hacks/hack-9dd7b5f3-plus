# Voice Router — frontend

Next.js 16 (App Router, React 19, TypeScript, Tailwind CSS v4) operator console for the Voice Router backend.

Pages: `/` call simulator (mic + VAD / push-to-talk, browser or server STT, live trace panel),
`/supervisor` metrics + sessions (`/supervisor/[id]` drill-down), `/eval` dev-set evaluation,
`/catalog` the 40 scenarios + system prompt, `/debug` live SSE event stream.

## Run

```bash
npm ci
echo "NEXT_PUBLIC_API_URL=http://localhost:8080" > .env.local   # backend URL (empty = same host, port 8080)
npm run dev            # http://localhost:3000
npm run build && npm start
```

`npm run lint` / `npm run build` must stay clean. Icons: `lucide-react`; everything else is hand-rolled.
The microphone needs HTTPS or localhost; browser STT uses the Web Speech API (Chrome).
