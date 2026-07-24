import { describe, expect, it } from "vitest";
import { dictionary, messages } from "./i18n";

describe("i18n dictionaries", () => {
  it("keep Spanish and English keys in parity", () => {
    expect(Object.keys(messages.en).sort()).toEqual(Object.keys(messages.es).sort());
  });

  it("returns the requested locale without shared mutable state", () => {
    expect(dictionary("es").newChat).toBe("Nuevo chat");
    expect(dictionary("en").newChat).toBe("New chat");
  });
});
