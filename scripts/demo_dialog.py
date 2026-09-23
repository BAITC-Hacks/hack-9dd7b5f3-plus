#!/usr/bin/env python3
"""Play scripted multi-turn dialogues (text channel) against the running
backend and print what the router decided at each step: scenario, path,
policy, identified client, topic stack, pending confirmation, actions and
timings. Use it to watch the dialogue state machine without the UI.

    python3 scripts/demo_dialog.py                  # all dialogues
    python3 scripts/demo_dialog.py --only cancel    # one dialogue by name
    python3 scripts/demo_dialog.py --api http://localhost:8080 --voice
"""
import argparse
import json
import os
import sys
import urllib.request

DIALOGS = {
    "claim_status_then_renewal": [
        ("Здравствуйте, хочу узнать, что с моим заявлением по каско.", ["SC17"]),
        ("Номер не помню, телефон плюс семь семьсот один ноль ноль ноль ноль ноль ноль семь.", ["SC17"]),
        ("Понял. А каско у меня скоро заканчивается, можно продлить в рассрочку?", ["SC27", "SC31"]),
        ("Давайте потом, сначала пусть решат по заявлению. А по заявлению мне что-то ещё нужно донести?", ["SC18"]),
        ("Отлично, спасибо.", ["SYS_GOODBYE"]),
    ],
    "topic_switch_payment_address": [
        ("Здравствуйте, я вчера оплатил полис, деньги списались, а полиса нет. А, и ещё адрес поменять надо, я переехал.", ["SC30", "SC29"]),
        ("Плюс 7 701 000 00 03.", ["SC30"]),
    ],
    "cancel_with_confirmation_kk": [
        ("Сәлеметсіз бе, көлігімді саттым, КАСКО шартын бұзып, қалған ақшаны қайтарғым келеді.", ["SC28"]),
        ("Телефон: плюс жеті, жеті жүз бір, нөл нөл нөл, нөл нөл, он.", ["SC28"]),
        ("Иә, растаймын.", ["SC28"]),
    ],
    "unclear_then_resend": [
        ("Здравствуйте, у меня проблема с полисом.", ["SYS_UNCLEAR"]),
        ("Нет, оплатил, в приложении полис есть, а на почту ничего не пришло.", ["SC26"]),
        ("Плюс 7 701 000 00 09.", ["SC26"]),
    ],
    "fraud_then_validity": [
        ("Мне сейчас звонили, сказали, что они из Saqta и страховку надо переоформить, просили назвать код из SMS.", ["SC38"]),
        ("Нет, не сообщил, сразу положил трубку.", ["SC38"]),
        ("Да, проверьте. Номер плюс 7 701 000 00 08.", ["SC25"]),
    ],
    "mixed_language_dms": [
        ("Сәлеметсіз бе, маған терапевтке жазылу керек, завтра утром можно?", ["SC21"]),
        ("Плюс жеті, жеті жүз бір, нөл нөл нөл, нөл нөл, нөл екі.", ["SC21"]),
        ("Иә, жазыңыз. А анализы тоже бесплатно по страховке?", ["SC21", "SC22"]),
    ],
    "out_of_scope_and_operator": [
        ("Можно у вас взять кредит на машину?", ["SYS_OUT_OF_SCOPE"]),
        ("Ладно. Соедините меня с оператором.", ["SC37"]),
    ],
}


def post(url, payload):
    req = urllib.request.Request(url, data=json.dumps(payload).encode(), headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=120) as resp:
        return json.load(resp)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--api", default=os.environ.get("API_URL", "http://localhost:8080"))
    ap.add_argument("--only", default="")
    ap.add_argument("--voice", action="store_true", help="also synthesize replies (needs a TTS provider or a connected UI)")
    args = ap.parse_args()
    cfg = json.load(urllib.request.urlopen(args.api + "/api/config", timeout=10))
    print(f"backend: llm={cfg['llm']['provider']}/{cfg['llm']['model']} fast_path={cfg['fast_path']}\n")
    total = ok = 0
    for name, turns in DIALOGS.items():
        if args.only and args.only not in name:
            continue
        sess = post(args.api + "/api/sessions", {"channel": "demo_dialog"})["session_id"]
        print(f"=== {name}  (session {sess}) ===")
        for text, expected in turns:
            r = post(f"{args.api}/api/sessions/{sess}/turn", {"text": text, "voice": args.voice})
            tr = r["trace"]
            got = r["scenarios"]
            hit = bool(got) and got[0] == expected[0]
            total += 1
            ok += hit
            st = tr["state"]
            mark = "✓" if hit else "✗"
            print(f"{mark} client: {text}")
            print(f"     → {got} (expected {expected})  path={tr['path']} policy={tr['policy']['action']} conf={r['confidence']}  lang {tr['language']['detected']}→{tr['reply']['lang']}")
            reason = tr["decision"]["scenarios"][0].get("reason", "")
            if reason:
                print(f"     reason: {reason}")
            acts = [f"{a['name']}[{a['mode']}]" + (" ERR " + a["error"] if a.get("error") else "") for a in tr["actions"]]
            state = f"client={st.get('client_name') or '-'} active={st.get('active_scenario') or '-'} stack={st['stack']} pending={(st.get('pending_confirmation') or {}).get('name') or '-'}"
            print(f"     state: {state}" + (f"  actions: {', '.join(acts)}" if acts else ""))
            t = tr["timings"]
            print(f"     bot: {r['reply']}   [route {t.get('route', 0)} ms, total {t.get('total', 0)} ms]")
        print()
    print(f"primary scenario correct: {ok}/{total}")
    sys.exit(0 if ok == total else 1)


if __name__ == "__main__":
    main()
