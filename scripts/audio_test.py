#!/usr/bin/env python3
"""End-to-end audio test: every recording in tests/audio/manifest.json is
sent to the running backend (POST /api/sessions/{id}/turn/audio), transcribed,
routed and answered; the expected scenario is checked and the per-stage
timings are printed.

    python scripts/audio_test.py                  # http://localhost:8080
    python scripts/audio_test.py --save-replies   # also save the spoken reply as tests/audio/out/<name>.reply.wav
    python scripts/audio_test.py --api http://host:8080 --only mixed

Keyless mode: with STT_PROVIDER=mock/browser the server cannot transcribe
audio, so the script passes the recording's known transcript
(tests/audio/<name>.txt) as `transcript_hint`; the routing and reply still
run for real. With a real STT provider the hint is ignored.
"""
import argparse
import base64
import io
import json
import mimetypes
import os
import sys
import time
import urllib.request
import uuid

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
AUDIO = os.path.join(ROOT, "tests", "audio")


def multipart(fields, file_field, filename, data):
    boundary = "----vr" + uuid.uuid4().hex
    body = io.BytesIO()
    for k, v in fields.items():
        body.write(f"--{boundary}\r\nContent-Disposition: form-data; name=\"{k}\"\r\n\r\n{v}\r\n".encode())
    body.write(f"--{boundary}\r\nContent-Disposition: form-data; name=\"{file_field}\"; filename=\"{filename}\"\r\nContent-Type: audio/wav\r\n\r\n".encode())
    body.write(data)
    body.write(f"\r\n--{boundary}--\r\n".encode())
    return body.getvalue(), f"multipart/form-data; boundary={boundary}"


def post_json(url, payload):
    req = urllib.request.Request(url, data=json.dumps(payload).encode(), headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=120) as resp:
        return json.load(resp)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--api", default=os.environ.get("API_URL", "http://localhost:8080"))
    ap.add_argument("--only", default="", help="substring filter on sample name")
    ap.add_argument("--save-replies", action="store_true")
    ap.add_argument("--no-voice", action="store_true", help="skip TTS of the reply")
    args = ap.parse_args()

    cfg = json.load(urllib.request.urlopen(args.api + "/api/config", timeout=10))
    print(f"backend: llm={cfg['llm']['provider']}/{cfg['llm']['model']} stt={cfg['stt']['provider']} tts={cfg['tts']['provider']} fast_path={cfg['fast_path']}")
    manifest = json.load(open(os.path.join(AUDIO, "manifest.json"), encoding="utf-8"))
    failures = 0
    for item in manifest["samples"]:
        name = os.path.splitext(item["file"])[0]
        if args.only and args.only not in name:
            continue
        wav_path = os.path.join(AUDIO, item["file"])
        if not os.path.exists(wav_path):
            print(f"  ! {name}: missing {wav_path} (run scripts/make_audio.py)")
            failures += 1
            continue
        hint = ""
        txt = os.path.join(AUDIO, name + ".txt")
        if os.path.exists(txt):
            hint = open(txt, encoding="utf-8").read().strip()
        sess = post_json(args.api + "/api/sessions", {"channel": "audio_test"})["session_id"]
        body, ctype = multipart({"transcript_hint": hint, "voice": "0" if args.no_voice else "1", "collect_audio": "1" if args.save_replies else "0"}, "file", item["file"], open(wav_path, "rb").read())
        req = urllib.request.Request(f"{args.api}/api/sessions/{sess}/turn/audio", data=body, headers={"Content-Type": ctype})
        t0 = time.time()
        with urllib.request.urlopen(req, timeout=180) as resp:
            r = json.load(resp)
        wall = int((time.time() - t0) * 1000)
        if "error" in r and not r.get("scenarios"):
            print(f"  ✗ {name}: {r['error']}")
            failures += 1
            continue
        tr = r["trace"]
        got = r["scenarios"]
        ok = bool(got) and got[0] == item["expected"][0]
        full = set(got) == set(item["expected"])
        mark = "✓" if ok else "✗"
        if not ok:
            failures += 1
        tm = tr["timings"]
        print(f"\n{mark} {name}  expected={item['expected']} got={got} (full match: {full}) path={tr['path']} lang={tr['language']['detected']}→{tr['reply']['lang']}")
        print(f"    transcript ({tr['input'].get('stt_provider') or 'hint'}): {tr['input']['transcript']}")
        print(f"    reason: {tr['decision']['scenarios'][0].get('reason', '')}")
        print(f"    reply: {r['reply']}")
        print(f"    timings ms: stt={tm.get('stt', 0)} route={tm.get('route', 0)} first_audio={tm.get('first_audio', 0)} total={tm.get('total', 0)} (http wall {wall})")
        if tr.get("errors"):
            print(f"    errors: {tr['errors']}")
        if args.save_replies and r.get("audio_wav_base64"):
            out_dir = os.path.join(AUDIO, "out")
            os.makedirs(out_dir, exist_ok=True)
            out = os.path.join(out_dir, name + ".reply.wav")
            open(out, "wb").write(base64.b64decode(r["audio_wav_base64"]))
            print(f"    reply audio saved: {out}")
        # multi-turn follow-ups defined in the manifest run as text turns in the same session
        for follow in item.get("follow_ups", []):
            fr = post_json(f"{args.api}/api/sessions/{sess}/turn", {"text": follow["text"], "voice": not args.no_voice})
            fgot = fr["scenarios"]
            fok = bool(fgot) and fgot[0] == follow["expected"][0]
            if not fok:
                failures += 1
            print(f"    {'✓' if fok else '✗'} follow-up «{follow['text']}» → {fgot} (expected {follow['expected']}) | {fr['reply']}")
    print("\nRESULT:", "all passed" if failures == 0 else f"{failures} failure(s)")
    sys.exit(1 if failures else 0)


if __name__ == "__main__":
    main()
