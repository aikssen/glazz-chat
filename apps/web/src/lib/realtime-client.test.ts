import { describe, expect, it } from "vitest";
import { clientEvent, type ClientEventType } from "./realtime-client";

describe("clientEvent", () => {
  it.each<ClientEventType>([
    "connection.resume",
    "chat.generate",
    "chat.cancel",
    "heartbeat.pong",
  ])("builds a contract-valid %s envelope", (type) => {
    const event = clientEvent(type, { value: "test" });

    expect(event).toMatchObject({
      version: 1,
      type,
      payload: { value: "test" },
    });
    expect(event.eventId.length).toBeGreaterThanOrEqual(8);
    expect(event.requestId.length).toBeGreaterThanOrEqual(8);
    expect(event.idempotencyKey.length).toBeGreaterThanOrEqual(16);
    expect(Number.isNaN(Date.parse(event.occurredAt))).toBe(false);
  });
});
