#!/usr/bin/env node
// Terminal client for the voice WebSocket: streams a WAV file exactly like the
// browser does (PCM16 16 kHz chunks + speech_start/speech_end), prints every
// event as it arrives and measures end-of-speech → first audio.
//
//   node scripts/ws_smoke.mjs tests/audio/02_ru_topic_switch.wav
//   node scripts/ws_smoke.mjs --api http://localhost:8080 --text "Сәлеметсіз бе, полисім жарамды ма?"
//   node scripts/ws_smoke.mjs file.wav --hint "transcript for keyless mock STT" --save reply.pcm
//
// Requires Node 22+ (built-in WebSocket). No dependencies.
import fs from "node:fs";
import path from "node:path";

const args = process.argv.slice(2);
const opt = (name, def) => { const i = args.indexOf(name); return i >= 0 ? args[i + 1] : def; };
const api = opt("--api", process.env.API_URL || "http://localhost:8080");
const text = opt("--text", null);
const save = opt("--save", null);
let hint = opt("--hint", null);
const file = args.find((a) => a.endsWith(".wav"));
if (!file && !text) { console.error("usage: ws_smoke.mjs <file.wav> | --text <utterance>"); process.exit(2); }
if (file && hint === null) { const t = file.replace(/\.wav$/, ".txt"); if (fs.existsSync(t)) hint = fs.readFileSync(t, "utf8").trim(); }

function parseWav(buf) {
  if (buf.toString("ascii", 0, 4) !== "RIFF") throw new Error("not a WAV");
  let pos = 12, rate = 16000, channels = 1, data = null;
  while (pos + 8 <= buf.length) {
    const id = buf.toString("ascii", pos, pos + 4), size = buf.readUInt32LE(pos + 4), body = pos + 8;
    if (id === "fmt ") { channels = buf.readUInt16LE(body + 2); rate = buf.readUInt32LE(body + 4); }
    if (id === "data") { data = buf.subarray(body, body + size); break; }
    pos = body + size + (size & 1);
  }
  if (channels !== 1 || rate !== 16000) throw new Error(`need mono 16 kHz PCM16 (got ${channels} ch, ${rate} Hz)`);
  return data;
}

const sess = await fetch(api + "/api/sessions", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ channel: "ws_smoke" }) }).then((r) => r.json());
const wsUrl = api.replace(/^http/, "ws") + "/ws?session_id=" + sess.session_id;
console.log("session", sess.session_id, "→", wsUrl);
const ws = new WebSocket(wsUrl);
ws.binaryType = "arraybuffer";
let tEnd = 0, firstAudio = 0, audioBytes = 0, replyText = "";
const out = save ? fs.createWriteStream(save) : null;

ws.addEventListener("message", (m) => {
  if (m.data instanceof ArrayBuffer) {
    if (!firstAudio) { firstAudio = performance.now(); console.log(`  ▶ first audio after ${Math.round(firstAudio - tEnd)} ms (client-measured)`); }
    audioBytes += m.data.byteLength; if (out) out.write(Buffer.from(m.data));
    return;
  }
  const ev = JSON.parse(m.data);
  const d = ev.data || {};
  switch (ev.type) {
    case "session": console.log("  config:", JSON.stringify({ llm: d.config.llm.provider, stt: d.config.stt.provider, tts: d.config.tts.provider, fast_path: d.config.fast_path })); start(); break;
    case "stt_partial": process.stdout.write(`\r  … ${d.text}`); break;
    case "stt_final": console.log(`\n  STT (${d.provider}, ${d.ms} ms): ${d.text}`); break;
    case "retrieval": console.log("  retrieval:", d.candidates.slice(0, 3).map((c) => `${c.id} ${c.score}`).join(", ")); break;
    case "fast_path": console.log("  fast path:", d.eligible ? "YES" : "no", "—", d.reason); break;
    case "llm_start": console.log("  llm:", d.model, d.prompt_chars, "chars" + (d.follow_up ? " (follow-up)" : "")); break;
    case "route": console.log(`  ROUTE (${d.since_t0_ms} ms after end of speech):`, d.decision.scenarios.map((s) => `${s.id} ${s.confidence} "${s.reason}"`).join(" | "), "→", d.verdict.action); break;
    case "reply_delta": replyText += d.text; break;
    case "reply_done": console.log("  reply:", d.text); break;
    case "speak": console.log("  speak (browser TTS):", d.text); break;
    case "action": console.log(`  action ${d.name} [${d.mode}]`, d.error ? "ERROR " + d.error : JSON.stringify(d.result).slice(0, 120)); break;
    case "tts_first_byte": console.log(`  tts first byte: ${d.tts_ms} ms (first audio ${d.first_audio_ms} ms after end of speech)`); break;
    case "turn_done": {
      const t = d.timings;
      console.log(`  DONE path=${d.path} lang=${d.language.detected}→${d.reply.lang} timings: stt=${t.stt || 0} route=${t.route} first_audio=${t.first_audio || "-"} total=${t.total} ms`);
      if (d.errors) console.log("  errors:", d.errors);
      setTimeout(() => { console.log(`  audio bytes received: ${audioBytes}${save ? " → " + save : ""}`); ws.close(); process.exit(0); }, 1500);
      break;
    }
    case "turn_error": console.log("  TURN ERROR:", d.error); ws.close(); process.exit(1);
    default: if (process.env.VERBOSE) console.log("  ev", ev.type, JSON.stringify(d).slice(0, 100));
  }
});
ws.addEventListener("error", (e) => { console.error("ws error", e.message || e); process.exit(1); });

function start() {
  if (text) {
    tEnd = performance.now();
    ws.send(JSON.stringify({ type: "text", text, source: "text", t: Date.now() }));
    return;
  }
  const pcm = parseWav(fs.readFileSync(path.resolve(file)));
  console.log(`  streaming ${file}: ${pcm.length} bytes (${Math.round(pcm.length / 32)} ms of audio)`);
  ws.send(JSON.stringify({ type: "speech_start", t: Date.now() }));
  const chunk = 3200; // 100 ms
  let i = 0;
  const timer = setInterval(() => {
    if (i >= pcm.length) {
      clearInterval(timer);
      tEnd = performance.now();
      ws.send(JSON.stringify({ type: "speech_end", t: Date.now(), transcript_hint: hint || undefined }));
      return;
    }
    ws.send(pcm.subarray(i, Math.min(i + chunk, pcm.length)));
    i += chunk;
  }, 20); // 5x real time
}
