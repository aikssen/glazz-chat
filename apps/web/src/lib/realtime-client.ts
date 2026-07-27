import { newUUID } from "./uuid";
import { log } from "./logger";

export type ClientEventType =
  | "connection.resume"
  | "chat.generate"
  | "chat.cancel"
  | "heartbeat.pong";

export function clientEvent(type: ClientEventType, payload: Record<string, unknown>) {
  const requestId = newUUID();
  const event = {
    version: 1,
    type,
    eventId: newUUID(),
    requestId,
    idempotencyKey: `ws-${newUUID()}`,
    occurredAt: new Date().toISOString(),
    payload,
  };
  log("debug", "realtime command created", {
    correlation_id: requestId,
    event_type: type,
    event_id: event.eventId,
  });
  return event;
}
