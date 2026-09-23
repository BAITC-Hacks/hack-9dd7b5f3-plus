# Validation — 2026-09-23

Verified locally on the implementation branch:

- `go test -race ./...` and `go vet ./...` in backend: pass. Tests cover schema/ID rejection, retry, one overall deadline, credentials error handling, context/topic changes, ten-turn limit, incremental trace delivery, correct latency categories, original catalog normalization, grounded facts, pending topic separation, actual audio byte forwarding and PCM.
- `npm run lint` and `npm run build` in frontend: pass.
- `python3 -m unittest discover -s scripts -p 'test_*.py'`: 3 pass.
- `docker compose up --build -d`: frontend + Go API + PostgreSQL start successfully; API reports organizer_saqta / 40 business + 3 system scenarios.
- PostgreSQL persistence: created a session, restarted backend, reloaded the same session and turn successfully.
- Browser walkthrough: submitted a Russian payment query, observed SC30 and stage trace; inspected the 43 catalog entries and the metric screen. Mock calls are excluded from real LLM p50/p95.
- WAV fixture served successfully over HTTP; ffprobe verified PCM16, mono, 16 kHz (first sample ~4.95 seconds).
- Official evaluator integration: generated predictions for all 104 dev utterances and ran unchanged data/evaluate.py. **Mock-only** primary accuracy = 0.577, full match = 0.558, multi-intent recall = 0.308. These numbers test evaluator plumbing, not model quality. No optimization of the mock against the dev labels was performed.
- Clean checkout: cloned committed source into a separate verification directory, copied .env.example, changed only host ports/origins to avoid colliding with the running demo, and ran Docker Compose from that checkout. Confirmed PostgreSQL, 40+3 catalog, SC30 route, CORS, frontend HTML and all three WAV downloads. Temporary verification containers/volume were then removed; primary demo remains running.
- Original files under data/ remain unchanged; backend/data/official contains identical copies of five runtime JSON files, verified with SHA256. Evaluation labels are not copied into the runtime image.

Not verified without external configuration:

- Real OpenAI/NVIDIA/compatible LLM accuracy and comparison with another model's baseline.
- Live STT/TTS/WebRTC round trip, Russian/Kazakh audio quality and real 500 ms / 1500 ms latency targets. HTTP adapters are covered by controlled local provider tests; that is not equivalent to real provider validation.
- Railway deployment: no linked project/CLI credentials were available; there is no deployed URL claim.

Key needed for the implemented full voice path: OPENAI_API_KEY with access to the configured routing, transcription, TTS and Realtime models. Set LLM_PROVIDER=openai and SPEECH_PROVIDER=openai in local .env. Do not paste keys into chat.
