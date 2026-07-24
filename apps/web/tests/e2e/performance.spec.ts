import { expect, test } from "@playwright/test";

test("chat meets loading and long-transcript budgets", async ({ page }, testInfo) => {
  test.skip(!["mobile-375", "wide-1440"].includes(testInfo.project.name));

  const browserErrors: string[] = [];
  page.on("pageerror", (error) => browserErrors.push(error.message));
  page.on("console", (message) => {
    const text = message.text();
    if (message.type() === "error" && !text.startsWith("Failed to load resource:"))
      browserErrors.push(text);
  });

  await page.addInitScript(() => {
    const state = window as Window & { __glazzCLS?: number };
    state.__glazzCLS = 0;
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        const shift = entry as PerformanceEntry & { hadRecentInput: boolean; value: number };
        if (!shift.hadRecentInput) state.__glazzCLS = (state.__glazzCLS ?? 0) + shift.value;
      }
    }).observe({ type: "layout-shift", buffered: true });
  });

  await page.goto("/");
  const composer = page.getByLabel("Pregunta a Glazz");
  await expect(composer).toBeEnabled({ timeout: 10_000 });
  const interactiveAt = await page.evaluate(() => performance.now());

  await composer.fill("Valida el rendimiento de una conversación extensa.");
  await page.getByRole("button", { name: "Enviar mensaje" }).click();
  await expect(page.getByText("Deterministic development response.")).toBeVisible();
  await page.waitForTimeout(250);

  const metrics = await page.evaluate(() => {
    const state = window as Window & { __glazzCLS?: number };
    const resources = performance.getEntriesByType("resource") as PerformanceResourceTiming[];
    const scripts = resources.filter(
      (entry) => entry.initiatorType === "script" || entry.name.endsWith(".js"),
    );
    const styles = resources.filter(
      (entry) => entry.initiatorType === "css" || entry.name.endsWith(".css"),
    );
    const fcp = performance.getEntriesByName("first-contentful-paint")[0]?.startTime ?? 0;
    return {
      cls: state.__glazzCLS ?? 0,
      fcp,
      jsBytes: scripts.reduce((total, entry) => total + entry.encodedBodySize, 0),
      cssBytes: styles.reduce((total, entry) => total + entry.encodedBodySize, 0),
      scriptCount: scripts.length,
    };
  });

  expect(interactiveAt, "composer interactive time").toBeLessThan(3_000);
  expect(metrics.fcp, "first contentful paint").toBeGreaterThan(0);
  expect(metrics.fcp, "first contentful paint").toBeLessThan(2_500);
  expect(metrics.cls, "cumulative layout shift").toBeLessThan(0.1);
  expect(metrics.scriptCount, "loaded scripts").toBeGreaterThan(0);
  expect(metrics.jsBytes, "encoded JavaScript bytes").toBeLessThan(400_000);
  expect(metrics.cssBytes, "encoded CSS bytes").toBeLessThan(100_000);

  const transcript = await page.locator(".transcript").evaluate(async (root) => {
    const source = Array.from(root.querySelectorAll<HTMLElement>(".message"));
    const marker = root.querySelector(".transcript-end");
    const scroll = root.closest<HTMLElement>(".chat-scroll");
    if (source.length !== 2 || !marker || !scroll)
      throw new Error("transcript fixture is incomplete");

    const startedAt = performance.now();
    for (let index = 0; index < 99; index += 1) {
      for (const message of source) marker.before(message.cloneNode(true));
    }
    const height = scroll.scrollHeight;
    scroll.scrollTop = height;
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
    return {
      elapsed: performance.now() - startedAt,
      messageCount: root.querySelectorAll(".message").length,
      horizontalOverflow: scroll.scrollWidth > scroll.clientWidth + 1,
      reachedBottom: scroll.scrollTop + scroll.clientHeight >= scroll.scrollHeight - 2,
    };
  });

  expect(transcript.messageCount).toBe(200);
  expect(transcript.elapsed, "200-message layout and scroll").toBeLessThan(250);
  expect(transcript.horizontalOverflow).toBe(false);
  expect(transcript.reachedBottom).toBe(true);
  expect(browserErrors).toEqual([]);

  testInfo.annotations.push({
    type: "performance",
    description: JSON.stringify({ interactiveAt, ...metrics, ...transcript }),
  });
});
