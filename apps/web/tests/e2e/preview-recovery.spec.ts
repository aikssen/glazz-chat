import { expect, test } from "@playwright/test";

test("LAN preview restores realtime and chat", async ({ page }) => {
  test.skip(
    process.env.E2E_PREVIEW_RECOVERY !== "true",
    "requires the configured development preview",
  );
  test.setTimeout(60_000);

  await page.goto("/");
  const composer = page.locator("#chat-message");
  await expect(composer).toBeEnabled({ timeout: 10_000 });
  await expect(page.locator(".connection")).toHaveAttribute("title", "Conectado");
  await expect(page.getByText("Load failed", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Reconnecting", { exact: true })).toHaveCount(0);

  if (process.env.E2E_PREVIEW_OAUTH === "true") {
    await page.getByRole("button", { name: /Abrir conversaciones|Open conversations/ }).click();
    await page
      .locator(".conversation-sidebar")
      .getByRole("button", { name: "Continuar con Google" })
      .click();
    const dialog = page.getByRole("dialog", { name: "Continuar con Google" });
    await dialog.getByRole("checkbox").nth(0).check();
    await dialog.getByRole("checkbox").nth(1).check();
    await dialog.getByRole("button", { name: "Continuar con Google" }).click();
    await page.getByRole("link", { name: "Approve" }).click();

    await expect(page).toHaveURL(/192\.168\.68\.210:3000/);
    await expect(composer).toBeEnabled({ timeout: 10_000 });
    await page.getByRole("button", { name: /Abrir conversaciones|Open conversations/ }).click();
    const sidebar = page.locator(".conversation-sidebar");
    await expect(sidebar.getByText("Glazz E2E Administrator")).toBeVisible();
    await sidebar.getByRole("button", { name: /Cerrar|Close/ }).click();
    await expect(sidebar).not.toHaveClass(/conversation-sidebar--open/);
    await expect(page.locator(".sidebar-backdrop")).toHaveCount(0);
  }

  const prompt = `Smoke de recuperación ${Date.now()}: responde solamente "conectado".`;
  await composer.fill(prompt);
  await page.getByRole("button", { name: /Enviar mensaje|Send message/ }).click();
  await expect(page.getByText(prompt)).toBeVisible();
  const response = page.locator(".message--assistant.message--complete").last();
  await expect(response).toBeVisible({ timeout: 45_000 });
  await expect(response).not.toContainText("Deterministic development response.");
  await cleanupSmokeConversations(page);
});

async function cleanupSmokeConversations(page: import("@playwright/test").Page) {
  const apiOrigin = new URL(process.env.E2E_API_URL ?? page.url());
  if (!process.env.E2E_API_URL) apiOrigin.port = "8080";
  await page.evaluate(async (origin) => {
    const csrf = document.cookie
      .split("; ")
      .find((value) => value.startsWith("glazz_csrf="))
      ?.slice("glazz_csrf=".length);
    if (!csrf) throw new Error("Authenticated CSRF cookie is missing");
    for (let attempt = 0; attempt < 20; attempt += 1) {
      const response = await fetch(`${origin}/api/v1/conversations?limit=100`, {
        credentials: "include",
      });
      if (!response.ok) throw new Error(`Conversation cleanup list failed: ${response.status}`);
      const conversationPage = (await response.json()) as {
        items: Array<{ id: string; title: string }>;
      };
      const smoke = conversationPage.items.filter((item) =>
        item.title.startsWith("Smoke de recuperación"),
      );
      if (!smoke.length) {
        await new Promise((resolve) => setTimeout(resolve, 250));
        continue;
      }
      for (const conversation of smoke) {
        const deletion = await fetch(`${origin}/api/v1/conversations/${conversation.id}`, {
          method: "DELETE",
          credentials: "include",
          headers: {
            "Idempotency-Key": `smoke-cleanup-${conversation.id}`,
            "X-CSRF-Token": decodeURIComponent(csrf),
          },
        });
        if (!deletion.ok) throw new Error(`Conversation cleanup failed: ${deletion.status}`);
      }
      return;
    }
    throw new Error("Smoke conversation title was not available for cleanup");
  }, apiOrigin.origin);
}
