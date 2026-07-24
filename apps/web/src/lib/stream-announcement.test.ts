import { describe, expect, it } from "vitest";
import { streamAnnouncement } from "./stream-announcement";

describe("streamAnnouncement", () => {
  it("announces lifecycle transitions without announcing streamed deltas", () => {
    expect(streamAnnouncement(false, true, "pending", "es")).toBe("Glazz está respondiendo.");
    expect(streamAnnouncement(true, true, "pending", "es")).toBe("");
    expect(streamAnnouncement(true, true, "pending", "es")).toBe("");
    expect(streamAnnouncement(true, false, "complete", "es")).toBe("Respuesta completada.");
  });

  it("announces terminal failure and cancellation states", () => {
    expect(streamAnnouncement(true, false, "failed", "en")).toBe(
      "The response stopped before it finished.",
    );
    expect(streamAnnouncement(true, false, "cancelled", "en")).toBe("Response stopped.");
  });
});
