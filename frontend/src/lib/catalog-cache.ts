"use client";

import { useMemo } from "react";
import { api } from "./api";
import type { CatalogResponse } from "./types";
import { useApi } from "./use-api";

let cached: Promise<CatalogResponse> | null = null;

/** Catalog is static per backend process; share one request across pages. */
export function loadCatalog(): Promise<CatalogResponse> {
  if (!cached) {
    cached = api.catalog().catch((e: unknown) => {
      cached = null;
      throw e;
    });
  }
  return cached;
}

export function invalidateCatalog() {
  cached = null;
}

export type ScenarioNames = Record<string, string>;

/** Map of scenario/system-intent id → human name (empty until loaded). */
export function useScenarioNames(): ScenarioNames {
  const { data } = useApi(loadCatalog);
  return useMemo(() => {
    const names: ScenarioNames = {};
    for (const s of data?.scenarios ?? []) names[s.scenario_id] = s.name;
    for (const s of data?.system_intents ?? []) {
      names[s.id] = s.id.replace(/^SYS_/, "").replaceAll("_", " ").toLowerCase();
    }
    return names;
  }, [data]);
}
