# ─── Builder ──────────────────────────────────────────────────────────
# go.mod требует go 1.xx. Если у вас другая версия — поменяйте тег ниже.
# Alpine выбран, чтобы финальный образ был ~30MB (glibc не нужен,
# CGO_ENABLED=0 → полностью статический бинарь).
FROM golang:1.27-alpine AS builder

# Разрешаем Go автоматически скачать нужный toolchain, если базовый образ
# отстаёт от go.mod (например, образ 1.27.0, а go.mod требует 1.27.3).
ENV GOTOOLCHAIN=auto

# git — на случай, если go mod download захочет VCS (не должно, но пусть).
# ca-certificates — пригодятся в runtime для HTTPS-вызовов (Yandex Maps,
# SMTP через TLS и т.д.).
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Зеркала Go-модулей: proxy.golang.org из РФ часто недоступен / медленный.
# goproxy.io и goproxy.cn — быстрые зеркала, direct — fallback на GitHub.
ENV GOPROXY=https://goproxy.io,https://goproxy.cn,direct

# Кэшируем зависимости: сначала go.mod/go.sum, потом код.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Весь остальной код.
COPY . .

# Статический бинарь. -trimpath убирает локальные пути из бинаря,
# -ldflags="-s -w" срезает debug-инфо (образ меньше на ~30%).
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/api \
    ./cmd/server

# Утилита bootstrap первого админа (см. README → «Создание администратора»).
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/create-admin \
    ./cmd/create-admin

# ─── Goose (migrator) ────────────────────────────────────────────────
# Пинуем версию goose: @latest каждый раз тянет с proxy.golang.org на
# проверку обновлений и качает свежие транзитивные зависимости
# (prometheus, genproto, exp) — из РФ это 3-5 минут зависания.
# v3.24.1 — последняя стабильная на ноябрь 2026, работает с нашими миграциями.
RUN GOBIN=/out go install github.com/pressly/goose/v3/cmd/goose@v3.27.3

# ─── Runtime ──────────────────────────────────────────────────────────
# Alpine минималистичен. ca-certificates нужны для HTTPS.
# tzdata — на случай, если TZ=Europe/Moscow пригодится.
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S -G app app

WORKDIR /app

WORKDIR /app

# Бинарники — от builder.
COPY --from=builder /out/api /app/api
COPY --from=builder /out/create-admin /app/create-admin
COPY --from=builder /out/goose /app/goose

# Миграции — нужны goose в runtime (для сервиса migrate).
COPY --from=builder /src/internal/db/migrations /app/migrations

# Storage: директория для загруженных файлов (аватары).
# Создаём заранее с владельцем app, иначе non-root пользователь не сможет
# писать туда (WORKDIR /app принадлежит root). В production сюда монтируется
# volume — см. docker-compose.prod.yml.
RUN mkdir -p /app/storage && chown -R app:app /app/storage

# Non-root. Не root ради безопасности: если процесс скомпрометируют,
# он не сможет писать в системные директории.
USER app

EXPOSE 8080

# Healthcheck — curl не нужен: используем встроенный wget из busybox.
# Это проверка liveness на уровне контейнера (оркестратор рестартит при падении).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/v1/health || exit 1

# По умолчанию — API-сервер. Миграции запускаются отдельной командой
# (см. docker-compose.prod.yml → migrate service).
ENTRYPOINT ["/app/api"]