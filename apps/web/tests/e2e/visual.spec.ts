import { expect, test, type Page } from "@playwright/test";

test.describe("visual regression matrix", () => {
  test("empty chat in Spanish and light theme", async ({ page }) => {
    await setPreferences(page, "es", "light");
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "¿Qué quieres explorar?" })).toBeVisible();
    await expect(page.locator(".connection")).toHaveAttribute("title", "Conectado");

    await expect(page).toHaveScreenshot("empty-es-light.png", screenshotOptions);
  });

  test("completed chat in English and dark theme", async ({ page }) => {
    await setPreferences(page, "en", "dark");
    await page.goto("/");
    const composer = page.getByLabel("Ask Glazz");
    await expect(composer).toBeEnabled();
    await composer.fill("Explain visual stability in one concise paragraph.");
    await page.getByRole("button", { name: "Send message" }).click();
    await expect(page.getByText("Deterministic development response.")).toBeVisible();

    await expect(page).toHaveScreenshot("complete-en-dark.png", screenshotOptions);
  });

  test("startup error in Spanish and dark theme", async ({ page }) => {
    await setPreferences(page, "es", "dark");
    await page.route("**/api/v1/models", (route) =>
      route.fulfill({
        status: 503,
        contentType: "application/problem+json",
        body: JSON.stringify({
          error: {
            code: "service_unavailable",
            message: "Fallo controlado para regresión visual.",
            requestId: "req_visual_failure",
          },
        }),
      }),
    );
    await page.goto("/");
    await expect(page.locator(".global-error")).toContainText(
      "Fallo controlado para regresión visual.",
    );

    await expect(page).toHaveScreenshot("error-es-dark.png", screenshotOptions);
  });

  test("guest quota gate in English and light theme", async ({ page }) => {
    await setPreferences(page, "en", "light");
    await page.goto("/");
    const composer = page.getByLabel("Ask Glazz");
    for (let prompt = 1; prompt <= 4; prompt += 1) {
      await composer.fill(`Visual quota prompt ${prompt}`);
      await page.getByRole("button", { name: "Send message" }).click();
      await expect(page.getByText("Deterministic development response.")).toHaveCount(prompt);
    }
    await expect(page.getByText("Continue your conversation")).toBeVisible();
    await expect(composer).not.toBeVisible();

    await expect(page).toHaveScreenshot("quota-en-light.png", screenshotOptions);
  });

  test("administration in Spanish and light theme", async ({ page }) => {
    await setPreferences(page, "es", "light");
    await page.route("**/api/v1/admin/models", (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          items: [
            {
              id: "00000000-0000-7000-8000-000000000101",
              name: "DeepSeek V4 Flash",
              description: "Default development chat model.",
              capabilities: { chatCompletions: true, markdown: true, code: true },
              enabled: true,
              available: true,
              supported: true,
              audience: ["guest", "user"],
              defaultFor: ["guest", "user"],
              order: 10,
              version: 1,
            },
          ],
        }),
      }),
    );
    await page.goto("/admin");
    await expect(page.getByRole("heading", { name: "Administración" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Catálogo de modelos" })).toBeVisible();

    await expect(page).toHaveScreenshot("admin-es-light.png", screenshotOptions);
  });
});

const screenshotOptions = {
  animations: "disabled" as const,
  caret: "hide" as const,
  fullPage: true,
  maxDiffPixelRatio: 0.001,
  scale: "css" as const,
};

async function setPreferences(page: Page, locale: "es" | "en", theme: "light" | "dark") {
  await page.addInitScript(
    ({ selectedLocale, selectedTheme }) => {
      localStorage.setItem("glazz-locale", selectedLocale);
      localStorage.setItem("glazz-theme", selectedTheme);
    },
    { selectedLocale: locale, selectedTheme: theme },
  );
}
