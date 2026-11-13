import type {
  AddExpenseRequest,
  Fund,
  FundPaginationRequest,
  FundRequest,
  JoinFundRequest,
  PaginationRequest,
  Purchase,
  RemoveUserRequest,
  Session,
  SessionCheckRequest,
  SessionStatus,
  Settlement,
  SettlementResponse,
  Tokens,
  User,
  VirtualUserRequest,
} from "./types";

const API_PREFIX = "/api/v1";
const ACCESS_TOKEN_KEY = "splitcore.access_token";
const USER_KEY = "splitcore.user";

// Auth paths that must NOT trigger a refresh loop
const AUTH_PATHS = new Set([
  "/auth/telegram/init",
  "/auth/telegram/status",
  "/auth/telegram/tokens",
  "/auth/telegram/access",
]);

export class ApiError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

export function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

export function getErrorMessage(err: unknown, fallback = "Something went wrong"): string {
  if (err instanceof Error && err.message.trim()) {
    return err.message;
  }
  return fallback;
}

function isPlainRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function extractPayloadError(value: unknown): string | null {
  if (!isPlainRecord(value)) return null;
  if (typeof value.error === "string" && value.error.trim()) {
    return value.error.trim();
  }
  return null;
}

function isValidUser(value: unknown): value is User {
  if (!isPlainRecord(value)) return false;
  return (
    typeof value.id === "number" &&
    Number.isFinite(value.id) &&
    typeof value.first_name === "string" &&
    typeof value.is_virtual === "boolean" &&
    (value.username === undefined || typeof value.username === "string")
  );
}

// Monotonically incremented on explicit logout so in-flight refreshes don't resurrect a logged-out session.
let authGeneration = 0;
// Shared promise – prevents multiple concurrent refresh calls.
let refreshPromise: Promise<string | null> | null = null;

export const authStore = {
  getToken(): string | null {
    try {
      return localStorage.getItem(ACCESS_TOKEN_KEY);
    } catch {
      return null;
    }
  },
  setToken(token: string) {
    try {
      localStorage.setItem(ACCESS_TOKEN_KEY, token);
    } catch {
      // Ignore storage quota / privacy mode errors
    }
  },
  getUser(): User | null {
    try {
      const raw = localStorage.getItem(USER_KEY);
      if (!raw) return null;
      const parsed: unknown = JSON.parse(raw);
      if (!isValidUser(parsed)) {
        localStorage.removeItem(USER_KEY);
        return null;
      }
      return parsed;
    } catch {
      return null;
    }
  },
  setUser(user: User | null) {
    try {
      if (user) {
        localStorage.setItem(USER_KEY, JSON.stringify(user));
      } else {
        localStorage.removeItem(USER_KEY);
      }
    } catch {
      // Ignore storage errors
    }
  },
  clear() {
    authGeneration += 1;
    try {
      localStorage.removeItem(ACCESS_TOKEN_KEY);
      localStorage.removeItem(USER_KEY);
    } catch {
      // Ignore storage errors
    }
  },
};

async function doRefresh(): Promise<string | null> {
  const startGeneration = authGeneration;
  try {
    const res = await fetch(`${API_PREFIX}/auth/telegram/access`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({}),
    });
    if (!res.ok) return null;
    const data: unknown = await res.json();
    if (!isPlainRecord(data) || typeof data.access_token !== "string" || !data.access_token) {
      return null;
    }
    if (startGeneration !== authGeneration) {
      return null;
    }
    authStore.setToken(data.access_token);
    window.dispatchEvent(
      new CustomEvent("splitcore:tokenrefreshed", { detail: data.access_token })
    );
    return data.access_token;
  } catch {
    return null;
  }
}

function triggerSessionExpired(): never {
  authStore.clear();
  window.dispatchEvent(new Event("splitcore:logout"));
  throw new ApiError("Session expired. Please log in again.", 401);
}

async function parseErrorResponse(response: Response): Promise<ApiError> {
  const text = await response.text().catch(() => "");
  if (text) {
    try {
      const parsed: unknown = JSON.parse(text);
      const apiErr = extractPayloadError(parsed);
      if (apiErr) {
        return new ApiError(apiErr, response.status);
      }
    } catch {
      // Plain text error body
    }
  }
  if (response.status === 403) {
    return new ApiError(
      text.trim() || "Access denied: you do not have permission for this action.",
      403
    );
  }
  if (response.status === 401) {
    return new ApiError(text.trim() || "Unauthorized. Please log in again.", 401);
  }
  return new ApiError(text.trim() || `Request failed with status ${response.status}`, response.status);
}

type RequestOptions = {
  token?: string | null;
  body?: unknown;
  method?: "GET" | "POST" | "DELETE";
  signal?: AbortSignal;
};

async function executeFetch(
  path: string,
  options: RequestOptions,
  tokenOverride?: string
): Promise<Response> {
  const headers = new Headers({ "Content-Type": "application/json" });
  const authToken = tokenOverride ?? options.token;
  if (authToken) {
    headers.set("Authorization", `Bearer ${authToken}`);
  }
  return fetch(`${API_PREFIX}${path}`, {
    method: options.method ?? "POST",
    headers,
    credentials: "include",
    signal: options.signal,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });
}

async function sendAuthorized(path: string, options: RequestOptions = {}): Promise<Response> {
  const response = await executeFetch(path, options);

  if (response.status === 401 && !AUTH_PATHS.has(path)) {
    if (!refreshPromise) {
      refreshPromise = doRefresh().finally(() => {
        refreshPromise = null;
      });
    }
    const newToken = await refreshPromise;
    if (!newToken) {
      triggerSessionExpired();
    }

    const retried = await executeFetch(path, options, newToken);
    if (retried.status === 401) {
      triggerSessionExpired();
    }
    if (!retried.ok) {
      throw await parseErrorResponse(retried);
    }
    return retried;
  }

  if (!response.ok) {
    throw await parseErrorResponse(response);
  }

  return response;
}

async function requestJson<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const response = await sendAuthorized(path, options);
  if (response.status === 204) {
    throw new ApiError("Unexpected empty response from server.", response.status);
  }
  const text = await response.text();
  if (!text || !text.trim()) {
    throw new ApiError("Unexpected empty response from server.", response.status);
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw new ApiError("Invalid JSON response from server.", response.status);
  }
  const payloadErr = extractPayloadError(parsed);
  if (payloadErr) {
    throw new ApiError(payloadErr, response.status);
  }
  return parsed as T;
}

async function requestVoid(path: string, options: RequestOptions = {}): Promise<void> {
  const response = await sendAuthorized(path, options);
  if (response.status === 204) {
    return;
  }
  const text = await response.text();
  if (!text || !text.trim()) {
    return;
  }
  try {
    const parsed: unknown = JSON.parse(text);
    const payloadErr = extractPayloadError(parsed);
    if (payloadErr) {
      throw new ApiError(payloadErr, 400);
    }
  } catch (err) {
    if (err instanceof ApiError) throw err;
    // Ignore non-JSON empty/ok text payloads
  }
}

function normalizeObject<T extends Record<string, unknown>>(value: T | null, label: string): T {
  if (!isPlainRecord(value)) {
    throw new ApiError(`Invalid ${label} response from server.`, 500);
  }
  return value;
}

function normalizeArray<T>(value: T[] | null): T[] {
  return Array.isArray(value) ? value : [];
}

function normalizeSettlement(raw: SettlementResponse | null): Settlement {
  const obj = normalizeObject(raw, "settlement");
  return {
    total_amount: typeof obj.total_amount === "number" && Number.isFinite(obj.total_amount)
      ? obj.total_amount
      : 0,
    average: typeof obj.average === "number" && Number.isFinite(obj.average)
      ? obj.average
      : 0,
    debts: Array.isArray(obj.debts) ? obj.debts : [],
  };
}

export const api = {
  createSession: () =>
    requestJson<Session>("/auth/telegram/init", { body: {} }).then((s) =>
      normalizeObject(s, "session")
    ),

  checkSession: (uuid: string, signal?: AbortSignal) =>
    requestJson<SessionStatus>("/auth/telegram/status", {
      body: { uuid } satisfies SessionCheckRequest,
      signal,
    }).then((status) => {
      if (typeof status !== "string" || !status) {
        throw new ApiError("Invalid session status from server.", 500);
      }
      return status;
    }),

  generateTokens: (uuid: string) =>
    requestJson<Tokens>("/auth/telegram/tokens", {
      body: { uuid } satisfies SessionCheckRequest,
    }).then((tokens) => {
      const obj = normalizeObject(tokens, "tokens");
      if (typeof obj.access_token !== "string" || !obj.access_token) {
        throw new ApiError("Missing access token in server response.", 500);
      }
      return obj;
    }),

  refreshAccessToken: () =>
    requestJson<Tokens>("/auth/telegram/access", { body: {} }).then((tokens) =>
      normalizeObject(tokens, "tokens")
    ),

  getMe: (token: string, signal?: AbortSignal) =>
    requestJson<User>("/user/me", { token, body: {}, signal }).then((u) =>
      normalizeObject(u, "user")
    ),

  listFunds: (token: string, limit = 20, offset = 0, signal?: AbortSignal) =>
    requestJson<Fund[] | null>("/fund/list", {
      token,
      body: { limit, offset } satisfies PaginationRequest,
      signal,
    }).then(normalizeArray),

  createFund: (token: string, name: string) =>
    requestJson<Fund>("/fund", { token, body: { name } }).then((f) =>
      normalizeObject(f, "fund")
    ),

  joinFund: (token: string, inviteCode: string) =>
    requestJson<Fund>("/fund/join", {
      token,
      body: { invite_code: inviteCode } satisfies JoinFundRequest,
    }).then((f) => normalizeObject(f, "fund")),

  getFundInfo: (token: string, id: number, signal?: AbortSignal) =>
    requestJson<Fund>("/fund/info", {
      token,
      body: { id } satisfies FundRequest,
      signal,
    }).then((f) => normalizeObject(f, "fund")),

  getMembers: (token: string, id: number, signal?: AbortSignal) =>
    requestJson<User[] | null>("/fund/members", {
      token,
      body: { id } satisfies FundRequest,
      signal,
    }).then(normalizeArray),

  getBalance: (token: string, id: number, signal?: AbortSignal) =>
    requestJson<SettlementResponse>("/fund/balance", {
      token,
      body: { id } satisfies FundRequest,
      signal,
    }).then(normalizeSettlement),

  getPurchases: (token: string, fundID: number, limit = 50, offset = 0, signal?: AbortSignal) =>
    requestJson<Purchase[] | null>("/fund/purchases", {
      token,
      body: { fund_id: fundID, limit, offset } satisfies FundPaginationRequest,
      signal,
    }).then(normalizeArray),

  addExpense: (token: string, fundID: number, cost: number, description: string) =>
    requestVoid("/fund/expense", {
      token,
      body: { fund_id: fundID, cost, description } satisfies AddExpenseRequest,
    }),

  addVirtualUser: (token: string, fundID: number, firstName: string) =>
    requestJson<User>("/fund/virtual-users", {
      token,
      body: { fund_id: fundID, first_name: firstName } satisfies VirtualUserRequest,
    }).then((u) => normalizeObject(u, "virtual user")),

  removeMember: (token: string, fundID: number, userID: number) =>
    requestVoid("/fund/member", {
      token,
      method: "DELETE",
      body: { fund_id: fundID, user_id: userID } satisfies RemoveUserRequest,
    }),

  deleteFund: (token: string, fundID: number) =>
    requestVoid("/fund", {
      token,
      method: "DELETE",
      body: { id: fundID },
    }),
};
