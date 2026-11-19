import { Page, APIRequestContext, request as pwRequest } from "@playwright/test";

// API endpoint — через Caddy (тот же origin, что web, если baseURL
// не переопределён). В CI может быть отдельный.
export const API_URL =
  process.env.E2E_API_URL ?? "https://localhost/api/v1";

// uniqueEmail — гарантированно уникальный email между прогонами и
// тестами. Формат "e2e+<prefix>+<ts>+<rand>@example.com" совместим с
// обычным email-форматом.
export function uniqueEmail(prefix = "e2e"): string {
  const ts = Date.now();
  const rnd = Math.random().toString(36).slice(2, 8);
  return `e2e+${prefix}+${ts}+${rnd}@example.com`;
}

// uniqueUsername — username тоже UNIQUE в схеме.
export function uniqueUsername(prefix = "u"): string {
  const ts = Date.now();
  const rnd = Math.random().toString(36).slice(2, 6);
  return `${prefix}_${ts}_${rnd}`;
}

export type TestUser = {
  email: string;
  username: string;
  password: string;
};

// newApiContext — Playwright request-context для API-вызовов.
// baseURL НЕ используем: в некоторых версиях @playwright/test
// request.newContext не применяет baseURL к относительным путям
// (post("/x") уходит на http://localhost:3000/x вместо API).
// Поэтому префиксуем API_URL вручную.
export async function newApiContext(): Promise<APIRequestContext> {
  return pwRequest.newContext({
    ignoreHTTPSErrors: true,
  });
}

// registerUserViaApi — быстрое создание юзера через API.
// Возвращает email/username/password, которые потом можно использовать
// для UI-логина.
export async function registerUserViaApi(
  prefix = "e2e"
): Promise<TestUser> {
  const email = uniqueEmail(prefix);
  const username = uniqueUsername(prefix);
  const password = "secret1234";

  const ctx = await newApiContext();
  try {
    const res = await ctx.post(`${API_URL}/auth/register`, {
      data: {
        email,
        password,
        username,
        city: "Саратов",
        consent_accepted: true,
        age_confirmed: true,
        consent_version: "1.0",
      },
    });
    if (!res.ok()) {
      const body = await res.text();
      throw new Error(`register failed: ${res.status()} ${body}`);
    }
  } finally {
    await ctx.dispose();
  }

  return { email, username, password };
}

// loginViaUi — заполняет форму логина и ждёт редирект на "/".
export async function loginViaUi(
  page: Page,
  email: string,
  password: string
): Promise<void> {
  await page.goto("/login");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Пароль").fill(password);
  await page.getByRole("button", { name: "Войти" }).click();
  await page.waitForURL("/");
}

// registerViaUi — заполняет форму регистрации. После успеха user
// автоматически залогинен (AuthProvider.login после register).
export async function registerViaUi(
  page: Page,
  user: TestUser
): Promise<void> {
  await page.goto("/register");
  await page.getByLabel("Email").fill(user.email);
  await page.getByLabel("Имя пользователя").fill(user.username);
  await page.getByLabel("Город").fill("Саратов");
  await page.getByLabel("Пароль").fill(user.password);

  await page.getByLabel(/18 лет/).check();
  await page.getByLabel(/политикой конфиденциальности/).check();

  await page.getByRole("button", { name: "Зарегистрироваться" }).click();
  await page.waitForURL("/");
}