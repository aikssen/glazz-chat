import type { Message } from "./types";

export interface StartedPayload {
  assistantMessageId: string;
  generationId: string;
  conversationId: string;
  modelId?: string;
  modelName?: string;
}

export interface DeltaPayload {
  generationId: string;
  offset: number;
  text: string;
}

export function startAssistant(
  messages: Message[],
  payload: StartedPayload,
  now = new Date().toISOString(),
): Message[] {
  if (messages.some((message) => message.id === payload.assistantMessageId)) return messages;
  return [
    ...messages,
    {
      id: payload.assistantMessageId,
      conversationId: payload.conversationId,
      role: "assistant",
      content: "",
      status: "pending",
      sequence: messages.length + 1,
      createdAt: now,
      generationId: payload.generationId,
      modelId: payload.modelId,
      modelName: payload.modelName,
    },
  ];
}

export function appendDelta(messages: Message[], payload: DeltaPayload): Message[] {
  return messages.map((message) => {
    if (message.generationId !== payload.generationId) return message;
    const currentOffset = new TextEncoder().encode(message.content).byteLength;
    if (payload.offset !== currentOffset) return message;
    return { ...message, content: message.content + payload.text };
  });
}

export function finishAssistant(
  messages: Message[],
  generationId: string,
  status: Message["status"],
): Message[] {
  return messages.map((message) =>
    message.generationId === generationId ? { ...message, status } : message,
  );
}
