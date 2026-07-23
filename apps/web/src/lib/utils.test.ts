import { describe, expect, it } from "vitest";

import { cn } from "./utils";

describe("cn", () => {
  it("merges Tailwind classes with the last conflicting value", () => {
    expect(cn("px-2", false && "hidden", "px-4")).toBe("px-4");
  });
});
