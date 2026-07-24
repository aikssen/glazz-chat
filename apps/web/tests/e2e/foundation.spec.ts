import { expect, test } from "@playwright/test";

test("renders the responsive chat without overflow", async ({ page }, testInfo) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { name: "¿Qué quieres explorar?" })).toBeVisible();
  await expect(page.getByLabel("Pregunta a Glazz")).toBeVisible();
  await expect(page.getByRole("button", { name: "Enviar mensaje" })).toBeVisible();
  await expect(page.locator(".connection")).toHaveAttribute("title", "Conectado");
  await expect(page.locator(".global-error")).toHaveCount(0);
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
    .toBe(true);
  const regions = await page.locator(".chat-main").evaluate((main) => {
    const box = (selector: string) => {
      const bounds = main.querySelector(selector)?.getBoundingClientRect();
      return bounds ? { top: bounds.top, bottom: bounds.bottom } : null;
    };
    return {
      topbar: box(".chat-topbar"),
      transcript: box(".chat-scroll"),
      footer: box(".chat-footer"),
    };
  });
  expect(regions.topbar?.bottom).toBeLessThanOrEqual(regions.transcript?.top ?? 0);
  expect(regions.transcript?.bottom).toBeLessThanOrEqual(regions.footer?.top ?? 0);
  await testInfo.attach("responsive-chat", {
    body: await page.screenshot({ animations: "disabled" }),
    contentType: "image/png",
  });
});

test("saved and system themes apply without a hydration flash", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  await page.emulateMedia({ colorScheme: "light" });
  await page.addInitScript(() => {
    if (sessionStorage.getItem("glazz-theme-test-ready")) return;
    localStorage.setItem("glazz-theme", "dark");
    sessionStorage.setItem("glazz-theme-test-ready", "true");
  });
  await page.goto("/");
  await expect
    .poll(() => page.locator("html").evaluate((node) => node.classList.contains("dark")))
    .toBe(true);
  await expect
    .poll(() => page.locator("html").evaluate((node) => node.style.colorScheme))
    .toBe("dark");

  await page.evaluate(() => localStorage.setItem("glazz-theme", "system"));
  await page.emulateMedia({ colorScheme: "dark" });
  await page.reload();
  await expect
    .poll(() => page.locator("html").evaluate((node) => node.classList.contains("dark")))
    .toBe(true);
  await expect(page.locator("html")).toHaveAttribute("data-theme-ready", "true");
  await page.emulateMedia({ colorScheme: "light" });
  await expect
    .poll(() => page.locator("html").evaluate((node) => node.classList.contains("dark")))
    .toBe(false);
});

test("composer remains visible when the mobile viewport height changes", async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  await page.goto("/");
  await page.setViewportSize({ width: 375, height: 500 });
  const composer = page.getByLabel("Pregunta a Glazz");
  await expect(composer).toBeVisible();
  await expect
    .poll(async () => {
      const box = await composer.boundingBox();
      return box ? box.y + box.height <= 500 : false;
    })
    .toBe(true);
});

test("recovers from cookies signed by a previous deployment", async ({
  context,
  page,
}, testInfo) => {
  const domain = new URL(String(testInfo.project.use.baseURL)).hostname;
  await context.addCookies([
    {
      name: "glazz_access",
      value: "stale-access",
      domain,
      path: "/",
      httpOnly: true,
    },
    {
      name: "glazz_refresh",
      value: "stale-refresh",
      domain,
      path: "/",
      httpOnly: true,
    },
    {
      name: "glazz_csrf",
      value: "stale-csrf",
      domain,
      path: "/",
    },
  ]);

  await page.goto("/");

  await expect(page.getByLabel("Pregunta a Glazz")).toBeEnabled({ timeout: 10_000 });
});

test("unsupported browser locale falls back to English", async ({ browser }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  const context = await browser.newContext({
    baseURL: String(testInfo.project.use.baseURL),
    locale: "fr-FR",
  });
  const page = await context.newPage();
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "What do you want to explore?" })).toBeVisible();
  await context.close();
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

test("Google approval covers the registered-user lifecycle", async ({
  browser,
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375" || process.env.E2E_OAUTH !== "true");
  let callbackURL = "";
  page.on("request", (request) => {
    if (request.url().includes("/api/v1/auth/google/callback?")) callbackURL = request.url();
  });

  await page.goto("/");
  const composer = page.getByLabel("Pregunta a Glazz");
  await composer.fill("Conserva esta conversación después del login.");
  await page.getByRole("button", { name: "Enviar mensaje" }).click();
  await expect(page.getByText("Deterministic development response.")).toBeVisible();

  await openLoginDialog(page);
  await page.getByRole("link", { name: "Approve" }).click();

  await page.getByRole("button", { name: "Abrir conversaciones" }).click();
  await expect(page.getByText("Glazz E2E Administrator")).toBeVisible();
  await expect(
    page.getByRole("paragraph").filter({
      hasText: "Conserva esta conversación después del login.",
    }),
  ).toBeVisible();
  await expect.poll(() => new URL(page.url()).searchParams.has("conversation")).toBe(true);
  await page.reload();
  await expect(
    page.getByRole("paragraph").filter({
      hasText: "Conserva esta conversación después del login.",
    }),
  ).toBeVisible();

  const apiOrigin = new URL(process.env.E2E_API_URL ?? page.url());
  if (!process.env.E2E_API_URL) apiOrigin.port = "8080";
  const migrated = await page.evaluate(async (origin) => {
    const response = await fetch(`${origin}/api/v1/conversations?limit=100`, {
      credentials: "include",
    });
    return (await response.json()) as { items: unknown[] };
  }, apiOrigin.origin);
  expect(migrated.items).toHaveLength(1);
  expect(callbackURL).toContain("code=glazz-e2e-approved");
  const replay = await page.request.get(callbackURL);
  expect(replay.status()).toBe(400);

  await page.getByRole("button", { name: "Abrir conversaciones" }).click();
  let conversation = page.locator(".conversation-item").first();
  await conversation.locator("summary").click();
  page.once("dialog", (dialog) => dialog.accept("Conversación E2E"));
  await conversation.getByRole("button", { name: "Renombrar" }).click();
  await expect(conversation).toContainText("Conversación E2E");

  await conversation.getByRole("button", { name: "Archivar" }).click();
  conversation = page.locator(".conversation-item").filter({ hasText: "Conversación E2E" });
  await expect(page.getByRole("heading", { name: "Archivadas" })).toBeVisible();
  await conversation.locator("summary").click();
  await conversation.getByRole("button", { name: "Restaurar" }).click();
  await expect(page.getByRole("heading", { name: "Archivadas" })).toHaveCount(0);

  const secondContext = await browser.newContext({
    baseURL: String(testInfo.project.use.baseURL),
    locale: "es-CO",
  });
  const secondPage = await secondContext.newPage();
  await secondPage.goto("/");
  await openLoginDialog(secondPage);
  await secondPage.getByRole("link", { name: "Approve" }).click();
  await secondPage.getByRole("button", { name: "Abrir conversaciones" }).click();
  await expect(secondPage.getByText("Glazz E2E Administrator")).toBeVisible();

  await page.goto("/settings");
  await expect(page.getByRole("heading", { name: "Ajustes" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Sesiones activas" })).toBeVisible();
  await expect(page.getByText(/Actual/)).toBeVisible();
  const sessionRows = page.locator(".session-row");
  await expect(sessionRows).toHaveCount(2);
  const sessionLabels = await sessionRows.allTextContents();
  const otherSession = sessionLabels.findIndex((label) => !label.includes("Actual"));
  expect(otherSession).toBeGreaterThanOrEqual(0);
  await sessionRows.nth(otherSession).getByRole("button", { name: "Revocar sesión" }).click();
  await expect(sessionRows).toHaveCount(1);
  await secondPage.goto("/settings");
  await expect(secondPage.getByText("Inicia sesión para abrir los ajustes.")).toBeVisible();
  await secondContext.close();

  await page.getByRole("button", { name: "English" }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();

  const thirdContext = await browser.newContext({
    baseURL: String(testInfo.project.use.baseURL),
    locale: "es-CO",
  });
  const thirdPage = await thirdContext.newPage();
  await thirdPage.goto("/");
  await openLoginDialog(thirdPage);
  await thirdPage.getByRole("link", { name: "Approve" }).click();
  await expect(thirdPage.getByRole("button", { name: "Open conversations" })).toBeVisible();
  await thirdPage.goto("/settings");
  await expect(thirdPage.getByRole("heading", { name: "Settings" })).toBeVisible();

  await page.getByRole("button", { name: "Español" }).click();
  await thirdPage.reload();
  await expect(thirdPage.getByRole("heading", { name: "Ajustes" })).toBeVisible();
  const thirdSessionRows = thirdPage.locator(".session-row");
  await expect(thirdSessionRows).toHaveCount(2);
  const thirdSessionLabels = await thirdSessionRows.allTextContents();
  const currentThirdSession = thirdSessionLabels.findIndex((label) => label.includes("Actual"));
  expect(currentThirdSession).toBeGreaterThanOrEqual(0);
  await thirdSessionRows
    .nth(currentThirdSession)
    .getByRole("button", { name: "Revocar sesión" })
    .click();
  await expect(thirdPage).toHaveURL("/");
  await expect(thirdPage.getByLabel("Pregunta a Glazz")).toBeEnabled();
  await thirdContext.close();

  await page.goto("/admin");
  await expect(page.getByRole("heading", { name: "Administración" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Catálogo de modelos" })).toBeVisible();
  for (const [tab, heading] of [
    ["Opciones", "Opciones en ejecución"],
    ["Usuarios", "Usuarios"],
    ["Uso", "Uso agregado"],
    ["Auditoría", "Auditoría"],
  ]) {
    await page.getByRole("tab", { name: tab }).click();
    await expect(page.getByRole("heading", { name: heading })).toBeVisible();
  }
  await expect(page.getByText("Conserva esta conversación después del login.")).toHaveCount(0);

  await page.goto("/");
  await page.getByRole("button", { name: "Abrir conversaciones" }).click();
  conversation = page.locator(".conversation-item").filter({ hasText: "Conversación E2E" });
  await conversation.locator("summary").click();
  page.once("dialog", (dialog) => dialog.accept());
  await conversation.getByRole("button", { name: "Eliminar" }).click();
  await expect(conversation).toHaveCount(0);

  await page.goto("/settings");
  await page.getByRole("button", { name: "Eliminar cuenta" }).click();
  const deletion = page.getByRole("alertdialog");
  await deletion.getByLabel("Confirmación").fill("DELETE");
  await deletion.getByRole("button", { name: "Eliminar cuenta" }).click();
  await expect(page).toHaveURL(/deleted=true/);
  await expect(page.getByRole("heading", { name: "¿Qué quieres explorar?" })).toBeVisible();
});

test("Google denial returns to a recoverable guest chat", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375" || process.env.E2E_OAUTH !== "true");
  await page.goto("/");
  await openLoginDialog(page);
  await page.getByRole("link", { name: "Deny" }).click();

  await expect(
    page.getByText(
      "Cancelaste el acceso con Google. Puedes continuar como invitado o intentarlo de nuevo.",
    ),
  ).toBeVisible();
  await expect(page.getByLabel("Pregunta a Glazz")).toBeVisible();
  await expect.poll(() => new URL(page.url()).searchParams.has("authError")).toBe(false);
});

async function openLoginDialog(page: import("@playwright/test").Page) {
  await page.getByRole("button", { name: "Abrir conversaciones" }).click();
  await page
    .locator(".conversation-sidebar")
    .getByRole("button", {
      name: "Continuar con Google",
    })
    .click();
  const dialog = page.getByRole("dialog");
  const consent = dialog.getByRole("checkbox");
  await consent.nth(0).check();
  await consent.nth(1).check();
  await dialog.getByRole("button", { name: "Continuar con Google" }).click();
  await expect(page.getByRole("heading", { name: "Test authorization" })).toBeVisible();
}
