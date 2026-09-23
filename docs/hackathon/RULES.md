# HackAlem AI — rules and logistics (digest)

> Source: the hackathon page at edu.astanahub.com (tabs "About the hackathon", "Regulations", "Tracks"), the HackAlem AI Telegram bot, and the channel t.me/hackalem. Captured on the morning of 23.09.2026. The regulations were updated on 22.09 at 22:00 — on any doubt, check against the original.
> Original (RU): docs/hackathon/raw/RULES.ru.md. On any doubt, the official regulations on edu.astanahub.com win.
> We do NOT write the personal registration code here (get it again: bot @decentra_world_bot → /mycode → HackAlem AI).

## 0. TL;DR — what actually kills us (disqualification / non-admission)

| # | Rule | Clause |
|---|---------|-------|
| 1 | **Every hour** (14:00, 15:00, 16:00, 17:00, 18:00) there must be a confirmed intermediate result (commit/feature/design/diagram/test). Missing any hour = disqualification | 5.4.8, 5.9.2 |
| 2 | All core development **only** in the repo created by the platform: `BAITC-Hacks/hack-9dd7b5f3-plus`. No third-party private repos. A verifiable commit history is required | 5.4.9, 5.4.11 |
| 3 | **18:00 — hard finish.** The state of the repo at 18:00 is the final version. Anything after that does not count | 5.4.13–5.4.14 |
| 4 | The project must **run for the experts using the README**. If it doesn't run — you're out, no explanations accepted | 5.4.15–5.4.16, 5.6.3–5.6.5 |
| 5 | Review without participants' personal accounts/subscriptions: provide demo access, test accounts, a way to verify without our keys | 5.6.6 |
| 6 | Disclose all third-party components (OSS, libraries, models, datasets, templates) | 5.4.4–5.4.6 |
| 7 | Do not present a finished product built before 13:00. Preparations (templates, own libs, infra) are allowed, but the core functionality must be built during 13:00–18:00 | 5.4.4.2, 5.4.5 |
| 8 | All three team members on-site with check-in before 18:00. Leaving only with the organizer's permission, ≤ 60 min total. Bot: "leaving before 18:00 is FORBIDDEN" | 5.1.7 |
| 9 | One project = one Task | 5.4.1.1 |
| 10 | **During the last hour (17:00–18:00), leaving the zone is forbidden.** Emergency exit — up to 5 min with a steward's permission. Whoever left earlier must return by 17:00 | Scoring rules |
| 11 | **Once it starts, you cannot change seats** (even within your sector) or move to a different sector. Sit together as a team — BEFORE 13:00 | Scoring rules |
| 12 | **Every participant must make a personal contribution** — otherwise participation is not counted → all three must have their own commits | Scoring rules |
| 13 | **Submit the solution on the platform:** Tracks → our case → "Сдать решение" ("Submit solution") → name + description. Can be updated until the deadline. A push to the repo by itself ≠ submission | Participant instructions, item 10 |
| 14 | Using an AI agent (Codex or any other) is mandatory | Participant instructions, item 5 |
| 15 | **Sharing internet from a phone / hotspots / your own routers is forbidden.** Only the official hackathon network (wired Ethernet, DHCP) | Cybersecurity §3–4 |
| 16 | No API keys/secrets in code, repo, presentations, or chats | Cybersecurity §5, Instructions item 1 |

## 1. General

- **Name:** HackAlem AI — "the world's largest agentic AI hackathon."
- **Organizer:** AKF "Astana Hub" (+ co-organizers, BAITC). Contact: team@baitc.org.
- **Prize fund and resources:** $1,100,000.
- **Format:** offline, Astana only. Online participation is forbidden for all team members.
- **Team:** 1–3 people. Our team: **Plus** — Tair, Alikhan, Ramazan. One person — one team, one project.
- **Venue limit:** 2,500 participants, first come, first served.
- **Location:** EXPO International Exhibition Centre, Astana, Mәңgilik El Ave., 53/1 — https://go.2gis.com/ukpPR
- **Participation is free.** Age 18+; working/studying in IT, student/graduate ≤12 months, or an IT enthusiast with projects.

## 2. Schedule, 23.09.2026 (GMT+5)

| Time | Stage |
|-------|------|
| 09:00–12:00 | Check-in: personal number, full name, phone number from the application |
| 12:00–13:00 | Opening ceremony |
| 12:30 | Promo codes appear on the hackathon page |
| **13:00–18:00** | **Competitive part (5 hours).** Track assignments are published at 13:00 |
| 18:00–18:30 | Wrap-up, closing of the offline stage |

Afterward:
| Date | Stage |
|------|------|
| 24.09–28.09 | Technical review by experts (+ possibly an AI judge), finalists selected |
| 29.09 | **Demo Day** (offline) — finalists pitch to the jury |
| 01.10 | Awards ceremony at the AI & Digital Bridge 2026 forum |

## 3. Resources (appear at 12:30, activate by 25.09.2026)

- ChatGPT & Codex — 1 month Pro 5x
- $50 OpenAI API credits
- $50 NVIDIA API tokens (NIM, OpenAI-compatible endpoint)
- Activation instructions — a single document from the post https://t.me/hackalem/1733

## 4. Tracks (12 total, assignments published at 13:00)

01 Energy · 02 Finance · 03 Governance · 04 Telecommunications · 05 Logistics · 06 Creative industries · 07 Education (pre-selected at registration) · 08 Innovation · **09 Communications (our track — Halyk Bank Voice Router case)** · 10 Trade · 11 Kazaktelecom special track · 12 Astana Innovations special track

The regulations say "10 Tasks and a unified scoring" — each Task has its own brief with its own criteria and points (5.5). Only criteria published before the competitive part begins apply.

## 5. Project and submission requirements

### 5.1 Repository
- Repo: https://github.com/BAITC-Hacks/hack-9dd7b5f3-plus (created by the platform, currently only a README).
- From 13:00 — the single primary working repo. The organizer can view history, metadata, and each participant's contribution (5.4.7) → **everyone commits from their own GitHub account.**
- Cloud services, external APIs, remote dev — allowed, but the code lives in this repo (5.4.12.1).

### 5.2 README (mandatory sections, 5.4.15 / 5.6.4)
1. Description of the solution and its purpose
2. Architecture
3. Technologies
4. Installation
5. Running the project
6. Dependencies / system requirements
7. Environment variables
8. Step-by-step procedure for verifying the main scenario

### 5.3 Third-party components
Disclose everything: OSS, models, datasets, templates, UI kits. A violation is passing off someone else's work as your own. Maintain a `THIRD_PARTY.md`.

### 5.4 AI
Any AI tools and agents are allowed. Codex is not mandatory.

## 6. Evaluation

### 6.1 Technical review (24–28.09)
- Initial check: compliance with the conditions, completeness, **whether it works**.
- Then experts review against the specific Task's brief criteria. They may look at the repo, demo, code, architecture, use of required tools, authenticity, and contribution.
- AI judges — used to assist preliminary ranking (→ the README and code must also be understandable to a machine: clear structure, explicit compliance with the brief's requirements).
- Results from different Tasks are normalized into an overall finalist ranking.

### 6.2 Demo Day (29.09) — jury criteria

| Criterion | What is assessed | Points |
|----------|---------------|------:|
| Value of the solution | A real, understandable problem, obvious benefit for the target audience | **25** |
| Result and quality | The prototype actually performs the stated scenario, coherence, UX | **20** |
| Innovativeness | A non-standard approach, a distinct advantage | **15** |
| Growth and scaling potential | Use after the hackathon, new users/organizations/industries | **20** |
| Presentation, demo, answers | Clarity, a convincing demo, answers to questions | **20** |
| **Total** | | **100** |

The jury decides collectively; the arithmetic is not binding. There is no appeal on the merits.

### 6.3 Prizes
- 10 main prize places, unified scoring, no quotas by task. One team — at most one place.
- The prize is received by the captain under a contract; taxes may be withheld; splitting it within the team is our own business.

## 7. Disqualification (5.9.2) — full list
- no intermediate result for any reporting hour;
- absence from the venue without permission;
- failed to check in;
- development outside the platform's repo / no history;
- inaccurate registration data;
- one person on multiple teams;
- the project was made by third parties without disclosure;
- interference with platforms/networks/other teams' projects;
- malicious code, attacks;
- unethical behavior, pressuring/bribing the jury;
- violation of venue rules/laws of the Republic of Kazakhstan;
- refusal to provide documents confirming the result/prize;
- any other violation of integrity and fairness of conditions.
May be applied without warning.

## 8. Intellectual property (6.1) — important
- The organizer receives a **free, perpetual, worldwide** right to use the Results (code, models, design, documentation, presentations), including modification and transfer to third parties.
- We guarantee the cleanliness of rights and licenses; in case of violation — joint and several liability of the team.
- → Do not bring proprietary code/data from our other projects or client NDAs into the repo.

## 9. Personal data and filming
Consent to processing of personal data, transfer to partners, photo/video, publication of names, team name, and project description.

## 10. Logistics
- Bring: laptop, charger, **LAN adapter**, all software and accounts set up in advance.
- Food from the organizer: 4 sandwich sets + 3 L of water per person. Own food — non-perishable, without a strong smell.
- Participation is at your own health risk; report any conditions requiring attention to the organizer in advance.

## 11. Official channels
- Landing page: https://hackalem.ai/
- Page: https://edu.astanahub.com/hackathons/df4743f5-c492-415c-b45a-1f13adb78e06
- Telegram channel: https://t.me/hackalem (post with the day's instructions: https://t.me/hackalem/1733)
- Telegram channel/chat from the regulations: https://t.me/+u6HpmeKGOdVlNzMy
- Q&A chat (FAQ): https://t.me/+pKzbwN43ot1hYTRi
- Bot: @decentra_world_bot (/mycode)
- Email: team@baitc.org
- Inquiries — from the captain through official channels: team name, circumstances, request, evidence.


## 12. Organizers' instructions (Google Doc "Instruction for participants", 6 PDF) — things not in the regulations

Source: https://docs.google.com/document/d/1DcG6BZKnESSGTA8HTz8szRWBRuWdzb0pRqonPsKESu0 (RU/KZ/EN PDF versions).

### 12.1 Rules for counting participation (badge, sector, exits)
- Enter the hall one at a time; the badge QR code is scanned. Using someone else's badge is forbidden; handing over your own badge or asking someone to scan it for you is forbidden.
- Find your sector, take a free seat. **Before the start**, you may change seats only within your own sector. **After the start — not at all**, and moving to another sector is not allowed. A steward is assigned to each sector.
- If you leave the zone before the start, you must return before the start is announced, otherwise participation is not counted.
- Breaks: **60 minutes total** per person. Tell the steward before leaving and after returning; the badge is scanned on every exit and entry.
- **During the last hour, everyone must be in the zone, exits are forbidden** (emergency exit ≤ 5 min with a steward's permission).
- Every participant must make a **personal contribution**; the project is created within the scope of the hackathon. Stewards/observers do not take part in development. For technical questions — raise your hand, a mentor will come over.
- Participation is NOT counted if: you did not return before the start; your badge was someone else's or was handed over; there is no personal contribution; you were off-site for more than 60 min total; you were absent during the last hour; you left before the official finish; you changed seats after the start or changed sector.

### 12.2 Instructions for participants (process)
1. Activate Codex, OpenAI API, NVIDIA. **Do not publish API keys on GitHub.**
2. **Choose ONE case** within the track — you do not need to solve all the cases in the track.
3. Study the case's requirements and evaluation criteria.
4. Assign roles and tasks.
5. Way of working: the Codex app (connect GitHub, give access to the team's repo), Codex in VS Code, or any other — **the main thing is to use an AI agent** and keep the code in the platform's repo.
6. Over 5 hours, implement the case's **mandatory requirements** and verify the main scenario from input data to result.
7. The README must contain: what the project does; what has been implemented; technologies and architecture; installation and running; a verification example; **the data and external services used; known limitations; a link to the deployed version**.
8. For the README you can use the **organizers' prompt from the case document** (if the case has its own, use that one). The base prompt is in `docs/hackathon/README_PROMPT.md`.
9. Push the latest version to the team's repo.
10. **Tracks page → select the case → «Сдать решение» ("Submit solution") → project name and description.** Can be updated until the deadline.

### 12.3 Activating OpenAI and NVIDIA
Personal links/promo codes are in the Telegram bot or on the platform.
- **ChatGPT Pro + Codex:** open the personal link → sign in → confirm **only if it says "Due today: $0"**, check the date of the next charge (set a reminder to cancel before it). Your profile should show Pro. Download Codex: chatgpt.com/codex, same account.
- **OpenAI API credits (per team):** separate link → API Credits → OpenAI Platform → confirm the promo code → Billing → Promotions → check under Applied promotions.
- **NVIDIA Brev** (GPU cloud): promo link → NVIDIA account (create an NVIDIA Cloud Account) → Billing → Redeem Code → check the Current Balance.
- **NVIDIA Build (API to models):** build.nvidia.com/settings/api-keys → Generate API Key (name, expiry) → save it somewhere safe.

### 12.4 Equipment and network
- Laptop, chargers (laptop + phone), mouse/headset optional, a **USB/USB-C → RJ-45 adapter** with its driver pre-installed.
- The on-site network is wired Ethernet. Settings: **obtain IP and DNS automatically (DHCP)**. Windows: `ncpa.cpl` → Ethernet → IPv4 → "Obtain IP/DNS automatically." Check with `ipconfig` — an address of `169.254.x.x` means DHCP did not work (cable/adapter/settings).
- Do not leave major OS updates for the day of the hackathon. Set up all software (IDE, Git, Go, Node, Docker, DB/API clients) in advance and log in to all accounts.

### 12.5 Cybersecurity (violation → network block, key revocation, project removal, suspension)
- Device: updates, screen lock, antivirus + firewall, no pirated software, do not leave it unlocked.
- **Official network only.** Forbidden: your own Wi-Fi access points, **phone in modem/hotspot mode**, sharing internet, mobile routers/USB modems, repeaters, networks with similar names, pentesting tools.
- Do not put secrets in code, repo, presentations, or chats. Unique passwords + MFA. After the hackathon, delete temporary keys/tokens and local copies of hackathon data.
- Use only data/services provided or permitted by the organizers; do not upload personal/confidential data to third-party services; do not use real production data; do not distribute hackathon data.
- Check AI outputs yourself for safety and correctness.
- Repo — access only for the team and organizers; do not publish the project before checking it for secrets.
- Without permission it is forbidden to: scan the network, intercept traffic, brute-force, run load/DoS tests, bypass limits, access other teams' resources.
- Incidents (leaked key, lost device, suspicious login, someone else's data) — report immediately to the cybersecurity headquarters. If a key leaks, rotate it immediately.

### 12.6 Bonus: Arizona State University online course
"Entrepreneurship Foundations" — https://share.articulate.com/Uh3iict_Yzqc3RBh7ylPd (no login). Complete all sections → upload the final assignment → fill out the Google Form (name + email) → digital ASU badge sent by email. Not part of the evaluation; do it after the hackathon.
