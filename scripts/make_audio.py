#!/usr/bin/env python3
"""Generate the test recordings in tests/audio/ from tests/audio/manifest.json.

Default: macOS `say` (voice Milena, Russian) — no keys needed. The Kazakh and
mixed samples are read by a Russian voice, which is exactly the kind of
accented input a robot must cope with; STT quality is not what the case
scores. With TTS keys configured you can produce natural samples instead:

    python scripts/make_audio.py                 # macOS say
    python scripts/make_audio.py --via-api       # uses the running backend's TTS provider (POST /api/tts)
"""
import argparse
import base64
import json
import os
import shutil
import subprocess
import sys
import urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
AUDIO = os.path.join(ROOT, "tests", "audio")


def to_wav16k(src, dst):
    subprocess.run(["ffmpeg", "-y", "-loglevel", "error", "-i", src, "-ar", "16000", "-ac", "1", "-sample_fmt", "s16", dst], check=True)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--via-api", action="store_true", help="synthesize with the backend TTS provider instead of macOS say")
    ap.add_argument("--api", default=os.environ.get("API_URL", "http://localhost:8080"))
    ap.add_argument("--voice", default="Milena")
    args = ap.parse_args()

    manifest = json.load(open(os.path.join(AUDIO, "manifest.json"), encoding="utf-8"))
    if not shutil.which("ffmpeg"):
        sys.exit("ffmpeg is required (brew install ffmpeg)")
    for item in manifest["samples"]:
        wav = os.path.join(AUDIO, item["file"])
        text = item["text"]
        if args.via_api:
            req = urllib.request.Request(args.api + "/api/tts", data=json.dumps({"text": text, "lang": item.get("tts_lang", "ru")}).encode(), headers={"Content-Type": "application/json"})
            with urllib.request.urlopen(req, timeout=60) as resp:
                raw = os.path.join(AUDIO, item["file"] + ".raw.wav")
                open(raw, "wb").write(resp.read())
            to_wav16k(raw, wav)
            os.remove(raw)
        else:
            if not shutil.which("say"):
                sys.exit("macOS `say` not found; use --via-api with a TTS provider configured")
            aiff = wav + ".aiff"
            subprocess.run(["say", "-v", args.voice, "-r", "175", "-o", aiff, text], check=True)
            to_wav16k(aiff, wav)
            os.remove(aiff)
        open(os.path.join(AUDIO, os.path.splitext(item["file"])[0] + ".txt"), "w", encoding="utf-8").write(text + "\n")
        print("wrote", wav)


if __name__ == "__main__":
    main()
