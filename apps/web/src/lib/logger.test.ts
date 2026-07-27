import { afterEach, describe, expect, it, vi } from "vitest";
import { log } from "./logger";

describe("logger", () => {
  afterEach(() => vi.restoreAllMocks());

  it("emits structured fields without changing the correlation id", () => {
    const output = vi.spyOn(console, "info").mockImplementation(() => undefined);

    log("info", "request completed", {
      correlation_id: "web-correlation-1",
      status: 204,
    });

    expect(output).toHaveBeenCalledWith(
      expect.objectContaining({
        level: "info",
        service: "web",
        message: "request completed",
        correlation_id: "web-correlation-1",
        status: 204,
      }),
    );
  });
});
