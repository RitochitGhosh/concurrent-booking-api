let refreshing: Promise<void> | null = null;
export class APIError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message);
  }
}
async function raw(path: string, options: RequestInit = {}) {
  return fetch(path, {
    ...options,
    credentials: "same-origin",
    cache: "no-store",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Protection": "1",
      ...options.headers,
    },
  });
}
async function refresh() {
  if (!refreshing) {
    const rotate = async () => {
      // A different tab may have rotated while we waited for its lock.
      const current = await raw("/api/auth/me");
      if (current.ok) return;
      if (current.status !== 401)
        throw new APIError("Unable to check your session.", current.status);
      const response = await raw("/api/auth/refresh", { method: "POST" });
      if (!response.ok)
        throw new APIError(
          response.status === 401
            ? "Please sign in again."
            : "Unable to refresh your session. Try again.",
          response.status,
        );
    };
    const synchronized = async () => {
      if (navigator.locks)
        await navigator.locks.request("frame-session-refresh", rotate);
      else await rotate();
    };
    refreshing = synchronized().finally(() => {
      refreshing = null;
    });
  }
  return refreshing;
}
export async function api<T>(
  path: string,
  options: RequestInit = {},
  retry = true,
): Promise<T> {
  let response = await raw(path, options);
  if (response.status === 401 && retry) {
    try {
      await refresh();
      response = await raw(path, options);
    } catch (error) {
      if (error instanceof APIError && error.status === 401)
        window.dispatchEvent(new Event("frame:signed-out"));
      throw error;
    }
  }
  if (response.status === 204) return undefined as T;
  const data = await response.json();
  if (!response.ok) {
    if (response.status === 401 && retry)
      window.dispatchEvent(new Event("frame:signed-out"));
    throw new APIError(
      data.error ?? "Something went wrong. Please try again.",
      response.status,
    );
  }
  return data as T;
}
export const post = (value: unknown): RequestInit => ({
  method: "POST",
  body: JSON.stringify(value),
});
