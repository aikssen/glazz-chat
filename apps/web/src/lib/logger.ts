export type LogLevel = "debug" | "info" | "warn" | "error";

const priorities: Record<LogLevel, number> = {
  debug: 10,
  info: 20,
  warn: 30,
  error: 40,
};

const configured = normalizeLevel(process.env.NEXT_PUBLIC_LOG_LEVEL);

function normalizeLevel(value: string | undefined): LogLevel {
  if (value === "debug" || value === "info" || value === "warn" || value === "error") {
    return value;
  }
  return process.env.NODE_ENV === "production" ? "info" : "debug";
}

export function log(level: LogLevel, message: string, fields: Record<string, unknown> = {}) {
  if (priorities[level] < priorities[configured]) return;
  console[level]({
    timestamp: new Date().toISOString(),
    level,
    service: "web",
    message,
    ...fields,
  });
}
