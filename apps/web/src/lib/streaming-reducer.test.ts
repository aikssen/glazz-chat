import { describe, expect, it } from "vitest";
import { appendDelta, finishAssistant, startAssistant } from "./streaming-reducer";

describe("streaming reducer", () => {
  it("does not duplicate a replayed start event", () => {
    const payload = {
      assistantMessageId: "assistant-1",
      generationId: "generation-1",
      conversationId: "conversation-1",
    };
    const started = startAssistant([], payload, "2026-07-23T00:00:00Z");
    expect(startAssistant(started, payload)).toBe(started);
  });

  it("applies ordered byte offsets and ignores duplicate or missing deltas", () => {
    const started = startAssistant([], {
      assistantMessageId: "assistant-1",
      generationId: "generation-1",
      conversationId: "conversation-1",
    });
    const first = appendDelta(started, {
      generationId: "generation-1",
      offset: 0,
      text: "Sí",
    });
    expect(first[0]?.content).toBe("Sí");
    expect(appendDelta(first, { generationId: "generation-1", offset: 0, text: "Sí" })).toEqual(
      first,
    );
    expect(
      appendDelta(first, { generationId: "generation-1", offset: 3, text: ", claro" })[0]?.content,
    ).toBe("Sí, claro");
  });

  it("moves the assistant message to a terminal state", () => {
    const started = startAssistant([], {
      assistantMessageId: "assistant-1",
      generationId: "generation-1",
      conversationId: "conversation-1",
    });
    expect(finishAssistant(started, "generation-1", "complete")[0]?.status).toBe("complete");
  });
});
