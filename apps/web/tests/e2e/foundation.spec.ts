import { expect, test } from "@playwright/test";

test("renders the chat foundation", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { name: /help you explore/i })).toBeVisible();
  await expect(page.getByText("Ask Glazz...")).toBeVisible();
});
