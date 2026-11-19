const API_BASE =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";

// Хендлеры из AuthProvider. Инжектятся через setter — чтобы api.ts не импортировал
// auth-context (иначе циклический импорт auth-context → api → auth-context).
let refreshFn: (() => Promise<string | null>) | null = null;
let authFailFn: (() => void) | null = null;

export function setRefreshHandler(fn: (() => Promise<string | null>) | null) {
  refreshFn = fn;
}

export function setAuthFailHandler(fn: (() => void) | null) {
  authFailFn = fn;
}

export async function apiFetch(
  path: string,
  options: RequestInit = {},
  token: string | null
): Promise<Response> {
  const makeRequest = (authToken: string | null) => {
    const headers: Record<string, string> = {
      ...((options.headers as Record<string, string> | undefined) ?? {}),
    };
    if (authToken) headers["Authorization"] = `Bearer ${authToken}`;
    return fetch(`${API_BASE}${path}`, {
      ...options,
      headers,
      credentials: "include",
    });
  };

  let res = await makeRequest(token);

  // /auth/* — 401 значит «неверный пароль/сессия», refresh не поможет.
  const isAuthPath = path.startsWith("/auth/");
  if (res.status === 401 && !isAuthPath && refreshFn) {
    const newToken = await refreshFn();
    if (newToken) {
      // Один повтор. Если и он 401 — дальше не крутим.
      res = await makeRequest(newToken);
    } else {
      // Refresh не удался — сессия мертва. AuthProvider сделает logout.
      authFailFn?.();
      throw new Error("Session expired");
    }
  }

  if (!res.ok) {
    const error = await res
      .json()
      .catch(() => ({ message: "Unknown error" }));
    throw new Error(error.message || `HTTP ${res.status}`);
  }
  return res;
}

export async function getEventById(id: string, token: string | null) {
  const res = await apiFetch(`/events/${id}`, {}, token);
  return res.json();
}