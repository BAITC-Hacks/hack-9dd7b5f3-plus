# Plus · Voice Router

**HackAlem AI · Halyk Bank · кейс Voice Router.** Голосовой симулятор страхового контакт-центра: клиент говорит по-русски или по-казахски, LLM выбирает сценарий с учётом контекста, супервизор видит причину, альтернативы и время по этапам.

> Статус: реализован стенд на официальном наборе **Saqta Insurance: 40 сценариев + 3 системных намерения**, 104 dev-реплики и исходный оценщик в `data/`. Автотесты и Docker-путь проверяются без ключей. Точность настоящей модели и достижение 500/1500 мс **не заявлены до запуска с API-ключом**. Mock — тест инфраструктуры. Deployed-ссылка пока отсутствует.

## Что реализовано

- Go API + Next.js: микрофон, текст, загрузка аудиофайла, озвучка и история диалогов.
- Настоящий LLM-выбор из каталога: OpenAI, NVIDIA или OpenAI-compatible endpoint; история до 10 реплик, смена темы, отложенные темы, RU/KK/mixed.
- Строгий JSON, проверка ID, короткое объяснение, альтернативы, извлечение параметров. Низкая уверенность → уточнение; повторное непонимание/сбой → контекст для оператора.
- Live NDJSON-события, время по этапам, токены, ошибки/повтор, выгрузка JSON и p50/p95 без mock.
- WebRTC streaming STT + клиентский VAD, потоковый PCM TTS через Web Audio; браузерная речь как вариант без ключа.
- PostgreSQL в Docker Compose, миграция на старте. Нативный режим без БД сохраняет сессии в памяти процесса.
- Три синтетических WAV, скрипт оценки по тексту/аудио, автоматические проверки HTTP, retry/deadline, JSON, контекста и речи.

Никакие страховые или финансовые операции не выполняются. Handoff готовит контекст для супервизора, подключения к реальному call-center нет. Голос ответа синтезирован AI.

## Как работает и почему так

```text
Микрофон → streaming STT → один LLM routing call → JSON validation / safety → ответ каталога → streaming TTS
                                    ↕                         ↓
                         полный каталог + история        PostgreSQL + live trace
```

Один короткий запрос выбирает смысловой сценарий. Стабильный префикс каталога пригоден для prompt caching провайдера. Дополнительного LLM для текста ответа нет: ответы берутся из проверенных RU/KK-шаблонов. Это сокращает задержку и ограничивает выдуманные факты. Весь каталог виден модели, а не отсекается поиском. Архитектура и точные JSON-контракты: [docs/SPEC.md](docs/SPEC.md); исходное ТЗ: [docs/CASE.md](docs/CASE.md).

## Технологии и зависимости

Go 1.26.5, chi, pgx, PostgreSQL 17; Next.js 16.3.6, React 19, TypeScript, Tailwind 4, Lucide. Node.js 24+ для native frontend; Python 3.10+ для оценки, без pip-зависимостей. Все компоненты и лицензии: [THIRD_PARTY.md](THIRD_PARTY.md).

Основной путь: Docker Engine + Compose v2, интернет для скачивания образов. Микрофон доступен на localhost или HTTPS. Web Speech работает не во всех браузерах и может использовать сетевой сервис браузера; Chrome — предпочтительный вариант для режима без ключей. Полностью офлайн гарантируется текстовый mock после сборки.

## Запуск одной командой

```bash
git clone https://github.com/BAITC-Hacks/hack-9dd7b5f3-plus.git
cd hack-9dd7b5f3-plus
cp .env.example .env
docker compose up --build
```

- Интерфейс: **http://localhost:3000**
- API/конфигурация: **http://localhost:8080/healthz**
- Авторизация в демо отсутствует: личные аккаунты/ключи для текстового mock не требуются.
- Для остановки: `docker compose down`. Том PostgreSQL сохраняется.

Без Docker, два терминала:

```bash
# Терминал 1 — из корня. Без DATABASE_URL используется memory store.
cd backend
go mod download
go run ./cmd/api
```

```bash
# Терминал 2 — из корня
cd frontend
npm ci
npm run dev
```

Go сам не читает `.env`. Для native запуска с настройками из файла выполните в backend: `set -a; . ../.env; set +a`, затем `go run ./cmd/api`. Не коммитьте `.env`.

## Настоящая модель и голос

Для всего пути достаточно **одного OpenAI API key** с доступом к выбранным LLM/STT/TTS/Realtime моделям. В локальном `.env`:

```dotenv
LLM_PROVIDER=openai
SPEECH_PROVIDER=openai
OPENAI_API_KEY=your-local-key
OPENAI_MODEL=gpt-4.1-mini
STT_MODEL=gpt-4o-mini-transcribe
TTS_MODEL=gpt-4o-mini-tts
REALTIME_MODEL=gpt-live-transcribe
```

Затем `docker compose up --build`. Эти имена — настраиваемые начальные значения, доступ к ним зависит от аккаунта. Вызовы платные по условиям провайдера. Ключ хранится только в Go, браузер получает SDP. Для выбранной Realtime-модели нужна поддержка transcription + manual commit; при недоступности доступен путь загрузки аудио.

NVIDIA: `LLM_PROVIDER=nvidia`, `NVIDIA_API_KEY`, `NVIDIA_MODEL`. Если endpoint не поддерживает strict JSON schema, задайте `LLM_JSON_SCHEMA=false`; локальная валидация остаётся обязательной. Речь по-прежнему выбирается независимо через SPEECH_PROVIDER.

OpenRouter для настоящей маршрутизации с браузерным голосом:

```dotenv
LLM_PROVIDER=openai_compatible
LLM_BASE_URL=https://openrouter.ai/api/v1
LLM_API_KEY=your-openrouter-key
LLM_MODEL=openai/gpt-4.1-mini
LLM_JSON_SCHEMA=true
SPEECH_PROVIDER=browser
```

Сохраните настройки в локальном `.env`, затем выполните `docker compose up -d --force-recreate backend`. Контейнер получит новые переменные; простой `docker compose restart` их не обновляет. Перезагрузите страницу [localhost:3000](http://localhost:3000). В `/healthz` должны появиться `provider: openai_compatible` и выбранная модель. Браузерный микрофон и озвучка зависят от поддержки браузера и установленных голосов; для проверки используйте Chrome. Загрузка аудиофайлов и серверные STT/TTS/Realtime в текущей реализации требуют отдельного `OPENAI_API_KEY` и `SPEECH_PROVIDER=openai`; ключ OpenRouter в `OPENAI_API_KEY` не подходит. [Документация OpenRouter](https://openrouter.ai/docs/quickstart).

Локальный OpenAI-compatible сервер: `LLM_PROVIDER=openai_compatible`, `LLM_BASE_URL=http://host.docker.internal:8000/v1`, `LLM_MODEL=<model>`, `LLM_API_KEY` при необходимости. Модель должна поддерживать chat completions и JSON. Локальный LLM runtime не входит в Compose; его нужно запустить отдельно. ElevenLabs не требуется, интеграция не добавлена.

## Переменные окружения

Все настройки находятся в [.env.example](.env.example).

| Переменная | Значение по умолчанию / назначение |
|---|---|
| LLM_PROVIDER | mock; openai / nvidia / openai_compatible |
| OPENAI_API_KEY | пусто; секрет на backend, также для речи |
| OPENAI_MODEL / OPENAI_BASE_URL | gpt-4.1-mini / https://api.openai.com/v1 |
| NVIDIA_API_KEY / NVIDIA_MODEL | пусто / meta/llama-3.3-70b-instruct |
| LLM_BASE_URL / LLM_API_KEY / LLM_MODEL | настройки compatible provider |
| LLM_JSON_SCHEMA | true; false только для несовместимого endpoint |
| LLM_TIMEOUT | 8s на обе попытки; максимум 30s |
| SPEECH_PROVIDER | browser; openai включает STT/TTS/WebRTC |
| STT_MODEL | gpt-4o-mini-transcribe для файлов |
| TTS_MODEL / TTS_VOICE | gpt-4o-mini-tts / coral |
| REALTIME_MODEL | gpt-live-transcribe; клиентский VAD и manual commit |
| CORS_ORIGINS | http://localhost:3000; список через запятую |
| NEXT_PUBLIC_API_URL | http://localhost:8080; фиксируется при сборке frontend |
| DATA_PATH | ./data; путь на хосте для Compose mount |
| DATA_DIR | ../data; путь для native Go; Compose задаёт /app/data |
| DATABASE_URL | native: пусто → память; Compose: postgres://plus:plus@db:5432/plus?sslmode=disable |
| PORT | native backend 8080 |
| API_PORT / WEB_PORT | порты хоста Compose: 8080 / 3000 |

## Как проверить решение

1. Откройте интерфейс; проверьте бейдж mock/LLM и источник каталога.
2. Включите микрофон и разрешите доступ. С настоящей speech API речь завершается автоматически по паузе 360 мс. Можно завершить кнопкой. Без ключа используется браузерное распознавание с выбором RU/KZ.
3. Скажите: «Хочу продлить действующий полис», затем «Где находится ваш офис?», затем «Вернёмся к первому вопросу, он завтра заканчивается». **Смысловой возврат проверяйте с LLM**, mock ограничен совпадениями каталога.
4. Проверьте казахский: «Өтемақы қашан түседі?»; смешанную речь: «Полисімді ұзартқым келеді, он завтра заканчивается».
5. Справа видны маршрут, обоснование, альтернативы, pending/slots, время. Раскройте **«События и ответы API»** для наблюдения в реальном времени. Кнопка загрузки справа внизу экспортирует сессию и trace в JSON.
6. Попросите оператора; увидите явный handoff без ложного сообщения о реальном соединении. История доступна во вкладке «История диалогов».

В режиме mock RU-озвучка зависит от установленных голосов ОС. При отсутствии казахского голоса интерфейс честно сообщает об этом; для KK используйте speech API. Текстовый ответ при ошибке озвучки сохраняется.

## Три аудиопримера и тесты на их основе

Все WAV — синтетическая русская речь Milena, mono PCM16 16 kHz, без личных данных:

1. [01-payment.wav](frontend/public/samples/01-payment.wav) — деньги списались, полиса нет.
2. [02-topic-switch.wav](frontend/public/samples/02-topic-switch.wav) — смена темы на офис.
3. [03-return-topic.wav](frontend/public/samples/03-return-topic.wav) — возврат к продлению.

Контекст и ожидаемые ID: [samples/audio_cases.json](samples/audio_cases.json). Аудиофайлы не проверяют качество KK STT; казахские/смешанные текстовые случаи есть в [samples/text_cases.json](samples/text_cases.json). Для KK-аудио запишите синтетический текст носителем языка или выбранным TTS и добавьте путь/ожидаемый ID в отдельный manifest.

Загрузите любой WAV кнопкой файла рядом с вводом. Для автоматического теста **после включения реального LLM и SPEECH_PROVIDER=openai**:

```bash
python3 scripts/evaluate.py --audio --cases samples/audio_cases.json \
  --with-tts --stream --output reports/audio.json
```

Скрипт передаёт настоящие WAV-байты в STT (без подстановки эталонного текста), выводит живые события, оценивает сценарий, считает STT WER по пробелам и routing p50/p95. `--with-tts` сохраняет ответные WAV рядом с отчётом. Задержка загрузки файла и первый TTS-байт **не выдаются за время до слышимого ответа**. Для 1,5 с используйте микрофон и браузерную телеметрию; это оценка playback timing, не аппаратный замер.

```bash
# Настоящий LLM, текстовые случаи. Достаточно LLM ключа, speech API не нужен.
python3 scripts/evaluate.py --cases data/dev_utterances.json --repeat 3 --output reports/llm.json
# Инфраструктура без ключа; низкая точность mock ожидаема и не скрывается.
python3 scripts/evaluate.py --allow-mock --cases data/dev_utterances.json --output reports/mock.json
# Сравнение после изменения модели/промпта на том же наборе и числе прогонов.
python3 scripts/evaluate.py --cases data/dev_utterances.json --repeat 3 \
  --compare reports/llm.json --output reports/candidate.json
# Автотесты без ключей:
(cd backend && go test -race ./... && go vet ./...)
(cd frontend && npm run lint && npm run build)
python3 -m unittest discover -s scripts -p 'test_*.py'
```

Критерий CI можно задать `--min-accuracy 0.9`. Ошибки провайдера не засчитываются как правильный handoff. API-ошибки/неуспех порога дают ненулевой exit code. Mock-бенчмарк требует явный `--allow-mock` и помечается отдельно. Оригинальный `data/evaluate.py` сохранён без изменений. Это оценщик predictions, а не модель baseline. Экспорт и официальный подсчёт:

```bash
python3 scripts/evaluate.py --cases data/dev_utterances.json \
  --predictions reports/predictions.json --output reports/official.json
python3 data/evaluate.py reports/predictions.json data/dev_utterances.json
```

104 реплики оцениваются по primary accuracy, full match и multi-intent recall; текущие дополнительные намерения экспортируются после главного. Исторические прерванные темы не примешиваются к текущему ответу. Для сравнения с другой реализацией нужен её файл predictions.

Повторная генерация аудио на macOS (не обязательна): `python3 scripts/make_samples.py`; требуется `say` с Milena и `ffmpeg`.

Терминальная отладка:

```bash
docker compose logs -f backend
curl -N http://localhost:8080/api/turns/stream -H 'Content-Type: application/json' \
  -d '{"text":"Деньги списались, а полиса нет"}'
curl http://localhost:8080/api/stats
```

## Данные и интеграции

Оригинальный стартовый кит находится в `data/` и не изменяется. Все клиенты, телефоны, полисы, адреса и условия в нём синтетические. **Дата среза — 2026-10-01**, именно она используется для проверки срока полиса. Каталог: 40 SCxx и 3 SYS_*; эталоны: 104 реплики и 10 диалогов. Приложение загружает только сценарии, слоты, knowledge_base и mock_backend; dev_utterances и разметку диалогов маршрутизатор не читает.

Backend нормализует оригинальные `scenario_id`, `not_this_if`, `priority`, RU/KK examples и responses, не меняя ID. LLM видит компактный каталог (без длинных ответных шаблонов) и историю. У `urgent` приоритет выше остальных намерений. Отдельно хранится стек прерванных тем. Слоты валидируются; при нехватке данных задаётся вопрос из `slots.json`.

Ответы основаны на opening-шаблонах; есть read-only lookup офиса/клиник по городу, статуса заявления, срока полиса и способов оплаты. Панель показывает конкретную запись-источник. Закрывающие шаблоны с выдуманным «SMS отправлено»/«полис оформлен» не используются. Мутации отключены, заполненные параметры дают только подтверждение подготовки обращения.

Compose монтирует `./data`. Native Go из `backend/` использует `../data`. Для Railway backend context содержит проверенную копию **только runtime-файлов** в `backend/data/official` с SHA256; обновить её можно `python3 scripts/sync_data.py`. Разметка и оценщик туда не копируются. При сборке из /backend DATA_DIR по умолчанию `/app/data/official`.

Независимый маленький каталог разработки сохранён в `backend/data/demo`: включается явно `DATA_PATH=./backend/data/demo`; он не предназначен для официальных аудио/текстовых manifests. Собственные перефразированные регрессии: `samples/text_cases.json`.

Официальные API-справочники: [structured output](https://developers.openai.com/api/docs/guides/structured-outputs), [file STT](https://developers.openai.com/api/docs/guides/speech-to-text), [Realtime transcription](https://developers.openai.com/api/docs/guides/realtime-transcription), [WebRTC](https://developers.openai.com/api/docs/guides/voice-webrtc?api=realtime), [streaming TTS](https://developers.openai.com/api/docs/guides/text-to-speech).

## Ограничения и измерения

- Настоящий `openai/gpt-4.1-mini` через OpenRouter проверен на 12 собственных текстовых регрессиях: 12/12 основных маршрутов/статусов, 11/12 полных наборов намерений, ошибок API нет; p50 1576 мс, p95 1801 мс. Это короткая проверка подключения и поведения, а не оценка на официальных 104 репликах или тесте жюри. Live STT/TTS/Realtime, качество казахской речи и улучшение baseline ещё не проверены. Подробности: [docs/VALIDATION.md](docs/VALIDATION.md).
- 500 мс / 1,5 с — отображаемые цели; сеть, объём каталога, output tokens, VAD и провайдер влияют на результат. UI показывает превышение; p50/p95 mock исключены.
- Самооценка confidence не калибрована. Короткое объяснение не является доказательством правильности; проверяйте по размеченным данным.
- Шумный микрофон может преждевременно завершить речь или не обнаружить паузу. У VAD фиксированный порог. Максимум 30 секунд записи / 10 реплик.
- Только одна backend-реплика; для нескольких нужны распределённые блокировки. Native memory store теряет историю после рестарта. PostgreSQL UI-метрики смотрят последние 100 сессий.
- Редактор каталога, телефония, операторский helpdesk, реальные платежи и авторизация не реализованы. Это стенд для синтетических данных, не публичный production контакт-центр.
- Нет автоматической проверки личности и доступа к реальной страховой системе. Read-only ответы берутся из организаторских синтетических records; остальные сценарии собирают параметры. Реальные мутации отключены.

## Railway и воспроизводимость

Создайте Postgres и два сервиса из этого репозитория: root `/backend` и `/frontend`, каждый использует свой Dockerfile. Backend: DATABASE_URL, LLM/speech variables, CORS_ORIGINS с HTTPS origin фронтенда. Frontend: NEXT_PUBLIC_API_URL с HTTPS backend **на этапе сборки**. Railway PORT использует приложение. Для официального каталога поместите данные в backend/data и задайте DATA_DIR.

**Deployed URL: не настроен.** Перед сдачей капитан должен отдельно нажать «Сдать решение» на платформе; git push не является сдачей.

Команда Plus: Tair, Alikhan, Ramazan. Коммиты сохраняют человеческую git identity; личные вклады участников требуют отдельных коммитов самих участников.
