import { expect, test } from "@playwright/test";

test("rail keeps new chat first and history opens a focused search drawer", async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  await page.goto("/");

  const rail = page.getByRole("navigation", { name: "Navegación principal" });
  const actions = rail.getByRole("button");
  await expect(actions.first()).toHaveAccessibleName("Nuevo chat");
  await expect(rail.getByRole("button", { name: "Chat actual" })).toBeVisible();

  const history = rail.getByRole("button", { name: "Abrir conversaciones" });
  await history.click();
  const drawer = page.getByRole("dialog", { name: "Conversaciones" });
  const search = drawer.getByRole("searchbox", { name: "Buscar conversaciones" });
  await expect(search).toBeFocused();
  await search.fill("sin coincidencias");
  await expect(drawer.getByText("No hay conversaciones que coincidan.")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(drawer).toBeHidden();
  await expect(history).toBeFocused();
});

test("context is a sheet on mobile and a stable lane on wide screens", async ({
  page,
}, testInfo) => {
  await page.goto("/");
  const context = page.getByLabel("Contexto de conversación");

  if (testInfo.project.name === "wide-1440") {
    await expect(context).toBeVisible();
  } else {
    await expect(context).toBeHidden();
    await page.getByRole("button", { name: "Abrir contexto" }).click();
    await expect(context).toBeVisible();
  }

  await expect(context.getByText("CONVERSATION", { exact: true })).toBeVisible();
  await expect(context.getByRole("tab", { name: "OUTLINE" })).toBeVisible();
  const close = context.getByRole("button", { name: "Cerrar contexto" });
  const closeBox = await close.boundingBox();
  await context.getByRole("tab", { name: "DETAILS" }).click();
  await expect(context.getByRole("tab", { name: "DETAILS" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await close.click();
  await expect(context).toBeHidden();
  const reopen = page.getByRole("button", { name: "Abrir contexto" });
  await expect(reopen).toBeVisible();
  const reopenBox = await reopen.boundingBox();
  expect(reopenBox?.x).toBeGreaterThan((page.viewportSize()?.width ?? 0) - 50);
  expect(Math.abs((reopenBox?.y ?? 0) - (closeBox?.y ?? 0))).toBeLessThanOrEqual(1);
  await expect(
    page.getByRole("navigation", { name: "Navegación principal" }).getByRole("button", {
      name: "Abrir contexto",
    }),
  ).toHaveCount(0);
  await reopen.click();
  await expect(context).toBeVisible();
});

test("model selection belongs to the composer instead of the global header", async ({ page }) => {
  await page.goto("/");
  const composer = page.locator(".composer");
  await expect(composer.getByRole("combobox", { name: "Modelo" })).toBeVisible();
  await expect(page.locator(".chat-topbar").getByRole("combobox")).toHaveCount(0);
});

test("the original Glazz mark geometry is recolored without a bitmap replacement", async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  await page.goto("/");
  const mark = page.locator(".rail-brand .glazz-mark");
  await expect(mark).toHaveText("G");
  await expect(mark).toHaveCSS("border-radius", "50%");
  const colors = await mark.evaluate((element) => {
    const style = getComputedStyle(element);
    return { top: style.borderTopColor, right: style.borderRightColor };
  });
  expect(colors.right).not.toBe(colors.top);
  await expect(page.locator(".rail-brand img")).toHaveCount(0);
});

test("the account avatar exposes admin and account actions without duplicating settings", async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  await page.route("**/api/v1/me", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        id: "00000000-0000-7000-8000-000000000001",
        email: "manual-20260724@glazz.test",
        displayName: "Glazz Administrator",
        locale: "es",
        role: "admin",
        status: "active",
      }),
    }),
  );
  await page.goto("/");

  const account = page.getByRole("button", { name: "Cuenta: Glazz Administrator" });
  await expect(account).toHaveCount(1);
  await account.click();
  const menu = page.getByRole("menu", { name: "Cuenta" });
  await expect(menu.getByRole("menuitem", { name: "Administración" })).toHaveAttribute(
    "href",
    "/admin",
  );
  await expect(menu.getByRole("menuitem", { name: "Ajustes" })).toHaveAttribute(
    "href",
    "/settings",
  );
  await expect(menu.getByRole("menuitem", { name: "Cerrar sesión" })).toBeVisible();
  await expect(page.locator(".navigation-rail .rail-avatar")).toHaveCount(0);
  await page.keyboard.press("Escape");
  await expect(
    page
      .getByRole("navigation", { name: "Navegación principal" })
      .getByRole("link", { name: "Administración" }),
  ).toHaveAttribute("href", "/admin");
  await page.getByRole("button", { name: "Abrir conversaciones" }).click();
  const sidebar = page.locator(".conversation-sidebar");
  await expect(sidebar.locator(".sidebar-account")).toHaveCount(1);
  expect(await sidebar.locator(".sidebar-account").evaluate((element) => element.tagName)).toBe(
    "DIV",
  );
  await expect(sidebar.getByRole("link", { name: "Ajustes" })).toHaveCount(1);
});
