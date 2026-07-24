import { expect, test } from "@playwright/test";

test("streams a response from the configured live provider", async ({ page }) => {
  test.skip(process.env.E2E_LIVE_PROVIDER !== "true", "requires explicit live-provider opt-in");
  test.setTimeout(60_000);

  await page.goto("/");
  const composer = page.getByLabel("Pregunta a Glazz");
  const send = page.getByRole("button", { name: "Enviar mensaje" });
  await composer.fill("Responde en español con una sola oración: ¿qué es WebSocket?");
  await expect(send).toBeEnabled({ timeout: 10_000 });
  await send.click();

  const response = page.locator(".message--assistant.message--complete").last();
  await expect(response).toBeVisible({ timeout: 45_000 });
  const content = (await response.textContent())?.trim() ?? "";

  expect(content).not.toBe("");
  expect(content).not.toContain("Deterministic development response.");
});
