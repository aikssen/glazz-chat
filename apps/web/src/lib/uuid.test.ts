import { describe, expect, it } from "vitest";

import { newUUID } from "./uuid";

describe("newUUID", () => {
  it("uses the platform UUID implementation when available", () => {
    const value = "11111111-2222-4333-8444-555555555555";
    expect(
      newUUID({
        randomUUID: () => value,
        getRandomValues: (array) => array,
      }),
    ).toBe(value);
  });

  it("creates an RFC 4122 UUID v4 when randomUUID is unavailable", () => {
    const value = newUUID({
      getRandomValues: (array) => {
        if (array) new Uint8Array(array.buffer, array.byteOffset, array.byteLength).fill(0xab);
        return array;
      },
    });

    expect(value).toBe("abababab-abab-4bab-abab-abababababab");
    expect(value).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
  });
});
