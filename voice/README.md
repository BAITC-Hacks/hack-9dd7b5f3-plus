# voice — ElevenLabs STT/TTS for the Voice Router (Go)

Real-time speech layer for the Halyk Bank **Voice Router** case (Saqta Insurance mock data): streaming **speech-to-text** and **text-to-speech** on ElevenLabs, tuned for Russian, Kazakh and mixed RU/KZ speech with the lowest latency we could get. It is a separate Go module (`hackathon/voice`), so it never conflicts with `backend/` or `frontend/`.

> Status: **phase 1 = STT/TTS core** (this push): `elevenlabs`, `speech`, `audio`, `lang`, `config`, demo CLI, tests.
> Phase 2 (next push): conversation engine, web voice gateway (`/ws/voice`), phone calls (Asterisk AudioSocket for a KZ number + Twilio), logs and live stats.

## Why these models

| Job | Model | Why | Measured here (warm, from Astana) |
|---|---|---|---|
| Speech-to-text | `scribe_v2_realtime` (WebSocket), `language_code=kk` | streaming, 90+ languages incl. Kazakh + Russian, word timestamps | final transcript **263–325 ms** after push-to-talk release; 577–778 ms after audio end with server VAD (0.5 s silence window) |
| Reply in **Russian** | `eleven_flash_v2_5` (stream-input WebSocket) | ~75 ms model latency, the fastest ElevenLabs TTS | first audio **245 ms** after first text (socket already open) |
| Reply in **Kazakh** / mixed | `eleven_v3_conversational` (text-to-dialogue WebSocket) | the only realtime ElevenLabs model that speaks **Kazakh** (Flash/Multilingual v2 do not); also speaks Russian in the same voice | first audio **211–221 ms** after first text |

Full table with audio files: [demos/RESULTS.md](demos/RESULTS.md).

### Findings that changed the defaults

- **Do not use STT auto-detect for Kazakh.** With no `language_code`, Scribe v2 Realtime transcribed our Kazakh phrase as *Turkish* ("Selam Eczacı Bey…", `lang=tr`). With `language_code=kk` Russian, Kazakh and mixed phrases all come out right, so `kk` is the default (`ELEVENLABS_STT_LANGUAGE`).
- **Do not add `ru` as a secondary language**: it bends mixed RU/KZ phrases towards Russian spelling ("аварияга түстым").
- Over WebSockets `eleven_v3_conversational` starts as fast as Flash (~210–250 ms), so Kazakh replies do not cost extra latency.

All three output formats work on both TTS models: `pcm_16000` (web), `pcm_8000` (Asterisk / KZ SIP number), `ulaw_8000` (Twilio) — no resampling on our side.

## Quick start

```bash
cd voice
# keys: the repo-root .env is found automatically (ELEVENLABS_API_KEY / OPENROUTER_API_KEY;
# hand-written aliases like "eleven-labs=sk_..." and "openrouter-api=sk-or-..." also work)
go test ./...                          # unit tests, no network, no credits
go run ./cmd/voicedemo quota           # remaining TTS characters on the plan
go run ./cmd/voicedemo tts             # synthesize RU / KZ / mixed demo phrases -> demos/*.wav (cached)
go run ./cmd/voicedemo stt             # stream those WAVs into Scribe v2 Realtime at real-time pace
VOICE_LIVE=1 go test ./elevenlabs/ -run Live -v   # live smoke tests (uses a few dozen characters)
```

Credits: the team key is on the Creator plan (~128k characters/month). `voicedemo tts` still caches WAVs and `speech.ElevenTTS.Say` caches fixed phrases on disk, so re-runs are free. `voicedemo quota` shows what is left.

## Packages

| Package | What it gives you |
|---|---|
| `elevenlabs` | Low-level client. `OpenSTT` (Scribe v2 Realtime session: `Send`, `Commit`, `Events`, `SpeechEnd`), `OpenSocket` (streaming TTS: `Send(text, flush)`, `Finish`, `Audio()`), `TTS`/`TTSStream` (HTTP), `Transcribe` (batch Scribe v2 for whole recordings), `Voices`, `Subscription`, `SingleUseToken`, `Warm`. |
| `speech` | The seam the rest of the system codes against: `Recognizer` / `Synthesizer` interfaces, ElevenLabs implementations with model routing per language (`ElevenSTT`, `ElevenTTS`), on-disk cache for fixed phrases (`Say`), and `Chunker` (turns LLM deltas into speakable pieces: first piece at the first comma so audio starts early). |
| `audio` | PCM16 / μ-law helpers, resampling, WAV read/write, framing, a tiny energy VAD for barge-in. |
| `lang` | Per-word RU/KZ tagging (Kazakh-only letters ә ғ қ ң ө ұ ү һ і, frequent Kazakh words, suffixes) and the reply-language policy: explicit request → dominant language (≥ 60 % of words) → sticky session language. |
| `config` | Env + `.env` loading with aliases, defaults for every knob (see `.env.example`). |
| `cmd/voicedemo` | Demo CLI that records latency measurements into `demos/RESULTS.md`. |

## Using it from Go (backend integration)

```go
import (
    "hackathon/voice/config"
    "hackathon/voice/elevenlabs"
    "hackathon/voice/lang"
    "hackathon/voice/speech"
)

cfg := config.Load()
el := elevenlabs.NewClient(cfg.ElevenLabsKey, cfg.ElevenLabsBase)
_ = el.Warm(ctx) // open TLS early

// --- STT: one session per call; send audio as it is captured ---
stt := &speech.ElevenSTT{Client: el, Cfg: cfg}
rec, _ := stt.Open(ctx, speech.RecognizerOptions{Format: "pcm_16000"}) // Manual: true for push-to-talk
go func() {
    for ev := range rec.Events() {
        switch ev.Type {
        case elevenlabs.EventPartial:   // live caption: ev.Text
        case elevenlabs.EventCommittedTimestamps: // final utterance: ev.Text, ev.Language ("kaz"/"rus"), ev.Words
            end, _ := rec.SpeechEnd(ev.Words) // wall-clock end of speech -> latency metrics
            _ = end
        }
    }
}()
rec.Send(pcmFrame) // 20-100 ms frames; rec.Commit() on push-to-talk release

// --- reply language ---
var policy lang.Policy
choice := policy.Choose(finalText, sttLanguage) // choice.Reply == lang.RU or lang.KK

// --- TTS: open the socket early, stream LLM deltas into it ---
tts := &speech.ElevenTTS{Client: el, Cfg: cfg}
out, _ := tts.Open(ctx, choice.Reply, "pcm_16000")  // Flash for RU, v3 conversational for KK
go func() { for chunk := range out.Audio() { play(chunk) } }()
ch := speech.NewChunker()
for delta := range llmDeltas {
    for _, p := range ch.Push(delta) { out.Send(p.Text, p.Flush) }
}
for _, p := range ch.Flush() { out.Send(p.Text, p.Flush) }
out.Finish()
```

Without importing Go code: phase 2 adds HTTP endpoints (`POST /api/voice/tts`, `POST /api/voice/stt`) and the `/ws/voice` gateway, which calls the backend's `POST /api/turn` (the contract in `frontend/src/lib/contract.ts`).

To import from `backend/`: add `require hackathon/voice v0.0.0` and `replace hackathon/voice => ../voice` to `backend/go.mod` (or `go work init ./backend ./voice` at the repo root), and build the backend Docker image with the repo root as context.

## Latency design (goal: end of speech → first audio < 1.5 s)

1. **One STT session per call**, audio streamed in 20 ms frames while the caller talks; partials every ~100–200 ms, so at the end of speech only the last words remain.
2. **Push-to-talk on web** (`Manual: true` + `Commit()` on release) removes the VAD silence wait; phone uses server VAD with a short 0.5 s silence window (`VOICE_VAD_SILENCE_SECS`).
3. **Real end-of-speech timestamp**: `SpeechEnd(words)` maps the last word's audio time back to wall-clock, so the latency we report starts when the caller actually stopped talking.
4. **TTS socket opened in advance** (on the first partial), so the WebSocket/TLS handshake is hidden; first piece sent with `flush` at the first comma.
5. **Raw PCM / μ-law output** in the transport's native format: no MP3 decode, no resampling.
6. **Keep-alive transport** + `Warm()`: the TLS handshake is paid once per process.
7. **Fixed phrases cached on disk** (greeting, fillers): zero latency and zero credits after the first run.

## Tests

- `go test ./...` — unit tests with fake ElevenLabs servers (STT session, both TTS sockets, HTTP TTS, token), chunker, audio, lang, config.
- `VOICE_LIVE=1 go test ./elevenlabs/ -run Live -v` — live smoke tests against ElevenLabs with the key from `.env`.
- `go run ./cmd/voicedemo tts && go run ./cmd/voicedemo stt` — live demos with measurements in [demos/RESULTS.md](demos/RESULTS.md).
