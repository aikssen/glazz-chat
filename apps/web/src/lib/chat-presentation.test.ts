import { describe, expect, it } from "vitest";
import { selectedModelUnavailable, usagePresentation } from "./chat-presentation";
import type { Model, Usage } from "./types";

const usage = (messagesUsed: number, outputUsed = 100): Usage => ({
  messages: { used: messagesUsed, limit: 50, resetAt: "2026-07-25T05:00:00Z" },
  outputTokens: { used: outputUsed, limit: 1000, resetAt: "2026-07-25T05:00:00Z" },
  concurrency: { used: 0, limit: 1, resetAt: null },
});

describe("chat presentation states", () => {
  it("marks approaching and exhausted quota states", () => {
    expect(usagePresentation(usage(39))).toMatchObject({ warning: false, exhausted: false });
    expect(usagePresentation(usage(40))).toMatchObject({
      warning: true,
      exhausted: false,
      remainingMessages: 10,
    });
    expect(usagePresentation(usage(50))).toMatchObject({
      warning: true,
      exhausted: true,
      remainingMessages: 0,
    });
    expect(usagePresentation(usage(10, 1000)).exhausted).toBe(true);
  });

  it("identifies a selected model removed from the visible catalog", () => {
    const models = [{ id: "visible" }] as Model[];
    expect(selectedModelUnavailable("missing", models)).toBe(true);
    expect(selectedModelUnavailable("visible", models)).toBe(false);
    expect(selectedModelUnavailable(undefined, models)).toBe(false);
  });
});
