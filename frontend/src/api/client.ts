import type { Fund, Purchase, Session, Settlement, Tokens, User } from "./types";

const API_PREFIX = "/api/v1";
const ACCESS_TOKEN_KEY = "splitcore.access_token";

// Auth paths that must NOT trigger a refresh loop
const AUTH_PATHS = new Set([
  "/auth/telegram/init",
  "/auth/telegram/status",
  "/auth/telegram/tokens",
  "/auth/telegram/access",
]);

// Shared promise – prevents multiple concurrent refresh calls
let refreshPromise: Promise<string | null> | null = null;

async function doRefresh(): Promise<string | null> {
  try {
    const res = await fetch(`${API_PREFIX}/auth/telegram/access`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({}),
    });
    if (!res.ok) return null;
    const data: Tokens = await res.json();
    if (data?.access_token) {
      authStore.setToken(data.access_token);
      window.dispatchEvent(
        new CustomEvent("splitcore:tokenrefreshed", { detail: data.access_token })
      );
      return data.access_token;
    }
    return null;
  } catch {
    return null;
  }
}

type RequestOptions = {
  token?: string | null;
  body?: unknown;
  method?: "GET" | "POST" | "DELETE";
};

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers({ "Content-Type": "application/json" });
  if (options.token) {
    headers.set("Authorization", `Bearer ${options.token}`);
  }

  const response = await fetch(`${API_PREFIX}${path}`, {
    method: options.method ?? "POST",
    headers,
    credentials: "include",
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  // ── 401: try silent token refresh, then retry ─────────────────────────────
  if (response.status === 401 && !AUTH_PATHS.has(path)) {
    if (!refreshPromise) {
      refreshPromise = doRefresh().finally(() => { refreshPromise = null; });
    }
    const newToken = await refreshPromise;
    if (!newToken) {
      authStore.clear();
      window.dispatchEvent(new Event("splitcore:logout"));
      throw new Error("Session expired. Please log in again.");
    }
    // Retry with the fresh access token
    const retryHeaders = new Headers({ "Content-Type": "application/json" });
    retryHeaders.set("Authorization", `Bearer ${newToken}`);
    const retried = await fetch(`${API_PREFIX}${path}`, {
      method: options.method ?? "POST",
      headers: retryHeaders,
      credentials: "include",
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    });
    if (!retried.ok) {
      const retryText = await retried.text().catch(() => "");
      throw new Error(retryText || `Request failed with status ${retried.status}`);
    }
    if (retried.status === 204) return undefined as T;
    const retryBody = await retried.text();
    if (!retryBody || !retryBody.trim()) return undefined as T;
    return JSON.parse(retryBody) as T;
  }
  // ─────────────────────────────────────────────────────────────────────────

  if (!response.ok) {
    const text = await response.text().catch(() => "");
    if (response.status === 403) {
      throw new Error(text || "Access forbidden: only the fund creator has permission for this action");
    }
    if (text) {
      try {
        const parsed = JSON.parse(text);
        if (parsed && typeof parsed.error === "string" && parsed.error.trim()) {
          throw new Error(parsed.error);
        }
      } catch (err) {
        if (err instanceof Error && err.message && !err.message.includes("JSON")) {
          throw err;
        }
      }
    }
    throw new Error(text || `Request failed with status ${response.status}`);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  if (!text || !text.trim()) {
    return undefined as T;
  }

  return JSON.parse(text) as T;
}

const USER_KEY = "splitcore.user";

export const authStore = {
  getToken() {
    return localStorage.getItem(ACCESS_TOKEN_KEY);
  },
  setToken(token: string) {
    localStorage.setItem(ACCESS_TOKEN_KEY, token);
  },
  getUser(): User | null {
    try {
      const raw = localStorage.getItem(USER_KEY);
      return raw ? JSON.parse(raw) : null;
    } catch {
      return null;
    }
  },
  setUser(user: User | null) {
    if (user) {
      localStorage.setItem(USER_KEY, JSON.stringify(user));
    } else {
      localStorage.removeItem(USER_KEY);
    }
  },
  clear() {
    localStorage.removeItem(ACCESS_TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
  },
};

export const api = {
  createSession: () => request<Session>("/auth/telegram/init", { body: {} }),
  checkSession: (uuid: string) =>
    request<string>("/auth/telegram/status", { body: { uuid } }),
  generateTokens: (uuid: string) =>
    request<Tokens>("/auth/telegram/tokens", { body: { uuid } }),
  refreshAccessToken: () => request<Tokens>("/auth/telegram/access", { body: {} }),
  getMe: (token: string) => request<User>("/user/me", { token, body: {} }),

  listFunds: (token: string, limit = 20, offset = 0) =>
    request<Fund[]>("/fund/list", { token, body: { limit, offset } }),
  createFund: (token: string, name: string) =>
    request<Fund>("/fund", { token, body: { name } }),
  joinFund: (token: string, inviteCode: string) =>
    request<Fund>("/fund/join", { token, body: { invite_code: inviteCode } }),
  getFundInfo: (token: string, id: number) =>
    request<Fund>("/fund/info", { token, body: { id } }),
  getMembers: (token: string, id: number) =>
    request<User[]>("/fund/members", { token, body: { id } }),
  getBalance: (token: string, id: number) =>
    request<Settlement>("/fund/balance", { token, body: { id } }),
  getPurchases: (token: string, fundID: number, limit = 12, offset = 0) =>
    request<Purchase[]>("/fund/purchases", {
      token,
      body: { fund_id: fundID, limit, offset }
    }),
  addExpense: (token: string, fundID: number, cost: number, description: string) =>
    request<void>("/fund/expense", {
      token,
      body: { fund_id: fundID, cost, description }
    }),
  addVirtualUser: (token: string, fundID: number, firstName: string) =>
    request<User>("/fund/virtual-users", {
      token,
      body: { fund_id: fundID, first_name: firstName }
    }),
  removeMember: (token: string, fundID: number, userID: number) =>
    request<void>("/fund/member", {
      token,
      method: "DELETE",
      body: { fund_id: fundID, user_id: userID }
    }),
  deleteFund: (token: string, fundID: number) =>
    request<void>("/fund", {
      token,
      method: "DELETE",
      body: { id: fundID }
    })
};
