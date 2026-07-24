import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const worker = readFileSync(new URL("../../public/sw.js", import.meta.url), "utf8");

describe("service worker cache policy", () => {
  it("does not activate updates before the user accepts them", () => {
    const installHandler = worker.match(/addEventListener\("install",[\s\S]*?\n\}\);/)?.[0];
    expect(installHandler).not.toContain("skipWaiting");
    expect(worker).toContain('event.data?.type === "SKIP_WAITING"');
  });

  it("handles only same-origin navigation and excludes API paths", () => {
    expect(worker).toContain('event.request.mode !== "navigate"');
    expect(worker).toContain("url.origin !== self.location.origin");
    expect(worker).toContain('url.pathname.startsWith("/api/")');
    expect(worker).not.toMatch(/caches\.put/);
  });
});
