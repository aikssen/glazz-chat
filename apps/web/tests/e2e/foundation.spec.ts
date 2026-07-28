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

test("administration renders access denial without conversation content", async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  await page.route("**/api/v1/admin/models", (route) =>
    route.fulfill({
      status: 403,
      contentType: "application/json",
      body: JSON.stringify({
        error: {
          code: "forbidden",
          message: "Administrator access is required.",
        },
      }),
    }),
  );
  await page.goto("/admin");
  await expect(page.getByText("No tienes acceso a administración.")).toBeVisible();
  await expect(page.locator(".message-content")).toHaveCount(0);
});

test("administrator can expose a model, transfer defaults, and disable the previous model", async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  type TestAdminModel = {
    id: string;
    name: string;
    description: string;
    capabilities: { chatCompletions: boolean };
    enabled: boolean;
    available: boolean;
    supported: boolean;
    audience: Array<"guest" | "user">;
    defaultFor: Array<"guest" | "user">;
    order: number;
    version: number;
  };
  let models: TestAdminModel[] = [
    {
      id: "00000000-0000-7000-8000-000000000101",
      name: "DeepSeek V4 Flash",
      description: "Current default model.",
      capabilities: { chatCompletions: true },
      enabled: true,
      available: true,
      supported: true,
      audience: ["guest", "user"],
      defaultFor: ["guest", "user"],
      order: 0,
      version: 1,
    },
    {
      id: "00000000-0000-7000-8000-000000000102",
      name: "GLM 5",
      description: "Discovered provider model.",
      capabilities: { chatCompletions: true },
      enabled: false,
      available: true,
      supported: true,
      audience: ["user"],
      defaultFor: [],
      order: 1,
      version: 2,
    },
  ];
  const mutations: Array<{
    id: string;
    ifMatch: string | undefined;
    patch: Partial<TestAdminModel>;
  }> = [];

  await page.route("**/api/v1/admin/models", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items: models }),
    }),
  );
  await page.route("**/api/v1/admin/models/*", async (route) => {
    const request = route.request();
    if (request.method() !== "PATCH") return route.fallback();
    const id = new URL(request.url()).pathname.split("/").at(-1) ?? "";
    const patch = request.postDataJSON() as Partial<TestAdminModel>;
    mutations.push({ id, ifMatch: request.headers()["if-match"], patch });

    if (patch.defaultFor) {
      models = models.map((model) => {
        if (model.id === id) return model;
        const retainedDefaults = model.defaultFor.filter(
          (actorType) => !patch.defaultFor?.includes(actorType),
        );
        return retainedDefaults.length === model.defaultFor.length
          ? model
          : { ...model, defaultFor: retainedDefaults, version: model.version + 1 };
      });
    }
    let updated = models.find((model) => model.id === id);
    if (!updated) return route.fulfill({ status: 404 });
    updated = { ...updated, ...patch, version: updated.version + 1 };
    models = models.map((model) => (model.id === id ? updated : model));
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(updated),
    });
  });

  await page.goto("/admin");
  const currentRow = page.locator("tbody tr").filter({ hasText: "DeepSeek V4 Flash" });
  const candidateRow = page.locator("tbody tr").filter({ hasText: "GLM 5" });
  const currentExposure = currentRow.getByRole("checkbox");
  const candidateExposure = candidateRow.getByRole("checkbox");

  await expect(currentExposure).toBeDisabled();
  await candidateExposure.click();
  await expect(candidateExposure).toBeChecked();

  await candidateRow.getByRole("button", { name: "Usuarios" }).click();
  await expect(candidateRow.getByRole("button", { name: "Usuarios" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await candidateRow.getByRole("button", { name: "Invitados" }).click();
  await expect(candidateRow.getByRole("button", { name: "Invitados" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );

  await expect(currentExposure).toBeEnabled();
  await currentExposure.click();
  await expect(currentExposure).not.toBeChecked();
  expect(mutations).toHaveLength(4);
  expect(mutations.every((mutation) => mutation.ifMatch?.startsWith('"'))).toBe(true);
  expect(mutations[2]?.patch.audience).toEqual(["user", "guest"]);
  expect(mutations[3]?.patch).toEqual({ enabled: false });
});

test("guest sends a message and receives a streamed response", async ({ page }) => {
  await page.goto("/");
  const composer = page.getByLabel("Pregunta a Glazz");
  const send = page.getByRole("button", { name: "Enviar mensaje" });
  await composer.fill("Resume por qué el cielo es azul.");
  await expect(send).toBeEnabled({ timeout: 10_000 });
  await send.click();

  await expect(
    page
      .getByRole("log", { name: "Transcripción de la conversación" })
      .getByText("Resume por qué el cielo es azul."),
  ).toBeVisible();
  await expect(page.getByText("Deterministic development response.")).toBeVisible({
    timeout: 10_000,
  });
  await expect(page.locator(".composer-allowance")).toHaveText("3 mensajes gratis disponibles");
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

test("guest output budget consumes the exact remaining token", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375" || process.env.E2E_GUEST_EDGES !== "true");
  await page.goto("/");
  const apiOrigin = new URL(process.env.E2E_API_URL ?? page.url());
  if (!process.env.E2E_API_URL) apiOrigin.port = "8080";
  const composer = page.getByLabel("Pregunta a Glazz");

  await composer.fill("Consume casi todo el presupuesto.");
  await page.getByRole("button", { name: "Enviar mensaje" }).click();
  await expect(page.getByText("Deterministic development response.")).toHaveCount(1);
  await expect
    .poll(() => readGuestAllowance(page, apiOrigin.origin))
    .toMatchObject({
      messagesUsed: 1,
      outputTokensUsed: 1999,
      outputTokenLimit: 2000,
      exhausted: false,
    });

  await composer.fill("Consume solamente el token restante.");
  await page.getByRole("button", { name: "Enviar mensaje" }).click();
  await expect(page.getByText("Deterministic development response.")).toHaveCount(2);
  await expect
    .poll(() => readGuestAllowance(page, apiOrigin.origin))
    .toMatchObject({
      messagesUsed: 2,
      outputTokensUsed: 2000,
      outputTokenLimit: 2000,
      exhausted: true,
    });
  await expect(page.getByText("Continúa tu conversación")).toBeVisible();
  await expect(composer).not.toBeVisible();
});

test("expired guest starts with a fresh empty allowance", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375" || process.env.E2E_GUEST_EDGES !== "true");
  const ttl = Number(process.env.E2E_GUEST_TTL_MS ?? "15000");
  await page.goto("/");
  const apiOrigin = new URL(process.env.E2E_API_URL ?? page.url());
  if (!process.env.E2E_API_URL) apiOrigin.port = "8080";
  const prompt = "Esta conversación debe expirar.";
  const composer = page.getByLabel("Pregunta a Glazz");
  await composer.fill(prompt);
  await page.getByRole("button", { name: "Enviar mensaje" }).click();
  await expect(page.getByText("Deterministic development response.")).toBeVisible();
  const original = await readGuestAllowance(page, apiOrigin.origin);

  await page.waitForTimeout(ttl + 1_000);
  await page.reload();

  await expect(page.getByLabel("Pregunta a Glazz")).toBeEnabled({ timeout: 10_000 });
  await expect(page.getByRole("paragraph").filter({ hasText: prompt })).toHaveCount(0);
  await expect
    .poll(() => readGuestAllowance(page, apiOrigin.origin))
    .toMatchObject({
      messagesUsed: 0,
      outputTokensUsed: 0,
      messageLimit: 4,
      outputTokenLimit: 2000,
      exhausted: false,
    });
  const renewed = await readGuestAllowance(page, apiOrigin.origin);
  expect(Date.parse(renewed.expiresAt)).toBeGreaterThan(Date.parse(original.expiresAt));
});

test("Google approval covers the registered-user lifecycle", async ({
  browser,
  context,
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
  const migratedConversationID = new URL(page.url()).searchParams.get("conversation");
  expect(migratedConversationID).toBeTruthy();

  const apiOrigin = new URL(process.env.E2E_API_URL ?? page.url());
  if (!process.env.E2E_API_URL) apiOrigin.port = "8080";
  const deniedConversation = await page.request.get(
    `${apiOrigin.origin}/api/v1/conversations/00000000-0000-4000-8000-000000000001/messages`,
  );
  expect(deniedConversation.status()).toBe(404);
  const migrated = await page.evaluate(async (origin) => {
    const response = await fetch(`${origin}/api/v1/conversations?limit=100`, {
      credentials: "include",
    });
    return (await response.json()) as { items: Array<{ id: string }> };
  }, apiOrigin.origin);
  expect(migrated.items.some((item) => item.id === migratedConversationID)).toBe(true);
  await keepOnlyConversation(page, apiOrigin.origin, migratedConversationID!);
  expect(callbackURL).toContain("code=glazz-e2e-approved");
  const replay = await page.request.get(callbackURL);
  expect(replay.status()).toBe(400);

  await composer.fill("Cancela esta respuesta y vuelve a intentarla.");
  await page.getByRole("button", { name: "Enviar mensaje" }).click();
  await page.getByRole("button", { name: "Detener respuesta" }).click();
  const cancelled = page.locator(".message--assistant.message--cancelled").last();
  await expect(cancelled).toBeVisible();
  const completedResponses = page.locator(".message--assistant.message--complete");
  const completedBeforeRetry = await completedResponses.count();
  await cancelled.getByRole("button", { name: "Reintentar" }).click();
  await expect(completedResponses).toHaveCount(completedBeforeRetry + 1);
  await expect(completedResponses.last()).toContainText("Deterministic development response.");

  const reconnectPrompt = "Conserva una sola respuesta después de reconectar.";
  const completedBeforeReconnect = await completedResponses.count();
  await composer.fill(reconnectPrompt);
  await page.getByRole("button", { name: "Enviar mensaje" }).click();
  await expect(page.getByRole("button", { name: "Detener respuesta" })).toBeVisible();
  await context.setOffline(true);
  await expect(page.locator(".connection")).toHaveAttribute("title", "Reconectando");
  await page.waitForTimeout(500);
  await context.setOffline(false);
  await expect(page.locator(".connection")).toHaveAttribute("title", "Conectado", {
    timeout: 10_000,
  });
  await expect(completedResponses).toHaveCount(completedBeforeReconnect + 1);
  await expect(page.getByRole("paragraph").filter({ hasText: reconnectPrompt })).toHaveCount(1);

  await page.evaluate(async (origin) => {
    const csrf = document.cookie
      .split("; ")
      .find((value) => value.startsWith("glazz_csrf="))
      ?.split("=")
      .slice(1)
      .join("=");
    if (!csrf) throw new Error("Authenticated CSRF cookie is missing");
    for (let index = 0; index < 20; index += 1) {
      const response = await fetch(`${origin}/api/v1/conversations`, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": `pagination-${Date.now()}-${index}`,
          "X-CSRF-Token": decodeURIComponent(csrf),
        },
        body: "{}",
      });
      if (!response.ok) throw new Error(`Conversation ${index} failed: ${response.status}`);
    }
  }, apiOrigin.origin);
  await page.reload();
  await page.getByRole("button", { name: "Abrir conversaciones" }).click();
  await expect(page.locator(".conversation-item")).toHaveCount(20);
  await page.getByRole("button", { name: "Cargar más" }).click();
  await expect(page.locator(".conversation-item")).toHaveCount(21);
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
  const secondSessionID = await secondPage.evaluate(async (origin) => {
    const response = await fetch(`${origin}/api/v1/me/sessions`, { credentials: "include" });
    const body = (await response.json()) as { items: Array<{ current: boolean; id: string }> };
    const current = body.items.find((session) => session.current);
    if (!current) throw new Error("second browser current session was not found");
    return current.id;
  }, apiOrigin.origin);

  await page.goto("/settings");
  await expect(page.getByRole("heading", { name: "Ajustes" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Sesiones activas" })).toBeVisible();
  await expect(page.getByText(/Actual/)).toBeVisible();
  const sessionRows = page.locator(".session-row");
  const sessionsBeforeRevocation = await sessionRows.count();
  expect(sessionsBeforeRevocation).toBeGreaterThanOrEqual(2);
  const secondSessionRow = page.locator(`.session-row[data-session-id="${secondSessionID}"]`);
  await expect(secondSessionRow).toBeVisible();
  await secondSessionRow.getByRole("button", { name: "Revocar sesión" }).click();
  await expect(sessionRows).toHaveCount(sessionsBeforeRevocation - 1);
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
  expect(await thirdSessionRows.count()).toBeGreaterThanOrEqual(2);
  const currentThirdSession = thirdSessionRows.filter({ hasText: "Actual" });
  await expect(currentThirdSession).toHaveCount(1);
  await currentThirdSession.getByRole("button", { name: "Revocar sesión" }).click();
  await expect(thirdPage).toHaveURL("/");
  await expect(thirdPage.getByLabel("Pregunta a Glazz")).toBeEnabled();
  await thirdContext.close();

  await page.goto("/admin");
  await expect(page.getByRole("heading", { name: "Administración" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Catálogo de modelos" })).toBeVisible();
  await page.getByRole("tab", { name: "Opciones" }).click();
  await expect(page.getByRole("heading", { name: "Opciones en ejecución" })).toBeVisible();
  await page.route("**/api/v1/admin/settings/quota.guest.messages", (route) => {
    if (route.request().method() !== "PATCH") return route.fallback();
    return route.fulfill({
      status: 412,
      contentType: "application/json",
      body: JSON.stringify({
        error: {
          code: "conflict",
          message: "El ajuste cambió en otra sesión.",
        },
      }),
    });
  });
  const guestLimit = page.locator(".setting-editor").filter({ hasText: "quota.guest.messages" });
  await guestLimit.locator("input").fill("5");
  await guestLimit.getByRole("button", { name: "Guardar quota.guest.messages" }).click();
  await expect(page.getByText("El ajuste cambió en otra sesión.")).toBeVisible();
  await page.unroute("**/api/v1/admin/settings/quota.guest.messages");

  for (const [tab, heading] of [
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
  await page.getByRole("searchbox", { name: "Buscar conversaciones" }).fill("Conversación E2E");
  conversation = page.locator(".conversation-item").filter({ hasText: "Conversación E2E" });
  await expect(conversation).toBeVisible();
  await conversation.locator("summary").click();
  page.once("dialog", (dialog) => dialog.accept());
  await conversation.getByRole("button", { name: "Eliminar" }).click();
  await expect(conversation).toHaveCount(0);

  await page.goto("/settings");
  await page.getByRole("button", { name: "Eliminar cuenta" }).click();
  let deletion = page.getByRole("alertdialog");
  await deletion.getByLabel("Confirmación").fill("DELETE");
  if (process.env.E2E_RECENT_AUTH === "true") {
    await page.waitForTimeout(5_500);
    await deletion.getByRole("button", { name: "Eliminar cuenta" }).click();
    await page.getByRole("link", { name: "Approve" }).click();
    await expect(page).toHaveURL(/\/settings$/);
    await page.getByRole("button", { name: "Eliminar cuenta" }).click();
    deletion = page.getByRole("alertdialog");
    await deletion.getByLabel("Confirmación").fill("DELETE");
  }
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

async function keepOnlyConversation(
  page: import("@playwright/test").Page,
  apiOrigin: string,
  conversationID: string,
) {
  const remaining = await page.evaluate(
    async ({ origin, keepID }) => {
      const csrf = document.cookie
        .split("; ")
        .find((value) => value.startsWith("glazz_csrf="))
        ?.split("=")
        .slice(1)
        .join("=");
      if (!csrf) throw new Error("Authenticated CSRF cookie is missing");

      const list = await fetch(`${origin}/api/v1/conversations?limit=100&archived=true`, {
        credentials: "include",
      });
      if (!list.ok) throw new Error(`Conversation cleanup list failed: ${list.status}`);
      const body = (await list.json()) as { items: Array<{ id: string }> };

      for (const conversation of body.items) {
        if (conversation.id === keepID) continue;
        const response = await fetch(`${origin}/api/v1/conversations/${conversation.id}`, {
          method: "DELETE",
          credentials: "include",
          headers: {
            "Idempotency-Key": `e2e-cleanup-${crypto.randomUUID()}`,
            "X-CSRF-Token": decodeURIComponent(csrf),
          },
        });
        if (!response.ok) {
          throw new Error(`Conversation cleanup failed: ${response.status}`);
        }
      }

      const finalList = await fetch(`${origin}/api/v1/conversations?limit=100&archived=true`, {
        credentials: "include",
      });
      if (!finalList.ok) throw new Error(`Conversation cleanup check failed: ${finalList.status}`);
      const finalBody = (await finalList.json()) as { items: Array<{ id: string }> };
      return finalBody.items.map((conversation) => conversation.id);
    },
    { origin: apiOrigin, keepID: conversationID },
  );

  expect(remaining).toEqual([conversationID]);
}

async function readGuestAllowance(page: import("@playwright/test").Page, apiOrigin: string) {
  return page.evaluate(async (origin) => {
    const response = await fetch(`${origin}/api/v1/guest-sessions/current`, {
      credentials: "include",
    });
    if (!response.ok) throw new Error(`Guest allowance failed: ${response.status}`);
    return response.json() as Promise<{
      messagesUsed: number;
      messageLimit: number;
      outputTokensUsed: number;
      outputTokenLimit: number;
      exhausted: boolean;
      expiresAt: string;
    }>;
  }, apiOrigin);
}
