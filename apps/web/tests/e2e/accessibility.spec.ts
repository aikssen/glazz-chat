import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const wcagTags = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"];

test("guest and legal routes have no WCAG A/AA violations", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  for (const route of ["/", "/legal/terms", "/legal/privacy"]) {
    await page.goto(route);
    await expect(page.locator("main")).toBeVisible();
    const result = await new AxeBuilder({ page }).withTags(wcagTags).analyze();
    expect(result.violations, `${route}: ${formatViolations(result.violations)}`).toEqual([]);
  }
});

test("login dialog traps focus, closes with Escape, and restores focus", async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  await page.goto("/");
  await page.getByRole("button", { name: "Abrir conversaciones" }).click();
  const trigger = page
    .locator(".conversation-sidebar")
    .getByRole("button", { name: "Continuar con Google" });
  await trigger.click();

  const dialog = page.locator(".login-dialog");
  await expect(dialog.getByRole("checkbox").first()).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(dialog.getByRole("button", { name: "Cancelar" })).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(dialog.getByRole("checkbox").first()).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(trigger).toBeFocused();
});

test("skip link and mobile conversation sheet preserve keyboard orientation", async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  await page.goto("/");

  await page.keyboard.press("Tab");
  const skip = page.getByRole("link", { name: "Saltar a la conversación" });
  await expect(skip).toBeFocused();
  await skip.press("Enter");
  await expect(page.locator("#chat-transcript")).toBeFocused();

  const trigger = page.getByRole("button", { name: "Abrir conversaciones" });
  await trigger.click();
  const sheet = page.getByRole("dialog", { name: "Conversaciones" });
  await expect(sheet).toBeVisible();
  await expect(sheet.getByRole("link", { name: "Glazz" })).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(sheet.getByRole("button", { name: "Continuar con Google" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(sheet).toBeHidden();
  await expect(trigger).toBeFocused();
});

test("IME composition does not submit and untrusted Markdown stays inert", async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  await page.goto("/");
  const composer = page.getByLabel("Pregunta a Glazz");
  await composer.fill('<img src=x onerror="window.__glazzXss=true">');
  await composer.dispatchEvent("compositionstart");
  await composer.press("Enter");
  await expect(page.getByText("Deterministic development response.")).toHaveCount(0);
  await composer.dispatchEvent("compositionend");
  await composer.press("Enter");
  await expect(page.getByText("Deterministic development response.")).toBeVisible({
    timeout: 10_000,
  });
  await expect(page.locator(".message img")).toHaveCount(0);
  expect(await page.evaluate(() => (window as Window & { __glazzXss?: boolean }).__glazzXss)).toBe(
    undefined,
  );
});

test("200 percent zoom reflows primary controls without overflow", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "desktop-1024");
  await page.setViewportSize({ width: 512, height: 384 });
  await page.goto("/");
  await expect(page.getByLabel("Pregunta a Glazz")).toBeVisible();
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
    .toBe(true);
});

test("PWA exposes a visible offline state", async ({ context, page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375" || process.env.E2E_PWA !== "true");
  await page.goto("/");
  await expect
    .poll(() =>
      page.evaluate(async () => {
        if (!("serviceWorker" in navigator)) return false;
        const registration = await navigator.serviceWorker.ready;
        return Boolean(registration.active);
      }),
    )
    .toBe(true);
  const cachedRequests = await page.evaluate(async () => {
    const keys = await caches.keys();
    const requests = await Promise.all(keys.map(async (key) => (await caches.open(key)).keys()));
    return requests.flat().map((request) => request.url);
  });
  expect(cachedRequests).toContain(new URL("/", String(testInfo.project.use.baseURL)).href);
  expect(cachedRequests.some((url) => new URL(url).pathname.startsWith("/api/"))).toBe(false);
  await context.setOffline(true);
  await expect(page.getByRole("status").filter({ hasText: "Sin conexión" })).toBeVisible();
  await context.setOffline(false);

  await page.evaluate(() => navigator.serviceWorker.register("/sw.js?test-version=2"));
  await expect(page.getByRole("button", { name: "Actualizar Glazz" })).toBeVisible();
});

function formatViolations(violations: Array<{ id: string; nodes: unknown[] }>) {
  return violations.map((violation) => `${violation.id} (${violation.nodes.length})`).join(", ");
}
