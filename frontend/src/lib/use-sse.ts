"use client";

import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";
import { sseUrl } from "./api";
import type { VREvent } from "./types";

export interface DebugRow {
  id: number;
  ev: VREvent;
  /** number of llm_delta events merged into this row */
  merged?: number;
  receivedAt: number;
}

export type SseStatus = "connecting" | "open" | "error";

const MAX_ROWS = 600;
const FLUSH_MS = 80;

const noopSubscribe = () => () => undefined;
const sseSupported = () => typeof EventSource !== "undefined";
const sseSupportedOnServer = () => true;

function mergeable(a: VREvent, b: VREvent): boolean {
  return a.type === "llm_delta" && b.type === "llm_delta" && a.session_id === b.session_id && a.turn === b.turn;
}

function appendRows(prev: DebugRow[], incoming: VREvent[], nextId: { current: number }, now: number): DebugRow[] {
  if (incoming.length === 0) return prev;
  const out = prev.slice();
  for (const ev of incoming) {
    const last = out[out.length - 1];
    if (last && mergeable(last.ev, ev)) {
      const prevText = (last.ev.data as { text?: string } | undefined)?.text ?? "";
      const text = (ev.data as { text?: string } | undefined)?.text ?? "";
      out[out.length - 1] = { ...last, ev: { ...last.ev, t: ev.t, data: { text: prevText + text } }, merged: (last.merged ?? 1) + 1 };
      continue;
    }
    out.push({ id: nextId.current++, ev, receivedAt: now });
  }
  return out.length > MAX_ROWS ? out.slice(out.length - MAX_ROWS) : out;
}

/** Subscribes to GET /api/debug/events (SSE). Events are batched every 80 ms; while paused they buffer. */
export function useDebugEvents(opts: { session?: string; replay?: boolean; paused: boolean }) {
  const { session, replay = true, paused } = opts;
  const [rows, setRows] = useState<DebugRow[]>([]);
  const [status, setStatus] = useState<SseStatus>("connecting");
  const [dropped, setDropped] = useState(0);
  const pending = useRef<VREvent[]>([]);
  const timer = useRef(0);
  const pausedRef = useRef(paused);
  const nextId = useRef(1);

  const flush = useCallback(() => {
    timer.current = 0;
    if (pausedRef.current) return;
    const batch = pending.current;
    if (batch.length === 0) return;
    pending.current = [];
    const now = Date.now();
    setRows((prev) => appendRows(prev, batch, nextId, now));
  }, []);

  const schedule = useCallback(() => {
    if (timer.current || pausedRef.current) return;
    timer.current = window.setTimeout(flush, FLUSH_MS);
  }, [flush]);

  useEffect(() => {
    pausedRef.current = paused;
    if (!paused) schedule();
  }, [paused, schedule]);

  const url = sseUrl({ session: session || undefined, replay });

  useEffect(() => {
    if (typeof EventSource === "undefined") {
      return;
    }
    let es: EventSource;
    try {
      es = new EventSource(url);
    } catch {
      return;
    }
    es.onopen = () => setStatus("open");
    es.onerror = () => setStatus("error");
    es.onmessage = (m: MessageEvent<string>) => {
      let ev: VREvent;
      try {
        ev = JSON.parse(m.data) as VREvent;
      } catch {
        setDropped((d) => d + 1);
        return;
      }
      pending.current.push(ev);
      if (pending.current.length > 5000) pending.current.splice(0, pending.current.length - 5000);
      schedule();
    };
    return () => {
      es.close();
      if (timer.current) {
        window.clearTimeout(timer.current);
        timer.current = 0;
      }
    };
  }, [url, schedule]);

  const clear = useCallback(() => {
    pending.current = [];
    setRows([]);
  }, []);

  // Hydration-safe: the server assumes support; the client corrects it after hydration.
  const supported = useSyncExternalStore(noopSubscribe, sseSupported, sseSupportedOnServer);

  return { rows, status, dropped, clear, supported };
}
