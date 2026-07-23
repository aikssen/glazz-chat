import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

const css = readFileSync(new URL("../app/globals.css", import.meta.url), "utf8");

function tokens(selector: string): Record<string, string> {
  const escaped = selector.replace(".", "\\.");
  const block = css.match(new RegExp(`${escaped}\\s*\\{([\\s\\S]*?)\\}`))?.[1];
  if (!block) {
    throw new Error(`Missing ${selector} token block`);
  }

  return Object.fromEntries(
    [...block.matchAll(/--([\w-]+):\s*(#[\da-fA-F]{6});/g)].map((match) => [match[1], match[2]]),
  );
}

function luminance(hex: string): number {
  const channels = [1, 3, 5].map(
    (offset) => Number.parseInt(hex.slice(offset, offset + 2), 16) / 255,
  );
  const [red, green, blue] = channels.map((channel) =>
    channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4,
  );
  return 0.2126 * red + 0.7152 * green + 0.0722 * blue;
}

function contrast(foreground: string, background: string): number {
  const values = [luminance(foreground), luminance(background)].sort((a, b) => b - a);
  return (values[0] + 0.05) / (values[1] + 0.05);
}

describe.each([
  ["light", tokens(":root")],
  ["dark", tokens(".dark")],
])("%s theme", (_, theme) => {
  it.each([
    ["foreground", "background"],
    ["muted-foreground", "background"],
    ["primary-foreground", "primary"],
  ])("keeps %s on %s at WCAG AA contrast", (foreground, background) => {
    expect(contrast(theme[foreground], theme[background])).toBeGreaterThanOrEqual(4.5);
  });
});
