import { expect, test } from "@playwright/test";

test("web responses enforce the browser security policy", async ({ request }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  const response = await request.get("/");
  expect(response.ok()).toBe(true);

  const headers = response.headers();
  expect(headers["content-security-policy"]).toContain("default-src 'self'");
  expect(headers["content-security-policy"]).toContain("object-src 'none'");
  expect(headers["content-security-policy"]).toContain("frame-ancestors 'none'");
  expect(headers["cross-origin-opener-policy"]).toBe("same-origin");
  expect(headers["cross-origin-resource-policy"]).toBe("same-origin");
  expect(headers["permissions-policy"]).toContain("camera=()");
  expect(headers["referrer-policy"]).toBe("strict-origin-when-cross-origin");
  expect(headers["x-content-type-options"]).toBe("nosniff");
  expect(headers["x-frame-options"]).toBe("DENY");
  expect(headers["x-powered-by"]).toBeUndefined();
});

test("service worker cannot be served from a shared cache", async ({ request }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-375");
  const response = await request.get("/sw.js");
  expect(response.ok()).toBe(true);
  expect(response.headers()["cache-control"]).toBe("no-cache, no-store, must-revalidate");
  expect(response.headers()["content-type"]).toContain("application/javascript");
});
