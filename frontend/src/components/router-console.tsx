"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import {
  Activity,
  ArrowDownToLine,
  ArrowRight,
  AudioLines,
  BookOpen,
  ChevronRight,
  Clock3,
  FileAudio,
  Headphones,
  Layers3,
  LoaderCircle,
  Mic,
  Plus,
  Radio,
  RotateCcw,
  Send,
  ShieldCheck,
  Square,
  Terminal,
  Volume2,
  VolumeX,
  Waypoints,
} from "lucide-react";
import {
  api,
  Health,
  Scenario,
  Session,
  streamTurn,
  TraceEvent,
  transcribe,
  Turn,
} from "@/lib/api";
import {
  browserRecognition,
  Capture,
  playPCM,
  realtimeCapture,
} from "@/lib/audio";

type Tab = "simulator" | "catalog" | "sessions" | "metrics";
type Stats = {
  source_counts: Record<string, number>;
  routing_n: number;
  routing_p50_ms: number | null;
  routing_p95_ms: number | null;
  audio_n: number;
  end_to_audio_p50_ms: number | null;
  end_to_audio_p95_ms: number | null;
};
const fmt = (n: number | null | undefined) =>
  n == null
    ? "—"
    : n > 0 && n < 1
      ? `${n.toFixed(2)} мс`
      : `${Math.round(n)} мс`;
const statusName = {
  route: "Сценарий выбран",
  clarify: "Нужно уточнение",
  handoff: "Нужен оператор",
};
const examples = [
  "Деньги списались, а полиса нет",
  "Өтемақы қашан түседі?",
  "Полисімді ұзартқым келеді, он завтра заканчивается",
];

export default function RouterConsole() {
  const [tab, setTab] = useState<Tab>("simulator");
  const [health, setHealth] = useState<Health | null>(null);
  const [catalog, setCatalog] = useState<Scenario[]>([]);
  const [session, setSession] = useState<Session | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [text, setText] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [listening, setListening] = useState(false);
  const [phase, setPhase] = useState("Готов к разговору");
  const [partial, setPartial] = useState("");
  const [events, setEvents] = useState<TraceEvent[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [sound, setSound] = useState(true);
  const [language, setLanguage] = useState("ru-RU");
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(false);
  const sessionRef = useRef<Session | null>(null);
  const busyRef = useRef(false);
  const captureRef = useRef<Capture | null>(null);
  const micControllerRef = useRef<AbortController | null>(null);
  const recognitionRef = useRef<ReturnType<typeof browserRecognition> | null>(
    null,
  );
  const controllerRef = useRef<AbortController | null>(null);
  const contextRef = useRef<AudioContext | null>(null);
  const fileRef = useRef<HTMLInputElement | null>(null);
  const chatRef = useRef<HTMLDivElement | null>(null);
  const alive = useRef(true);
  const turns = session?.turns || [];
  const activeTurn = turns.find((t) => t.id === selected) || turns.at(-1);
  const locked = busy || listening;
  useEffect(() => {
    alive.current = true;
    Promise.all([
      api<Health>("/healthz"),
      api<{ scenarios: Scenario[] }>("/api/catalog"),
    ])
      .then(([h, c]) => {
        if (alive.current) {
          setHealth(h);
          setCatalog(c.scenarios);
        }
      })
      .catch((e) => {
        if (alive.current) setError(`API недоступен: ${e.message}`);
      });
    return () => {
      alive.current = false;
      controllerRef.current?.abort();
      captureRef.current?.close();
      micControllerRef.current?.abort();
      recognitionRef.current?.cancel();
      window.speechSynthesis?.cancel();
      void contextRef.current?.close();
      contextRef.current = null;
    };
  }, []);
  useEffect(() => {
    chatRef.current?.scrollTo({
      top: chatRef.current.scrollHeight,
      behavior: "smooth",
    });
  }, [turns.length, partial, busy]);
  function remember(s: Session | null) {
    sessionRef.current = s;
    setSession(s);
  }
  function unlockAudio() {
    if (!contextRef.current || contextRef.current.state === "closed")
      contextRef.current = new AudioContext();
    void contextRef.current.resume();
  }
  function addEvent(event: TraceEvent) {
    setEvents((old) => [...old, event]);
  }
  async function loadTab(next: Tab) {
    setTab(next);
    if (next === "sessions" || next === "metrics") {
      setLoading(true);
      setError("");
      try {
        if (next === "sessions")
          setSessions(await api<Session[]>("/api/sessions"));
        else setStats(await api<Stats>("/api/stats"));
      } catch (e) {
        setError((e as Error).message);
      } finally {
        setLoading(false);
      }
    }
  }
  function newSession() {
    remember(null);
    setEvents([]);
    setSelected(null);
    setPartial("");
    setError("");
    setPhase("Готов к разговору");
  }
  async function speak(
    turn: Turn,
    sessionID: string,
    endAt: number | undefined,
    signal: AbortSignal,
  ) {
    setPhase("Отвечаю голосом");
    async function measured(ttsMS: number | undefined, playbackAt: number) {
      const endMS =
        endAt !== undefined && Number.isFinite(endAt)
          ? Math.max(0, playbackAt - endAt)
          : undefined;
      const updated = sessionRef.current;
      if (updated)
        remember({
          ...updated,
          turns: updated.turns.map((t) =>
            t.id === turn.id
              ? {
                  ...t,
                  timing: {
                    ...t.timing,
                    end_to_audio_ms: endMS,
                    tts_first_byte_ms: ttsMS,
                  },
                }
              : t,
          ),
        });
      addEvent({
        type: "audio_started",
        elapsed_ms: 0,
        data: {
          end_to_audio_ms: endMS ?? "Нет акустической отметки конца речи",
          tts_first_byte_ms: ttsMS ?? "Browser TTS",
          measurement: "browser playback timing estimate",
        },
      });
      try {
        await api(`/api/sessions/${sessionID}/turns/${turn.id}/metrics`, {
          method: "POST",
          body: JSON.stringify({
            end_to_audio_ms: endMS,
            tts_first_byte_ms: ttsMS,
          }),
        });
      } catch {
        setError("Ответ готов, но метрика воспроизведения не сохранилась.");
      }
    }
    if (health?.speech_ready && contextRef.current)
      await playPCM(
        sessionID,
        turn.id,
        contextRef.current,
        signal,
        (ms, at) => {
          void measured(ms, at);
        },
      );
    else {
      if (!window.speechSynthesis)
        throw new Error("Озвучка не поддерживается этим браузером");
      await new Promise<void>((resolve, reject) => {
        const utterance = new SpeechSynthesisUtterance(turn.reply);
        utterance.lang = turn.decision.language === "kk" ? "kk-KZ" : "ru-RU";
        const voices = window.speechSynthesis.getVoices();
        const voice = voices.find((v) =>
          v.lang.startsWith(utterance.lang.slice(0, 2)),
        );
        if (utterance.lang === "kk-KZ" && !voice) {
          reject(
            new Error(
              "В системе нет казахского голоса. Для озвучки kk подключите OpenAI TTS.",
            ),
          );
          return;
        }
        if (voice) utterance.voice = voice;

        const abort = () => {
          window.speechSynthesis.cancel();
          clearTimeout(timer);
          reject(new DOMException("Aborted", "AbortError"));
        };
        signal.addEventListener("abort", abort, { once: true });
        const finish = () => {
          clearTimeout(timer);
          signal.removeEventListener("abort", abort);
        };
        utterance.onstart = () => {
          void measured(undefined, performance.now());
        };
        utterance.onend = () => {
          finish();
          resolve();
        };
        utterance.onerror = (e) => {
          finish();
          reject(new Error(`Озвучка: ${e.error}`));
        };
        const timer = setTimeout(() => {
          window.speechSynthesis.cancel();
          finish();
          reject(new Error("Истекло время ожидания озвучки"));
        }, 30000);
        window.speechSynthesis.speak(utterance);
      });
    }
  }
  async function run(
    value: string,
    inputKind = "text",
    sttMS?: number,
    endAt?: number,
  ) {
    if (!value.trim() || busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    setListening(false);
    setError("");
    setEvents([]);
    setPartial(value);
    setPhase("Выбираю сценарий");
    setText("");
    const controller = new AbortController();
    controllerRef.current = controller;
    try {
      const result = await streamTurn(
        {
          session_id: sessionRef.current?.id || "",
          text: value,
          input_kind: inputKind,
          stt_ms: sttMS,
        },
        (event) => {
          addEvent(event);
          if (event.type === "policy") setPhase("Готовлю ответ");
        },
        controller.signal,
      );
      const previous = sessionRef.current;
      remember({
        id: result.session_id,
        turns: [...(previous?.turns || []), result.turn],
        active:
          result.turn.decision.status === "route"
            ? result.turn.decision.scenario_id
            : previous?.active || "",
        pending: result.turn.pending_topics || result.turn.decision.pending,
        created_at: previous?.created_at || new Date().toISOString(),
      });
      setSelected(result.turn.id);
      setPartial("");
      if (sound)
        await speak(result.turn, result.session_id, endAt, controller.signal);
      setPhase("Готов к разговору");
    } catch (e) {
      if ((e as Error).name !== "AbortError") {
        setError((e as Error).message);
        setPhase("Требуется внимание");
      }
    } finally {
      busyRef.current = false;
      if (alive.current) setBusy(false);
    }
  }
  async function microphone() {
    if (listening) {
      if (captureRef.current) captureRef.current.commit();
      else if (recognitionRef.current) recognitionRef.current.stop();
      else {
        micControllerRef.current?.abort();
        setListening(false);
        setPhase("Готов к разговору");
      }
      return;
    }
    const micController = new AbortController();
    micControllerRef.current = micController;
    setError("");
    setPartial("");
    setListening(true);
    setPhase("Подключаю микрофон");
    unlockAudio();
    const failed = (message: string) => {
      setListening(false);
      setError(message);
      setPhase("Требуется внимание");
    };
    const final = (value: string, endAt: number, stt: number) => {
      captureRef.current = null;
      recognitionRef.current = null;
      void run(value, "microphone", stt, endAt);
    };
    try {
      if (health?.speech_ready)
        captureRef.current = await realtimeCapture(
          setPhase,
          setPartial,
          final,
          failed,
          micController.signal,
        );
      else {
        recognitionRef.current = browserRecognition(
          language,
          setPartial,
          final,
          failed,
        );
        setPhase("Слушаю · браузер");
      }
    } catch (e) {
      if (!micController.signal.aborted) failed((e as Error).message);
    }
  }
  async function upload(file: File) {
    if (file.size > 24 * 1024 * 1024) {
      setError("Максимальный размер аудио — 24 МБ");
      return;
    }
    setBusy(true);
    busyRef.current = true;
    setError("");
    setPhase("Распознаю аудиофайл");
    unlockAudio();
    try {
      const result = await transcribe(file, file.name);
      busyRef.current = false;
      await run(result.text, "audio_file", result.stt_ms);
    } catch (e) {
      setError((e as Error).message);
      setPhase("Требуется внимание");
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }
  function download() {
    if (!session) return;
    const blob = new Blob(
      [
        JSON.stringify(
          {
            session,
            live_events: events,
            configuration: health,
            exported_at: new Date().toISOString(),
          },
          null,
          2,
        ),
      ],
      { type: "application/json" },
    );
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `voice-router-${session.id.slice(0, 8)}.json`;
    a.click();
    URL.revokeObjectURL(url);
  }
  return (
    <div className="workspace">
      <aside className="sidebar">
        <Link className="brand" href="/">
          <span className="brand-mark">+</span>
          <span>
            plus<span className="brand-dot">.</span>
          </span>
        </Link>
        <div className="sidebar-label">VOICE INTELLIGENCE</div>
        <nav>
          {(
            [
              { id: "simulator", label: "Симулятор", icon: Headphones },
              { id: "catalog", label: "Каталог сценариев", icon: Layers3 },
              { id: "sessions", label: "История диалогов", icon: BookOpen },
              { id: "metrics", label: "Метрики", icon: Activity },
            ] as const
          ).map((item) => (
            <button
              key={item.id}
              aria-label={item.label}
              title={item.label}
              className={`nav-item ${tab === item.id ? "active" : ""}`}
              onClick={() => void loadTab(item.id)}
            >
              <item.icon size={18} strokeWidth={1.75} />
              <span>{item.label}</span>
              {tab === item.id && <ChevronRight size={14} />}
            </button>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="sidebar-line">
            <span className={`dot ${health ? "green" : ""}`} />
            {health ? "API подключён" : "Ожидаем API"}
          </div>
          <span>HackAlem AI · Halyk Bank</span>
          <small>Team Plus / 2026</small>
        </div>
      </aside>
      <main className="main">
        <header className="topbar">
          <div className="breadcrumb">
            Рабочее пространство <ChevronRight size={13} />
            <strong>Voice Router</strong>
          </div>
          <span className="pill neutral">
            RU <span>/</span> KZ
          </span>
        </header>
        <div className="page-heading">
          <div>
            <div className="eyebrow">
              <Radio size={13} /> OBSERVABLE VOICE AI
            </div>
            <h1>
              {tab === "simulator"
                ? "Разговор со смыслом"
                : tab === "catalog"
                  ? "Каталог сценариев"
                  : tab === "sessions"
                    ? "История диалогов"
                    : "Скорость в цифрах"}
            </h1>
            <p>
              {tab === "simulator"
                ? "Клиент говорит. Модель выбирает. Вы видите каждое решение."
                : "Данные и решения маршрутизатора — в одном рабочем пространстве."}
            </p>
          </div>
          <button
            className="button secondary"
            onClick={() => {
              newSession();
              setTab("simulator");
            }}
            disabled={locked}
          >
            <Plus size={16} />
            Новый диалог
          </button>
        </div>
        {error && (
          <div className="notice error" role="alert">
            {error}
            <button onClick={() => setError("")} aria-label="Закрыть ошибку">
              ×
            </button>
          </div>
        )}
        {health?.catalog_source === "synthetic_demo" && (
          <div className="notice">
            <ShieldCheck size={16} />
            <span>
              Демо-каталог · {health.catalog_count} синтетических сценариев. Для
              оценки жюри подключите исходные 40 через DATA_DIR.
            </span>
          </div>
        )}
        {tab === "simulator" && (
          <>
            <div className="summary-grid">
              <div className="summary-card">
                <div className="summary-label">
                  <Waypoints size={15} />
                  Маршрутизатор
                </div>
                <strong>
                  {health?.provider === "mock"
                    ? "Mock / демо"
                    : health?.model || "Подключение…"}
                </strong>
                <span>
                  {health?.provider === "mock"
                    ? "Тестовый дубль, без вызова LLM"
                    : "LLM · полный каталог + контекст"}
                </span>
              </div>
              <div className="summary-card">
                <div className="summary-label">
                  <Clock3 size={15} />
                  Выбор сценария
                </div>
                <strong
                  className={
                    activeTurn && activeTurn.timing.routing_ms > 500
                      ? "amber-text"
                      : ""
                  }
                >
                  {fmt(activeTurn?.timing.routing_ms)}
                  <small> / 500 мс</small>
                </strong>
                <span>
                  {activeTurn?.source === "mock"
                    ? "Mock — не замер производительности LLM"
                    : "Последняя реплика · цель, не гарантия"}
                </span>
              </div>
              <div className="summary-card">
                <div className="summary-label">
                  <AudioLines size={15} />
                  До начала ответа
                </div>
                <strong
                  className={
                    (activeTurn?.timing.end_to_audio_ms || 0) > 1500
                      ? "amber-text"
                      : ""
                  }
                >
                  {fmt(activeTurn?.timing.end_to_audio_ms)}
                  <small> / 1 500 мс</small>
                </strong>
                <span>Конец речи → старт воспроизведения</span>
              </div>
            </div>
            <div className="console-grid">
              <section className="panel conversation">
                <div className="panel-head">
                  <div>
                    <span className={`dot ${listening ? "pulse" : "green"}`} />
                    <h2>Живой диалог</h2>
                    <span className="subtle">{turns.length} / 10</span>
                  </div>
                  <button
                    className="icon-button"
                    title={sound ? "Отключить озвучку" : "Включить озвучку"}
                    aria-label={
                      sound ? "Отключить озвучку" : "Включить озвучку"
                    }
                    onClick={() => setSound(!sound)}
                    disabled={locked}
                  >
                    {sound ? <Volume2 size={17} /> : <VolumeX size={17} />}
                  </button>
                </div>
                <div className="chat" ref={chatRef} aria-live="polite">
                  {!turns.length && !partial && (
                    <div className="empty-chat">
                      <div className="voice-orb">
                        <AudioLines size={38} strokeWidth={1.35} />
                      </div>
                      <h3>Начните с простого «Здравствуйте»</h3>
                      <p>
                        Расскажите о своей ситуации.
                        <br />
                        Можно сменить тему или язык в середине диалога.
                      </p>
                      <span className="pill neutral">
                        Страховая компания · симуляция
                      </span>
                    </div>
                  )}
                  {turns.map((turn, i) => (
                    <div
                      key={turn.id}
                      className={`exchange ${activeTurn?.id === turn.id ? "selected-exchange" : ""}`}
                    >
                      <div className="message user">
                        <span className="message-label">
                          КЛИЕНТ <span>{String(i + 1).padStart(2, "0")}</span>
                        </span>
                        <p>{turn.text}</p>
                      </div>
                      <button
                        className="message assistant"
                        onClick={() => setSelected(turn.id)}
                      >
                        <span className="message-label">
                          <span className="tiny-plus">+</span> VOICE ROUTER{" "}
                          <span>{turn.decision.language.toUpperCase()}</span>
                        </span>
                        <p>{turn.reply}</p>
                        <span className={`route-chip ${turn.decision.status}`}>
                          <Waypoints size={12} />
                          {turn.scenario_name ||
                            statusName[turn.decision.status]}
                          <ChevronRight size={12} />
                        </span>
                      </button>
                    </div>
                  ))}
                  {partial && (
                    <div className="message user pending">
                      <span className="message-label">
                        {listening ? "РАСПОЗНАЁМ РЕЧЬ" : "КЛИЕНТ"}
                      </span>
                      <p>{partial}</p>
                    </div>
                  )}
                  {busy && (
                    <div className="processing">
                      <LoaderCircle size={14} className="spin" />
                      {phase}
                    </div>
                  )}
                </div>
                {!turns.length && (
                  <div className="examples">
                    {examples.map((e) => (
                      <button
                        key={e}
                        disabled={locked}
                        onClick={() => setText(e)}
                      >
                        {e}
                        <ArrowRight size={12} />
                      </button>
                    ))}
                  </div>
                )}
                <div className="composer">
                  <div className="voice-controls">
                    <button
                      className={`mic-button ${listening ? "recording" : ""}`}
                      disabled={busy || turns.length >= 10 || !health}
                      onClick={() => void microphone()}
                    >
                      {listening ? (
                        <Square size={17} fill="currentColor" />
                      ) : (
                        <Mic size={17} />
                      )}
                      <span>
                        {listening ? "Завершить реплику" : "Говорить"}
                      </span>
                    </button>
                    <div className="voice-status">
                      <strong>{phase}</strong>
                      <span>
                        {health?.speech_ready
                          ? "Потоковый STT · автоматическая пауза 360 мс"
                          : "Web Speech · доступность зависит от браузера"}
                      </span>
                    </div>
                    <select
                      aria-label="Язык браузерного распознавания"
                      value={language}
                      disabled={locked || health?.speech_ready}
                      onChange={(e) => setLanguage(e.target.value)}
                    >
                      <option value="ru-RU">RU</option>
                      <option value="kk-KZ">KZ</option>
                    </select>
                  </div>
                  <form
                    onSubmit={(e) => {
                      e.preventDefault();
                      unlockAudio();
                      void run(text);
                    }}
                  >
                    <input
                      aria-label="Текст реплики"
                      placeholder="Или напишите сообщение…"
                      value={text}
                      maxLength={4000}
                      onChange={(e) => setText(e.target.value)}
                      disabled={locked || turns.length >= 10}
                    />
                    <button
                      className="icon-button"
                      type="button"
                      title="Загрузить аудио"
                      aria-label="Загрузить аудио"
                      onClick={() => fileRef.current?.click()}
                      disabled={locked || !health || turns.length >= 10}
                    >
                      <FileAudio size={18} />
                    </button>
                    <button
                      className="send-button"
                      type="submit"
                      aria-label="Отправить реплику"
                      disabled={
                        locked || !text.trim() || !health || turns.length >= 10
                      }
                    >
                      <Send size={16} />
                    </button>
                  </form>
                  <input
                    className="hidden"
                    ref={fileRef}
                    type="file"
                    accept="audio/*"
                    onChange={(e) => {
                      const f = e.target.files?.[0];
                      if (f) void upload(f);
                      e.target.value = "";
                    }}
                  />
                  <div className="composer-note">
                    Голос синтезирован AI. Используйте только вымышленные
                    данные.
                  </div>
                </div>
              </section>
              <section className="panel trace-panel">
                <div className="panel-head">
                  <div>
                    <Terminal size={16} />
                    <h2>Трассировка решения</h2>
                  </div>
                  <span className="live-tag">
                    <span className="dot green" />
                    LIVE
                  </span>
                </div>
                <div className="trace-body">
                  <div className="pipeline">
                    {[
                      { text: "Речь", icon: AudioLines },
                      { text: "LLM", icon: Waypoints },
                      { text: "Проверка", icon: ShieldCheck },
                      { text: "Ответ", icon: Volume2 },
                    ].map((step, i) => (
                      <div
                        key={step.text}
                        className={activeTurn ? "complete" : ""}
                      >
                        <span>
                          <step.icon size={16} />
                        </span>
                        <small>{step.text}</small>
                        {i < 3 && (
                          <ChevronRight size={12} className="connector" />
                        )}
                      </div>
                    ))}
                  </div>
                  {activeTurn ? (
                    <>
                      <div className="trace-section">
                        <div className="section-label">
                          РЕШЕНИЕ{" "}
                          <span
                            className={`status-text ${activeTurn.decision.status}`}
                          >
                            {statusName[activeTurn.decision.status]}
                          </span>
                        </div>
                        <h3>
                          {activeTurn.scenario_name ||
                            "Перед выбором нужно уточнение"}
                        </h3>
                        <code>{activeTurn.decision.scenario_id || "—"}</code>
                        <p>{activeTurn.decision.reason}</p>
                        <div className="confidence">
                          <span>Самооценка модели</span>
                          <b>
                            {Math.round(activeTurn.decision.confidence * 100)}%
                          </b>
                        </div>
                        <div className="meter">
                          <span
                            style={{
                              width: `${activeTurn.decision.confidence * 100}%`,
                            }}
                          />
                        </div>
                        <small className="muted">
                          Не калиброванная вероятность правильности
                        </small>
                      </div>
                      {activeTurn.topic_changed && (
                        <div className="topic-change">
                          <RotateCcw size={14} />
                          Тема изменена: {activeTurn.previous_scenario} →{" "}
                          {activeTurn.decision.scenario_id}
                        </div>
                      )}
                      <div className="trace-section">
                        <div className="section-label">АЛЬТЕРНАТИВЫ</div>
                        {activeTurn.decision.alternatives.length ? (
                          activeTurn.decision.alternatives.map((a, i) => (
                            <div
                              className="alternative"
                              key={`${a.scenario_id}-${i}`}
                            >
                              <span>{String(i + 1).padStart(2, "0")}</span>
                              <div>
                                <strong>
                                  {catalog.find((s) => s.id === a.scenario_id)
                                    ?.name || a.scenario_id}
                                </strong>
                                <p>{a.reason}</p>
                              </div>
                            </div>
                          ))
                        ) : (
                          <p className="muted">Альтернативы не предложены</p>
                        )}
                      </div>
                      <div className="trace-section">
                        <div className="section-label">ВРЕМЯ ПО ЭТАПАМ</div>
                        {[
                          [
                            "STT / завершение транскрипта",
                            activeTurn.timing.stt_ms,
                          ],
                          ["Выбор сценария", activeTurn.timing.routing_ms],
                          ["Проверка решения", activeTurn.timing.policy_ms],
                          [
                            "TTS / первый байт",
                            activeTurn.timing.tts_first_byte_ms,
                          ],
                        ].map(([label, value]) => (
                          <div className="timing-row" key={String(label)}>
                            <span>{label}</span>
                            <b>{fmt(value as number | undefined)}</b>
                          </div>
                        ))}
                        <p className="muted">
                          Для текста и файла нет метрики «конец речи». STT
                          микрофона включает ожидание паузы. PCM: оценка времени
                          воспроизведения браузером.
                        </p>
                      </div>
                      <div className="trace-section">
                        <div className="section-label">
                          КОНТЕКСТ И ПАРАМЕТРЫ
                        </div>
                        <p>
                          Отложенные темы:{" "}
                          {(
                            activeTurn.pending_topics ||
                            activeTurn.decision.pending
                          ).join(", ") || "нет"}
                        </p>
                        {activeTurn.decision.slots.map((s) => (
                          <div className="timing-row" key={s.name}>
                            <code>{s.name}</code>
                            <span>{s.value}</span>
                          </div>
                        ))}
                        {activeTurn.decision.slots.length === 0 && (
                          <p className="muted">Параметры не извлечены</p>
                        )}
                      </div>
                      {activeTurn.evidence?.length > 0 && (
                        <div className="trace-section">
                          <div className="section-label">
                            ПРОВЕРЕННЫЕ ДАННЫЕ
                          </div>
                          {activeTurn.evidence.map((e) => (
                            <details key={e.path} className="debug">
                              <summary>
                                {e.source} · {e.path}
                              </summary>
                              <pre className="evidence-json">
                                {JSON.stringify(e.value, null, 2)}
                              </pre>
                            </details>
                          ))}
                        </div>
                      )}
                      {activeTurn.warnings.map((w) => (
                        <p key={w} className="trace-warning">
                          {w}
                        </p>
                      ))}
                    </>
                  ) : (
                    <div className="empty-trace">
                      <Waypoints size={27} />
                      <h3>Решение появится здесь</h3>
                      <p>
                        Сценарий, короткое обоснование, альтернативы и задержки
                        — после каждой реплики.
                      </p>
                    </div>
                  )}
                  <details className="debug" open={busy}>
                    <summary>
                      <Terminal size={14} />
                      События и ответы API <span>{events.length}</span>
                    </summary>
                    <div className="event-log">
                      {events.length ? (
                        events.map((event, i) => (
                          <div key={i}>
                            <span>+{Math.round(event.elapsed_ms)} ms</span>
                            <strong>{event.type}</strong>
                            <pre>{JSON.stringify(event.data, null, 2)}</pre>
                          </div>
                        ))
                      ) : (
                        <p>Ожидаем первую реплику…</p>
                      )}
                    </div>
                  </details>
                </div>
                <div className="trace-footer">
                  <span>Публичное объяснение, не скрытые мысли модели</span>
                  <button
                    className="icon-button"
                    title="Скачать сессию и трассу JSON"
                    aria-label="Скачать сессию и трассу JSON"
                    onClick={download}
                    disabled={!session}
                  >
                    <ArrowDownToLine size={16} />
                  </button>
                </div>
              </section>
            </div>
            <div className="bottom-note">
              <span>
                <ShieldCheck size={13} />
                Операции не выполняются. Передача оператору — подготовка
                контекста.
              </span>
              <a href="/samples/01-payment.wav" download>
                Скачать тестовое аудио <ArrowDownToLine size={12} />
              </a>
            </div>
          </>
        )}
        {tab === "catalog" && (
          <section className="panel data-panel">
            <div className="panel-head">
              <h2>
                {catalog.length} сценариев ·{" "}
                {health?.catalog_source || "загрузка"}
              </h2>
              <input
                className="search"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Поиск по сценарию…"
                aria-label="Поиск сценариев"
              />
            </div>
            {!catalog.length ? (
              <div className="empty-state">
                Каталог загружается или API недоступен.
              </div>
            ) : (
              catalog
                .filter((s) =>
                  `${s.name} ${s.id} ${s.description}`
                    .toLowerCase()
                    .includes(search.toLowerCase()),
                )
                .map((s) => (
                  <details className="catalog-row" key={s.id}>
                    <summary>
                      <span className="catalog-icon">
                        <Layers3 size={17} />
                      </span>
                      <div>
                        <strong>{s.name}</strong>
                        <code>{s.id}</code>
                      </div>
                      <ChevronRight size={16} />
                    </summary>
                    <div>
                      <p>{s.description}</p>
                      <p>
                        <b>Границы:</b>{" "}
                        {s.boundaries || "Смотрите исходный каталог"}
                      </p>
                      <p>
                        <b>Ответ RU:</b> {s.response_ru}
                      </p>
                      <p>
                        <b>Ответ KZ:</b> {s.response_kk}
                      </p>
                      {s.requires_confirmation && (
                        <span className="pill neutral">
                          Требует подтверждения
                        </span>
                      )}
                    </div>
                  </details>
                ))
            )}
          </section>
        )}
        {tab === "sessions" && (
          <section className="panel data-panel">
            <div className="panel-head">
              <h2>Сохранённые сессии</h2>
              <button
                className="icon-button"
                onClick={() => void loadTab("sessions")}
                aria-label="Обновить историю"
              >
                <RotateCcw size={16} />
              </button>
            </div>
            {loading ? (
              <div className="empty-state">
                <LoaderCircle className="spin" />
                Загрузка…
              </div>
            ) : !sessions.length ? (
              <div className="empty-state">
                Пока нет диалогов. Начните с микрофона или текста.
              </div>
            ) : (
              sessions.map((s) => (
                <button
                  key={s.id}
                  className="session-row"
                  disabled={locked}
                  onClick={() => {
                    remember(s);
                    setSelected(null);
                    setEvents([]);
                    setTab("simulator");
                  }}
                >
                  <span className="catalog-icon">
                    <Headphones size={18} />
                  </span>
                  <div>
                    <strong>{s.turns[0]?.text || "Новый диалог"}</strong>
                    <small>
                      {new Date(s.created_at).toLocaleString("ru-RU")} ·{" "}
                      {s.id.slice(0, 8)}
                    </small>
                  </div>
                  <span>{s.turns.length} реплик</span>
                  <ChevronRight size={16} />
                </button>
              ))
            )}
          </section>
        )}
        {tab === "metrics" && (
          <section className="panel data-panel">
            <div className="panel-head">
              <h2>Измерения реальных LLM-вызовов</h2>
              <button
                className="icon-button"
                onClick={() => void loadTab("metrics")}
                aria-label="Обновить метрики"
              >
                <RotateCcw size={16} />
              </button>
            </div>
            {loading ? (
              <div className="empty-state">
                <LoaderCircle className="spin" />
                Загрузка…
              </div>
            ) : (
              <div className="metrics-body">
                <div className="summary-grid">
                  <div className="summary-card">
                    <span>Routing p50 / p95</span>
                    <strong>
                      {fmt(stats?.routing_p50_ms)} /{" "}
                      {fmt(stats?.routing_p95_ms)}
                    </strong>
                    <small>{stats?.routing_n || 0} запросов</small>
                  </div>
                  <div className="summary-card">
                    <span>Конец речи → аудио p50 / p95</span>
                    <strong>
                      {fmt(stats?.end_to_audio_p50_ms)} /{" "}
                      {fmt(stats?.end_to_audio_p95_ms)}
                    </strong>
                    <small>{stats?.audio_n || 0} голосовых реплик</small>
                  </div>
                  <div className="summary-card">
                    <span>Целевые задержки</span>
                    <strong>
                      500 / 1 500 <small>мс</small>
                    </strong>
                    <small>Проверяются на ваших данных и соединении</small>
                  </div>
                </div>
                <p>
                  Mock и ошибки провайдера исключены. Аудио-метрики получены от
                  браузера; точность маршрутизации измеряется отдельно скриптом
                  оценки с ожидаемыми сценариями.
                </p>
                <pre>{JSON.stringify(stats?.source_counts || {}, null, 2)}</pre>
              </div>
            )}
          </section>
        )}
        <footer className="page-footer">
          <span>
            <span className="tiny-plus">+</span> Plus Voice Router
          </span>
          <span>
            {health?.storage === "postgres" ? "PostgreSQL" : "Память процесса"}{" "}
            · {health?.catalog_hash || "…"}
          </span>
        </footer>
      </main>
    </div>
  );
}
