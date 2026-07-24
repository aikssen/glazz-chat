import { expect, test } from "@playwright/test";

test("renders the responsive chat without overflow", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { name: "¿Qué quieres explorar?" })).toBeVisible();
  await expect(page.getByLabel("Pregunta a Glazz")).toBeVisible();
  await expect(page.getByRole("button", { name: "Enviar mensaje" })).toBeVisible();
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
    .toBe(true);
});

test("guest sends a message and receives a streamed response", async ({ page }) => {
  await page.goto("/");
  const composer = page.getByLabel("Pregunta a Glazz");
  const send = page.getByRole("button", { name: "Enviar mensaje" });
  await composer.fill("Resume por qué el cielo es azul.");
  await expect(send).toBeEnabled({ timeout: 10_000 });
  await send.click();

  await expect(page.getByText("Resume por qué el cielo es azul.")).toBeVisible();
  await expect(page.getByText("Deterministic development response.")).toBeVisible({
    timeout: 10_000,
  });
  await expect(
    page.getByRole("paragraph").filter({ hasText: "3 mensajes gratis disponibles" }),
  ).toBeVisible();
});

test("guest limit becomes a focused login gate", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  await page.goto("/");
  const composer = page.getByLabel("Pregunta a Glazz");
  for (let index = 1; index <= 4; index += 1) {
    await composer.fill(`Pregunta gratuita ${index}`);
    await page.getByRole("button", { name: "Enviar mensaje" }).click();
    await expect(page.getByText("Deterministic development response.")).toHaveCount(index, {
      timeout: 10_000,
    });
  }
  await expect(page.getByText("Continúa tu conversación")).toBeVisible();
  await expect(
    page.locator(".guest-gate").getByRole("button", { name: "Continuar con Google" }),
  ).toBeVisible();
  await expect(composer).not.toBeVisible();
});
