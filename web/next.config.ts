import type { NextConfig } from "next";
import { PHASE_PRODUCTION_BUILD } from "next/constants";

// NEXT_PUBLIC_* инлайнятся в JS-бандл на этапе `next build`.
// Если они не заданы — Next молча подставит пустую строку, и в проде
// `fetch(`${API_BASE}${path}`)` резолвится в мусор (относительный путь
// без origin, file:///..., и т.п.) — это обнаруживается только руками.
// Поэтому: на фазе production-build (docker build / pnpm build) требуем
// явно задать оба URL. В dev (pnpm dev) — не трогаем, там fallback в
// lib/api.ts даёт http://localhost:8080/api/v1.
export default function config(phase: string): NextConfig {
  if (phase === PHASE_PRODUCTION_BUILD) {
    if (!process.env.NEXT_PUBLIC_API_URL) {
      throw new Error(
        "NEXT_PUBLIC_API_URL is required for `next build`. " +
          "Set it via env (e.g. `NEXT_PUBLIC_API_URL=/api/v1 pnpm build`) " +
          "or `web/.env.production.local`."
      );
    }
    if (!process.env.NEXT_PUBLIC_WS_URL) {
      throw new Error(
        "NEXT_PUBLIC_WS_URL is required for `next build`. " +
          "Set it via env (e.g. `NEXT_PUBLIC_WS_URL=ws://localhost/ws pnpm build`) " +
          "or `web/.env.production.local`."
      );
    }
  }

  return {
    /* config options here */
    output: "standalone",
    cacheComponents: false,
    partialPrefetching: false,
    turbopack: {
      rules: {
        "*.css": {
          loaders: ["@tailwindcss/turbopack"],
          as: "*.css",
        },
      },
    },
    async headers() {
      return [
        {
          source: "/:path*",
          headers: [
            { key: "X-Content-Type-Options", value: "nosniff" },
            { key: "X-Frame-Options", value: "DENY" },
            { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
            {
              key: "Permissions-Policy",
              value: "camera=(), microphone=(), geolocation=(self)",
            },
          ],
        },
      ];
    },
  };
}