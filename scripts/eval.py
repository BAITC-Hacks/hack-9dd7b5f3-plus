#!/usr/bin/env python3
"""Run the official dev set through the running router and score it with the
organizer's evaluate.py.

    python scripts/eval.py                      # against http://localhost:8080
    python scripts/eval.py --api http://host:8080 --concurrency 8
    python scripts/eval.py --out predictions.json

Prints the evaluate.py table (primary accuracy, full match, intent recall
by language and type), plus routing latency percentiles and the share of
utterances answered by the fast path. Only the standard library is used.
"""
import argparse
import json
import os
import subprocess
import sys
import time
import urllib.request
from concurrent.futures import ThreadPoolExecutor

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DATA = os.path.join(ROOT, "data")


def route(api, text):
    body = json.dumps({"text": text}).encode("utf-8")
    req = urllib.request.Request(api + "/api/route", data=body, headers={"Content-Type": "application/json"})
    t0 = time.time()
    with urllib.request.urlopen(req, timeout=60) as resp:
        data = json.load(resp)
    data["_wall_ms"] = int((time.time() - t0) * 1000)
    return data


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--api", default=os.environ.get("API_URL", "http://localhost:8080"))
    ap.add_argument("--concurrency", type=int, default=6)
    ap.add_argument("--out", default=os.path.join(ROOT, "var", "predictions.json"))
    ap.add_argument("--dev", default=os.path.join(DATA, "dev_utterances.json"))
    ap.add_argument("--limit", type=int, default=0)
    args = ap.parse_args()

    utts = json.load(open(args.dev, encoding="utf-8"))["utterances"]
    if args.limit:
        utts = utts[: args.limit]
    cfg = json.load(urllib.request.urlopen(args.api + "/api/config", timeout=10))
    print(f"router: llm={cfg['llm']['provider']}/{cfg['llm']['model']}  fast_path={cfg['fast_path']}  n={len(utts)}")

    results = {}
    t0 = time.time()
    with ThreadPoolExecutor(max_workers=args.concurrency) as ex:
        for u, r in zip(utts, ex.map(lambda u: route(args.api, u["text"]), utts)):
            results[u["id"]] = r
            ok = "✓" if r["scenarios"] and r["scenarios"][0] == u["expected"][0] else "✗"
            print(f"  {ok} {u['id']} {r['path']:<5} {r['route_ms']:>5}ms  got={r['scenarios']} exp={u['expected']}  | {u['text'][:70]}")
    wall = time.time() - t0

    preds = {uid: r["scenarios"] for uid, r in results.items()}
    os.makedirs(os.path.dirname(args.out), exist_ok=True)
    json.dump(preds, open(args.out, "w", encoding="utf-8"), ensure_ascii=False, indent=1)

    print("\n=== data/evaluate.py ===")
    sys.stdout.flush()
    subprocess.run([sys.executable, os.path.join(DATA, "evaluate.py"), args.out, args.dev], check=False)

    lat = sorted(r["route_ms"] for r in results.values())
    walls = sorted(r["_wall_ms"] for r in results.values())
    paths = {}
    for r in results.values():
        paths[r["path"]] = paths.get(r["path"], 0) + 1
    p = lambda xs, q: xs[min(len(xs) - 1, int(len(xs) * q))]
    print(f"\nroute latency (server-side, ms): p50={p(lat, .5)} p90={p(lat, .9)} max={lat[-1]}")
    print(f"end-to-end HTTP (ms):            p50={p(walls, .5)} p90={p(walls, .9)} max={walls[-1]}")
    print(f"paths: {paths}   total wall {wall:.1f}s")
    print(f"predictions written to {args.out}")


if __name__ == "__main__":
    main()
