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
  for (const lang of ["ru", "kk"]) {
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
