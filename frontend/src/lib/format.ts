export function fmtMs(v: number | null | undefined): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "—";
  return `${Math.round(v)} ms`;
}

export function fmtDuration(ms: number | null | undefined): string {
  if (ms === null || ms === undefined || Number.isNaN(ms)) return "—";
  if (ms < 1000) return `${Math.round(ms)} ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(ms < 10_000 ? 2 : 1)} s`;
  const m = Math.floor(ms / 60_000);
  const s = Math.round((ms % 60_000) / 1000);
  return `${m}m ${String(s).padStart(2, "0")}s`;
}

export function fmtPct(v: number | null | undefined, digits = 0): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "—";
  return `${(v * 100).toFixed(digits)}%`;
}

export function fmtNum(v: number | null | undefined): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "—";
  return v.toLocaleString("en-US");
}

export function fmtTime(input: string | number | null | undefined, withMs = false): string {
  if (input === null || input === undefined || input === "") return "—";
  const d = new Date(input);
  if (Number.isNaN(d.getTime())) return "—";
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  const ss = String(d.getSeconds()).padStart(2, "0");
  if (!withMs) return `${hh}:${mm}:${ss}`;
  return `${hh}:${mm}:${ss}.${String(d.getMilliseconds()).padStart(3, "0")}`;
}

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

export function fmtDateTime(input: string | number | null | undefined): string {
  if (input === null || input === undefined || input === "") return "—";
  const d = new Date(input);
  if (Number.isNaN(d.getTime())) return "—";
  return `${d.getDate()} ${MONTHS[d.getMonth()]} ${fmtTime(input)}`;
}

export function shortId(id: string | null | undefined, n = 6): string {
  if (!id) return "—";
  return id.length > n ? id.slice(0, n) : id;
}

export function ratio(a: number | undefined, b: number | undefined): number | undefined {
  if (!a || !b || b <= 0) return a === 0 && b && b > 0 ? 0 : undefined;
  return a / b;
}

export function truncate(s: string, n: number): string {
  if (!s) return "";
  return s.length > n ? `${s.slice(0, n - 1)}…` : s;
}

export function langLabel(lang: string | null | undefined): string {
  const l = (lang ?? "").toLowerCase();
  if (l === "ru") return "RU";
  if (l === "kk") return "KK";
  if (l === "mixed") return "MIXED";
  return l ? l.toUpperCase() : "—";
}
