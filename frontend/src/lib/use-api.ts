"use client";

import { useCallback, useEffect, useState } from "react";
import { errorMessage } from "./api";

export interface ApiState<T> {
  data: T | null;
  error: string | null;
  loading: boolean;
  refreshing: boolean;
  fetchedAt: number | null;
}

export type Interval<T> = number | ((data: T | null) => number);

/**
 * Small client-side fetch hook. `loader` (and a function `interval`) must be
 * referentially stable — a module function or a `useCallback` — otherwise the
 * effect re-runs every render. A function interval is re-evaluated after each
 * response, so polling can depend on the payload (e.g. "still running").
 */
export function useApi<T>(loader: () => Promise<T>, opts: { interval?: Interval<T>; enabled?: boolean } = {}) {
  const { interval = 0, enabled = true } = opts;
  const [tick, setTick] = useState(0);
  const [state, setState] = useState<ApiState<T>>({
    data: null,
    error: null,
    loading: true,
    refreshing: false,
    fetchedAt: null,
  });

  useEffect(() => {
    if (!enabled) return;
    let alive = true;
    let timer = 0;
    const schedule = (data: T | null) => {
      const ms = typeof interval === "function" ? interval(data) : interval;
      if (ms > 0) timer = window.setTimeout(run, ms);
    };
    function run() {
      timer = 0;
      loader().then(
        (data) => {
          if (!alive) return;
          setState({ data, error: null, loading: false, refreshing: false, fetchedAt: Date.now() });
          schedule(data);
        },
        (e: unknown) => {
          if (!alive) return;
          setState((s) => ({ ...s, error: errorMessage(e), loading: false, refreshing: false }));
          schedule(null);
        },
      );
    }
    run();
    return () => {
      alive = false;
      if (timer) window.clearTimeout(timer);
    };
  }, [loader, interval, enabled, tick]);

  const refresh = useCallback(() => {
    setState((s) => ({ ...s, refreshing: true }));
    setTick((t) => t + 1);
  }, []);

  return { ...state, refresh };
}
