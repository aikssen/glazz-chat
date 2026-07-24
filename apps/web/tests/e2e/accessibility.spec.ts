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

  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("checkbox").first()).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(dialog.getByRole("button", { name: "Cancelar" })).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(dialog.getByRole("checkbox").first()).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
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
  await context.setOffline(true);
  await expect(page.getByRole("status").filter({ hasText: "Sin conexión" })).toBeVisible();
  await context.setOffline(false);
});

function formatViolations(violations: Array<{ id: string; nodes: unknown[] }>) {
  return violations.map((violation) => `${violation.id} (${violation.nodes.length})`).join(", ");
}
