# web — фронтенд «Намылимся»

Next.js 16 (App Router, standalone) + React 19 + Tailwind v4 + shadcn/ui. Общается с Go-бэкендом (`../api`) через REST и WebSocket.

## Быстрый старт (dev)

```bash
pnpm install
pnpm dev          # http://localhost:3000
```

Бэкенд должен быть поднят на `http://localhost:8080` (см. корневой `README.md`).

## Переменные окружения

Скопируйте `.env.example` → `.env.local` и заполните:

| Переменная | Пример (dev) | Обязательна? | Описание |
|---|---|---|---|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080/api/v1` | да, на сборку | Базовый URL API. В проде — `/api/v1` (через Caddy). **Вкомпилируется в бандл на `next build`** |
| `NEXT_PUBLIC_WS_URL` | `ws://localhost:8080/ws` | да, на сборку | WebSocket URL. В проде — `wss://<домен>/ws` |
| `NEXT_PUBLIC_YANDEX_MAPS_API_KEY` | — | нет | Ключ Яндекс.Карт (сейчас карта на OSM/Photon) |
| `NEXT_PUBLIC_EMAIL_VERIFICATION_ENABLED` | `true`/`false` | нет | Показывать ли баннер «подтвердите email». Должен совпадать с `EMAIL_VERIFICATION_ENABLED` на бэке |

Важно: `NEXT_PUBLIC_*` читаются **только на этапе сборки** и вкомпиливаются в бандл. В Docker — передавайте через `--build-arg` (см. `web/Dockerfile`); без них `next.config.ts` упадёт на сборке.

## Сборка и запуск в проде

```bash
# Standalone-сборка
NEXT_PUBLIC_API_URL=/api/v1 \
NEXT_PUBLIC_WS_URL=wss://localhost/ws \
pnpm build

pnpm start        # Node.js standalone-сервер на :3000
```

Либо через Docker (см. `../infra/docker-compose.prod.yml`):

```bash
docker build \
  --build-arg NEXT_PUBLIC_API_URL=/api/v1 \
  --build-arg NEXT_PUBLIC_WS_URL=wss://localhost/ws \
  --build-arg NEXT_PUBLIC_EMAIL_VERIFICATION_ENABLED=true \
  -f web/Dockerfile web/
```

## Тесты

```bash
pnpm lint                    # ESLint
pnpm exec tsc --noEmit       # типы
pnpm test:e2e                # Playwright (нужен поднятый prod-like стек на https://localhost)
```

## Структура

```
app/          роуты (App Router): (app) — основное приложение, (auth) — login/register, (public) — privacy/terms, admin — админка
components/   UI-компоненты (event-*, ui/ — shadcn-обёртки)
lib/          apiFetch, auth-context, хуки (use-chat, use-notifications), типы
e2e/          Playwright-тесты
```
