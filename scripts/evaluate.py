#!/usr/bin/env python3
"""Evaluate real HTTP routing, optionally from audio bytes. Standard library only.
No transcript is used as STT output, and mock results require explicit opt-in.
"""
import argparse
import json
import math
from pathlib import Path
import sys
import time
import urllib.error
import urllib.request
import uuid
import wave


def request(base, path, data=None, content_type="application/json", stream=False):
    body = json.dumps(data, ensure_ascii=False).encode() if isinstance(data, (dict, list)) else data
    req = urllib.request.Request(base.rstrip("/") + path, data=body, headers={"Content-Type": content_type})
    try:
        response = urllib.request.urlopen(req, timeout=65)
    except urllib.error.HTTPError as e:
        try: message = json.load(e).get("error", f"HTTP {e.code}")
        except (ValueError, AttributeError): message = f"HTTP {e.code}"
        raise RuntimeError(message) from None
    if stream: return response
    with response: return json.load(response)


def percentile(values, fraction):
    return sorted(values)[max(0, math.ceil(len(values) * fraction) - 1)] if values else None


def word_error_rate(reference, hypothesis):
    a, b = reference.lower().split(), hypothesis.lower().split()
    row = list(range(len(b) + 1))
    for i, word in enumerate(a, 1):
        new = [i]
        for j, other in enumerate(b, 1):
            new.append(min(new[-1] + 1, row[j] + 1, row[j - 1] + (word != other)))
        row = new
    return row[-1] / max(1, len(a))


def load_cases(path):
    data = json.loads(path.read_text())
    if isinstance(data, dict): data = data.get("cases", data.get("utterances", data.get("dev_utterances", [])))
    if not isinstance(data, list) or not data: raise ValueError("Expected non-empty list or {cases:[...]} / {utterances:[...]}")
    return data


def route(base, text, session_id="", kind="text", stt_ms=None, verbose=False):
    body = {"session_id": session_id, "text": text, "input_kind": kind}
    if stt_ms is not None: body["stt_ms"] = stt_ms
    if not verbose: return request(base, "/api/route", body)
    result = None
    with request(base, "/api/turns/stream", body, stream=True) as response:
        for line in response:
            event = json.loads(line)
            print(json.dumps(event, ensure_ascii=False), flush=True)
            if event["type"] == "error": raise RuntimeError(event["data"]["error"])
            if event["type"] == "result": result = event["data"]
    if result is None: raise RuntimeError("Stream ended without result")
    return result


def audio_input(base, file):
    if not file.is_file(): raise ValueError(f"Missing audio: {file}")
    if file.stat().st_size > 24 * 1024 * 1024: raise ValueError("Audio exceeds 24 MiB")
    boundary = "voice-router-" + uuid.uuid4().hex
    # Use a generic filename: classification must depend on bytes, not filename/label.
    name = "input" + file.suffix
    body = (f'--{boundary}\r\nContent-Disposition: form-data; name="audio"; filename="{name}"\r\nContent-Type: application/octet-stream\r\n\r\n'.encode()
            + file.read_bytes() + f"\r\n--{boundary}--\r\n".encode())
    return request(base, "/api/transcribe", body, f"multipart/form-data; boundary={boundary}")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://localhost:8080")
    parser.add_argument("--cases", type=Path, default=Path("data/dev_utterances.json"))
    parser.add_argument("--audio", action="store_true", help="Send actual audio bytes through STT, never fixture transcripts")
    parser.add_argument("--with-tts", action="store_true", help="Download reply PCM into WAV; measures TTS first byte, NOT audible end-to-end")
    parser.add_argument("--stream", action="store_true", help="Print live stage events")
    parser.add_argument("--allow-mock", action="store_true", help="Infrastructure smoke only; not model quality")
    parser.add_argument("--repeat", type=int, default=1)
    parser.add_argument("--min-accuracy", type=float, default=0)
    parser.add_argument("--output", type=Path, default=Path("reports/evaluation.json"))
    parser.add_argument("--predictions", type=Path, help="Export original evaluate.py predictions (requires repeat=1)")
    parser.add_argument("--compare", type=Path, help="Compare to a prior report made on the same cases")
    args = parser.parse_args(argv)
    if args.repeat < 1 or not 0 <= args.min_accuracy <= 1: parser.error("repeat >= 1 and 0 <= min-accuracy <= 1 required")
    if args.predictions and args.repeat != 1: parser.error("--predictions requires --repeat 1")
    health = request(args.base_url, "/healthz")
    if health["provider"] == "mock" and not args.allow_mock: parser.error("MOCK is not a model benchmark. Select a real LLM or explicitly use --allow-mock.")
    if args.audio and not health["speech_ready"]: parser.error("Audio evaluation requires SPEECH_PROVIDER=openai and OPENAI_API_KEY. Fixture transcripts are never substituted.")
    cases = load_cases(args.cases)
    results = []
    args.output.parent.mkdir(parents=True, exist_ok=True)
    for repeat in range(args.repeat):
        for index, case in enumerate(cases):
            item = {"id": case.get("id", index), "repeat": repeat, "expected": case.get("expected_scenario", case.get("scenario_id", case.get("expected"))), "expected_status": case.get("expected_status")}
            try:
                if not item["expected"] and not item["expected_status"]: raise ValueError("Case requires expected label(s) or expected_status")
                session_id = ""
                for history in case.get("history", []):
                    history_text = history if isinstance(history, str) else history["text"]
                    prior = route(args.base_url, history_text, session_id)
                    session_id = prior["session_id"]
                stt_ms = None
                start = time.perf_counter()
                if args.audio:
                    file = (args.cases.parent / case["file"]).resolve()
                    transcript = audio_input(args.base_url, file)
                    text, stt_ms = transcript["text"], transcript["stt_ms"]
                    if case.get("transcript"): item["word_error_rate_whitespace"] = word_error_rate(case["transcript"], text)
                else:
                    text = case.get("text", case.get("utterance", case.get("input")))
                    if not isinstance(text, str): raise ValueError("Case needs text/utterance/input string")
                result = route(args.base_url, text, session_id, "audio_file" if args.audio else "text", stt_ms, args.stream)
                turn = result["turn"]
                item.update({"transcript": text, "scenario": turn["decision"]["scenario_id"], "status": turn["decision"]["status"], "source": turn["source"], "routing_ms": turn["timing"]["routing_ms"], "stt_ms": stt_ms, "request_to_result_ms": (time.perf_counter() - start) * 1000, "turn": turn})
                expected = item["expected"] if isinstance(item["expected"], list) else [item["expected"]] if item["expected"] else []
                item["predicted"] = list(dict.fromkeys(([item["scenario"]] if item["scenario"] else []) + turn["decision"]["pending"])) if turn["source"] != "provider_error" else []
                item["correct"] = turn["source"] != "provider_error" and (not item["expected_status"] or item["status"] == item["expected_status"]) and (not expected or (bool(item["predicted"]) and item["predicted"][0] == expected[0]))
                item["full_match"] = set(item["predicted"]) == set(expected) if expected else item["correct"]
                if args.with_tts:
                    tts_start = time.perf_counter()
                    with request(args.base_url, f'/api/sessions/{result["session_id"]}/turns/{turn["id"]}/speech', stream=True) as response:
                        first = response.read(2)
                        if not first: raise RuntimeError("Empty speech stream")
                        item["tts_first_pcm_byte_ms"] = (time.perf_counter() - tts_start) * 1000
                        pcm = first + response.read()
                    output = args.output.parent / f"reply-{repeat}-{index}.wav"
                    with wave.open(str(output), "wb") as wav: wav.setnchannels(1); wav.setsampwidth(2); wav.setframerate(24000); wav.writeframes(pcm)
                    item["reply_audio"] = str(output)
                print(f'{item["id"]}: {item["scenario"] or item["status"]} | correct={item["correct"]} | routing={item["routing_ms"]:.1f} ms', flush=True)
            except (ValueError, KeyError, RuntimeError, OSError) as e:
                item.update({"error": str(e), "correct": False})
                print(f'{item["id"]}: ERROR {e}', file=sys.stderr, flush=True)
            results.append(item)
    timings = [r["routing_ms"] for r in results if "routing_ms" in r and r.get("source") != "provider_error"]
    accuracy = sum(r["correct"] for r in results) / len(results)
    report = {"benchmark_kind": "mock_infrastructure_only" if health["provider"] == "mock" else "audio_pipeline" if args.audio else "llm_routing", "configuration": health, "cases_file": str(args.cases), "n": len(results), "accuracy": accuracy, "routing_p50_ms": percentile(timings, .5), "routing_p95_ms": percentile(timings, .95), "routing_under_500ms_fraction": sum(t <= 500 for t in timings) / len(timings) if timings else None, "errors": sum("error" in r or r.get("source") == "provider_error" for r in results), "end_of_speech_to_audible_ms": None, "timing_note": "File upload is not real-time speech. Use browser microphone telemetry for end-of-speech to playback. Mock timings are not LLM timings.", "results": results}
    if args.compare:
        baseline = json.loads(args.compare.read_text())
        if baseline.get("cases_file") != report["cases_file"] or baseline.get("n") != report["n"]: raise ValueError("Comparison requires the same cases path and count")
        if baseline.get("benchmark_kind") != report["benchmark_kind"]: raise ValueError("Cannot compare mock, text, and audio benchmarks")
        report["comparison"] = {"accuracy_delta": accuracy - baseline["accuracy"], "routing_p95_delta_ms": report["routing_p95_ms"] - baseline["routing_p95_ms"] if report["routing_p95_ms"] is not None and baseline.get("routing_p95_ms") is not None else None}
    if args.predictions:
        args.predictions.parent.mkdir(parents=True, exist_ok=True)
        args.predictions.write_text(json.dumps({str(r["id"]): r.get("predicted", []) for r in results}, ensure_ascii=False, indent=2)+"\n")
    report["full_match"] = sum(r.get("full_match", False) for r in results) / len(results)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({k: v for k, v in report.items() if k not in ("results", "configuration")}, ensure_ascii=False, indent=2))
    return int(report["errors"] > 0 or accuracy < args.min_accuracy)

if __name__ == "__main__":
    try: sys.exit(main())
    except (ValueError, OSError, RuntimeError) as exc: print(f"Evaluation failed: {exc}", file=sys.stderr); sys.exit(2)
