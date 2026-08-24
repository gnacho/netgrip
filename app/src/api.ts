export class UnauthorizedError extends Error {}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (res.status === 401) throw new UnauthorizedError();
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error || `HTTP ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const api = {
  login: (password: string) =>
    request<void>("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password }),
    }),
  logout: () => request<void>("/api/logout", { method: "POST" }),
  board: () => request<import("./types").Board>("/api/board"),
  system: () => request<import("./types").SystemInfo>("/api/system"),
  wan: () => request<import("./types").WanStatus>("/api/wan"),
  wireless: () => request<import("./types").WirelessRadio[]>("/api/wireless"),
  leases: () => request<import("./types").Lease[]>("/api/leases"),
};
