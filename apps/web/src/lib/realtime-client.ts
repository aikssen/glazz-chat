import { newUUID } from "./uuid";

export type ClientEventType =
  | "connection.resume"
  | "chat.generate"
  | "chat.cancel"
  | "heartbeat.pong";

export function clientEvent(type: ClientEventType, payload: Record<string, unknown>) {
  return {
    version: 1,
    type,
    eventId: newUUID(),
    requestId: newUUID(),
    idempotencyKey: `ws-${newUUID()}`,
    occurredAt: new Date().toISOString(),
    payload,
  };
}
