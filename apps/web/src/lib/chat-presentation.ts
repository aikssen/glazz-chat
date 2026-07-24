import type { Model, Usage } from "./types";

export function selectedModelUnavailable(
  selectedModelID: string | undefined,
  models: Model[],
): boolean {
  return Boolean(selectedModelID) && !models.some((model) => model.id === selectedModelID);
}

export function usagePresentation(usage: Usage | null) {
  if (!usage) {
    return { exhausted: false, remainingMessages: null, warning: false };
  }
  const remainingMessages = Math.max(usage.messages.limit - usage.messages.used, 0);
  return {
    exhausted:
      usage.messages.used >= usage.messages.limit ||
      usage.outputTokens.used >= usage.outputTokens.limit,
    remainingMessages,
    warning: usage.messages.limit > 0 && remainingMessages / usage.messages.limit <= 0.2,
  };
}
