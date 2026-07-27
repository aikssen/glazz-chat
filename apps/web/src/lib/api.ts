import { newUUID } from "./uuid";
import { log } from "./logger";

function runtimeAPIURL() {
  if (process.env.NEXT_PUBLIC_API_URL) return process.env.NEXT_PUBLIC_API_URL;
  if (typeof window !== "undefined") {
    const url = new URL(window.location.origin);
    url.port = "8080";
    return url.origin;
  }
  return "http://localhost:8080";
}

export const API_URL = runtimeAPIURL();

export class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

function cookie(name: string) {
  if (typeof document === "undefined") return "";
  const prefix = `${name}=`;
  return (
    document.cookie
      .split("; ")
      .find((value) => value.startsWith(prefix))
      ?.slice(prefix.length) ?? ""
  );
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const started = performance.now();
  const method = init.method?.toUpperCase() ?? "GET";
  const route = path.split("?")[0] || "/";
  const headers = new Headers(init.headers);
  const correlationId = headers.get("X-Correlation-ID") ?? `web-${newUUID()}`;
  headers.set("X-Correlation-ID", correlationId);
  if (init.body) headers.set("Content-Type", "application/json");
  if (!["GET", "HEAD", "OPTIONS"].includes(method)) {
    const csrf = cookie("glazz_csrf") || cookie("glazz_guest_csrf");
    if (csrf) headers.set("X-CSRF-Token", decodeURIComponent(csrf));
    if (["POST", "DELETE"].includes(method) && !headers.has("Idempotency-Key")) {
      headers.set("Idempotency-Key", `http-${newUUID()}`);
    }
  }
  log("debug", "http request started", { correlation_id: correlationId, method, route });
  let response: Response;
  try {
    response = await fetch(`${API_URL}${path}`, {
      ...init,
      headers,
      credentials: "include",
    });
  } catch (cause) {
    log("error", "http request failed", {
      correlation_id: correlationId,
      method,
      route,
      duration_ms: Math.round(performance.now() - started),
      error_type: cause instanceof Error ? cause.name : typeof cause,
    });
    throw cause;
  }
  const responseCorrelationId = response.headers.get("X-Correlation-ID") ?? correlationId;
  log(response.ok ? "info" : "warn", "http request completed", {
    correlation_id: responseCorrelationId,
    method,
    route,
    status: response.status,
    duration_ms: Math.round(performance.now() - started),
  });
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as {
      error?: { code?: string; message?: string };
    } | null;
    throw new APIError(
      response.status,
      body?.error?.code ?? "request_failed",
      body?.error?.message ?? "Request could not be completed.",
    );
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export function websocketURL(ticket: string) {
  const url = new URL(`${API_URL}/api/v1/ws`);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.searchParams.set("ticket", ticket);
  return url.toString();
}
