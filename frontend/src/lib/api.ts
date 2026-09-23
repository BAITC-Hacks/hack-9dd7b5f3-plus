import type {
  AppConfig,
  CatalogResponse,
  EvalLast,
  EvalReport,
  Health,
  RouteResponse,
  SessionDetail,
  SessionState,
  SessionSummary,
  Stats,
  TurnResponse,
} from "./types";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

/** Base URL of the Go backend. Build-time env wins; otherwise same host, port 8080. */
export function apiBase(): string {
  const env = process.env.NEXT_PUBLIC_API_URL;
  if (env && env.trim()) return env.trim().replace(/\/+$/, "");
  if (typeof window !== "undefined" && window.location) {
    return `${window.location.protocol}//${window.location.hostname}:8080`;
  }
  return "http://localhost:8080";
}

export function wsUrl(sessionId?: string | null): string {
  const base = apiBase().replace(/^http/i, "ws");
  const q = sessionId ? `?session_id=${encodeURIComponent(sessionId)}` : "";
  return `${base}/ws${q}`;
}

export function sseUrl(opts: { session?: string; replay?: boolean } = {}): string {
  const params = new URLSearchParams();
  if (opts.session) params.set("session", opts.session);
  if (opts.replay === false) params.set("replay", "0");
  const q = params.toString();
  return `${apiBase()}/api/debug/events${q ? `?${q}` : ""}`;
}

export function errorMessage(e: unknown): string {
  if (e instanceof Error) return e.message;
  if (typeof e === "string") return e;
  try {
    return JSON.stringify(e);
  } catch {
    return "Unknown error";
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const url = `${apiBase()}${path}`;
  let res: Response;
  try {
    res = await fetch(url, {
      ...init,
      headers: {
        Accept: "application/json",
        ...(init.body ? { "Content-Type": "application/json" } : {}),
        ...(init.headers ?? {}),
      },
    });
  } catch (e) {
    throw new ApiError(0, `Cannot reach the backend at ${apiBase()} (${errorMessage(e)})`);
  }
  const text = await res.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }
  if (!res.ok) {
    let msg = `${res.status} ${res.statusText || "error"}`;
    if (data && typeof data === "object" && "error" in data) {
      const err = (data as { error: unknown }).error;
      if (typeof err === "string" && err) msg = err;
    }
    throw new ApiError(res.status, msg);
  }
  return data as T;
}

export const api = {
  health: () => request<Health>("/health"),
  config: () => request<AppConfig>("/api/config"),
  prompt: async (): Promise<string> => {
    let res: Response;
    try {
      res = await fetch(`${apiBase()}/api/prompt`, { headers: { Accept: "text/plain" } });
    } catch (e) {
      throw new ApiError(0, `Cannot reach the backend (${errorMessage(e)})`);
    }
    if (!res.ok) throw new ApiError(res.status, `${res.status} ${res.statusText}`);
    return res.text();
  },
  createSession: (channel = "web") =>
    request<{ session_id: string; session: SessionState }>("/api/sessions", {
      method: "POST",
      body: JSON.stringify({ channel }),
    }),
  sessions: (limit = 50) => request<{ sessions: SessionSummary[] | null }>(`/api/sessions?limit=${limit}`),
  session: (id: string) => request<SessionDetail>(`/api/sessions/${encodeURIComponent(id)}`),
  turn: (id: string, body: { text: string; voice?: boolean; source?: string }) =>
    request<TurnResponse>(`/api/sessions/${encodeURIComponent(id)}/turn`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  route: (text: string) => request<RouteResponse>("/api/route", { method: "POST", body: JSON.stringify({ text }) }),
  evalRun: (body: { concurrency?: number; limit?: number } = {}) =>
    request<EvalReport>("/api/eval/run", { method: "POST", body: JSON.stringify(body) }),
  evalLast: () => request<EvalLast>("/api/eval/last"),
  stats: () => request<Stats>("/api/supervisor/stats"),
  catalog: () => request<CatalogResponse>("/api/catalog"),
  catalogReload: () => request<{ ok: boolean; scenarios: number }>("/api/catalog/reload", { method: "POST", body: "{}" }),
};
