import type { Locale, Message } from "./types";

export function streamAnnouncement(
  wasStreaming: boolean,
  streaming: boolean,
  status: Message["status"] | undefined,
  locale: Locale,
): string {
  if (!wasStreaming && streaming) {
    return locale === "es" ? "Glazz está respondiendo." : "Glazz is responding.";
  }
  if (!wasStreaming || streaming) return "";
  if (status === "failed") {
    return locale === "es"
      ? "La respuesta se detuvo antes de terminar."
      : "The response stopped before it finished.";
  }
  if (status === "cancelled") {
    return locale === "es" ? "Respuesta detenida." : "Response stopped.";
  }
  return locale === "es" ? "Respuesta completada." : "Response complete.";
}
