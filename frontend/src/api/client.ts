import type { Fund, Purchase, Session, Settlement, Tokens, User } from "./types";

const API_PREFIX = "/api/v1";
const ACCESS_TOKEN_KEY = "splitcore.access_token";

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
    body: options.body === undefined ? undefined : JSON.stringify(options.body)
  });

  if (!response.ok) {
    const text = await response.text().catch(() => "");
    throw new Error(text || `Request failed with status ${response.status}`);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

export const authStore = {
  getToken() {
    return localStorage.getItem(ACCESS_TOKEN_KEY);
  },
  setToken(token: string) {
    localStorage.setItem(ACCESS_TOKEN_KEY, token);
  },
  clear() {
    localStorage.removeItem(ACCESS_TOKEN_KEY);
  }
};

export const api = {
  createSession: () => request<Session>("/auth/telegram/init", { body: {} }),
  checkSession: (uuid: string) =>
    request<string>("/auth/telegram/status", { body: { uuid } }),
  generateTokens: (uuid: string) =>
    request<Tokens>("/auth/telegram/tokens", { body: { uuid } }),
  refreshAccessToken: () => request<Tokens>("/auth/telegram/access", { body: {} }),

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
    })
};
