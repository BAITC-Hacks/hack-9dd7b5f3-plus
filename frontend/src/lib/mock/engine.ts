/**
 * MOCK dialog engine — turns a client utterance into the same event stream the
 * Go backend will emit (see ../contract.ts). Runs entirely in the browser.
 *
 * Pipeline: stt → triage → router (live candidates) → policy → executor (mock actions,
 * slots, confirmation, stack) → response → tts → trace.
 */
import {
  AS_OF_DATE,
  CONFIDENCE_CLARIFY,
  CONFIDENCE_RUN,
  type ActionCall,
  type Candidate,
  type DialogState,
  type LatencyMs,
  type PolicyVerdict,
  type ReplyLang,
  type RouterDecision,
  type ScenarioId,
  type Trace,
  type TurnEvent,
} from "../contract";
import { backend, isSystemIntent, scenarioById, scenarioLabel, slotByNameOf, systemIntentById, type Scenario } from "../catalog";
import { fmtMoney, isIrreversible, maskEmail, runAction } from "./actions";
import { ENGLISH_GREETING, detectLanguage, dominantLanguage, mockRoute, NO, parseSlotAnswer, splitParts, YES, type MockRouteResult } from "./router";

/** Slots whose answer is an identifier — a bare answer is ALWAYS a continuation, never a new intent. */
const ID_SLOTS = new Set(["phone", "iin", "policy_number", "claim_number", "vehicle_plate", "culprit_vehicle_plate", "new_driver_iin", "drivers_iin", "email", "callback_time", "preferred_date", "incident_date", "payment_date"]);

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

export function newDialogState(session_id: string): DialogState {
  return {
    session_id,
    language: "ru",
    client_id: null,
    client_name: null,
    active_scenario: null,
    stack: [],
    slots: {},
    awaiting: null,
    low_conf_streak: 0,
    turn: 0,
    ended: false,
  };
}

interface ExecResult {
  text: string;
  actions: ActionCall[];
  done: boolean; // scenario finished (closing said)
  handoff?: string; // queue
}

/* ------------------------------------------------------------------ */

export async function* runMockTurn(state: DialogState, input: { text: string; t0: number; speed?: number; sttMs?: number; route?: (text: string) => Promise<MockRouteResult> }): AsyncGenerator<TurnEvent> {
  const speed = input.speed ?? 1;
  const t0 = input.t0;
  const now = () => Math.round(performance.now());
  const start = now();
  const lat: LatencyMs = {};
  const actionsLog: ActionCall[] = [];
  const text = input.text.trim();
  state.turn += 1;

  yield { type: "turn.start", turn: state.turn, session_id: state.session_id, t0 };

  // --- STT (text already recognised by the browser; simulate the finalisation cost)
  await sleep(40 * speed);
  const language = detectLanguage(text);
  lat.stt = input.sttMs ?? now() - start;
  yield { type: "stt.final", text, language, ms: lat.stt };

  // --- triage
  const tTri = now();
  const parts = splitParts(text);
  const normalized = text.replace(/\s+/g, " ");
  const replyLang: ReplyLang = pickReplyLanguage(state, text);
  state.language = replyLang;
  // "Да, оформляйте. А анализы тоже покрываются?" / "Давайте потом. А по заявлению…" — a yes/no prefix
  // answers the pending confirmation, the rest is routed as a new request.
  let confirmPrefix: "yes" | "no" | null = null;
  let routeText = text;
  if (state.awaiting?.kind === "confirmation") {
    const m = text.match(/^([^.!?]{1,40}?)[.!?…]\s+(.{6,})$/);
    if (m && (YES.test(m[1].trim()) || NO.test(m[1].trim()))) {
      confirmPrefix = YES.test(m[1].trim()) ? "yes" : "no";
      routeText = m[2];
    }
  }
  const routeStart = now();
  const routed = input.route ? await input.route(routeText) : mockRoute(routeText, { awaiting: confirmPrefix ? false : !!state.awaiting });
  await sleep(15 * speed);
  lat.triage = input.route ? routeStart - tTri : now() - tTri;
  yield { type: "triage", language, urgent: routed.urgent, parts, normalized, ms: lat.triage };

  // --- router (with live candidate snapshots)
  const tRoute = input.route ? routeStart : now();
  const history: Candidate[][] = [];
  if (routed.kind === "route") {
    for (let i = 0; i < routed.snapshots.length; i++) {
      await sleep((i === 0 ? 90 : 70) * speed);
      history.push(routed.snapshots[i]);
      yield { type: "router.candidates", candidates: routed.snapshots[i], partial: i < routed.snapshots.length - 1 };
    }
  } else {
    await sleep(60 * speed);
  }

  // continuation check: are we answering a pending slot?
  let decision: RouterDecision = routed.decision;
  if (routed.kind === "route" && state.awaiting?.kind === "slot" && state.active_scenario) {
    const parsed = parseSlotAnswer(state.awaiting.slot, text);
    const top = decision.scenarios[0];
    const isId = ID_SLOTS.has(state.awaiting.slot);
    const contentWords = splitParts(text).join(" ").split(/\s+/).length;
    const looksLikeNewIntent = !isId && top && !isSystemIntent(top.scenario_id) && top.confidence >= 0.85 && top.scenario_id !== state.active_scenario && contentWords >= 4;
    if (decision.route_status !== "greeting" && parsed !== null && !looksLikeNewIntent && (!input.route || (decision.route_status === "route" && top?.scenario_id === state.active_scenario))) {
      decision = {
        ...decision,
        is_continuation: true,
        scenarios: [{ scenario_id: state.active_scenario, confidence: input.route ? top.confidence : 0.97, reason: `answers pending slot "${state.awaiting.slot}"` }],
        alternatives: decision.scenarios.slice(0, 2).map((s) => ({ scenario_id: s.scenario_id, confidence: s.confidence })),
        slots: { ...decision.slots, [state.awaiting.slot]: parsed },
        reason: `Continuation of ${state.active_scenario}: client answered "${state.awaiting.slot}"`,
      };
    }
  }
  if (routed.kind === "yes" || routed.kind === "no") {
    const active = state.active_scenario ?? "SYS_UNCLEAR";
    decision = {
      ...decision,
      scenarios: [{ scenario_id: active, confidence: input.route ? (decision.scenarios[0]?.confidence ?? 0) : 0.98, reason: routed.kind === "yes" ? "explicit confirmation" : "client declined" }],
    };
  }
  lat.router = now() - tRoute;
  yield { type: "router.decision", decision, ms: lat.router };

  // --- pending confirmation answered by a prefix ("Да, оформляйте. А ещё…")
  let prefixText = "";
  if (confirmPrefix) {
    const r0 = await confirmFlow(state, confirmPrefix, replyLang, actionsLog);
    for (const a of r0.actions) yield { type: "action", call: a };
    prefixText = r0.text.replace(/\s*(Есть ещё вопросы\?|Тағы сұрағыңыз бар ма\?|Тағы қалай көмектесе аламын\?|Чем ещё могу помочь\?)\s*$/, "");
    state.active_scenario = null;
  }

  // --- policy
  const tPol = now();
  const verdict = decide(state, decision, routed.kind);
  await sleep(5 * speed);
  lat.policy = now() - tPol;
  yield { type: "policy", verdict, ms: lat.policy };

  // --- executor
  const tExec = now();
  let text_out = "";
  const chain: string[] = [];
  const lang = state.language;
  const responseLang = verdict.action === "greeting" && ENGLISH_GREETING.test(text.trim()) ? "en" : lang;
  const mergedSlots = { ...decision.slots };
  for (const [k, v] of Object.entries(mergedSlots)) if (v !== undefined && v !== null && v !== "") state.slots[k] = v;

  if (verdict.action === "greeting") {
    text_out = responseLang === "en" ? "Hello, how can I help you?" : systemIntentById("SYS_GREETING")!.response[lang];
  } else if (verdict.action === "goodbye") {
    text_out = systemIntentById("SYS_GOODBYE")!.response[lang];
    state.ended = true;
    state.awaiting = null;
  } else if (verdict.action === "out_of_scope") {
    text_out = systemIntentById("SYS_OUT_OF_SCOPE")!.response[lang];
    state.awaiting = null;
  } else if (verdict.action === "clarify") {
    const opts = clarifyOptions(decision, lang);
    text_out = systemIntentById("SYS_UNCLEAR")!.response[lang].replace("{option_a}", opts[0]).replace("{option_b}", opts[1]);
    state.awaiting = null;
  } else if (verdict.action === "handoff" && !verdict.scenario_id) {
    const summary = summaryFor(state, lang);
    const call = runAction("transfer_to_operator", { queue: verdict.queue ?? "operator_general", summary, __mode: "execute" });
    actionsLog.push(call);
    yield { type: "action", call };
    text_out = lang === "kk" ? `Операторға қосамын, ол сұрағыңыздың мәнін көріп тұр: ${summary}` : `Соединяю с оператором, он уже видит суть вопроса: ${summary}`;
    state.awaiting = null;
  } else {
    // run / continue the scenario
    const sid = verdict.scenario_id!;
    const r = routed.kind === "yes" || routed.kind === "no" ? await confirmFlow(state, routed.kind, lang, actionsLog) : execute(state, scenarioById(sid)!, lang, actionsLog, verdict.action === "run" && !decision.is_continuation);
    for (const a of r.actions) yield { type: "action", call: a };
    text_out = r.text;
    if (r.handoff) {
      const summary = summaryFor(state, lang);
      const call = runAction("transfer_to_operator", { queue: r.handoff, summary, __mode: "execute" });
      actionsLog.push(call);
      yield { type: "action", call };
      text_out += lang === "kk" ? ` Маманға қосамын, ол контекстті көріп тұр.` : ` Соединяю со специалистом, он уже видит контекст.`;
      state.awaiting = null;
      state.active_scenario = null;
    }
    // multi-intent: acknowledge the rest, and if the first is done, chain the next
    if (verdict.action === "run" && decision.scenarios.length > 1 && !decision.is_continuation) {
      const rest = decision.scenarios.slice(1).map((s) => scenarioLabel(s.scenario_id, lang).toLowerCase());
      text_out = (lang === "kk" ? `Түсіндім, екінші сұрағыңызды да (${rest.join(", ")}) қарастырамыз. ` : `Поняла, второй вопрос (${rest.join(", ")}) тоже разберём. `) + text_out;
    }
    if (r.done && state.stack.length) {
      const next = state.stack.shift()!;
      const nextSc = scenarioById(next);
      if (nextSc) {
        state.active_scenario = next;
        const r2 = execute(state, nextSc, lang, actionsLog, true);
        for (const a of r2.actions) yield { type: "action", call: a };
        chain.push(next);
        text_out += (lang === "kk" ? ` Енді «${scenarioLabel(next, lang)}» сұрағы бойынша: ` : ` Теперь по вопросу «${scenarioLabel(next, lang)}»: `) + r2.text;
        if (r2.done) state.active_scenario = null;
      }
    } else if (r.done) {
      state.active_scenario = null;
      state.awaiting = null;
    }
  }
  if (prefixText) text_out = `${prefixText} ${text_out}`.trim();
  lat.executor = now() - tExec;
  yield { type: "state", state: structuredClone(state) };

  // --- response (streamed)
  const tResp = now();
  const chunks = chunk(text_out);
  for (const c of chunks) {
    await sleep(35 * speed);
    yield { type: "response.delta", text: c };
  }
  lat.response = now() - tResp;
  yield { type: "response.final", text: text_out, language: responseLang, ms: lat.response };

  // --- tts
  const total = now() - start;
  lat.tts_first_audio = 0; // browser speechSynthesis: measured client-side, reported by the hook
  lat.total = total;
  yield { type: "tts.audio", browser_tts: true, ms_first_audio: 0 };

  const trace: Trace = {
    turn: state.turn,
    transcript: text,
    language: decision.language,
    scenarios: decision.scenarios,
    alternatives: decision.alternatives,
    reason: decision.reason,
    slots: decision.slots,
    actions: actionsLog.map((a) => (a.mode === "read" ? a.name : `${a.name}:${a.mode}`)),
    latency_ms: lat,
    policy: verdict,
    response_text: text_out,
    response_lang: responseLang,
    model: decision.model,
    tier: decision.tier,
    candidates_history: history,
  };
  yield { type: "turn.done", trace, latency_ms: lat };
}

/* ------------------------------ reply language ------------------------------ */

const GREETINGS = /(сәлеметсіз бе|сәлем|здравствуйте|добрый день|доброе утро|добрый вечер|алло|привет)/gi;
const SWITCH_TO_RU = /(давайте на русском|можно на русском|по-русски|орысша)/i;
const SWITCH_TO_KK = /(қазақша|на казахском|по-казахски)/i;

/**
 * README "Триаж": answer in the client's dominant language and switch when the client switches.
 * Sticky: once a session language is established, keep it unless the client clearly switches
 * (explicit request, or a whole utterance in the other language). Greetings and product loanwords
 * (ОГПО/КАСКО/ДМС) do not count.
 */
function pickReplyLanguage(state: DialogState, text: string): ReplyLang {
  if (SWITCH_TO_RU.test(text)) return "ru";
  if (SWITCH_TO_KK.test(text)) return "kk";
  const stripped = text.replace(GREETINGS, " ").replace(/(огпо|каско|дмс|полис[а-я]*|saqta)/gi, " ");
  if (!/[\p{L}\p{N}]/u.test(stripped)) return dominantLanguage(text);
  const lang = detectLanguage(stripped);
  const dominant = dominantLanguage(stripped);
  if (state.turn <= 1 || !state.language) return dominant;
  if (lang === "mixed") return state.language; // keep the session language on code-switched turns
  return dominant; // whole utterance in one language → follow it (D10: kk → ru)
}

/* ------------------------------ policy ------------------------------ */

function decide(state: DialogState, d: RouterDecision, kind: "route" | "yes" | "no" | "goodbye"): PolicyVerdict {
  const top = d.scenarios[0];
  const base = { stack: state.stack, low_conf_streak: state.low_conf_streak };
  if (d.route_status === "greeting" || top?.scenario_id === "SYS_GREETING") return { action: "greeting", scenario_id: "SYS_GREETING", reason: d.reason, ...base };
  if (kind === "goodbye" || d.route_status === "goodbye") return { action: "goodbye", scenario_id: "SYS_GOODBYE", reason: "client ends the call", ...base };
  if (kind === "yes" || kind === "no") return { action: "continue", scenario_id: state.active_scenario, reason: kind === "yes" ? "confirmation received → execute" : "client declined → cancel preview", ...base };
  if (!top) return { action: "clarify", scenario_id: null, reason: "no candidates", ...base };
  if (top.scenario_id === "SYS_OUT_OF_SCOPE") return { action: "out_of_scope", scenario_id: null, reason: d.reason, ...base };
  if (top.scenario_id === "SYS_UNCLEAR") {
    state.low_conf_streak += 1;
    if (state.low_conf_streak >= 2) return { action: "handoff", scenario_id: null, queue: "operator_general", reason: "two unclear turns in a row → operator with context", ...base, low_conf_streak: state.low_conf_streak };
    return { action: "clarify", scenario_id: null, reason: d.reason, ...base, low_conf_streak: state.low_conf_streak };
  }
  if (d.is_continuation) return { action: "continue", scenario_id: top.scenario_id, reason: "continuation of the active scenario (slot filled)", ...base };
  if (top.scenario_id === "SC37") return { action: "run", scenario_id: "SC37", reason: "client asks for a human", ...base };
  if (d.route_status === "clarify") {
    state.low_conf_streak += 1;
    return { action: state.low_conf_streak >= 2 ? "handoff" : "clarify", scenario_id: null, queue: "operator_general", reason: d.reason, ...base, low_conf_streak: state.low_conf_streak };
  }
  if (!d.route_status && top.confidence < CONFIDENCE_CLARIFY) {
    state.low_conf_streak += 1;
    if (state.low_conf_streak >= 2) return { action: "handoff", scenario_id: null, queue: "operator_general", reason: `confidence ${top.confidence} < ${CONFIDENCE_CLARIFY} twice → operator`, ...base, low_conf_streak: state.low_conf_streak };
    return { action: "clarify", scenario_id: null, reason: `confidence ${top.confidence} < ${CONFIDENCE_CLARIFY}`, ...base, low_conf_streak: state.low_conf_streak };
  }
  if (!d.route_status && top.confidence < CONFIDENCE_RUN) {
    return { action: "clarify", scenario_id: null, reason: `confidence ${top.confidence} in [${CONFIDENCE_CLARIFY}, ${CONFIDENCE_RUN}) → one clarifying question`, ...base };
  }
  state.low_conf_streak = 0;
  // topic switch: park the active scenario
  if (state.active_scenario && state.active_scenario !== top.scenario_id && !isSystemIntent(state.active_scenario)) {
    if (!state.stack.includes(state.active_scenario)) state.stack.unshift(state.active_scenario);
  }
  // multi-intent: queue the rest
  for (const s of d.scenarios.slice(1)) if (!state.stack.includes(s.scenario_id) && !isSystemIntent(s.scenario_id)) state.stack.push(s.scenario_id);
  state.active_scenario = top.scenario_id;
  return { action: "run", scenario_id: top.scenario_id, reason: `${d.route_status ? d.reason : `confidence ${top.confidence} ≥ ${CONFIDENCE_RUN}`} → run${d.scenarios.length > 1 ? `, ${d.scenarios.length - 1} more queued` : ""}`, stack: state.stack, low_conf_streak: 0 };
}

function clarifyOptions(d: RouterDecision, lang: ReplyLang): [string, string] {
  const pool = [...d.scenarios, ...d.alternatives].filter((s) => !isSystemIntent(s.scenario_id) && s.confidence >= 0.3).map((s) => s.scenario_id);
  const a = pool[0];
  const b = pool.find((x) => x !== a);
  if (!a || !b) {
    // content-free utterance → generic top-level fork (D05 style)
    return lang === "kk" ? ["полис рәсімдеу немесе тексеру", "сақтандыру жағдайы туралы хабарлау"] : ["оформить или проверить полис", "заявить о страховом случае"];
  }
  return [scenarioLabel(a, lang).toLowerCase(), scenarioLabel(b, lang).toLowerCase()];
}

function summaryFor(state: DialogState, lang: ReplyLang): string {
  const bits: string[] = [];
  if (state.client_name) bits.push(state.client_name);
  if (state.active_scenario) bits.push(scenarioLabel(state.active_scenario, lang));
  const s = Object.entries(state.slots).filter(([k]) => !["phone", "iin"].includes(k)).slice(0, 3).map(([k, v]) => `${k}=${String(v)}`);
  bits.push(...s);
  return bits.join(", ") || (lang === "kk" ? "жаңа қоңырау" : "новый звонок");
}

/* ------------------------------ executor ------------------------------ */

function firstMissing(sc: Scenario, slots: Record<string, unknown>, state: DialogState): string | null {
  for (const s of sc.slots.required) {
    if (slots[s] !== undefined && slots[s] !== null && slots[s] !== "") continue;
    // auto-fill from the identified client / catalogue
    const auto = autoFill(s, sc, state);
    if (auto !== undefined) { slots[s] = auto; continue; }
    return s;
  }
  return null;
}

function autoFill(slot: string, sc: Scenario, state: DialogState): unknown {
  const client = state.client_id ? backend.clients.find((c) => c.client_id === state.client_id) : undefined;
  if (!client) return undefined;
  if (slot === "phone") return client.phone;
  if (slot === "email") return client.email;
  if (slot === "iin") return client.iin;
  if (slot === "city") return client.city;
  if (slot === "drivers_iin") return [client.iin];
  if (slot === "policy_number") {
    const mine = backend.policies.filter((p) => p.client_id === client.client_id);
    const byDomain: Record<string, string[]> = { auto: ["ogpo", "casco"], health: ["dms"], travel: ["travel"], property: ["property"], accident: ["accident"] };
    const prefer = byDomain[sc.domain];
    const pick = (prefer && mine.find((p) => prefer.includes(p.product))) ?? mine[0];
    return pick?.policy_number;
  }
  if (slot === "claim_number") return backend.claims.find((c) => c.client_id === client.client_id)?.claim_number;
  return undefined;
}

function execute(state: DialogState, sc: Scenario, lang: ReplyLang, log: ActionCall[], firstEntry: boolean): ExecResult {
  const actions: ActionCall[] = [];
  const push = (c: ActionCall) => { actions.push(c); log.push(c); return c; };
  const slots = state.slots;
  const R = sc.responses[lang];

  // 1. identification
  if (sc.requires_identification && !state.client_id) {
    const ident = slots.phone ?? slots.iin;
    if (ident) {
      const call = push(runAction("find_client", slots.phone ? { phone: slots.phone } : { iin: slots.iin }));
      if (call.error) {
        state.awaiting = { kind: "slot", slot: "phone" };
        const retry = (state.slots.__ident_retry as number | undefined) ?? 0;
        state.slots.__ident_retry = retry + 1;
        if (retry >= 1) {
          return { text: lang === "kk" ? "Бұл нөмір бойынша клиент табылмады. ЖСН айтыңыз немесе операторға қосайын ба?" : "По этому номеру клиента не нашла. Назовите ИИН или соединить с оператором?", actions, done: false };
        }
        return { text: lang === "kk" ? "Бұл нөмір табылмады. Нөмірді қайта айтыңызшы." : "Этот номер не нашла. Повторите, пожалуйста, номер телефона.", actions, done: false };
      }
      state.client_id = String(call.result!.client_id);
      state.client_name = String(call.result!.full_name);
    } else {
      state.awaiting = { kind: "slot", slot: "phone" };
      const p = slotByNameOf("phone")!.prompt[lang];
      return { text: firstEntry ? `${openingLead(R.opening)} ${p}`.trim() : p, actions, done: false };
    }
  }

  // 2. slots
  const missing = firstMissing(sc, slots, state);
  if (missing) {
    state.awaiting = { kind: "slot", slot: missing };
    const p = slotByNameOf(missing)?.prompt[lang] ?? (lang === "kk" ? "Нақтылаңызшы." : "Уточните, пожалуйста.");
    // On first entry use the scenario's own opening line (dataset style guide); it usually asks for the first slot.
    return { text: firstEntry && /[?？]/.test(R.opening) ? R.opening : p, actions, done: false };
  }

  // 3. actions (read + calc), collecting placeholders
  const ph: Record<string, unknown> = { ...slots };
  const client = state.client_id ? backend.clients.find((c) => c.client_id === state.client_id) : undefined;
  if (client) { ph.phone = client.phone; ph.client_id = client.client_id; }
  const topic = kbTopicFor(sc.scenario_id, String(slots.product_type ?? ""), String(slots.topic ?? "") + " " + String(slots.service_name ?? ""));
  const readActions = sc.actions.filter((a) => !isIrreversible(a) && a !== "send_sms" && a !== "transfer_to_operator" && a !== "find_client");
  let handoff: string | undefined;

  for (const name of readActions) {
    const input: Record<string, unknown> = { ...ph, lang, topic };
    if (name === "get_bm_class") input.iin = slots.iin ?? (Array.isArray(slots.drivers_iin) ? (slots.drivers_iin as string[])[0] : client?.iin);
    const call = push(runAction(name, input));
    if (call.result) Object.assign(ph, call.result);
    if (call.error) {
      if (name === "get_claim" || name === "get_policy" || name === "get_policies" || name === "check_payment") {
        state.awaiting = { kind: "slot", slot: name === "get_claim" ? "claim_number" : name === "check_payment" ? "payment_date" : "policy_number" };
        delete state.slots[state.awaiting.slot];
        return { text: lang === "kk" ? "Деректер табылмады. Нөмірді қайта айтыңызшы." : "Не нашла данные. Повторите, пожалуйста, номер.", actions, done: false };
      }
      if (name === "calc_casco_price" && call.error.code === "not_eligible") {
        return { text: lang === "kk" ? "Өкінішке қарай, 10 жастан асқан көлікке КАСКО Стандарт рәсімделмейді. Lite бағдарламасы (ұрлық және толық жойылу) 15 жасқа дейін қолжетімді." : "К сожалению, на машину старше 10 лет КАСКО Стандарт не оформляется. Есть программа Lite (угон и полная гибель) до 15 лет.", actions, done: true };
      }
    }
  }

  // scenario-specific handoff rules
  if (sc.scenario_id === "SC30" && ph.payment_status === "charged_policy_not_issued") handoff = "operator_general";
  if (sc.scenario_id === "SC15" || sc.scenario_id === "SC37" || sc.scenario_id === "SC10") handoff = sc.handoff?.queue;
  if (sc.scenario_id === "SC11" && slots.injured === true) handoff = "claims_team";

  // status text for SC17
  if (sc.scenario_id === "SC17" && ph.status) ph.status = statusText(String(ph.status), lang);

  // 4. irreversible → preview + confirmation
  const irreversible = sc.actions.filter(isIrreversible);
  if (sc.requires_confirmation && irreversible.length && state.awaiting?.kind !== "confirmation") {
    const name = irreversible[0];
    const call = push(runAction(name, { ...ph, __mode: "preview", product_type: productFor(sc), product: productFor(sc) }));
    // read back the CLIENT's data (existing policy number etc.) plus computed amounts — not the new ids the action would create
    const computed: Record<string, unknown> = {};
    for (const k of ["price", "refund_amount", "slot_datetime", "clinic_name", "address", "extra_premium", "coverage", "zone"]) if (call.result?.[k] !== undefined) computed[k] = call.result[k];
    const readback = { ...ph, ...computed };
    state.awaiting = { kind: "confirmation", action: name };
    state.slots.__preview = readback;
    return { text: previewText(sc, readback, lang), actions, done: false };
  }

  // 5. non-irreversible finishing actions (send_sms, create_complaint, etc. already run above)
  const text = fill(R.closing, ph, lang);
  state.awaiting = null;
  return { text, actions, done: true, handoff };
}

async function confirmFlow(state: DialogState, kind: "yes" | "no", lang: ReplyLang, log: ActionCall[]): Promise<ExecResult> {
  const actions: ActionCall[] = [];
  const sc = state.active_scenario ? scenarioById(state.active_scenario) : undefined;
  if (!sc || state.awaiting?.kind !== "confirmation") {
    state.awaiting = null;
    return { text: lang === "kk" ? "Жарайды. Тағы қалай көмектесе аламын?" : "Хорошо. Чем ещё могу помочь?", actions, done: true };
  }
  const ph = (state.slots.__preview as Record<string, unknown>) ?? { ...state.slots };
  if (kind === "no") {
    state.awaiting = null;
    delete state.slots.__preview;
    return { text: lang === "kk" ? "Жарайды, рәсімдемеймін. Тағы сұрағыңыз бар ма?" : "Хорошо, не оформляю. Есть ещё вопросы?", actions, done: true };
  }
  const name = state.awaiting.action;
  const call = runAction(name, { ...ph, __mode: "execute", product_type: productFor(sc), product: productFor(sc) });
  actions.push(call); log.push(call);
  if (call.result) Object.assign(ph, call.result);
  if (sc.actions.includes("send_sms") && ph.phone) { const s = runAction("send_sms", { phone: ph.phone, __mode: "execute" }); actions.push(s); log.push(s); }
  state.awaiting = null;
  delete state.slots.__preview;
  const handoff = sc.scenario_id === "SC13" && /(угнал|ұрла|полная гибель)/i.test(String(ph.incident_description ?? "")) ? "claims_team" : undefined;
  return { text: fill(sc.responses[lang].closing, ph, lang), actions, done: true, handoff };
}

/* ------------------------------ helpers ------------------------------ */

function openingLead(opening: string): string {
  // keep only the empathetic lead ("Сейчас проверю.") and drop the question, we ask our own
  const m = opening.split(/[.!]\s/)[0];
  return m.length < 60 ? m + "." : "";
}

function productFor(sc: Scenario): string {
  const map: Record<string, string> = { SC02: "ogpo", SC06: "travel", SC27: "ogpo", SC13: "casco", SC12: "ogpo", SC14: "property", SC16: "accident", SC21: "dms" };
  return map[sc.scenario_id] ?? (sc.domain === "auto" ? "ogpo" : sc.domain === "health" ? "dms" : sc.domain);
}

function kbTopicFor(id: ScenarioId, product: string, hint: string): string {
  switch (id) {
    case "SC31": return /(рассрочк|бөліп)/i.test(hint) ? "payments.installments" : "payments.methods";
    case "SC18": return `claims.documents.${product === "ogpo" ? "ogpo_victim" : product || "casco"}`;
    case "SC34": return "app_help.login";
    case "SC38": return "fraud_policy";
    case "SC11": return "claims.road_accident_now";
    case "SC24": return "dms.e_card";
    case "SC09": return "dms.individual";
    case "SC32": return "bonus_malus";
    case "SC28": return "cancellation";
    case "SC40": return /(франшиз)/i.test(hint) ? "terms.franchise" : /(лимит|limit)/i.test(hint) ? "terms.limit" : "terms.exclusions";
    default: return "payments.methods";
  }
}

function statusText(s: string, lang: ReplyLang): string {
  const m: Record<string, { ru: string; kk: string }> = {
    paid: { ru: "выплата произведена", kk: "төлем жасалды" },
    approved: { ru: "решение положительное, выплата назначена", kk: "шешім оң, төлем тағайындалды" },
    under_review: { ru: "на рассмотрении", kk: "қаралуда" },
    documents_requested: { ru: "ожидаем документы", kk: "құжаттар күтілуде" },
  };
  return m[s]?.[lang] ?? s;
}

function previewText(sc: Scenario, ph: Record<string, unknown>, lang: ReplyLang): string {
  const parts: string[] = [];
  const show = (k: string, label: { ru: string; kk: string }) => { if (ph[k] !== undefined && ph[k] !== null && ph[k] !== "") parts.push(`${label[lang]} ${fmt(k, ph[k], lang)}`); };
  show("policy_number", { ru: "полис", kk: "полис" });
  show("vehicle_plate", { ru: "машина", kk: "көлік" });
  show("culprit_vehicle_plate", { ru: "машина виновника", kk: "кінәлі көлік" });
  show("incident_date", { ru: "дата", kk: "күні" });
  show("trip_country", { ru: "страна", kk: "ел" });
  show("travelers_count", { ru: "человек:", kk: "адам саны:" });
  show("doctor_specialty", { ru: "врач", kk: "дәрігер" });
  show("preferred_date", { ru: "дата", kk: "күні" });
  show("new_driver_iin", { ru: "ИИН водителя", kk: "жүргізуші ЖСН" });
  show("contact_field", { ru: "меняем", kk: "өзгертеміз" });
  show("new_value", { ru: "на", kk: "жаңа мән" });
  show("price", { ru: "стоимость", kk: "бағасы" });
  show("refund_amount", { ru: "к возврату", kk: "қайтарылады" });
  show("phone", { ru: "телефон", kk: "телефон" });
  const body = parts.join(", ");
  const irreversibleNote = lang === "kk" ? " Бұл әрекетті кейін қайтару мүмкін емес." : " Это действие нельзя будет отменить.";
  const note = ["cancel_policy", "create_policy", "renew_policy"].some((a) => sc.actions.includes(a)) ? irreversibleNote : "";
  return lang === "kk" ? `Тексерейін: ${body}. Растайсыз ба?${note}` : `Проверю: ${body}. Всё верно?${note}`;
}

function fmt(k: string, v: unknown, lang: ReplyLang): string {
  if (typeof v === "number" && /(price|amount|premium|refund)/.test(k)) return `${fmtMoney(v)} ${lang === "kk" ? "теңге" : "тенге"}`;
  if (k === "email" || k === "sent_to") return maskEmail(String(v));
  if (k === "phone") return String(v).replace(/^\+7(\d{3})(\d{3})(\d{2})(\d{2})$/, "+7 $1 $2 $3 $4");
  if (Array.isArray(v)) return v.join(", ");
  return String(v);
}

function fill(template: string, ph: Record<string, unknown>, lang: ReplyLang): string {
  return template.replace(/\{(\w+)\}(\s*(?:тенге|теңге))?/g, (_, k: string, unit?: string) => {
    const v = ph[k];
    if (v === undefined || v === null) return (k === "answer" ? "" : "…") + (unit ?? "");
    // the template already carries the currency word → format the bare number
    if (unit && typeof v === "number") return fmtMoney(v) + unit;
    return fmt(k, v, lang) + (unit ?? "");
  }).replace(/\s{2,}/g, " ").trim();
}

function chunk(text: string): string[] {
  const parts = text.match(/[^.!?]+[.!?]?\s*/g) ?? [text];
  return parts.filter((p) => p.trim().length);
}

export { AS_OF_DATE };
