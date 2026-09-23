/* eslint-disable @typescript-eslint/no-require-imports -- CommonJS loader hook for TS domain tests. */
/* Run TS domain code with the project's existing compiler; no test runtime dependency. */
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const ts = require("typescript");
const { test } = require("node:test");
require.extensions[".ts"] = (module, filename) => {
  const source = fs.readFileSync(filename, "utf8").replaceAll("@/data/", path.resolve(__dirname, "../src/data") + "/");
  module._compile(ts.transpileModule(source, { compilerOptions: {
    module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true,
  } }).outputText, filename);
};
const { adaptCoreRoute } = require("../src/lib/core-router.ts");
const { newDialogState, runMockTurn } = require("../src/lib/mock/engine.ts");

function result(ids, status = "route") {
  const entries = ids.map(([id, pct]) => ({ id, pct, name: id }));
  return { primary: entries[0]?.id ?? null, status, intents: entries, alternatives: [], scenarios: entries,
    predicted: entries.map((e) => e.id), language: "ru", model: "test-core", route_ms: 12 };
}
async function turn(state, text, response) {
  const events = [];
  for await (const event of runMockTurn(state, { text, t0: Date.now(), speed: 0,
    route: async (routedText) => adaptCoreRoute(response, routedText, state) })) events.push(event);
  return events.find((e) => e.type === "turn.done").trace;
}

test("core 60% policy wins over mock 75% and additional intent is queued", async () => {
  const state = newDialogState("test");
  const trace = await turn(state, "Хочу продлить полис и добавить водителя", result([["SC27", 65], ["SC04", 40]]));
  assert.equal(trace.policy.action, "run");
  assert.equal(trace.scenarios[0].confidence, 0.65);
  assert.ok(state.stack.includes("SC04"));
  assert.equal(trace.model, "test-core");
  assert.ok(trace.response_text.length > 0);
});

test("core clarification is not overridden by high confidence", async () => {
  const trace = await turn(newDialogState("test"), "Страховка", result([["SC27", 90]], "clarify"));
  assert.equal(trace.policy.action, "clarify");
});

test("topic switch while awaiting ID follows the model", async () => {
  const state = newDialogState("test");
  state.active_scenario = "SC27";
  state.awaiting = { kind: "slot", slot: "phone" };
  const trace = await turn(state, "Какие документы нужны при ДТП?", result([["SC18", 95]]));
  assert.equal(trace.scenarios[0].scenario_id, "SC18");
  assert.ok(state.stack.includes("SC27"));
});

test("goodbye and out-of-scope produce safe catalog replies", async () => {
  for (const [id, status, action] of [["SYS_GOODBYE", "goodbye", "goodbye"], ["SYS_OUT_OF_SCOPE", "out_of_scope", "out_of_scope"]]) {
    const trace = await turn(newDialogState("test"), "Спасибо", result([[id, 95]], status));
    assert.equal(trace.policy.action, action);
    assert.ok(trace.response_text.length > 0);
  }
});

test("failed core request is not silently replaced with lexical routing", async () => {
  await assert.rejects(async () => {
    for await (const event of runMockTurn(newDialogState("test"), { text: "Полис", t0: Date.now(), speed: 0,
      route: async () => { throw new Error("core unavailable"); } })) void event;
  }, /core unavailable/);
});

const { POST: ttsProxy } = require("../src/app/api/tts/route.ts");
const { speakElevenLabs, stopSpeaking } = require("../src/lib/voice.ts");

test("TTS proxy forwards RU/KZ text as MP3 and does not expose credentials", async (t) => {
  for (const lang of ["ru", "kk", "en"]) {
    t.mock.method(globalThis, "fetch", async (url, init) => {
      assert.ok(String(url).endsWith("/api/voice/tts"));
      assert.deepEqual(JSON.parse(init.body), { text: "Тест", lang, format: "mp3_22050_32" });
      assert.equal(init.headers["xi-api-key"], undefined);
      return new Response(new Uint8Array([1, 2, 3]), { headers: { "content-type": "audio/mpeg", "x-tts-model": "test" } });
    });
    const response = await ttsProxy(new Request("http://test/api/tts", { method: "POST", body: JSON.stringify({ text: "Тест", lang }) }));
    assert.equal(response.status, 200);
    assert.equal(response.headers.get("x-tts-provider"), "elevenlabs");
    assert.equal((await response.arrayBuffer()).byteLength, 3);
    t.mock.restoreAll();
  }
});

test("TTS proxy rejects invalid input and sanitizes upstream errors", async (t) => {
  const bad = await ttsProxy(new Request("http://test/api/tts", { method: "POST", body: '{"text":"","lang":"ru"}' }));
  assert.equal(bad.status, 400);
  t.mock.method(globalThis, "fetch", async () => new Response("private provider detail", { status: 401 }));
  const response = await ttsProxy(new Request("http://test/api/tts", { method: "POST", body: '{"text":"Тест","lang":"ru"}' }));
  assert.equal(response.status, 502);
  assert.ok(!(await response.text()).includes("private provider detail"));
});

test("ElevenLabs playback resolves on playing and stops/releases on mute", async (t) => {
  const originalWindow = globalThis.window;
  const originalAudio = globalThis.Audio;
  let paused = false;
  let ended = false;
  globalThis.window = { speechSynthesis: { cancel() {} } };
  globalThis.Audio = class {
    play() { queueMicrotask(() => this.onplaying?.()); return Promise.resolve(); }
    pause() { paused = true; }
  };
  t.after(() => { stopSpeaking(); globalThis.window = originalWindow; globalThis.Audio = originalAudio; });
  t.mock.method(globalThis, "fetch", async () => new Response(new Uint8Array([1, 2, 3]), { headers: { "content-type": "audio/mpeg" } }));
  const ms = await speakElevenLabs("Тест", "ru", { onEnd: () => { ended = true; } });
  assert.ok(ms >= 0);
  assert.equal(ended, false);
  stopSpeaking();
  assert.equal(paused, true);
  assert.equal(ended, true);
});

test("muting during synthesis cancels the request without starting audio", async (t) => {
  const originalWindow = globalThis.window;
  globalThis.window = { speechSynthesis: { cancel() {} } };
  t.after(() => { globalThis.window = originalWindow; });
  t.mock.method(globalThis, "fetch", async (_url, init) => new Promise((_resolve, reject) => {
    init.signal.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")));
  }));
  const pending = speakElevenLabs("Тест", "ru");
  stopSpeaking();
  assert.equal(await pending, 0);
});

const { POST: sttProxy } = require("../src/app/api/stt/route.ts");
function recordingRequest(type = "audio/webm", data = [1, 2, 3]) {
  const form = new FormData();
  form.append("file", new Blob([new Uint8Array(data)], { type }), "recording.webm");
  return new Request("http://test/api/stt", { method: "POST", body: form });
}
test("STT forwards audio to voice gateway and returns sanitized transcript metadata", async (t) => {
  t.mock.method(globalThis, "fetch", async (url, init) => {
    assert.ok(String(url).endsWith("/api/voice/stt"));
    const audio = init.body.get("file");
    assert.equal(audio.type, "audio/webm");
    assert.equal(audio.size, 3);
    return Response.json({ text: " Полисті ұзарту ", language: "kk", ms: 123 });
  });
  const response = await sttProxy(recordingRequest());
  assert.equal(response.status, 200);
  const result = await response.json();
  assert.equal(result.text, "Полисті ұзарту");
  assert.equal(result.provider, "elevenlabs");
  assert.equal(result.language, "kk");
});
test("STT rejects empty/non-audio input and hides provider errors", async (t) => {
  assert.equal((await sttProxy(recordingRequest("audio/webm", []))).status, 400);
  assert.equal((await sttProxy(recordingRequest("text/plain"))).status, 415);
  t.mock.method(globalThis, "fetch", async () => new Response("private upstream error", { status: 401 }));
  const response = await sttProxy(recordingRequest());
  assert.equal(response.status, 502);
  assert.ok(!(await response.text()).includes("private upstream"));
});
test("recorded speech preserves actual STT latency in final trace", async () => {
  let trace;
  for await (const event of runMockTurn(newDialogState("test"), { text: "Хочу продлить полис", t0: Date.now(), speed: 0, sttMs: 417 })) {
    if (event.type === "turn.done") trace = event.trace;
  }
  assert.equal(trace.latency_ms.stt, 417);
});

const { startRecording } = require("../src/lib/voice.ts");
test("microphone recorder supports MP4 and releases tracks after stop or cancellation", async (t) => {
  const oldNavigator = Object.getOwnPropertyDescriptor(globalThis, "navigator");
  const oldRecorder = globalThis.MediaRecorder;
  const oldWindow = globalThis.window;
  globalThis.window = { setInterval, clearInterval, AudioContext: class {
    createAnalyser() { return { fftSize: 1024, getByteTimeDomainData(buffer) { buffer.fill(128); } }; }
    createMediaStreamSource() { return { connect() {} }; }
    close() { return Promise.resolve(); }
  } };

  let released = 0;
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: { mediaDevices: {
    getUserMedia: async () => ({ getTracks: () => [{ stop: () => { released++; } }] }),
  } } });
  globalThis.MediaRecorder = class {
    static isTypeSupported(type) { return type === "audio/mp4"; }
    constructor(_stream, options) { this.mimeType = options.mimeType; this.state = "inactive"; }
    start() { this.state = "recording"; }
    stop() {
      this.state = "inactive";
      queueMicrotask(() => { this.ondataavailable?.({ data: new Blob(["audio"], { type: this.mimeType }) }); this.onstop?.(); });
    }
  };
  t.after(() => {
    if (oldNavigator) Object.defineProperty(globalThis, "navigator", oldNavigator);
    else delete globalThis.navigator;
    globalThis.MediaRecorder = oldRecorder;
    globalThis.window = oldWindow;
  });
  const recorder = await startRecording();
  const result = await recorder.stop();
  assert.equal(result.mime, "audio/mp4");
  assert.equal(atob(result.audio_base64), "audio");
  assert.equal(released, 1);
  const cancelled = await startRecording();
  cancelled.abort();
  assert.equal(released, 2);
});

test("greetings reply in RU/KK and preserve a pending scenario in core and mock", async () => {
  for (const core of [false, true]) {
    for (const [text, reply] of [["Здравствуйте!", "Здравствуйте, чем могу помочь?"], ["Сәлеметсіз бе!", "Сәлеметсіз бе, қалай көмектесе аламын?"], ["Hello!", "Hello, how can I help you?"]]) {
      const state = newDialogState("greeting");
      state.active_scenario = "SC27";
      state.awaiting = { kind: "slot", slot: "customer_name" };
      state.stack = ["SC04"];
      state.low_conf_streak = 1;
      const before = JSON.parse(JSON.stringify(state));
      let trace;
      if (core) trace = await turn(state, text, result([["SYS_GREETING", 100]], "greeting"));
      else for await (const event of runMockTurn(state, { text, t0: Date.now(), speed: 0 })) {
        if (event.type === "turn.done") trace = event.trace;
      }
      assert.equal(trace.policy.action, "greeting");
      assert.equal(trace.response_text, reply);
      for (const key of ["active_scenario", "awaiting", "stack", "slots", "low_conf_streak", "ended"]) assert.deepEqual(state[key], before[key], key);
    }
  }
});

test("greeting with a request is routed normally; vague insurance need still clarifies", () => {
  const { mockRoute } = require("../src/lib/mock/router.ts");
  assert.equal(mockRoute("Здравствуйте, хочу продлить полис", { awaiting: false }).decision.scenarios[0].scenario_id, "SC27");
  assert.equal(mockRoute("Здравствуйте, я по поводу страховки", { awaiting: false }).decision.scenarios[0].scenario_id, "SYS_UNCLEAR");
});

test("saved MP3 replays without synthesis and survives stop until explicitly released", async (t) => {
  const { replayAudio, releaseSavedAudio } = require("../src/lib/voice.ts");
  const originalWindow = globalThis.window;
  const originalAudio = globalThis.Audio;
  globalThis.window = { speechSynthesis: { cancel() {} } };
  let plays = 0, requests = 0, revokes = 0;
  globalThis.Audio = class {
    constructor(url) { this.src = url; }
    play() { plays++; this.onplaying?.(); return Promise.resolve(); }
    pause() {}
    removeAttribute() {}
    load() {}
  };
  t.after(() => { stopSpeaking(); globalThis.window = originalWindow; globalThis.Audio = originalAudio; });
  t.mock.method(globalThis, "fetch", async () => { requests++; return new Response(new Uint8Array([1, 2]), { headers: { "content-type": "audio/mpeg" } }); });
  t.mock.method(URL, "revokeObjectURL", () => { revokes++; });
  let saved;
  await speakElevenLabs("Hello", "en", { autoplay: false, onAudio: (value) => { saved = value; } });
  assert.equal(plays, 0);
  assert.ok(saved.blob.size > 0);
  await replayAudio(saved);
  stopSpeaking();
  await replayAudio(saved);
  assert.equal(plays, 2);
  assert.equal(requests, 1);
  assert.equal(revokes, 0);
  releaseSavedAudio(saved);
  assert.equal(revokes, 1);
});

test("screenshot dialog offers options after noisy input and accepts a concrete choice", async () => {
  const state = newDialogState("help");
  const unclear = await turn(state, "Здравствуйте. И мне нужно типа саму нажать, а-а-а", result([["SYS_UNCLEAR", 100]], "clarify"));
  assert.equal(unclear.policy.action, "clarify");
  const help = await turn(state, "А что ещё доступно?", result([["SYS_HELP", 100]], "help"));
  assert.equal(help.policy.action, "help");
  assert.match(help.response_text, /автомобиля.*поездки.*жилья/);
  assert.equal(help.actions.length, 0);
  assert.equal(state.low_conf_streak, 0);
  const choice = await turn(state, "Хочу продлить полис", result([["SC27", 95]]));
  assert.equal(choice.policy.scenario_id, "SC27");
});

test("repeated uncertainty never transfers without request, for core and keyless mode", async () => {
  for (const core of [true, false]) {
    const state = newDialogState("unclear");
    let trace;
    for (let i = 0; i < 3; i++) {
      if (core) trace = await turn(state, "эээ ммм", result([["SYS_UNCLEAR", 100]], "clarify"));
      else for await (const event of runMockTurn(state, { text: "эээ ммм", t0: Date.now(), speed: 0 })) if (event.type === "turn.done") trace = event.trace;
      assert.notEqual(trace.policy.action, "handoff");
      assert.ok(!trace.actions.some((action) => action.startsWith("transfer_to_operator")));
    }
    assert.equal(trace.policy.action, "help");
  }
});

test("service help in RU/KK preserves a pending confirmation and never executes it", async () => {
  for (const [text, lang] of [["Какие есть варианты?", "ru"], ["Тағы қандай қызметтер бар?", "kk"]]) {
    const state = newDialogState("help");
    state.active_scenario = "SC27";
    state.awaiting = { kind: "confirmation", action: "renew_policy" };
    const pending = structuredClone(state.awaiting);
    const trace = await turn(state, text, result([["SYS_HELP", 100]], "help"));
    assert.equal(trace.response_lang, lang);
    assert.deepEqual(state.awaiting, pending);
    assert.equal(state.active_scenario, "SC27");
    assert.deepEqual(trace.actions, []);
  }
});

test("explicit operator requests still transfer and product questions do not become a menu", async () => {
  const trace = await turn(newDialogState("operator"), "Соедините с оператором", result([["SC37", 100]], "handoff"));
  assert.ok(trace.actions.some((a) => a.startsWith("transfer_to_operator")));
  const { mockRoute } = require("../src/lib/mock/router.ts");
  assert.equal(mockRoute("А что ещё доступно?", { awaiting: false }).decision.route_status, "help");
  assert.notEqual(mockRoute("Какие клиники доступны по ДМС?", { awaiting: false }).decision.route_status, "help");
});
