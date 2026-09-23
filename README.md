# <Название проекта> — команда Plus · HackAlem AI

> Трек 07 · Образование · Кейс: <название кейса>
> Deployed: <https://...up.railway.app> · Демо-доступ: `<demo@plus.kz / demo1234>` (если есть авторизация)

## 1. Кратко
<Какую проблему решает, для кого. 2–3 предложения.>

## 2. Что реализовано
- <функция 1>
- <функция 2>
- <AI-агент: что делает>

## 3. Как работает (основной сценарий)
<Входные данные → шаги → результат.>

## 4. Технологии
- Backend: Go, chi, pgx, PostgreSQL
- Frontend: Next.js (App Router, TypeScript), Tailwind CSS, shadcn/ui, ObsidianUI
- AI: <модели и провайдеры: OpenAI / NVIDIA / ...>; mock-режим для проверки без ключей
- Деплой: Railway; локально — Docker Compose
- Сторонние компоненты: [THIRD_PARTY.md](./THIRD_PARTY.md)

## 5. Архитектура
```
[Next.js frontend] --REST/JSON--> [Go API] --> [PostgreSQL]
                                     |
                                     +--> [LLM: OpenAI | NVIDIA | OpenAI-compatible | mock]
```
<Компоненты и как взаимодействуют. Подробнее — docs/SPEC.md.>

## 6. Требования и зависимости
- Docker 24+ и Docker Compose v2 (рекомендуемый путь), **или**
- Go <версия>, Node.js 20+, PostgreSQL 15+ для ручного запуска
- API-ключи для проверки **не нужны**: по умолчанию `LLM_PROVIDER=mock`

## 7. Установка и запуск
```bash
git clone https://github.com/BAITC-Hacks/hack-9dd7b5f3-plus.git
cd hack-9dd7b5f3-plus
cp .env.example .env          # работает как есть в mock-режиме
docker compose up --build
```
- Frontend: http://localhost:3000
- API: http://localhost:8080/healthz

Без Docker:
```bash
cd backend && go mod download && go run ./cmd/api
cd frontend && npm ci && npm run dev
```

## 8. Переменные окружения
| Переменная | По умолчанию | Назначение |
|-----|---------|---------|
| `PORT` | 8080 | порт API |
| `DATABASE_URL` | Postgres из compose | подключение к БД |
| `LLM_PROVIDER` | `mock` | `mock` \| `openai` \| `nvidia` \| `openai_compatible` |
| `OPENAI_API_KEY` / `OPENAI_MODEL` | — | для `openai` |
| `NVIDIA_API_KEY` / `NVIDIA_MODEL` | — | для `nvidia` |
| `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` | — | для `openai_compatible` (любой OpenAI-совместимый API) |
| `CORS_ORIGINS` | http://localhost:3000 | разрешённые origin |
| `NEXT_PUBLIC_API_URL` | http://localhost:8080 | адрес API для фронтенда |

## 9. Как проверить решение
1. Открыть http://localhost:3000 (или deployed-версию).
2. <шаг>
3. <шаг>
4. Ожидаемый результат: <что увидит эксперт>

## 10. Данные и интеграции
- <источники данных: синтетические seed-данные / открытые датасеты / данные кейса>
- <внешние API и сервисы>

## 11. Ограничения
- <что не реализовано / известные ограничения>

## 12. Команда Plus
- Tair Kaldybayev — <роль>
- Alikhan — <роль>
- Ramazan — <роль>
