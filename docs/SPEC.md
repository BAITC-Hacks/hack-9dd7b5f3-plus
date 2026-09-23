# Voice Router — технический контракт

## Архитектура

```mermaid
flowchart LR
  Mic[Микрофон / RU + KK] --> RTC[WebRTC STT / клиентский VAD]
  File[Аудиофайл] --> STT[Go: multipart STT]
  RTC --> UI[Next.js console]
  STT --> UI
  UI -->|POST NDJSON: текст + session_id| Go[Go / chi]
  Go --> LLM[OpenAI-compatible LLM: весь каталог + история]
  LLM --> Validate[JSON + проверка ID + порог уверенности]
  Validate --> Reply[Ответ из каталога / уточнение / handoff]
  Reply --> DB[(PostgreSQL)]
  Reply --> UI
  UI --> TTS[Go: потоковый PCM TTS]
  TTS --> Audio[Web Audio playback]
```

Один LLM-вызов выбирает сценарий, кратко объясняет границу с альтернативами, извлекает явно названные параметры и перечисляет отложенные темы. Энкодерного intent-классификатора в рабочем LLM-пути нет. Весь каталог передаётся стабильно в system prefix; история в user JSON. Это избегает потери нужного сценария из-за предварительного retrieval. Цена: большой каталог и объём вывода могут мешать цели 500 мс. Provider prompt caching отражается в cached_tokens, если провайдер возвращает их.

Ответы не генерируются вторым LLM. Используются RU/KK opening templates оригинального каталога, вопросы из slots.json и read-only lookup в knowledge_base/mock_backend. Доступны офисы, клиники, статус заявления, срок полиса на дату 2026-10-01, способы оплаты. `evidence[]` с source/path/value показывает запись-основание. Mutation actions никогда не исполняются; closing templates с заявлениями о выполненной операции не используются.

## Данные

Оригинальный starter kit находится в `data/` (read-only). Каталог: 40 business `scenarios` + 3 `system_intents`, meta/as_of_date. Загрузчик нормализует scenario_id, not_this_if, slots.required, priority, examples.ru/kk, responses.ru/kk.opening. Raw runtime prompt содержит компактный JSON всех 43 вариантов, границы, приоритеты, примеры и слоты; ответные шаблоны не расходуют контекст routing. Идентификаторы сохранены. У urgent приоритет выше обычных намерений, включая mixed-language multi-intent.

Также поддерживается простой массив либо `{scenarios:[...]}` со строковыми id/name/description/boundaries/response_ru/response_kk; examples string[], requires_confirmation bool. Алиасы scenario_id/code → id, title/name_ru → name, purpose/description_ru → description. Неправильные/повторяющиеся ID останавливают запуск. `SYNTHETIC_DEMO` явно обозначает отдельный демонстрационный каталог из 12 сценариев. `organizer_saqta` — распознанный формат кита; SHA256 в health/trace позволяет сверить точный файл. Backend не загружает evaluation labels.

`DATA_DIR=../data` для native Go; Compose монтирует DATA_PATH=./data в /app/data. Railway root /backend использует `backend/data/official` — побайтовую копию только runtime-файлов, обновляемую scripts/sync_data.py. SHA256 записаны в checksums.json. DEV разметка/оценщик не входят в эту копию.

`voice_sessions(id text PK, payload jsonb, updated_at timestamptz)`: полный JSON сессии. Миграция embed SQL на старте. PostgreSQL в Compose; native без DATABASE_URL — явно обозначенный memory store до 1000 сессий. Один Go-процесс сериализует запросы к одной сессии. Поддерживается одна backend-реплика; распределённый режим потребует DB locking/optimistic concurrency. UI истории/статистика PostgreSQL ограничены последними 100 сессиями.

## HTTP API

Ошибки до начала stream: HTTP 4xx/5xx `{ "error": "..." }`. JSON ≤64 KiB, текст 1..4000 Unicode-символов. 409 — занятая сессия или лимит 10 реплик. Неизвестная сессия 404. 503 — недоступное хранилище. CORS допускает только явные origin из конфигурации.

| Метод / путь | Вход | Выход |
|---|---|---|
| GET /healthz (также /health) | — | status, provider, model, speech_provider, speech_ready, realtime_model, catalog_source/count/hash, system_count, storage, max_turns |
| GET /api/catalog | — | source, hash, business_count, system_count, as_of_date, scenarios[] |
| POST /api/sessions | — | новая Session |
| GET /api/sessions | — | Session[] |
| GET /api/sessions/{id} | — | Session, включая handoff-контекст |
| POST /api/route | RouteRequest | `{session_id, turn}` |
| POST /api/turns/stream | RouteRequest | NDJSON Event по мере этапов; последняя строка result |
| POST /api/transcribe | multipart: audio, ≤24 MiB | `{text, stt_ms, source}`; 503 без speech API |
| POST /api/realtime/connect | application/sdp offer, ≤128 KiB | application/sdp answer; server key остаётся в Go |
| GET /api/sessions/{id}/turns/{turn}/speech | — | signed PCM16 LE mono 24 kHz; поток, заголовок X-TTS-First-Byte-MS |
| POST /api/sessions/{id}/turns/{turn}/metrics | `{end_to_audio_ms?,tts_first_byte_ms?}` | `{ok:true}`; конечные числа 0..120000 |
| GET /api/stats | — | source_counts, routing_n, routing_p50_ms/p95_ms, audio_n, end_to_audio_p50_ms/p95_ms, note |

RouteRequest:
```json
{"session_id":"optional; empty creates session","text":"Полисімді ұзартқым келеді","input_kind":"text","stt_ms":120.0}
```
`input_kind`: text / microphone / audio_file. `stt_ms` необязателен. История достаётся сервером из session_id, а не доверяется клиентскому массиву.

Decision:
```json
{"scenario_id":"SC27","status":"route","confidence":0.9,"language":"kk","reason":"Клиент хочет продлить существующий полис.","alternatives":[],"slots":[],"pending":[]}
```
Все восемь полей обязательны. status=route|clarify|handoff, language=ru|kk|mixed, confidence ∈ [0,1]. ID должен существовать; для route обязателен. Альтернативы `{scenario_id,reason}` (до 3), slots `{name,value}` (до 12, значение ≤200 байт), pending до 4 существующих ID. Unknown JSON fields, null required fields, обрезанные ответы и придуманные ID отклоняются. Strict JSON Schema провайдера включена по умолчанию; локальная проверка всегда обязательна.

Turn: id, text, reply, decision, evidence[], pending_topics[], scenario_name, previous_scenario, topic_changed, source, calls[], timing, warnings[], created_at. Call: attempt, provider, model, latency_ms, prompt_tokens, completion_tokens, cached_tokens, error?. Session: id, turns[], active, pending[], created_at.

Event: `{type,elapsed_ms,data}`; типы input → routing_started → llm_attempt (0 в mock; до 2 для LLM) → routing_complete → policy → result. Ошибка сохранения даёт error и не даёт result. `routing_complete` — исходное решение LLM, `policy`/`result` — результат после порога уверенности. Фронтенд дополнительно показывает audio_started. Это события исполнения и публичное обоснование, не скрытые рассуждения модели.

## Отказы и контекст

Одна повторная попытка после временной ошибки/невалидной схемы, общий deadline `LLM_TIMEOUT` (по умолчанию 8s, максимум 30s) на обе попытки. 401/403 и другие постоянные HTTP 4xx не повторяются. У LLM-вызова логируются модель, провайдер, задержка, токены; ключи и сырой ответ провайдера не логируются. После исчерпания попыток — `source=provider_error`, handoff, явное предупреждение. **Автоперехода с реальной LLM на mock нет.**

Самооценка ниже 0.70 превращает route в clarify. После двух подряд нерешённых уточнений очередное clarify превращается в handoff. Смена active обновляется только при business route. Decision.pending содержит дополнительные намерения только текущей реплики для совместимости с официальным multi-intent evaluator. Session.pending/Turn.pending_topics — отдельный стек прерванных тем; возвращённая тема удаляется из стека, предыдущая добавляется. Полная история до 10 реплик передаётся LLM. Mock — изолированный детерминированный дубль на текстах каталога; он не претендует на смысловую точность и не входит в статистику LLM.

## Речь и измерения

WebRTC — только транскрипция, никаких автономных ответов speech-модели. Клиентский RMS VAD (порог 0.018, 3 voiced frames, пауза 360ms) отправляет input_audio_buffer.commit. Одна committed реплика на соединение исключает перестановку финальных событий между репликами. В шуме VAD нужно настраивать; лимит записи 30s, ожидания транскрипта 20s. File STT — запасной путь; аудиобайты реально отправляются API.

TTS выдаёт PCM по мере готовности; клиент планирует буферы Web Audio. Первое планирование + baseLatency/outputLatency — **оценка браузерного времени воспроизведения**, не аппаратный замер звука. `routing_ms` охватывает обе попытки и JSON validation; `policy_ms` — правила/шаблон; `server_ms` — время до сохранения; `stt_ms` для realtime — остаток после последнего voiced frame, включая VAD; для файла — HTTP STT. `tts_first_byte_ms` — запрос upstream → первые PCM bytes. `end_to_audio_ms` существует только при акустической отметке конца речи. Browser SpeechRecognition такой отметки не даёт, поэтому в этом пути E2E остаётся пустым. Клиентская телеметрия может быть неточной и не является доверенным benchmark.

Цели: routing ≤500ms, конец речи → начало ответа ≤1500ms. UI показывает превышения. Достижение целей не заявлено до измерения с настоящей моделью, RU/KK аудио, реалистичной сетью. Синтетический тест оценивает регрессии, не обобщающую точность.

## Деплой

Backend root /backend, frontend root /frontend: Dockerfile. Railway Postgres DATABASE_URL → backend; CORS_ORIGINS → HTTPS origin фронтенда. NEXT_PUBLIC_API_URL — HTTPS адрес backend **на этапе сборки** frontend. Данные входят в backend/data или подключаются томом. Один backend instance. Публичный production потребует авторизации и ограничения расходов; текущая версия — стенд синтетической симуляции. Deployed URL пока отсутствует.

Сохранённый ранний командный проект архитектуры: docs/design/earlier-spec-proposal.md. Реализованный путь использует OpenAI speech; geko/ElevenLabs из раннего проекта не реализованы (пользователь разрешил выбрать архитектуру).
