import { defineConfig, devices } from "@playwright/test";

// E2E-смоук против prod-like стека (Caddy + TLS + api + web).
// Запускается через `pnpm test:e2e`. По умолчанию бьёт в https://localhost.
// Для другого окружения — E2E_BASE_URL=https://staging.example.com pnpm test:e2e.
//
// Все тесты изолированы: каждый регистрирует своих юзеров через uniqueEmail().
// Параллельность выключена (workers: 1), потому что:
//   - тесты делят один бэкенд и одну БД;
//   - rate-limit /auth/* — 10/мин на IP, при параллельности Caddy-source
//     IP общий → быстрый флак.
// Для локальной отладки: pnpm test:e2e --headed или --ui.
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: process.env.CI ? "github" : "list",
  timeout: 30_000,
  expect: {
    timeout: 10_000,
  },
  use: {
    // В проде через Caddy с local_certs — self-signed.
    baseURL: process.env.E2E_BASE_URL ?? "https://localhost",
    ignoreHTTPSErrors: true,
    trace: "retain-on-failure",
    video: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});