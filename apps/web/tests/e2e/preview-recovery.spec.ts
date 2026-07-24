import { expect, test } from "@playwright/test";

test("LAN preview restores realtime, development login, and chat", async ({ page }) => {
  test.skip(
    process.env.E2E_PREVIEW_RECOVERY !== "true",
    "requires the configured development preview",
  );
  test.setTimeout(60_000);

  await page.goto("/");
  const composer = page.locator("#chat-message");
  await expect(composer).toBeEnabled({ timeout: 10_000 });
  await expect(page.getByText("Conectado", { exact: true })).toBeVisible();
  await expect(page.getByText("Load failed", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Reconnecting", { exact: true })).toHaveCount(0);

  await page.getByRole("button", { name: "Continuar con Google" }).click();
  const dialog = page.getByRole("dialog", { name: "Continuar con Google" });
  await dialog.getByRole("checkbox").nth(0).check();
  await dialog.getByRole("checkbox").nth(1).check();
  await dialog.getByRole("button", { name: "Continuar con Google" }).click();
  await page.getByRole("link", { name: "Approve" }).click();

  await expect(page).toHaveURL(/192\.168\.68\.210:3000/);
  await expect(composer).toBeEnabled({ timeout: 10_000 });
  const openConversations = page.getByRole("button", { name: "Abrir conversaciones" });
  if (await openConversations.isVisible()) {
    await openConversations.click();
  }
  await expect(page.getByText("Glazz E2E Administrator")).toBeVisible();

  const prompt = `Smoke de recuperación ${Date.now()}: responde solamente "conectado".`;
  await composer.fill(prompt);
  await page.getByRole("button", { name: /Enviar mensaje|Send message/ }).click();
  await expect(page.getByText(prompt)).toBeVisible();
  const response = page.locator(".message--assistant.message--complete").last();
  await expect(response).toBeVisible({ timeout: 45_000 });
  await expect(response).not.toContainText("Deterministic development response.");
});
