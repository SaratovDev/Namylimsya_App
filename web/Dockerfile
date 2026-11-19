# ─── deps: установка зависимостей ─────────────────────────────────────
# Node 26 — соответствует локальному окружению (v26.13.0).
# Если Next 16.4 + Turbopack на 26 упадёт с внутренней ошибкой —
# переключитесь на node:24-alpine (тоже LTS, обкатаннее).
FROM node:26-alpine AS deps

# libc6-compat — на случай, если нативные бинарники (Tailwind/Turbopack
# или @tailwindcss/turbopack) ждут glibc-совместимости на Alpine.
RUN apk add --no-cache libc6-compat

WORKDIR /app

# pnpm ставим явно, не полагаясь на corepack: в Node 26 corepack
# может быть не включён по умолчанию.
RUN npm install -g pnpm@12.16.1

# Кэш-слой: только манифесты. При изменении кода этот слой переиспользуется.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

# ─── builder: next build ──────────────────────────────────────────────
FROM node:26-alpine AS builder

WORKDIR /app

RUN npm install -g pnpm@12.16.1

# NEXT_PUBLIC_* вкомпилируются в бандл на этапе build, а НЕ читаются
# в runtime. Поэтому их надо передавать как build args:
#   docker build --build-arg NEXT_PUBLIC_API_URL=/api/v1 ...
# Дефолты НЕ ставим: иначе next.config.ts assert никогда не сработает,
# и `docker build` без --build-arg молча соберёт бандл с localhost:8080.
ARG NEXT_PUBLIC_API_URL=
ARG NEXT_PUBLIC_WS_URL=
# ARG NEXT_PUBLIC_YANDEX_MAPS_API_KEY=""

ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_WS_URL=$NEXT_PUBLIC_WS_URL
# ENV NEXT_PUBLIC_YANDEX_MAPS_API_KEY=$NEXT_PUBLIC_YANDEX_MAPS_API_KEY
ENV NEXT_TELEMETRY_DISABLED=1

# Сначала node_modules из deps-стейджа (не тащим заново из контекста).
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# next build → .next/standalone, .next/static, public/
RUN pnpm build

# ─── runner: минимальный runtime ──────────────────────────────────────
FROM node:26-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
# HOSTNAME=0.0.0.0 — иначе server.js слушает 127.0.0.1 внутри контейнера,
# и проброс порта -p 3000:3000 снаружи не видит приложение.
ENV HOSTNAME=0.0.0.0

# Non-root пользователь.
RUN addgroup -S nodejs -g 1001 && \
    adduser -S nextjs -u 1001 -G nodejs

# public — статика (аватары, шрифты).
# Копируем от builder, т.к. в runner-стейдже исходников нет.
COPY --from=builder /app/public ./public

# .next/standalone — самодостаточный сервер: server.js + минимальный
# node_modules (Next кладёт только реально используемые пакеты).
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./

# .next/static — клиентские JS/CSS. Standalone их НЕ включает, копируем
# отдельно.
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000

# Healthcheck на корень. Next отвечает 200 на / даже для SSR-страниц.
# Node 26-alpine не содержит wget — используем встроенный fetch (Node 18+).
# Без внешних зависимостей.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD node -e "fetch('http://localhost:3000/').then(r => process.exit(r.ok ? 0 : 1)).catch(() => process.exit(1))"

CMD ["node", "server.js"]