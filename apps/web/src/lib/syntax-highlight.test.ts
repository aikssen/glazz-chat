import { describe, expect, it } from "vitest";
import { highlightSyntax } from "./syntax-highlight";

describe("highlightSyntax", () => {
  it("highlights registered language aliases", () => {
    const result = highlightSyntax("const answer: number = 42;", "ts");
    expect(result?.language).toBe("typescript");
    expect(result?.html).toContain("hljs-keyword");
  });

  it("escapes executable markup inside code", () => {
    const result = highlightSyntax('<script>alert("unsafe")</script>', "html");
    expect(result?.html).not.toContain("<script>");
    expect(result?.html).toContain("&lt;");
  });

  it("leaves unknown languages to the safe plain-text renderer", () => {
    expect(highlightSyntax("opaque content", "unknown-language")).toBeNull();
  });
});
