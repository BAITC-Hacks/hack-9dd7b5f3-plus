"use client";

import { useMemo, useState } from "react";
import { FileText, RefreshCw, Search } from "lucide-react";
import { api, errorMessage } from "@/lib/api";
import { cn } from "@/lib/cn";
import { invalidateCatalog, loadCatalog } from "@/lib/catalog-cache";
import type { Scenario } from "@/lib/types";
import { useApi } from "@/lib/use-api";
import { ScenarioCard } from "@/components/catalog/scenario-card";
import { PageBody, PageHeader } from "@/components/shell/page-header";
import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/modal";
import { Label, Panel } from "@/components/ui/panel";
import { Pill } from "@/components/ui/pill";
import { EmptyState, ErrorState, LoadingState } from "@/components/ui/states";

function matches(s: Scenario, needle: string): boolean {
  if (!needle) return true;
  const hay = [
    s.scenario_id,
    s.slug,
    s.name,
    s.domain,
    s.category,
    s.description,
    ...(s.actions ?? []),
    ...(s.slots?.required ?? []),
    ...(s.slots?.optional ?? []),
    ...(s.examples?.ru ?? []),
    ...(s.examples?.kk ?? []),
    s.handoff?.queue ?? "",
  ]
    .join(" ")
    .toLowerCase();
  return hay.includes(needle);
}

export default function CatalogPage() {
  const { data, error, loading, refresh, refreshing } = useApi(loadCatalog);
  const [q, setQ] = useState("");
  const [domain, setDomain] = useState("all");
  const [category, setCategory] = useState("all");
  const [reload, setReload] = useState<{ busy: boolean; msg: string | null; error: boolean }>({ busy: false, msg: null, error: false });
  const [prompt, setPrompt] = useState<{ open: boolean; text: string | null; error: string | null }>({ open: false, text: null, error: null });

  const scenarios = useMemo(() => data?.scenarios ?? [], [data]);
  const names = useMemo(() => {
    const m: Record<string, string> = {};
    for (const s of scenarios) m[s.scenario_id] = s.name;
    return m;
  }, [scenarios]);
  const domains = useMemo(() => Array.from(new Set(scenarios.map((s) => s.domain))).sort(), [scenarios]);
  const categories = useMemo(() => Array.from(new Set(scenarios.map((s) => s.category))).sort(), [scenarios]);
  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return scenarios.filter((s) => (domain === "all" || s.domain === domain) && (category === "all" || s.category === category) && matches(s, needle));
  }, [scenarios, q, domain, category]);

  const doReload = async () => {
    setReload({ busy: true, msg: null, error: false });
    try {
      const r = await api.catalogReload();
      invalidateCatalog();
      refresh();
      setReload({ busy: false, msg: `Reloaded ${r.scenarios} scenarios from disk`, error: false });
    } catch (e) {
      setReload({ busy: false, msg: errorMessage(e), error: true });
    }
  };

  const openPrompt = async () => {
    setPrompt({ open: true, text: null, error: null });
    try {
      const text = await api.prompt();
      setPrompt({ open: true, text, error: null });
    } catch (e) {
      setPrompt({ open: true, text: null, error: errorMessage(e) });
    }
  };

  return (
    <PageBody className="space-y-4">
      <PageHeader
        title="Catalog"
        meta={
          data ? (
            <span className="flex items-center gap-1.5">
              <Pill>{scenarios.length} scenarios</Pill>
              <Pill>{(data.system_intents ?? []).length} system intents</Pill>
              {data.as_of_date && <span className="text-[11px] text-muted-foreground/55">as of {data.as_of_date}</span>}
            </span>
          ) : undefined
        }
        actions={
          <>
            <Button size="sm" onClick={() => void openPrompt()}>
              <FileText className="size-3.5" /> System prompt
            </Button>
            <Button size="sm" onClick={() => void doReload()} disabled={reload.busy || refreshing}>
              <RefreshCw className={cn("size-3.5", (reload.busy || refreshing) && "animate-spin")} /> Reload catalog from disk
            </Button>
          </>
        }
      />

      {reload.msg && (
        <p className={cn("text-[12px]", reload.error ? "text-destructive" : "text-success")} role="status">
          {reload.msg}
        </p>
      )}

      {error && !data ? (
        <ErrorState message={error} onRetry={refresh} />
      ) : loading && !data ? (
        <div className="rounded-[15px] border border-border bg-card">
          <LoadingState rows={6} />
        </div>
      ) : data ? (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <label className="relative">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground/50" />
              <input
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder="Search id, name, slot, action, example…"
                aria-label="Search scenarios"
                className="h-8 w-72 max-w-full rounded-[10px] border border-input bg-background/60 pl-8 pr-3 text-[12.5px] outline-none placeholder:text-muted-foreground/50 focus-visible:border-primary/60 focus-visible:ring-2 focus-visible:ring-primary/25"
              />
            </label>
            <div className="flex flex-wrap gap-1">
              {["all", ...domains].map((d) => (
                <button
                  key={d}
                  type="button"
                  onClick={() => setDomain(d)}
                  className={cn(
                    "rounded-full border px-2.5 py-0.5 text-[11.5px] font-medium transition-colors active:scale-[0.97]",
                    domain === d ? "border-primary/40 bg-primary/[0.12] text-primary" : "border-border text-muted-foreground hover:bg-foreground/[0.04]",
                  )}
                >
                  {d}
                </button>
              ))}
            </div>
            <select value={category} onChange={(e) => setCategory(e.target.value)} aria-label="Category" className="h-7 rounded-[8px] border border-border bg-muted px-1.5 text-[11.5px]">
              <option value="all">all categories</option>
              {categories.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
            <span className="text-[11px] tabular-nums text-muted-foreground/55">{filtered.length} shown</span>
          </div>

          {filtered.length === 0 ? (
            <div className="rounded-[15px] border border-border bg-card">
              <EmptyState title="No scenarios match" hint="Try another word — examples in Russian and Kazakh are searched too." />
            </div>
          ) : (
            <div className="grid gap-4 lg:grid-cols-2">
              {filtered.map((s) => (
                <ScenarioCard key={s.scenario_id} s={s} names={names} />
              ))}
            </div>
          )}

          <Panel title="System intents" note="not scenarios — handled by policy">
            <div className="grid gap-3 md:grid-cols-3">
              {(data.system_intents ?? []).map((si) => (
                <div key={si.id} id={si.id} className="target-highlight scroll-mt-4 rounded-[12px] border border-border bg-foreground/[0.02] p-3">
                  <p className="font-mono text-[12px] tracking-normal text-foreground/90">{si.id}</p>
                  <p className="mt-1 text-[12px] leading-relaxed text-foreground/80">{si.description}</p>
                  <Label className="mt-2">behavior</Label>
                  <p className="text-[12px] leading-relaxed text-muted-foreground">{si.behavior}</p>
                  {si.response && (
                    <div className="mt-2 space-y-1 text-[11.5px] leading-snug text-foreground/80">
                      {Object.entries(si.response).map(([l, r]) => (
                        <p key={l}>
                          <span className="text-muted-foreground/60">{l.toUpperCase()} · </span>
                          {r}
                        </p>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </Panel>
        </>
      ) : null}

      <Modal open={prompt.open} onClose={() => setPrompt((p) => ({ ...p, open: false }))} title="LLM system prompt" wide>
        {prompt.error ? (
          <ErrorState message={prompt.error} onRetry={() => void openPrompt()} />
        ) : prompt.text === null ? (
          <LoadingState rows={8} />
        ) : (
          <pre className="whitespace-pre-wrap break-words font-mono text-[11.5px] leading-[1.6] tracking-normal text-foreground/85">{prompt.text}</pre>
        )}
      </Modal>
    </PageBody>
  );
}
