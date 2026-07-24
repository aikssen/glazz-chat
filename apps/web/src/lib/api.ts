export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

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
  const method = init.method?.toUpperCase() ?? "GET";
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (!["GET", "HEAD", "OPTIONS"].includes(method)) {
    const csrf = cookie("glazz_csrf") || cookie("glazz_guest_csrf");
    if (csrf) headers.set("X-CSRF-Token", decodeURIComponent(csrf));
  }
  const response = await fetch(`${API_URL}${path}`, {
    ...init,
    headers,
    credentials: "include",
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
