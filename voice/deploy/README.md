# Phone calls: a Kazakhstan number → the Saqta voice agent

```
caller (any phone)
   │  PSTN
   ▼
KZ number = SIP trunk from a provider
   │  SIP + RTP (alaw/ulaw)
   ▼
Asterisk (our server)  ── GET /asterisk/call?uuid=…&caller=… ──►  voice gateway :8090   (caller ID)
   │  AudioSocket TCP :9092, slin 8 kHz, 20 ms frames, both directions
   ▼
voice gateway ── Scribe v2 Realtime (STT) ── brain (Ramazan's /api/turn or built-in OpenRouter) ── ElevenLabs TTS (pcm_8000)
```

The gateway is the same one the web UI uses: same engine, same logs, same live stats. The phone only adds a transport (`transport/audiosocket.go`).

Status: the AudioSocket and Twilio transports are covered by unit tests with a simulated Asterisk and a simulated Twilio. A real SIP trunk has not been connected yet: the steps below are what's left.

## 1. Get a number

You need a **Kazakhstan DID delivered over SIP** and its credentials:

- `SIP_HOST` (the provider's registrar), `SIP_USER`, `SIP_PASSWORD`
- inbound calls routed to the trunk, codecs alaw/ulaw

Options: a local operator's business SIP trunk, or a virtual-number provider that sells +7 7xx numbers with SIP access. Kazakhstan numbers usually need KYC, so start early. Backup for the demo: a Twilio number from another country (section 5).

## 2. Server

- A VPS with a public IP, close to the callers and to ElevenLabs (a KZ or EU region).
- Open **UDP 5060** (SIP) and **UDP 10000–20000** (RTP, Asterisk default) to the provider. Open **TCP 8090** only if you also serve the web UI or Twilio. AudioSocket (9092) stays on localhost.

## 3. Configure

In the repository-root `.env` on the server (canonical names):

```bash
ELEVENLABS_API_KEY=sk_...
OPENROUTER_API_KEY=sk-or-...        # built-in brain, or:
BACKEND_URL=http://127.0.0.1:8080   # Ramazan's backend (POST /api/turn); VOICE_BRAIN=backend to force it
SIP_HOST=sip.provider.kz
SIP_USER=77170000000
SIP_PASSWORD=...
PUBLIC_IP=203.0.113.10
VOICE_HOST=127.0.0.1
```

## 4. Run

```bash
cd voice/deploy
docker compose --profile phone up -d --build
docker compose logs -f asterisk          # look for "Registered" for kz-trunk
docker compose exec asterisk asterisk -rx "pjsip show registrations"
docker compose exec asterisk asterisk -rx "module show like audiosocket"
```

Call the number. The agent greets in Kazakh and Russian, then answers in the caller's language.

Watch it live:

- `curl -N http://SERVER:8090/api/voice/events`: every partial transcript, decision and latency, as server-sent events
- `http://SERVER:8090/api/voice/stats`: p50/p95 end-of-speech → first audio
- JSONL per call in the `voice-logs` volume (`/app/logs/<date>/<time>_phone_ast-<uuid>.jsonl`)

If the `andrius/asterisk` image lacks the AudioSocket modules, install Asterisk ≥ 18 from the distro instead. Ubuntu 24.04: `apt install asterisk` (Asterisk 20 includes `res_audiosocket`/`app_audiosocket`). Then render the two templates in `asterisk/` the same way `entrypoint.sh` does.

Troubleshooting:
- **One-way audio or silence**: NAT. Check `PUBLIC_IP` and the RTP port range.
- **Call not answered**: the trunk is not registered, or calls arrive in a different context. The dialplan catches `_X.`, `s` and `_+X.` in `[from-trunk]`.
- **Robotic or choppy audio**: check that the AudioSocket frames are 320 bytes every 20 ms (`transport/audiosocket.go` paces output).

## 5. Twilio instead (number from another country, or bring the KZ trunk to Twilio)

1. Buy a Twilio number. Twilio does not sell Kazakhstan numbers; with Twilio **BYOC** you can route the KZ SIP trunk into Twilio instead.
2. Voice webhook (HTTP POST): `https://SERVER/twilio/voice`. Set `VOICE_PUBLIC_URL=https://SERVER` (TLS is required for the media WebSocket).
3. Twilio opens `wss://SERVER/twilio/stream`: μ-law 8 kHz both ways; barge-in uses Twilio's `clear` message.

## Latency on the phone

| Stage | Budget |
|---|---|
| Carrier + Asterisk, both directions | ~100–250 ms (estimate, measure it) |
| End of speech detection (server VAD, 0.5 s window) | hidden by the speculative reply: the brain starts on a stable partial transcript |
| STT final | ~100–300 ms after the VAD window |
| Brain first text (gemini-2.5-flash-lite via OpenRouter, measured TTFT ~370 ms) | 300–600 ms |
| TTS first audio (socket pre-opened) | ~210–250 ms |
| **Target** | **≤ 1.5 s web, ≈ 1.7 s phone**; a cached filler ("Секунду.") covers slow turns |

Plan C: ElevenLabs Agents has native SIP-trunk telephony with a "custom LLM" hook. It is the fastest way to get *a* phone agent, but it replaces our pipeline, trace and logs, so we keep it only as a fallback.
