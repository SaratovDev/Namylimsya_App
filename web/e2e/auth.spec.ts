import { test, expect } from "@playwright/test";
import {
  registerViaUi,
  loginViaUi,
  registerUserViaApi,
  uniqueEmail,
  uniqueUsername,
} from "./helpers";



test.describe("auth", () => {
  test("register через UI → автоматический логин → видно username в шапке", async ({
    page,
  }) => {
    const user = {
      email: uniqueEmail("auth"),
      username: uniqueUsername("auth"),
      password: "secret1234",
    };

    await registerViaUi(page, user);

    // После register клиент делает login и редирект на "/".
    // В шапке — аватар с инициалом; username видно в dropdown
    // (не показываем в шапке как текст). Проверим через /settings —
    // это страница с профилем, где username отображается.
    await page.goto("/settings");
    await expect(page.getByText(user.username).first()).toBeVisible();

    // Плюс — /my-events открывается (значит, access-токен есть).
    await page.goto("/my-events");
    await expect(page.getByRole("heading", { name: "Мои события" })).toBeVisible();
  });

  test("login через UI для существующего юзера", async ({ page }) => {
    const user = await registerUserViaApi("login");

    await loginViaUi(page, user.email, user.password);

    // После логина — лента.
    await expect(page.getByRole("heading", { name: "Лента событий" })).toBeVisible();
  });

  test("неверный пароль — остаётся на /login, ошибка в toast", async ({
    page,
  }) => {
    const user = await registerUserViaApi("badpw");

    await page.goto("/login");
    await page.getByLabel("Email").fill(user.email);
    await page.getByLabel("Пароль").fill("wrong-password-123");
    await page.getByRole("button", { name: "Войти" }).click();

    // URL не меняется, появляется toast с ошибкой.
    // API возвращает message="invalid email or password" — AuthProvider
    // пробрасывает его в toast.error. Сейчас текст английский, ловим оба
    // варианта (и английский, и на случай будущей локализации).
    await expect(page).toHaveURL(/\/login/);
    await expect(
      page.getByText(/invalid email or password|Неверный email или пароль|Ошибка входа/i)
    ).toBeVisible({ timeout: 5_000 });
  });
});