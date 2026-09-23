#!/usr/bin/env python3
"""Voice Router core: transcribed text -> LLM (OpenRouter) -> scenario percentages.

    python router.py "Хочу продлить ОГПО и заодно добавить сына"
    python router.py --eval                       # data/dev_utterances.json -> metrics + predictions
    python router.py --dialogs                    # data/dialogs_sample.json, turn by turn with context
    python router.py --bench m1,m2,...            # compare models on the dev set
    python router.py --model openai/gpt-4.1-mini --eval

The model answers with one line of "ID:PERCENT" pairs and nothing else, e.g. "SC27:95 SC04:90".
Config comes from core-llm/.env (OPENROUTER_API_KEY, ROUTER_MODEL, ...). Standard library only.
"""
from __future__ import annotations

import argparse
import concurrent.futures
import http.client
import json
import os
import re
import statistics
import sys
import threading
import time
import urllib.parse
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
DEFAULT_MODEL = "google/gemini-2.5-flash-lite"
DEFAULT_BASE_URL = "https://openrouter.ai/api/v1"
URGENT = ("SC11", "SC15", "SC38")

PAIR_RE = re.compile(r"(SC\s?\d{1,2}|SYS_OUT_OF_SCOPE|SYS_UNCLEAR|SYS_GOODBYE|SYS_GREETING|SYS_HELP)\s*[:=]\s*(\d{1,3})", re.I)
INDEX_LINE_RE = re.compile(r"^(SC\d{2}|SYS_[A-Z_]+)\s*\|\s*([^|]+?)\s*\|")


def load_env(path: str = os.path.join(HERE, ".env")) -> None:
    """Minimal .env loader; existing environment variables win."""
    if not os.path.isfile(path):
        return
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            k, v = line.split("=", 1)
            k, v = k.strip(), v.strip().strip('"').strip("'")
            if k and k not in os.environ:
                os.environ[k] = v


def load_system_prompt() -> tuple[str, dict[str, str]]:
    """System prompt = prompt.md + scenarios.index.txt (stable bytes -> provider prompt caching)."""
    with open(os.path.join(HERE, "prompt.md"), encoding="utf-8") as f:
        prompt = f.read().rstrip()
    with open(os.path.join(HERE, "scenarios.index.txt"), encoding="utf-8") as f:
        index = f.read().strip()
    names: dict[str, str] = {}
    for line in index.splitlines():
        m = INDEX_LINE_RE.match(line)
        if m:
            names[m.group(1)] = m.group(2).strip()
    return prompt + "\n\n" + index + "\n", names


# ----------------------------------------------------------------------------- language (local, 0 ms)
KK_LETTERS = set("әіңғүұқөһӘІҢҒҮҰҚӨҺ")
RU_WORDS = {
    "и", "ещё", "еще", "но", "я", "на", "для", "можно", "где", "что", "как", "мне", "у", "вы", "ваш",
    "ваша", "ваши", "вас", "есть", "хочу", "нужно", "надо", "скажите", "подскажите", "пожалуйста", "ли", "по",
    "с", "в", "или", "когда", "почему", "сколько", "какой", "какая", "какие", "это", "меня", "мой", "моя",
    "мою", "деньги", "страховка", "страховку", "страховки", "машина", "машину", "машиной", "здравствуйте",
    "добрый", "день", "справка", "справку", "виноват", "виновник", "английском", "посольства", "осмотр",
    "делают", "офис", "приходит", "действует", "уже", "только", "сейчас", "вчера", "неделю", "назад", "потом",
    "давайте", "нет", "спасибо", "тоже", "ещё", "заодно", "также", "куда", "чтобы", "если", "там", "здесь",
}
RU_SUFFIXES = ("ить", "ать", "ять", "еть", "ует", "ится", "ется", "ает", "яет", "ите", "айте", "ьте", "ого", "ему")


def detect_language(text: str) -> str:
    """ru | kk | mixed by Kazakh-only letters vs Russian function words. Loanwords (полис, код) do not count."""
    kk = ru = 0
    for tok in re.findall(r"[А-Яа-яЁёӘәІіҢңҒғҮүҰұҚқӨөҺһ]+", text):
        if any(ch in KK_LETTERS for ch in tok):
            kk += 1
            continue
        low = tok.lower()
        if low in RU_WORDS or (len(low) > 4 and low.endswith(RU_SUFFIXES)):
            ru += 1
    if kk and ru:
        return "mixed"
    return "kk" if kk else "ru"


# ----------------------------------------------------------------------------- transport
class RouterError(RuntimeError):
    pass


class Transport:
    """Keep-alive HTTPS connection per thread: saves a TCP+TLS handshake (~100-300 ms) on every call."""

    def __init__(self, base_url: str, api_key: str, timeout: float):
        u = urllib.parse.urlsplit(base_url)
        self.host = u.netloc
        self.path = u.path.rstrip("/") + "/chat/completions"
        self.timeout = timeout
        self.headers = {
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
            "Accept": "application/json",
            "X-Title": "Voice Router core-llm",
        }
        self._local = threading.local()

    def _conn(self, fresh: bool = False) -> http.client.HTTPSConnection:
        conn = getattr(self._local, "conn", None)
        if conn is None or fresh:
            if conn is not None:
                conn.close()
            conn = http.client.HTTPSConnection(self.host, timeout=self.timeout)
            self._local.conn = conn
        return conn

    def post(self, body: dict) -> tuple[int, bytes]:
        data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        last: Exception | None = None
        for attempt in (0, 1):  # a dropped keep-alive socket is retried once on a fresh connection
            conn = self._conn(fresh=attempt == 1)
            try:
                conn.request("POST", self.path, body=data, headers=self.headers)
                resp = conn.getresponse()
                return resp.status, resp.read()
            except (http.client.HTTPException, OSError) as e:
                last = e
                conn.close()
                self._local.conn = None
        raise RouterError(f"transport: {last}")


# ----------------------------------------------------------------------------- router
class Router:
    def __init__(self, model: str | None = None, api_key: str | None = None, base_url: str | None = None,
                 timeout: float | None = None, provider_sort: str | None = None, threshold: int | None = None,
                 reasoning: str | None = None, max_tokens: int = 64):
        load_env()
        self.model = model or os.environ.get("ROUTER_MODEL") or DEFAULT_MODEL
        self.api_key = api_key or os.environ.get("OPENROUTER_API_KEY", "")
        if not self.api_key:
            raise RouterError("OPENROUTER_API_KEY is not set (core-llm/.env)")
        self.base_url = base_url or os.environ.get("ROUTER_BASE_URL") or DEFAULT_BASE_URL
        self.timeout = timeout or float(os.environ.get("ROUTER_TIMEOUT", "8"))
        self.provider_sort = provider_sort if provider_sort is not None else os.environ.get("ROUTER_PROVIDER_SORT", "latency")
        self.threshold = threshold if threshold is not None else int(os.environ.get("ROUTER_THRESHOLD", "60"))
        self.second_threshold = int(os.environ.get("ROUTER_SECOND_THRESHOLD", "20"))
        self.reasoning = reasoning if reasoning is not None else os.environ.get("ROUTER_REASONING", "off")
        self.max_tokens = max_tokens
        self.system, self.names = load_system_prompt()
        self.valid = set(self.names)
        self.transport = Transport(self.base_url, self.api_key, self.timeout)
        self._no_temperature = False
        self._no_reasoning = False

    # -- request body -----------------------------------------------------------------------
    def _messages(self, user: str) -> list[dict]:
        if self.model.startswith("anthropic/"):
            system: object = [{"type": "text", "text": self.system, "cache_control": {"type": "ephemeral"}}]
        else:
            system = self.system
        return [{"role": "system", "content": system}, {"role": "user", "content": user}]

    def _body(self, messages: list[dict]) -> dict:
        body: dict = {"model": self.model, "messages": messages, "max_tokens": self.max_tokens,
                      "stream": False, "usage": {"include": True}}
        if not self._no_temperature:
            body["temperature"] = 0
        if not self._no_reasoning and self.reasoning and self.reasoning != "skip":
            body["reasoning"] = {"enabled": False} if self.reasoning == "off" else {"effort": self.reasoning}
        if self.provider_sort:
            body["provider"] = {"sort": self.provider_sort, "allow_fallbacks": True}
        return body

    def _complete(self, messages: list[dict]) -> tuple[str, dict]:
        last = "no attempts"
        for attempt in range(4):
            body = self._body(messages)
            t0 = time.perf_counter()
            status, raw = self.transport.post(body)
            ms = (time.perf_counter() - t0) * 1000
            if status == 200:
                try:
                    data = json.loads(raw)
                except ValueError:
                    raise RouterError("invalid JSON from provider")
                if "error" in data and not data.get("choices"):
                    last = f"provider error: {data['error']}"
                    time.sleep(0.3 * (attempt + 1))
                    continue
                choice = (data.get("choices") or [{}])[0]
                content = (choice.get("message") or {}).get("content") or ""
                usage = data.get("usage") or {}
                meta = {"model": data.get("model", self.model), "provider": data.get("provider"),
                        "latency_ms": round(ms, 1),
                        "prompt_tokens": usage.get("prompt_tokens"), "completion_tokens": usage.get("completion_tokens"),
                        "cached_tokens": (usage.get("prompt_tokens_details") or {}).get("cached_tokens"),
                        "cost_usd": usage.get("cost"), "finish_reason": choice.get("finish_reason"), "attempts": attempt + 1}
                return content, meta
            err = raw.decode("utf-8", "replace")[:400]
            last = f"HTTP {status}: {err}"
            if status == 400:  # provider-specific parameter support: drop the parameter and retry
                low = err.lower()
                if "temperature" in low and not self._no_temperature:
                    self._no_temperature = True
                    continue
                if "reasoning" in low and not self._no_reasoning:
                    self._no_reasoning = True
                    continue
                raise RouterError(last)
            if status in (401, 402, 403, 404):
                raise RouterError(last)
            time.sleep(0.3 * (attempt + 1))  # 408/429/5xx
        raise RouterError(last)

    # -- public API -------------------------------------------------------------------------
    @staticmethod
    def build_user(text: str, history: list[dict] | None = None, active: str | None = None,
                   last_bot: str | None = None) -> str:
        text = " ".join(text.split())
        if not history and not active and not last_bot:
            return f"CLIENT: {text}"
        lines = ["CONTEXT:"]
        for h in (history or [])[-4:]:
            lines.append(f"[{h.get('scenario') or '?'}] {' '.join(str(h.get('text', '')).split())[:160]}")
        if last_bot:
            lines.append(f"BOT: {' '.join(last_bot.split())[:160]}")
        if active:
            lines.append(f"ACTIVE: {active}")
        lines.append(f"CLIENT: {text}")
        return "\n".join(lines)

    def parse(self, raw: str) -> list[dict]:
        out, seen = [], set()
        for m in PAIR_RE.finditer(raw):
            sid = m.group(1).upper().replace(" ", "")
            if sid.startswith("SC"):
                sid = "SC" + sid[2:].zfill(2)
            pct = max(0, min(100, int(m.group(2))))
            if sid not in self.valid or sid in seen or pct <= 0:
                continue
            seen.add(sid)
            out.append({"id": sid, "pct": pct, "name": self.names.get(sid, "")})
        return out

    def decide(self, entries: list[dict]) -> dict:
        """Decision policy over the model's pairs (model order = handling order).

        primary  = first pair >= second_threshold (else the top pair)
        route    = primary >= threshold; further business pairs >= second_threshold are additional intents
                   (the model keeps pure alternatives at 1-10, so anything above that was asked for)
        clarify  = primary < threshold or SYS_UNCLEAR: one intent, the other pairs are the options to ask about
        """
        if not entries:
            return {"primary": None, "status": "clarify", "intents": [], "alternatives": [], "predicted": []}
        requested = [e for e in entries if e["pct"] >= self.second_threshold]
        primary = requested[0] if requested else max(entries, key=lambda e: e["pct"])
        pid = primary["id"]
        if pid == "SYS_UNCLEAR" or primary["pct"] < self.threshold:
            status, intents = "clarify", [primary]
        elif pid == "SYS_OUT_OF_SCOPE":
            status, intents = "out_of_scope", [primary]
        elif pid == "SYS_HELP":
            status, intents = "help", [primary]
        elif pid == "SYS_GREETING":
            status, intents = "greeting", [primary]
        elif pid == "SYS_GOODBYE":
            status, intents = "goodbye", [primary]
        else:
            status = "handoff" if pid == "SC37" else "route"
            intents = [e for e in requested if e["id"].startswith("SC")]
        alternatives = [e for e in entries if e not in intents]
        return {"primary": pid, "status": status, "intents": intents, "alternatives": alternatives,
                "predicted": [e["id"] for e in intents]}

    def route(self, text: str, history: list[dict] | None = None, active: str | None = None,
              last_bot: str | None = None) -> dict:
        t0 = time.perf_counter()
        messages = self._messages(self.build_user(text, history, active, last_bot))
        raw, meta = self._complete(messages)
        entries = self.parse(raw)
        if not entries:  # one nudge if the model wrote prose instead of pairs
            messages += [{"role": "assistant", "content": raw or "-"},
                         {"role": "user", "content": "Answer with ID:PERCENT pairs only."}]
            raw2, meta2 = self._complete(messages)
            entries = self.parse(raw2)
            meta["latency_ms"] = round(meta["latency_ms"] + meta2["latency_ms"], 1)
            meta["attempts"] += meta2["attempts"]
            raw = raw2
        decision = self.decide(entries)
        return {"text": text, "language": detect_language(text), **decision, "scenarios": entries,
                "raw": raw.strip(), "route_ms": round((time.perf_counter() - t0) * 1000, 1), **meta}

    def warmup(self) -> dict:
        """Prime the connection and the provider's prompt cache before the first real turn."""
        return self.route("Здравствуйте")


# ----------------------------------------------------------------------------- evaluation
def percentile(values: list[float], p: float) -> float | None:
    if not values:
        return None
    s = sorted(values)
    return round(s[max(0, int(len(s) * p + 0.999999) - 1)], 1)


def official_metrics(utts: list[dict], preds: dict[str, list[str]]) -> tuple[dict, float | None, list]:
    """Same arithmetic as data/evaluate.py: primary_accuracy, full_match, intent_recall."""
    groups: dict = defaultdict(lambda: {"n": 0, "primary": 0, "full": 0})
    hit = total = 0
    errors = []
    for u in utts:
        exp, got = u["expected"], preds.get(u["id"], [])
        primary = bool(got) and got[0] == exp[0]
        full = set(got) == set(exp)
        if u["type"] == "multi_intent":
            hit += len(set(exp) & set(got))
            total += len(exp)
        for key in ("all", f"lang={u['lang']}", f"type={u['type']}"):
            g = groups[key]
            g["n"] += 1
            g["primary"] += primary
            g["full"] += full
        if not full:
            errors.append((u["id"], u["text"], exp, got))
    return groups, (hit / total if total else None), errors


def run_many(router: Router, items: list[tuple], workers: int) -> list[dict]:
    """items: (key, kwargs for router.route). Returns results in input order; failures become errors."""
    def one(item):
        key, kwargs = item
        try:
            r = router.route(**kwargs)
        except RouterError as e:
            r = {"error": str(e), "predicted": [], "primary": None, "raw": "", "route_ms": None}
        return key, r
    with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as pool:
        return [r for _, r in pool.map(one, items)]


def evaluate(router: Router, dev_path: str, workers: int, quiet: bool = False) -> dict:
    with open(dev_path, encoding="utf-8") as f:
        utts = json.load(f)["utterances"]
    t0 = time.perf_counter()
    results = run_many(router, [(u["id"], {"text": u["text"]}) for u in utts], workers)
    wall = time.perf_counter() - t0
    preds = {u["id"]: r["predicted"] for u, r in zip(utts, results)}
    groups, recall, errors = official_metrics(utts, preds)
    lat = [r["route_ms"] for r in results if r.get("route_ms") is not None]
    llm = [r["latency_ms"] for r in results if r.get("latency_ms") is not None]
    prompt_tokens = [r["prompt_tokens"] for r in results if r.get("prompt_tokens")]
    cached = [r["cached_tokens"] or 0 for r in results if r.get("prompt_tokens")]
    cost = sum(r.get("cost_usd") or 0 for r in results)
    providers = sorted({str(r.get("provider")) for r in results if r.get("provider")})
    failures = sum(1 for r in results if r.get("error"))
    lang_ok = sum(1 for u, r in zip(utts, results) if r.get("language") == u["lang"])
    summary = {
        "model": router.model, "providers": providers, "n": len(utts), "failures": failures,
        "primary_accuracy": round(groups["all"]["primary"] / groups["all"]["n"], 3),
        "full_match": round(groups["all"]["full"] / groups["all"]["n"], 3),
        "intent_recall": round(recall, 3) if recall is not None else None,
        "language_accuracy": round(lang_ok / len(utts), 3),
        "route_p50_ms": percentile(lat, .5), "route_p95_ms": percentile(lat, .95), "route_max_ms": percentile(lat, 1),
        "llm_p50_ms": percentile(llm, .5),
        "under_500ms": round(sum(1 for x in lat if x <= 500) / len(lat), 3) if lat else None,
        "prompt_tokens_median": int(statistics.median(prompt_tokens)) if prompt_tokens else None,
        "cached_share": round(sum(cached) / sum(prompt_tokens), 2) if prompt_tokens and sum(prompt_tokens) else None,
        "cost_usd": round(cost, 4), "wall_s": round(wall, 1), "workers": workers,
    }
    if not quiet:
        print(f"{'group':<22}{'n':>5}{'primary_acc':>14}{'full_match':>12}")
        order = ["all"] + sorted(k for k in groups if k.startswith("lang=")) + sorted(k for k in groups if k.startswith("type="))
        for k in order:
            g = groups[k]
            print(f"{k:<22}{g['n']:>5}{g['primary'] / g['n']:>14.3f}{g['full'] / g['n']:>12.3f}")
        if recall is not None:
            print(f"\nintent_recall (multi-intent): {recall:.3f}")
        if errors:
            print(f"\nErrors ({len(errors)}):")
            by_id = {u["id"]: r for u, r in zip(utts, results)}
            for i, t, e, g in errors:
                print(f"  {i}  expected={e}  got={g}  raw={by_id[i].get('raw') or by_id[i].get('error')!r} | {t}")
        print("\n" + json.dumps(summary, ensure_ascii=False, indent=2))
    return {"summary": summary, "predictions": preds, "results": results}


def evaluate_dialogs(router: Router, path: str, workers: int) -> dict:
    """Replay dialogs_sample.json: every client turn sees the previous client turns (with their expected
    scenarios), the bot's last reply and the active scenario. Teacher-forced context, per-turn scoring."""
    with open(path, encoding="utf-8") as f:
        dialogs = json.load(f)["dialogs"]
    items, keys = [], []
    for d in dialogs:
        history, last_bot, active = [], None, None
        for i, t in enumerate(d["turns"]):
            if t["role"] == "bot":
                last_bot = t["text"]
                continue
            keys.append((d["dialog_id"], i, t["text"], t["scenarios"]))
            items.append(((d["dialog_id"], i), {"text": t["text"], "history": list(history), "active": active, "last_bot": last_bot}))
            history.append({"scenario": t["scenarios"][0], "text": t["text"]})
            if t["scenarios"][0].startswith("SC"):
                active = t["scenarios"][0]
    results = run_many(router, items, workers)
    n = primary = full = 0
    for (did, i, text, exp), r in zip(keys, results):
        got = r["predicted"]
        p, fm = bool(got) and got[0] == exp[0], set(got) == set(exp)
        n += 1
        primary += p
        full += fm
        flag = "  " if fm else ("~ " if p else "X ")
        print(f"{flag}{did}#{i:<2} expected={exp} got={got} raw={r.get('raw') or r.get('error')!r} | {text[:90]}")
    summary = {"model": router.model, "turns": n, "primary_accuracy": round(primary / n, 3), "full_match": round(full / n, 3),
               "route_p50_ms": percentile([r["route_ms"] for r in results if r.get("route_ms")], .5)}
    print(json.dumps(summary, ensure_ascii=False))
    return summary


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("text", nargs="?", help="client utterance (transcribed)")
    ap.add_argument("--model", help="OpenRouter model id (default: ROUTER_MODEL or %s)" % DEFAULT_MODEL)
    ap.add_argument("--eval", nargs="?", const=os.path.join(ROOT, "data", "dev_utterances.json"), metavar="DEV_JSON")
    ap.add_argument("--dialogs", nargs="?", const=os.path.join(ROOT, "data", "dialogs_sample.json"), metavar="DIALOGS_JSON")
    ap.add_argument("--bench", metavar="MODELS", help="comma-separated model ids to compare on the dev set")
    ap.add_argument("--workers", type=int, default=6)
    ap.add_argument("--reports", default=os.path.join(HERE, "reports"))
    ap.add_argument("--provider-sort", dest="provider_sort", help="latency|throughput|price|'' (default from env)")
    ap.add_argument("--reasoning", help="off|skip|minimal|low (default: off)")
    ap.add_argument("--prev", action="append", default=[], metavar="SCXX|text", help="previous client turn (repeatable)")
    ap.add_argument("--bot", help="bot's last reply (context)")
    ap.add_argument("--active", help="active scenario id (context)")
    ap.add_argument("--raw", action="store_true", help="print only the model's raw line")
    args = ap.parse_args(argv)

    def make(model: str | None) -> Router:
        return Router(model=model, provider_sort=args.provider_sort, reasoning=args.reasoning)

    if args.bench:
        rows = []
        for model in [m.strip() for m in args.bench.split(",") if m.strip()]:
            try:
                r = make(model)
                r.warmup()
                s = evaluate(r, args.eval or os.path.join(ROOT, "data", "dev_utterances.json"), args.workers, quiet=True)["summary"]
            except RouterError as e:
                s = {"model": model, "error": str(e)}
            rows.append(s)
            print(json.dumps(s, ensure_ascii=False), flush=True)
        os.makedirs(args.reports, exist_ok=True)
        with open(os.path.join(args.reports, "bench.json"), "w", encoding="utf-8") as f:
            json.dump(rows, f, ensure_ascii=False, indent=2)
        print(f"\n{'model':<42}{'primary':>8}{'full':>7}{'recall':>8}{'lang':>6}{'p50ms':>7}{'p95ms':>7}{'<500':>6}{'tok':>6}{'cost$':>7}")
        for s in sorted(rows, key=lambda s: (-(s.get("primary_accuracy") or 0), s.get("route_p50_ms") or 1e9)):
            if "error" in s:
                print(f"{s['model']:<42} ERROR {s['error'][:60]}")
                continue
            print(f"{s['model']:<42}{s['primary_accuracy']:>8.3f}{s['full_match']:>7.3f}{(s['intent_recall'] or 0):>8.3f}"
                  f"{s['language_accuracy']:>6.2f}{s['route_p50_ms']:>7.0f}{s['route_p95_ms']:>7.0f}{s['under_500ms']:>6.2f}"
                  f"{s['prompt_tokens_median']:>6}{s['cost_usd']:>7.3f}")
        return 0

    router = make(args.model)
    if args.eval:
        router.warmup()
        out = evaluate(router, args.eval, args.workers)
        os.makedirs(args.reports, exist_ok=True)
        tag = router.model.replace("/", "_").replace(":", "_")
        pred_path = os.path.join(args.reports, f"predictions-{tag}.json")
        with open(pred_path, "w", encoding="utf-8") as f:
            json.dump(out["predictions"], f, ensure_ascii=False, indent=2)
        with open(os.path.join(args.reports, f"report-{tag}.json"), "w", encoding="utf-8") as f:
            json.dump(out, f, ensure_ascii=False, indent=2)
        print(f"\npredictions: {pred_path}\nofficial check: python3 data/evaluate.py {os.path.relpath(pred_path, ROOT)} data/dev_utterances.json")
        return 0
    if args.dialogs:
        router.warmup()
        evaluate_dialogs(router, args.dialogs, args.workers)
        return 0
    if not args.text:
        ap.print_help()
        return 2
    history = []
    for p in args.prev:
        sid, _, txt = p.partition("|")
        history.append({"scenario": sid.strip(), "text": txt.strip()})
    result = router.route(args.text, history=history or None, active=args.active, last_bot=args.bot)
    if args.raw:
        print(result["raw"])
    else:
        print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except RouterError as e:
        print(f"router error: {e}", file=sys.stderr)
        sys.exit(1)
