# Намылимся

Площадка для организации живых офлайн-встреч. Go API + Next.js 16 + PostgreSQL/PostGIS + Redis. Аудитория — РФ, 18+, 152-ФЗ (согласие на обработку ПДн при регистрации).

## Стек

- **Бэкенд:** Go 1.27, chi, pgx/v5, Redis, WebSocket (gorilla), JWT, bcrypt, sqlc + goose
- **Фронтенд:** Next.js 16 (App Router, standalone), React 19, Tailwind v4, shadcn/ui
- **Инфра:** PostgreSQL 16 + PostGIS 3.4, Redis 7, Caddy 2 (TLS через local_certs для dev), Mailpit (SMTP-эмулятор для dev)

## Быстрый старт (dev)

```bash
# 1. Поднять Postgres/Redis/Mailpit
docker compose -f infra/docker-compose.yml up -d

# 2. Применить миграции
cd api
go install github.com/pressly/goose/v3/cmd/goose@latest
export DATABASE_URL="postgres://namylimsya:devpass@localhost:5432/namylimsya?sslmode=disable"
goose -dir internal/db/migrations postgres "$DATABASE_URL" up

# 3. Сгенерировать код из SQL-запросов (если меняли *.sql)
sqlc generate

# 4. Запустить API (терминал 1)
go run ./cmd/server

# 5. Запустить фронт (терминал 2)
cd ../web
pnpm install
pnpm dev
```

Открыть http://localhost:3000. Mailpit: http://localhost:8025. Health: `curl http://localhost:8080/api/v1/health`.

`.env` в корне репо — см. `.env.example`.

## Production-like стек (локально)

Полный стек «как в проде»: Caddy (TLS self-signed) + api + web + Postgres + Redis + Mailpit.

```bash
# 1. Скопировать и заполнить .env.production
cp .env.production.example .env.production
# обязательно: POSTGRES_PASSWORD, JWT_ACCESS_SECRET (32+ символов),
# JWT_REFRESH_SECRET (32+ символов, ОТЛИЧАЕТСЯ от access)

# 2. Собрать образы
docker build -t namylimsya-api:dev -f api/Dockerfile api/

MSYS_NO_PATHCONV=1 docker build -t namylimsya-web:prod-local \
  --build-arg NEXT_PUBLIC_API_URL=/api/v1 \
  --build-arg NEXT_PUBLIC_WS_URL=wss://localhost/ws \
  -f web/Dockerfile web/

# 3. Поднять стек
docker compose -f infra/docker-compose.prod.yml --env-file .env.production up -d

# 4. Открыть https://localhost (принять self-signed сертификат)
```

**Важно:** `NEXT_PUBLIC_API_URL` и `NEXT_PUBLIC_WS_URL` **вкомпилируются в бандл на этапе `next build`**. Без явных `--build-arg` assert в `next.config.ts` **упадёт** — это защита от случайной сборки с `localhost:8080` в проде.

## Структура

```
api/                       Go-бэкенд
  cmd/server/              main.go
  cmd/clean-events/        чистка events (dev-утилита)
  internal/
    auth/                  JWT, bcrypt, refresh-сессии (Redis)
    categories/            канонический список 12 категорий
    config/                Config + Load() + prod-валидация
    db/
      migrations/          goose-миграции (SQL)
      queries/             sqlc-запросы (*.sql)
      dbgen/               сгенерированный код (sqlc)
    email/                 SMTP-отправка (с таймаутом и заголовочной санитизацией)
    handlers/              HTTP-хендлеры (chi), WS-хендлеры
    httpx/                 JSON-ответы, InternalError (без утечки err.Error в проде)
    legal/                 ConsentVersion, MinAge
    middleware/            AuthRequired, OptionalAuth, RequireRole, IPRateLimit, ScrubLogFormatter
    ratelimit/             Redis sliding window
    router/                chi-роутер + SafeStaticHandler
    services/              бизнес-логика (events, applications, chat, notifications, admin, support)
    storage/               FileStorage (local; s3 — TODO)
    ws/                    WebSocket hub + client + notification hub
  sqlc.yaml

web/                       Next.js 16 фронтенд
  app/
    (app)/                 основные страницы (лента, мои события, настройки, ...)
    (auth)/                login, register
    (public)/              privacy, terms
    admin/                 админка (только admin/moderator)
  components/              UI-компоненты
  lib/                     apiFetch, auth-context, hooks (use-chat, use-notifications)
  public/                  статика

infra/
  Caddyfile                reverse-proxy (TLS local_certs)
  docker-compose.yml       dev (Postgres, Redis, Mailpit)
  docker-compose.prod.yml  prod-like (всё + Caddy)
```

## Переменные окружения

### Обязательные (без них `config.Load()` упадёт)

| Переменная | Пример | Описание |
|---|---|---|
| `APP_ENV` | `development` \| `production` | Влияет на валидацию JWT, HTTPS-требование `PUBLIC_BASE_URL`, маскировку `err.Error()` в 500 |
| `DATABASE_URL` | `postgres://user:pass@host:5432/db?sslmode=disable` | Postgres DSN, должен быть с PostGIS |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis DSN |
| `JWT_ACCESS_SECRET` | 64 hex-символа | В проде: ≥32 символа, ≠ refresh |
| `JWT_REFRESH_SECRET` | 64 hex-символа | В проде: ≥32 символа, ≠ access |

### С дефолтами (можно не задавать в dev)

| Переменная | Дефолт | Описание |
|---|---|---|
| `HTTP_PORT` | `8080` | Порт API |
| `LOG_LEVEL` | `debug` (dev) / `info` (prod) | Уровень zerolog: `debug`\|`info`\|`warn`\|`error`. В проде по умолчанию `info` — SQL-трейсер и debug-строки не пишутся |
| `PUBLIC_BASE_URL` | `http://localhost:3000` | CORS-origin, ссылки в письмах, WS `CheckOrigin`. В проде — `https://` |
| `JWT_ACCESS_TTL` | `15m` | |
| `JWT_REFRESH_TTL` | `720h` | 30 дней |
| `STORAGE_DRIVER` | `local` | `local` \| `s3` (s3 — TODO) |
| `STORAGE_LOCAL_PATH` | `./storage` | Куда пишутся аватары |
| `STORAGE_PUBLIC_BASE_URL` | `http://localhost:<HTTP_PORT>` | Базовый URL для `/static/*` |
| `SMTP_ENABLED` | `false` | Если `false` — все письма отключены. **В проде warn, если `false`** |
| `SMTP_HOST` | `localhost` | Mailpit: `localhost` |
| `SMTP_PORT` | `1025` | |
| `EMAIL_VERIFICATION_ENABLED` | `false` | Верификация email (пока flow не реализован) |
| `YANDEX_MAPS_API_KEY` | — | Если пусто, карта показывает fallback-блок |

### `NEXT_PUBLIC_*` (вкомпилируются на build)

| Переменная | Пример | Описание |
|---|---|---|
| `NEXT_PUBLIC_API_URL` | `/api/v1` (prod) \| `http://localhost:8080/api/v1` (dev) | Базовый URL API. **Обязателен на `next build`** |
| `NEXT_PUBLIC_WS_URL` | `wss://localhost/ws` (prod) \| `ws://localhost:8080/ws` (dev) | WS URL. **Обязателен** |
| `NEXT_PUBLIC_YANDEX_MAPS_API_KEY` | — | Публичный ключ Яндекс.Карт |

### Прод-специфичное

При `APP_ENV=production` `config.Load()`:
- **fail**, если `JWT_ACCESS_SECRET`/`JWT_REFRESH_SECRET` <32 символов, дефолтные, или совпадают;
- **fail**, если `PUBLIC_BASE_URL` не начинается с `https://`;
- **warn**, если `SMTP_ENABLED=false` (письма молча не уходят).

## Безопасность

- **Avatar XSS:** сервер снифает `http.DetectContentType` + allowlist (jpeg/png/webp/gif), добавляет расширение, отдаёт через `SafeStaticHandler` с `nosniff` + CSP. `PATCH /users/me` валидирует `avatar_url` (только наш префикс).
- **Hidden events:** `GetByID` и `CanAccessChat` возвращают 404/403 для скрытых событий всем, кроме орга/админа/модератора. `Apply` отвергает скрытые.
- **Approve/Reject/Cancel:** advisory locks + `allowed_from` в SQL guard от гонки.
- **WS:** бан-чек на handshake, `upgrade-then-close` с кодами 4400/4401/4403 (клиент делает refresh / denied вместо 10 retry). `?token=` скраббится в логах (`ScrubLogFormatter`), access log Caddy для `/ws/*` отключён.
- **Email:** `SanitizeHeader` (CRLF) + `html.EscapeString` для event_title/ответа поддержки.
- **Rate-limit:** `/auth/*` (10/мин/IP), `/support/tickets` (5/10мин/IP), `/complaints` (10/10мин/IP), WS-чат (5 сообщений/10сек/юзер).
- **JWT:** bcrypt-14 (в тестах — 4 для скорости), refresh-rotation с jti в Redis, revoke-all при бане.
- **Error masking:** `httpx.InternalError` — `err.Error()` только при `APP_ENV=development`.

## Тесты

```bash
cd api

# Юнит-тесты (быстрые)
go test ./internal/categories/ ./internal/ws/ ./internal/email/ ./internal/services/ -count=1

# Интеграционные (нужен dev-стек: Postgres + Redis)
INTEGRATION=1 go test ./internal/integration/ -count=1

# Полная регрессия
INTEGRATION=1 go test ./... -count=1
```

## Эксплуатация

### Создание администратора

Первый админ назначается через утилиту `create-admin` (собрана в образ API как `/app/create-admin`).

```bash
# Локально (из каталога api, нужен .env с DATABASE_URL):
go run ./cmd/create-admin -email admin@example.com -username admin

# В Docker (стек namylimsya-prod запущен):
docker compose -f infra/docker-compose.prod.yml --env-file .env.production \
  exec api /app/create-admin -email admin@example.com -username admin
```

Пароль читается из env `ADMIN_PASSWORD`; если не задан — генерируется случайный
и выводится в stdout. Если пользователь с таким email уже есть — команда обновит
его пароль и повысит роль до `admin`.

### Бэкапы и восстановление

```powershell
# Дамп PostgreSQL → infra/backups/namylimsya-<timestamp>.sql.gz
# Хранит последние 14 дампов (меняется флагом -Keep).
.\infra\backup-db.ps1
.\infra\backup-db.ps1 -Keep 30
```

Расписание (Windows Task Scheduler / cron) не настроено — добавьте задачу,
когда определитесь с политикой хранения и внешним хранилищем.

**Восстановление** (полная замена данных текущей БД):

```bash
# 1. Остановить API, чтобы не писать в БД во время restore
docker compose -f infra/docker-compose.prod.yml stop api

# 2. Распаковать и залить дамп в контейнер postgres
gunzip -c infra/backups/namylimsya-YYYYMMDD-HHmmss.sql.gz | \
  docker exec -i namylimsya-prod-postgres-1 \
  psql -U namylimsya -d namylimsya

# 3. Поднять API обратно
docker compose -f infra/docker-compose.prod.yml start api
```

`backup-db.ps1` использует `pg_dump` внутри контейнера Postgres (его собственные
`POSTGRES_USER`/`POSTGRES_DB`/`POSTGRES_PASSWORD`), поэтому имя контейнера
должно совпадать с `namylimsya-prod-postgres-1` (или передайте `-ComposeProject`).

### Домен и TLS

Локально Caddy подписывает самоподписанный сертификат (`local_certs`) для
`localhost`. Для публичного запуска:

1. В `infra/Caddyfile` уберите `local_certs`.
2. В `.env.production` задайте `DOMAIN=namylimsya.ru`,
   `PUBLIC_BASE_URL=https://namylimsya.ru`, `STORAGE_PUBLIC_BASE_URL=https://namylimsya.ru`.
3. Пересоздайте стек — Caddy получит Let's Encrypt сертификат автоматически.

## Что НЕ реализовано (Phase 3+)

- S3-хранилище (только local).
- Email verification (flow регистрации/подтверждения).
- Password reset.
- Archive (`GET /events/archive` готов на бэке, но страница `/archive` — stub).
- `/profile`, `/support` — stub-страницы.
- Удаление аккаунта / экспорт данных (152-ФЗ).
- WS re-auth / eviction на бан (сейчас бан блокирует только новое соединение).
- Rate-limit на `/complaints` уже есть, но UI-формы для создания жалоб нет.
- **Заполнить `web/lib/legal.ts`** реальными реквизитами оператора (сейчас плейсхолдеры `[ФАМИЛИЯ]`, `[ИНН]`; на `/privacy`, `/terms` показывается красный баннер — не для публикации).