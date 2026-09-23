#!/usr/bin/env python3
"""Generate three synthetic Russian audio fixtures with macOS Milena + ffmpeg.
No recordings or personal data. Existing WAVs are committed; generation is optional.
"""
import json
from pathlib import Path
import subprocess
import tempfile
ROOT = Path(__file__).resolve().parents[1]
OUTPUT = ROOT / "frontend/public/samples"
CASES = [
    {"id": "payment_ru", "file": "../frontend/public/samples/01-payment.wav", "transcript": "Здравствуйте. Я оплатил страховку. Деньги списались, а полиса нет.", "expected_scenario": "payment_failed", "history": []},
    {"id": "topic_switch_ru", "file": "../frontend/public/samples/02-topic-switch.wav", "transcript": "А теперь другой вопрос. Где находится ваш офис и когда он работает?", "expected_scenario": "office", "history": ["Я оплатил страховку. Деньги списались, а полиса нет."]},
    {"id": "return_topic_ru", "file": "../frontend/public/samples/03-return-topic.wav", "transcript": "Вернёмся к полису. Хочу продлить страховку, которая заканчивается завтра.", "expected_scenario": "renew_policy", "history": ["Хочу продлить действующий полис", "Где находится ваш офис?"]},
]
def main():
    OUTPUT.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory() as tmp:
        for case in CASES:
            text = Path(tmp) / "text.txt"; audio = Path(tmp) / "speech.aiff"
            text.write_text(case["transcript"])
            subprocess.run(["say", "-v", "Milena", "-r", "165", "-f", str(text), "-o", str(audio)], check=True)
            subprocess.run(["ffmpeg", "-y", "-loglevel", "error", "-i", str(audio), "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", str(OUTPUT / Path(case["file"]).name)], check=True)
    (ROOT / "samples/audio_cases.json").write_text(json.dumps({"source": "synthetic_demo", "voice": "macOS Milena; Russian only", "cases": CASES}, ensure_ascii=False, indent=2) + "\n")
if __name__ == "__main__": main()
