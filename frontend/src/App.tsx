import {
  AlertCircle,
  ArrowRight,
  BadgeDollarSign,
  BanknoteArrowUp,
  Calendar,
  CheckCircle,
  ChevronRight,
  Copy,
  Crown,
  Loader2,
  LogOut,
  Moon,
  Plus,
  ReceiptText,
  RefreshCw,
  Send,
  ShieldCheck,
  Sun,
  Trash2,
  User as UserIcon,
  UserPlus,
  Users,
  Wallet,
  X,
  XCircle,
} from "lucide-react";
import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ApiError, api, authStore, getErrorMessage, isAbortError } from "./api/client";
import type { Debt, Fund, Purchase, SessionStatus, Settlement, User } from "./api/types";
import { displayUser, fullDateTime, money, shortDate } from "./lib/format";

const botName = import.meta.env.VITE_BOT_NAME || "SplitCoreBot";

type FundDetails = {
  fundId?: number;
  fund?: Fund;
  members: User[];
  settlement?: Settlement;
  purchases: Purchase[];
};

const EMPTY_DETAILS: FundDetails = {
  members: [],
  purchases: [],
};

type Toast = {
  id: number;
  type: "success" | "error";
  message: string;
};

type BusyAction = "create" | "join" | "delete" | "expense" | "addMember" | null;
type AuthUIStatus = SessionStatus | "idle" | "expired" | "error";

// ─── Utility ────────────────────────────────────────────────────────────────

function parseJwtUserId(jwtToken: string | null): number | null {
  if (!jwtToken) return null;
  try {
    const parts = jwtToken.split(".");
    if (parts.length < 2) return null;
    const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), "=");
    const jsonPayload = decodeURIComponent(
      atob(padded)
        .split("")
        .map((c) => "%" + ("00" + c.charCodeAt(0).toString(16)).slice(-2))
        .join("")
    );
    const data: unknown = JSON.parse(jsonPayload);
    if (typeof data === "object" && data !== null && "user_id" in data) {
      const uid = (data as { user_id: unknown }).user_id;
      return typeof uid === "number" && Number.isFinite(uid) ? uid : null;
    }
    return null;
  } catch {
    return null;
  }
}

function telegramLink(uuid: string) {
  return `https://t.me/${botName}?start=auth_${uuid}`;
}

let toastCounter = 0;

// ─── App ─────────────────────────────────────────────────────────────────────

function App() {
  const [token, setToken] = useState(() => authStore.getToken());
  const [currentUser, setCurrentUser] = useState<User | null>(() => authStore.getUser());
  const [theme, setTheme] = useState<"dark" | "light">(() => {
    try {
      const saved = localStorage.getItem("splitcore.theme");
      return saved === "light" ? "light" : "dark";
    } catch {
      return "dark";
    }
  });

  const [funds, setFunds] = useState<Fund[]>([]);
  const [fundsLoading, setFundsLoading] = useState(false);
  const [fundsError, setFundsError] = useState<string | null>(null);

  const [activeFundID, setActiveFundID] = useState<number | null>(null);
  const [details, setDetails] = useState<FundDetails>(EMPTY_DETAILS);
  const [detailsLoading, setDetailsLoading] = useState(false);
  const [detailsError, setDetailsError] = useState<string | null>(null);

  const [sessionID, setSessionID] = useState("");
  const [status, setStatus] = useState<AuthUIStatus>("idle");
  const [authStarting, setAuthStarting] = useState(false);
  const [authError, setAuthError] = useState<string | null>(null);
  const [authPopupBlocked, setAuthPopupBlocked] = useState(false);

  const [busyAction, setBusyAction] = useState<BusyAction>(null);
  const [removingUserID, setRemovingUserID] = useState<number | null>(null);
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [deleteConfirm, setDeleteConfirm] = useState<number | null>(null);

  const authPopupRef = useRef<Window | null>(null);
  const authAttemptRef = useRef(0);
  const tokenRef = useRef(token);
  tokenRef.current = token;
  const fundsReqRef = useRef(0);
  const fundsAbortRef = useRef<AbortController | null>(null);
  const detailsReqRef = useRef(0);
  const detailsAbortRef = useRef<AbortController | null>(null);
  const toastTimersRef = useRef<Set<number>>(new Set());

  const busy = busyAction !== null || removingUserID !== null;

  // Clean up toast timers on unmount
  useEffect(() => {
    const timers = toastTimersRef.current;
    return () => {
      timers.forEach((timerId) => window.clearTimeout(timerId));
      timers.clear();
    };
  }, []);

  const toast = useCallback((type: "success" | "error", message: string) => {
    const id = ++toastCounter;
    setToasts((prev) => [...prev, { id, type, message }]);
    const timerId = window.setTimeout(() => {
      toastTimersRef.current.delete(timerId);
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 4000);
    toastTimersRef.current.add(timerId);
  }, []);

  // Sync theme class to documentElement
  useEffect(() => {
    if (theme === "light") {
      document.documentElement.classList.add("light");
      document.documentElement.classList.remove("dark");
    } else {
      document.documentElement.classList.remove("light");
      document.documentElement.classList.add("dark");
    }
    try {
      localStorage.setItem("splitcore.theme", theme);
    } catch {
      // Ignore storage errors
    }
  }, [theme]);

  const toggleTheme = useCallback(() => {
    setTheme((prev) => (prev === "dark" ? "light" : "dark"));
  }, []);

  const currentUserID = useMemo(() => parseJwtUserId(token), [token]);
  // Key changes on login/logout/user-change, NOT on silent access-token refresh of the same user.
  const authSessionKey = token ? String(currentUserID ?? "session") : null;

  // Fetch current user info whenever authenticated user session changes
  useEffect(() => {
    const activeToken = authStore.getToken() ?? tokenRef.current;
    if (!authSessionKey || !activeToken) {
      setCurrentUser(null);
      return;
    }
    const controller = new AbortController();
    let cancelled = false;
    api
      .getMe(activeToken, controller.signal)
      .then((u) => {
        if (!cancelled) {
          setCurrentUser(u);
          authStore.setUser(u);
        }
      })
      .catch((err) => {
        if (cancelled || isAbortError(err)) return;
        // Non-fatal: fallback to cached user or fund members list if getMe fails transiently
      });
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [authSessionKey]);

  const hasLoadedActiveDetails = activeFundID !== null && details.fundId === activeFundID;
  const activeFund = useMemo(() => {
    if (activeFundID === null) return undefined;
    if (details.fund && details.fund.id === activeFundID) {
      return details.fund;
    }
    return funds.find((f) => f.id === activeFundID);
  }, [activeFundID, details.fund, funds]);

  const isOwnerOfActiveFund = Boolean(
    activeFund && currentUserID !== null && activeFund.author_id === currentUserID
  );
  const activeFundOwner = useMemo(() => {
    if (!activeFund || !hasLoadedActiveDetails) return undefined;
    return details.members.find((m) => m.id === activeFund.author_id);
  }, [activeFund, details.members, hasLoadedActiveDetails]);

  const resolvedCurrentUser = useMemo(() => {
    if (currentUser) return currentUser;
    if (currentUserID !== null && hasLoadedActiveDetails) {
      const fromMembers = details.members.find((m) => m.id === currentUserID);
      if (fromMembers) return fromMembers;
    }
    return null;
  }, [currentUser, currentUserID, details.members, hasLoadedActiveDetails]);

  const totalMembers = hasLoadedActiveDetails ? details.members.length : 0;
  const totalSpent = hasLoadedActiveDetails ? (details.settlement?.total_amount ?? 0) : 0;
  const average = hasLoadedActiveDetails ? (details.settlement?.average ?? 0) : 0;

  const debtRows = useMemo(() => {
    if (!hasLoadedActiveDetails) return [];
    const byID = new Map(details.members.map((m) => [m.id, m]));
    return (details.settlement?.debts ?? []).map((debt) => ({
      ...debt,
      from: displayUser(byID.get(debt.from_id)),
      to: displayUser(byID.get(debt.to_id)),
    }));
  }, [details.members, details.settlement, hasLoadedActiveDetails]);

  const getFreshToken = useCallback(
    (explicitToken?: string | null) => authStore.getToken() ?? explicitToken ?? tokenRef.current,
    []
  );

  const loadFunds = useCallback(
    async (nextToken?: string | null): Promise<boolean> => {
      const activeToken = getFreshToken(nextToken);
      if (!activeToken) return false;
      const reqId = ++fundsReqRef.current;
      fundsAbortRef.current?.abort();
      const controller = new AbortController();
      fundsAbortRef.current = controller;

      setFundsLoading(true);
      setFundsError(null);
      try {
        const list = await api.listFunds(activeToken, 20, 0, controller.signal);
        if (reqId !== fundsReqRef.current || controller.signal.aborted) {
          return false;
        }
        setFunds(list);
        setActiveFundID((prev) => {
          if (prev !== null && list.some((f) => f.id === prev)) {
            return prev;
          }
          return list[0]?.id ?? null;
        });
        return true;
      } catch (err) {
        if (isAbortError(err) || reqId !== fundsReqRef.current) {
          return false;
        }
        const message = getErrorMessage(err, "Failed to load funds");
        setFundsError(message);
        return false;
      } finally {
        if (reqId === fundsReqRef.current) {
          setFundsLoading(false);
        }
      }
    },
    [getFreshToken]
  );

  const loadDetails = useCallback(
    async (fundID: number, nextToken?: string | null): Promise<boolean> => {
      const activeToken = getFreshToken(nextToken);
      if (!activeToken) return false;
      const reqId = ++detailsReqRef.current;
      detailsAbortRef.current?.abort();
      const controller = new AbortController();
      detailsAbortRef.current = controller;

      setDetailsLoading(true);
      setDetailsError(null);
      try {
        const [fund, members, settlement, purchases] = await Promise.all([
          api.getFundInfo(activeToken, fundID, controller.signal),
          api.getMembers(activeToken, fundID, controller.signal),
          api.getBalance(activeToken, fundID, controller.signal),
          api.getPurchases(activeToken, fundID, 50, 0, controller.signal),
        ]);
        if (reqId !== detailsReqRef.current || controller.signal.aborted) {
          return false;
        }
        setDetails({
          fundId: fundID,
          fund,
          members,
          settlement,
          purchases,
        });
        return true;
      } catch (err) {
        if (isAbortError(err) || reqId !== detailsReqRef.current) {
          return false;
        }
        const message =
          err instanceof ApiError && err.status === 403
            ? "You no longer have access to this fund."
            : err instanceof ApiError && err.status === 404
            ? "This fund no longer exists."
            : getErrorMessage(err, "Failed to load fund details");
        setDetailsError(message);
        return false;
      } finally {
        if (reqId === detailsReqRef.current) {
          setDetailsLoading(false);
        }
      }
    },
    [getFreshToken]
  );

  useEffect(() => {
    if (!authSessionKey) return;
    void loadFunds();
    return () => {
      fundsAbortRef.current?.abort();
    };
  }, [authSessionKey, loadFunds]);

  useEffect(() => {
    if (!activeFundID || !authSessionKey) {
      detailsAbortRef.current?.abort();
      setDetails(EMPTY_DETAILS);
      setDetailsLoading(false);
      setDetailsError(null);
      return;
    }
    void loadDetails(activeFundID);
    return () => {
      detailsAbortRef.current?.abort();
    };
  }, [activeFundID, authSessionKey, loadDetails]);

  // Close delete modal on Escape when not submitting
  useEffect(() => {
    if (deleteConfirm === null) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) {
        setDeleteConfirm(null);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [deleteConfirm, busy]);

  // ── Listen for silent token refresh / forced logout from client.ts ────────
  useEffect(() => {
    const onRefreshed = (e: Event) => {
      const newToken = (e as CustomEvent<string>).detail;
      if (typeof newToken === "string" && newToken) {
        authStore.setToken(newToken);
        setToken(newToken);
      }
    };
    const onLogout = () => {
      fundsAbortRef.current?.abort();
      detailsAbortRef.current?.abort();
      authStore.clear();
      setToken(null);
      setCurrentUser(null);
      setFunds([]);
      setFundsLoading(false);
      setFundsError(null);
      setActiveFundID(null);
      setDetails(EMPTY_DETAILS);
      setDetailsLoading(false);
      setDetailsError(null);
      setDeleteConfirm(null);
    };
    window.addEventListener("splitcore:tokenrefreshed", onRefreshed);
    window.addEventListener("splitcore:logout", onLogout);
    return () => {
      window.removeEventListener("splitcore:tokenrefreshed", onRefreshed);
      window.removeEventListener("splitcore:logout", onLogout);
    };
  }, []);

  // ── Sequential Telegram auth session polling ──────────────────────────────
  useEffect(() => {
    if (!sessionID || status !== "pending" || token) return;

    let cancelled = false;
    let timerId: number | null = null;
    let controller: AbortController | null = null;
    let consecutiveErrors = 0;

    const poll = async () => {
      if (cancelled) return;
      controller = new AbortController();
      try {
        const nextStatus = await api.checkSession(sessionID, controller.signal);
        if (cancelled) return;
        consecutiveErrors = 0;

        if (nextStatus === "authenticated") {
          if (authPopupRef.current && !authPopupRef.current.closed) {
            authPopupRef.current.close();
            authPopupRef.current = null;
          }
          const tokens = await api.generateTokens(sessionID);
          if (cancelled) return;

          setStatus("authenticated");
          setAuthError(null);
          setAuthPopupBlocked(false);
          authStore.setToken(tokens.access_token);
          setToken(tokens.access_token);
          toast("success", "Telegram connected!");
          return;
        }

        setStatus(nextStatus);
        if (nextStatus === "pending") {
          timerId = window.setTimeout(poll, 2200);
        }
      } catch (err) {
        if (cancelled || isAbortError(err)) return;

        // Backend returns 400 when session does not exist or expired (3 min TTL in Redis)
        if (err instanceof ApiError && err.status === 400) {
          const msg = "Login session expired. Click \"Connect with Telegram\" to start a new one.";
          setStatus("expired");
          setAuthError(msg);
          toast("error", msg);
          return;
        }

        consecutiveErrors += 1;
        if (consecutiveErrors >= 3) {
          const msg = getErrorMessage(err, "Failed to verify Telegram auth status.");
          setStatus("error");
          setAuthError(msg);
          toast("error", msg);
          return;
        }

        timerId = window.setTimeout(poll, 2200);
      }
    };

    timerId = window.setTimeout(poll, 2200);

    return () => {
      cancelled = true;
      if (timerId !== null) window.clearTimeout(timerId);
      controller?.abort();
    };
  }, [sessionID, status, token, toast]);

  async function startTelegramAuth() {
    const attempt = ++authAttemptRef.current;
    setAuthStarting(true);
    setAuthError(null);
    setAuthPopupBlocked(false);
    try {
      const session = await api.createSession();
      if (attempt !== authAttemptRef.current) return;

      setSessionID(session.uuid);
      setStatus(session.status);

      const tgWebUrl = telegramLink(session.uuid);
      const tgAppUrl = `tg://resolve?domain=${botName}&start=auth_${session.uuid}`;

      const iframe = document.createElement("iframe");
      iframe.style.display = "none";
      iframe.src = tgAppUrl;
      document.body.appendChild(iframe);
      window.setTimeout(() => iframe.remove(), 2000);

      const width = 540;
      const height = 670;
      const left = window.screenX + Math.max(0, (window.outerWidth - width) / 2);
      const top = window.screenY + Math.max(0, (window.outerHeight - height) / 2);

      const popup = window.open(
        tgWebUrl,
        "SplitCoreTelegramAuth",
        `width=${width},height=${height},left=${left},top=${top},scrollbars=yes,resizable=yes`
      );
      authPopupRef.current = popup;
      if (!popup) {
        setAuthPopupBlocked(true);
      }
    } catch (err) {
      if (attempt !== authAttemptRef.current) return;
      const message = getErrorMessage(err, "Failed to start auth");
      setAuthError(message);
      setStatus("error");
      toast("error", message);
    } finally {
      if (attempt === authAttemptRef.current) {
        setAuthStarting(false);
      }
    }
  }

  async function refreshAll() {
    const activeToken = getFreshToken();
    if (!activeToken) return;
    const okFunds = await loadFunds(activeToken);
    if (!okFunds) {
      toast("error", "Failed to refresh funds");
      return;
    }
    if (activeFundID) {
      const okDetails = await loadDetails(activeFundID, activeToken);
      if (!okDetails) {
        toast("error", "Failed to refresh fund details");
      }
    }
  }

  async function submitCreateFund(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const activeToken = getFreshToken();
    if (!activeToken || busy) return;
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    const name = String(form.get("name") ?? "").trim();
    if (!name) {
      toast("error", "Fund name is required");
      return;
    }
    setBusyAction("create");
    try {
      const fund = await api.createFund(activeToken, name);
      formEl.reset();
      setFunds((prev) => [fund, ...prev.filter((f) => f.id !== fund.id)]);
      setFundsError(null);
      setActiveFundID(fund.id);
      toast("success", `Fund "${fund.name}" created!`);
    } catch (err) {
      toast("error", getErrorMessage(err, "Failed to create fund"));
    } finally {
      setBusyAction(null);
    }
  }

  async function submitJoinFund(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const activeToken = getFreshToken();
    if (!activeToken || busy) return;
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    const code = String(form.get("invite") ?? "").trim();
    if (!code) {
      toast("error", "Invite code is required");
      return;
    }
    setBusyAction("join");
    try {
      const fund = await api.joinFund(activeToken, code);
      formEl.reset();
      setFunds((prev) => (prev.some((f) => f.id === fund.id) ? prev : [fund, ...prev]));
      setFundsError(null);
      toast("success", `Joined "${fund.name}"!`);
      if (activeFundID === fund.id) {
        await loadDetails(fund.id);
      } else {
        setActiveFundID(fund.id);
      }
    } catch (err) {
      const msg =
        err instanceof ApiError && (err.status === 400 || err.status === 404)
          ? "Invalid invite code or fund not found."
          : err instanceof ApiError && err.status === 409
          ? "You are already a member of this fund."
          : getErrorMessage(err, "Failed to join fund");
      toast("error", msg);
    } finally {
      setBusyAction(null);
    }
  }

  async function confirmDeleteFund(fundID: number) {
    const activeToken = getFreshToken();
    if (!activeToken || busy) return;
    setBusyAction("delete");
    try {
      await api.deleteFund(activeToken, fundID);
      setDeleteConfirm(null);
      const remaining = funds.filter((f) => f.id !== fundID);
      setFunds(remaining);
      if (activeFundID === fundID) {
        const next = remaining[0] ?? null;
        setDetails(EMPTY_DETAILS);
        setDetailsError(null);
        setActiveFundID(next?.id ?? null);
      }
      toast("success", "Fund deleted");
    } catch (err) {
      toast("error", getErrorMessage(err, "Failed to delete fund"));
    } finally {
      setBusyAction(null);
    }
  }

  async function submitExpense(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const activeToken = getFreshToken();
    if (!activeToken || !activeFundID || busy) return;
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    const cost = Number(form.get("cost"));
    const description = String(form.get("description") ?? "").trim();
    if (!description) {
      toast("error", "Description is required");
      return;
    }
    if (!Number.isFinite(cost) || cost <= 0) {
      toast("error", "Expense amount must be greater than 0");
      return;
    }
    setBusyAction("expense");
    try {
      await api.addExpense(activeToken, activeFundID, cost, description);
      formEl.reset();
      toast("success", "Expense added!");
      await loadDetails(activeFundID);
    } catch (err) {
      toast("error", getErrorMessage(err, "Failed to add expense"));
    } finally {
      setBusyAction(null);
    }
  }

  async function submitVirtualUser(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const activeToken = getFreshToken();
    if (!activeToken || !activeFundID || busy) return;
    const formEl = event.currentTarget;
    const form = new FormData(formEl);
    const firstName = String(form.get("firstName") ?? "").trim();
    if (!firstName) {
      toast("error", "Member name is required");
      return;
    }
    setBusyAction("addMember");
    try {
      await api.addVirtualUser(activeToken, activeFundID, firstName);
      formEl.reset();
      toast("success", "Member added!");
      await loadDetails(activeFundID);
    } catch (err) {
      toast("error", getErrorMessage(err, "Failed to add member"));
    } finally {
      setBusyAction(null);
    }
  }

  async function removeMember(userID: number) {
    const activeToken = getFreshToken();
    if (!activeToken || !activeFundID || busy) return;
    setRemovingUserID(userID);
    try {
      await api.removeMember(activeToken, activeFundID, userID);
      toast("success", "Member removed");
      await loadDetails(activeFundID);
    } catch (err) {
      toast("error", getErrorMessage(err, "Failed to remove member"));
    } finally {
      setRemovingUserID(null);
    }
  }

  function copyInviteCode(code: string) {
    if (!navigator.clipboard?.writeText) {
      toast("error", "Clipboard is not available in this browser context");
      return;
    }
    navigator.clipboard
      .writeText(code)
      .then(() => toast("success", "Invite code copied!"))
      .catch(() => toast("error", "Failed to copy invite code"));
  }

  function logout() {
    authAttemptRef.current += 1;
    fundsAbortRef.current?.abort();
    detailsAbortRef.current?.abort();
    if (authPopupRef.current && !authPopupRef.current.closed) {
      authPopupRef.current.close();
      authPopupRef.current = null;
    }
    authStore.clear();
    setToken(null);
    setCurrentUser(null);
    setFunds([]);
    setFundsLoading(false);
    setFundsError(null);
    setActiveFundID(null);
    setDetails(EMPTY_DETAILS);
    setDetailsLoading(false);
    setDetailsError(null);
    setDeleteConfirm(null);
    setSessionID("");
    setStatus("idle");
    setAuthStarting(false);
    setAuthError(null);
    setAuthPopupBlocked(false);
  }

  return (
    <div className="min-h-screen text-sc-text font-sans antialiased">
      {/* Toast stack */}
      <div className="fixed bottom-6 right-4 left-4 sm:left-auto sm:right-6 z-50 flex flex-col gap-2 items-end pointer-events-none">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`pointer-events-auto flex items-center gap-3 rounded-2xl px-4 py-3 text-sm font-semibold shadow-2xl backdrop-blur-xl border max-w-sm w-full sm:w-auto transition-all
              ${
                t.type === "success"
                  ? "bg-sc-green/15 border-sc-green/30 text-sc-green"
                  : "bg-sc-red/15 border-sc-red/30 text-sc-red"
              }`}
          >
            {t.type === "success" ? <CheckCircle size={16} /> : <XCircle size={16} />}
            <span>{t.message}</span>
          </div>
        ))}
      </div>

      {/* Delete confirmation modal */}
      {deleteConfirm !== null && (
        <div
          className="fixed inset-0 z-40 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm"
          onClick={() => !busy && setDeleteConfirm(null)}
        >
          <div
            className="sc-card rounded-3xl p-6 max-w-sm w-full"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center gap-3 mb-4">
              <div className="size-10 rounded-2xl bg-sc-red/15 grid place-items-center text-sc-red flex-shrink-0">
                <Trash2 size={18} />
              </div>
              <div>
                <p className="font-bold text-sc-text">Delete fund?</p>
                <p className="text-sm text-sc-muted">This action cannot be undone.</p>
              </div>
            </div>
            <div className="flex gap-2 mt-5">
              <button
                type="button"
                onClick={() => setDeleteConfirm(null)}
                disabled={busy}
                className="flex-1 sc-btn-soft rounded-2xl py-2.5 text-sm font-bold disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => confirmDeleteFund(deleteConfirm)}
                disabled={busy}
                className="flex-1 bg-sc-red/90 hover:bg-sc-red text-white rounded-2xl py-2.5 text-sm font-bold transition disabled:opacity-50"
              >
                {busyAction === "delete" ? "Deleting…" : "Delete"}
              </button>
            </div>
          </div>
        </div>
      )}

      {!token ? (
        <AuthPage
          busy={authStarting}
          status={status}
          sessionID={sessionID}
          error={authError}
          popupBlocked={authPopupBlocked}
          onStart={startTelegramAuth}
        />
      ) : (
        <div className="flex flex-col min-h-screen">
          {/* Navbar */}
          <header
            className="sticky top-0 z-30 border-b border-sc-border backdrop-blur-xl"
            style={{ background: "var(--sc-header-bg)" }}
          >
            <div className="mx-auto max-w-7xl px-4 sm:px-6 h-16 flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <img src="/logo.png" alt="SplitCore" className="size-9 object-contain flex-shrink-0" />
                <span className="font-black text-lg tracking-tight">SplitCore</span>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => void refreshAll()}
                  disabled={fundsLoading || detailsLoading}
                  className="sc-btn-ghost rounded-xl p-2.5 disabled:opacity-50"
                  title="Refresh"
                  aria-label="Refresh funds and active fund details"
                >
                  <RefreshCw size={17} className={fundsLoading || detailsLoading ? "animate-spin" : ""} />
                </button>
                <button
                  type="button"
                  onClick={toggleTheme}
                  className="sc-btn-ghost rounded-xl p-2.5"
                  title={theme === "dark" ? "Switch to Light mode" : "Switch to Dark mode"}
                >
                  {theme === "dark" ? <Moon size={17} /> : <Sun size={17} />}
                </button>
                <button
                  type="button"
                  onClick={logout}
                  className="sc-btn-ghost rounded-xl p-2.5 text-sc-muted hover:text-sc-red"
                  title="Logout"
                  aria-label="Logout"
                >
                  <LogOut size={17} />
                </button>
                {resolvedCurrentUser && (
                  <div className="flex items-center gap-1 px-3 py-1.5 rounded-xl bg-sc-surface border border-sc-border text-sm text-sc-text">
                    <UserIcon size={14} className="text-sc-muted" />
                    <span>{resolvedCurrentUser.username || resolvedCurrentUser.first_name || "User"}</span>
                  </div>
                )}
              </div>
            </div>
          </header>

          <div className="flex-1 mx-auto w-full max-w-7xl px-4 sm:px-6 py-6">
            <div className="grid gap-6 lg:grid-cols-[300px_1fr]">
              {/* Sidebar */}
              <aside className="space-y-4">
                {/* Create / Join */}
                <div className="sc-card rounded-3xl p-5 space-y-4">
                  <form onSubmit={submitCreateFund} className="space-y-3">
                    <label className="block text-xs font-bold uppercase tracking-wider text-sc-muted">
                      New fund
                    </label>
                    <input
                      name="name"
                      placeholder="Trip to Bali, Roommates…"
                      className="sc-input w-full rounded-2xl px-4 py-3 text-sm"
                    />
                    <button
                      type="submit"
                      disabled={busy}
                      className="sc-btn-primary w-full rounded-2xl py-2.5 text-sm font-bold flex items-center justify-center gap-2 disabled:opacity-50"
                    >
                      <Plus size={16} /> {busyAction === "create" ? "Creating…" : "Create"}
                    </button>
                  </form>

                  <div className="border-t border-sc-border" />

                  <form onSubmit={submitJoinFund} className="space-y-3">
                    <label className="block text-xs font-bold uppercase tracking-wider text-sc-muted">
                      Join by code
                    </label>
                    <input
                      name="invite"
                      placeholder="aB12cD"
                      className="sc-input w-full rounded-2xl px-4 py-3 text-sm font-mono"
                    />
                    <button
                      type="submit"
                      disabled={busy}
                      className="sc-btn-soft w-full rounded-2xl py-2.5 text-sm font-bold flex items-center justify-center gap-2 disabled:opacity-50"
                    >
                      <ArrowRight size={16} /> {busyAction === "join" ? "Joining…" : "Join"}
                    </button>
                  </form>
                </div>

                {/* Fund list */}
                <div className="sc-card rounded-3xl p-4">
                  <div className="flex items-center justify-between px-1 mb-3">
                    <h2 className="text-sm font-bold uppercase tracking-wider text-sc-muted">My funds</h2>
                    <span className="text-xs bg-sc-surface rounded-lg px-2 py-0.5 font-bold text-sc-muted">
                      {funds.length}
                    </span>
                  </div>

                  {fundsError && (
                    <div className="mb-3 rounded-2xl border border-sc-red/30 bg-sc-red/10 p-3 text-xs text-sc-red space-y-2">
                      <div className="flex items-start gap-2">
                        <AlertCircle size={15} className="flex-shrink-0 mt-0.5" />
                        <span className="font-medium break-words">{fundsError}</span>
                      </div>
                      <button
                        type="button"
                        onClick={() => void loadFunds(token)}
                        disabled={fundsLoading}
                        className="sc-btn-soft w-full rounded-xl py-1.5 text-xs font-bold disabled:opacity-50"
                      >
                        {fundsLoading ? "Retrying…" : "Retry"}
                      </button>
                    </div>
                  )}

                  <div className="space-y-1 max-h-[380px] overflow-y-auto pr-1">
                    {fundsLoading && funds.length === 0 ? (
                      <div className="text-center py-8 text-sc-muted text-sm space-y-2">
                        <Loader2 size={24} className="mx-auto animate-spin opacity-60" />
                        <p>Loading funds…</p>
                      </div>
                    ) : (
                      <>
                        {funds.map((fund) => (
                          <div
                            key={fund.id}
                            className={`group relative flex items-center rounded-2xl transition cursor-pointer
                              ${
                                activeFundID === fund.id
                                  ? "bg-sc-green/12 text-sc-text"
                                  : "hover:bg-sc-surface text-sc-text/80 hover:text-sc-text"
                              }`}
                          >
                            <button
                              type="button"
                              className="flex-1 flex items-center gap-3 p-3 text-left min-w-0"
                              onClick={() => setActiveFundID(fund.id)}
                            >
                              <div
                                className={`size-8 rounded-xl grid place-items-center flex-shrink-0 text-sm font-black
                                  ${
                                    activeFundID === fund.id
                                      ? "bg-sc-green/20 text-sc-green"
                                      : "bg-sc-surface text-sc-muted"
                                  }`}
                              >
                                <Wallet size={14} />
                              </div>
                              <div className="min-w-0">
                                <p className="truncate text-sm font-bold">{fund.name}</p>
                                <p className="text-xs text-sc-muted font-mono">{fund.invite_code}</p>
                              </div>
                              {activeFundID === fund.id && (
                                <ChevronRight size={14} className="flex-shrink-0 text-sc-green ml-auto" />
                              )}
                            </button>
                            {currentUserID !== null && fund.author_id === currentUserID && (
                              <button
                                type="button"
                                disabled={busy}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setDeleteConfirm(fund.id);
                                }}
                                className="opacity-0 group-hover:opacity-100 p-2 mr-1 rounded-xl text-sc-muted hover:text-sc-red hover:bg-sc-red/10 transition disabled:opacity-30"
                                title="Delete fund"
                              >
                                <Trash2 size={14} />
                              </button>
                            )}
                          </div>
                        ))}
                        {!fundsLoading && !fundsError && funds.length === 0 && (
                          <div className="text-center py-8 text-sc-muted text-sm">
                            <Wallet size={28} className="mx-auto mb-2 opacity-30" />
                            No funds yet
                          </div>
                        )}
                      </>
                    )}
                  </div>
                </div>
              </aside>

              {/* Main content */}
              <main className="space-y-5 min-w-0">
                {activeFund ? (
                  <>
                    {/* Fund hero */}
                    <div className="sc-card rounded-3xl overflow-hidden">
                      <div className="h-1.5 bg-gradient-to-r from-sc-green via-sc-blue to-sc-purple" />
                      <div className="p-6 space-y-4">
                        <div className="flex flex-wrap items-start justify-between gap-4">
                          <div>
                            <div className="flex items-center gap-2 mb-1">
                              <p className="text-xs font-bold uppercase tracking-wider text-sc-green">
                                Active fund
                              </p>
                              {isOwnerOfActiveFund ? (
                                <span className="flex items-center gap-1 text-xs font-bold px-2 py-0.5 rounded-lg bg-sc-amber/15 text-sc-amber">
                                  <Crown size={10} /> Owner
                                </span>
                              ) : (
                                <span className="text-xs font-bold px-2 py-0.5 rounded-lg bg-sc-surface text-sc-muted">
                                  Member
                                </span>
                              )}
                              {detailsLoading && (
                                <span className="inline-flex items-center gap-1 text-xs font-semibold text-sc-muted">
                                  <Loader2 size={12} className="animate-spin" /> Syncing…
                                </span>
                              )}
                            </div>
                            <h1 className="text-2xl sm:text-3xl font-black tracking-tight">
                              {activeFund.name}
                            </h1>
                          </div>
                          <div className="flex items-center gap-2">
                            <button
                              type="button"
                              onClick={() => copyInviteCode(activeFund.invite_code)}
                              className="flex items-center gap-3 rounded-2xl border border-sc-amber/30 bg-sc-amber/8 hover:bg-sc-amber/14 px-4 py-3 text-sc-amber transition group"
                              title="Copy invite code"
                            >
                              <div>
                                <p className="text-xs font-bold uppercase opacity-60">Invite code</p>
                                <p className="font-mono text-xl font-black">{activeFund.invite_code}</p>
                              </div>
                              <Copy size={16} className="group-hover:scale-110 transition" />
                            </button>
                            {isOwnerOfActiveFund && (
                              <button
                                type="button"
                                disabled={busy}
                                onClick={() => setDeleteConfirm(activeFund.id)}
                                className="flex items-center gap-2 rounded-2xl border border-sc-red/30 bg-sc-red/10 hover:bg-sc-red/20 px-3.5 py-3.5 text-sc-red transition disabled:opacity-50"
                                title="Delete fund (Owner only)"
                              >
                                <Trash2 size={18} />
                              </button>
                            )}
                          </div>
                        </div>

                        {/* Fund meta info row */}
                        <div className="flex flex-wrap gap-3 pt-1 border-t border-sc-border">
                          <div className="flex items-center gap-1.5 text-sm text-sc-muted">
                            <Calendar size={13} className="text-sc-blue" />
                            <span className="font-medium">Created</span>
                            <span className="text-sc-text font-semibold">
                              {fullDateTime(activeFund.created_at)}
                            </span>
                          </div>
                          <div className="flex items-center gap-1.5 text-sm text-sc-muted">
                            <ShieldCheck size={13} className="text-sc-purple" />
                            <span className="font-medium">Owner</span>
                            <span className="text-sc-text font-semibold">
                              {activeFundOwner ? displayUser(activeFundOwner) : `#${activeFund.author_id}`}
                            </span>
                          </div>
                          <div className="flex items-center gap-1.5 text-sm text-sc-muted">
                            <UserIcon size={13} className="text-sc-green" />
                            <span className="font-medium">Fund ID</span>
                            <span className="text-sc-text font-semibold">#{activeFund.id}</span>
                          </div>
                        </div>
                      </div>
                    </div>

                    {detailsError && (
                      <div className="sc-card rounded-3xl p-4 border border-sc-red/30 bg-sc-red/10 flex flex-wrap items-center justify-between gap-3 text-sm text-sc-red">
                        <div className="flex items-center gap-2.5 min-w-0">
                          <AlertCircle size={18} className="flex-shrink-0" />
                          <span className="font-semibold break-words">{detailsError}</span>
                        </div>
                        <button
                          type="button"
                          onClick={() => void loadDetails(activeFund.id, token)}
                          disabled={detailsLoading}
                          className="sc-btn-soft rounded-xl px-4 py-2 text-xs font-bold disabled:opacity-50"
                        >
                          {detailsLoading ? "Retrying…" : "Retry"}
                        </button>
                      </div>
                    )}

                    {!hasLoadedActiveDetails && detailsLoading ? (
                      <DetailsLoadingState />
                    ) : hasLoadedActiveDetails ? (
                      <>
                        {/* Metrics */}
                        <div className="grid grid-cols-2 xl:grid-cols-4 gap-3">
                          <MetricCard
                            label="Total spent"
                            value={`$${money(totalSpent)}`}
                            icon={<BadgeDollarSign size={18} />}
                            color="green"
                          />
                          <MetricCard
                            label="Average"
                            value={`$${money(average)}`}
                            icon={<BanknoteArrowUp size={18} />}
                            color="amber"
                          />
                          <MetricCard
                            label="Members"
                            value={String(totalMembers)}
                            icon={<Users size={18} />}
                            color="blue"
                          />
                          <MetricCard
                            label="Transfers"
                            value={String(debtRows.length)}
                            icon={<ReceiptText size={18} />}
                            color="purple"
                          />
                        </div>

                        {/* Expense + Settlement */}
                        <div className="grid gap-5 xl:grid-cols-2">
                          <ExpensePanel
                            busy={busy}
                            submitting={busyAction === "expense"}
                            onSubmit={submitExpense}
                          />
                          <SettlementPanel debts={debtRows} />
                        </div>

                        {/* Members + Purchases */}
                        <div className="grid gap-5 xl:grid-cols-2">
                          <MembersPanel
                            busy={busy}
                            adding={busyAction === "addMember"}
                            removingUserID={removingUserID}
                            members={details.members}
                            onAdd={submitVirtualUser}
                            onRemove={removeMember}
                          />
                          <PurchasesPanel purchases={details.purchases} />
                        </div>
                      </>
                    ) : null}
                  </>
                ) : fundsLoading ? (
                  <DetailsLoadingState />
                ) : fundsError ? (
                  <div className="sc-card rounded-3xl min-h-[320px] grid place-items-center p-8 text-center">
                    <div className="max-w-sm space-y-4">
                      <div className="size-14 rounded-3xl bg-sc-red/15 border border-sc-red/30 grid place-items-center mx-auto text-sc-red">
                        <AlertCircle size={24} />
                      </div>
                      <div className="space-y-1">
                        <h2 className="text-xl font-black">Could not load funds</h2>
                        <p className="text-sm text-sc-muted">{fundsError}</p>
                      </div>
                      <button
                        type="button"
                        onClick={() => void loadFunds(token)}
                        disabled={fundsLoading}
                        className="sc-btn-primary rounded-2xl px-6 py-2.5 text-sm font-bold disabled:opacity-50"
                      >
                        Try again
                      </button>
                    </div>
                  </div>
                ) : (
                  <EmptyState />
                )}
              </main>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Auth page ───────────────────────────────────────────────────────────────

function AuthPage({
  busy,
  status,
  sessionID,
  error,
  popupBlocked,
  onStart,
}: {
  busy: boolean;
  status: AuthUIStatus;
  sessionID: string;
  error: string | null;
  popupBlocked: boolean;
  onStart: () => void;
}) {
  const link = sessionID ? telegramLink(sessionID) : "";
  const dotColor =
    status === "authenticated"
      ? "bg-sc-green"
      : status === "pending"
      ? "bg-sc-amber animate-pulse"
      : status === "expired" || status === "error"
      ? "bg-sc-red"
      : "bg-sc-surface";

  return (
    <div className="min-h-screen flex flex-col items-center justify-center px-4 py-12 relative overflow-hidden">
      {/* Background blobs */}
      <div className="absolute inset-0 pointer-events-none overflow-hidden">
        <div className="absolute -top-40 -left-40 size-[500px] rounded-full bg-sc-green/8 blur-3xl" />
        <div className="absolute -bottom-40 -right-40 size-[500px] rounded-full bg-sc-blue/8 blur-3xl" />
      </div>

      <div className="relative z-10 w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-10">
          <img
            src="/logo.jpg"
            alt="SplitCore"
            className="inline-block size-20 rounded-3xl object-cover mb-4 shadow-2xl shadow-sc-green/20"
          />
          <h1 className="text-3xl font-black tracking-tight">SplitCore</h1>
          <p className="text-sc-muted mt-2 text-sm leading-relaxed max-w-xs mx-auto">
            Shared expenses made simple. Connect with Telegram to get started.
          </p>
        </div>

        <div className="sc-card rounded-3xl p-6 space-y-5">
          <div>
            <h2 className="text-lg font-bold mb-1">Sign in</h2>
            <p className="text-sm text-sc-muted">We'll open your Telegram app for a one-tap login.</p>
          </div>

          {error && (
            <div className="rounded-2xl border border-sc-red/30 bg-sc-red/10 p-3.5 text-sm text-sc-red flex items-start gap-2.5">
              <AlertCircle size={17} className="flex-shrink-0 mt-0.5" />
              <span>{error}</span>
            </div>
          )}

          <button
            type="button"
            onClick={onStart}
            disabled={busy}
            className="w-full sc-btn-primary rounded-2xl py-3.5 text-sm font-bold flex items-center justify-center gap-2.5 disabled:opacity-50"
          >
            {busy ? <Loader2 size={17} className="animate-spin" /> : <Send size={17} />}
            {busy ? "Connecting…" : "Connect with Telegram"}
          </button>

          {link && status === "pending" && (
            <div className="space-y-1.5 text-center">
              {popupBlocked && (
                <p className="text-xs text-sc-amber font-medium">
                  Browser blocked the popup window. Use the link below:
                </p>
              )}
              <a
                href={link}
                target="_blank"
                rel="noreferrer"
                className="block text-sm text-sc-muted hover:text-sc-text underline underline-offset-2 transition"
              >
                Open auth link manually
              </a>
            </div>
          )}

          {/* Status steps */}
          {sessionID && (
            <div className="rounded-2xl bg-sc-surface border border-sc-border p-4 space-y-3">
              <p className="text-xs font-bold uppercase tracking-wider text-sc-muted">Auth status</p>
              <div className="flex items-center gap-2">
                <div className={`size-2 rounded-full flex-shrink-0 ${dotColor}`} />
                <p className="text-sm font-semibold capitalize">{status}</p>
              </div>
              <p className="font-mono text-xs text-sc-muted break-all">{sessionID}</p>
            </div>
          )}
        </div>

        <p className="text-center text-xs text-sc-muted mt-6 opacity-60">
          No password needed · Telegram-powered · Open source
        </p>
      </div>
    </div>
  );
}

// ─── Loading skeleton ────────────────────────────────────────────────────────

function DetailsLoadingState() {
  return (
    <div className="space-y-5 animate-pulse">
      <div className="grid grid-cols-2 xl:grid-cols-4 gap-3">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="sc-card rounded-3xl p-4 h-[76px] bg-sc-surface/50" />
        ))}
      </div>
      <div className="grid gap-5 xl:grid-cols-2">
        <div className="sc-card rounded-3xl p-5 h-52 bg-sc-surface/50" />
        <div className="sc-card rounded-3xl p-5 h-52 bg-sc-surface/50" />
      </div>
      <div className="grid gap-5 xl:grid-cols-2">
        <div className="sc-card rounded-3xl p-5 h-56 bg-sc-surface/50" />
        <div className="sc-card rounded-3xl p-5 h-56 bg-sc-surface/50" />
      </div>
    </div>
  );
}

// ─── Metric card ─────────────────────────────────────────────────────────────

const colorMap = {
  green: { bg: "bg-sc-green/12", text: "text-sc-green", icon: "bg-sc-green/20 text-sc-green" },
  amber: { bg: "bg-sc-amber/12", text: "text-sc-amber", icon: "bg-sc-amber/20 text-sc-amber" },
  blue: { bg: "bg-sc-blue/12", text: "text-sc-blue", icon: "bg-sc-blue/20 text-sc-blue" },
  purple: { bg: "bg-sc-purple/12", text: "text-sc-purple", icon: "bg-sc-purple/20 text-sc-purple" },
};

function MetricCard({
  label,
  value,
  icon,
  color,
}: {
  label: string;
  value: string;
  icon: React.ReactNode;
  color: keyof typeof colorMap;
}) {
  const c = colorMap[color];
  return (
    <div className={`sc-card rounded-3xl p-4 flex items-center gap-3 ${c.bg} border-none`}>
      <div className={`size-10 rounded-2xl grid place-items-center flex-shrink-0 ${c.icon}`}>
        {icon}
      </div>
      <div className="min-w-0">
        <p className="text-xs font-bold uppercase tracking-wider text-sc-muted truncate">{label}</p>
        <p className={`text-xl font-black ${c.text} truncate`}>{value}</p>
      </div>
    </div>
  );
}

// ─── Expense panel ────────────────────────────────────────────────────────────

function ExpensePanel({
  busy,
  submitting,
  onSubmit,
}: {
  busy: boolean;
  submitting: boolean;
  onSubmit: (e: FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <form onSubmit={onSubmit} className="sc-card rounded-3xl p-5 space-y-4">
      <div>
        <h3 className="text-base font-bold">Log expense</h3>
        <p className="text-xs text-sc-muted mt-0.5">Add an amount with a description.</p>
      </div>
      <div className="flex gap-3">
        <input
          name="cost"
          type="number"
          min="0.01"
          step="any"
          placeholder="0.01"
          required
          className="sc-input w-28 rounded-2xl px-3 py-3 text-sm flex-shrink-0 text-center font-mono"
        />
        <input
          name="description"
          placeholder="Dinner, taxi, groceries…"
          maxLength={200}
          required
          className="sc-input flex-1 rounded-2xl px-4 py-3 text-sm"
        />
      </div>
      <button
        type="submit"
        disabled={busy}
        className="sc-btn-primary w-full rounded-2xl py-2.5 text-sm font-bold flex items-center justify-center gap-2 disabled:opacity-50"
      >
        {submitting ? <Loader2 size={16} className="animate-spin" /> : <ReceiptText size={16} />}
        {submitting ? "Adding…" : "Add expense"}
      </button>
    </form>
  );
}

// ─── Settlement panel ─────────────────────────────────────────────────────────

function SettlementPanel({ debts }: { debts: Array<Debt & { from: string; to: string }> }) {
  return (
    <div className="sc-card rounded-3xl p-5 space-y-4">
      <div>
        <h3 className="text-base font-bold">Settlement</h3>
        <p className="text-xs text-sc-muted mt-0.5">Optimized payments to settle all debts.</p>
      </div>
      <div className="space-y-2">
        {debts.length === 0 ? (
          <div className="rounded-2xl bg-sc-green/8 border border-sc-green/20 p-4 text-center text-sm font-semibold text-sc-green">
            ✓ All settled up!
          </div>
        ) : (
          debts.map((debt, i) => (
            <div
              key={`${debt.from_id}-${debt.to_id}-${i}`}
              className="flex items-center justify-between gap-3 rounded-2xl bg-sc-surface border border-sc-border px-4 py-3"
            >
              <span className="text-sm font-bold truncate">{debt.from}</span>
              <div className="flex items-center gap-2 flex-shrink-0">
                <span className="text-xs text-sc-muted">owes</span>
                <span className="rounded-xl bg-sc-red/12 text-sc-red px-2.5 py-1 text-xs font-black">
                  ${money(debt.amount)}
                </span>
                <span className="text-xs text-sc-muted">to</span>
              </div>
              <span className="text-sm font-bold text-sc-green truncate">{debt.to}</span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

// ─── Members panel ────────────────────────────────────────────────────────────

function MembersPanel({
  busy,
  adding,
  removingUserID,
  members,
  onAdd,
  onRemove,
}: {
  busy: boolean;
  adding: boolean;
  removingUserID: number | null;
  members: User[];
  onAdd: (e: FormEvent<HTMLFormElement>) => void;
  onRemove: (id: number) => void;
}) {
  return (
    <div className="sc-card rounded-3xl p-5 space-y-4">
      <div>
        <h3 className="text-base font-bold">Members</h3>
        <p className="text-xs text-sc-muted mt-0.5">Real and virtual participants.</p>
      </div>
      <form onSubmit={onAdd} className="flex gap-2">
        <input
          name="firstName"
          placeholder="Add virtual member…"
          className="sc-input flex-1 rounded-2xl px-4 py-2.5 text-sm"
        />
        <button
          type="submit"
          disabled={busy}
          className="sc-btn-soft rounded-2xl px-3 py-2.5 flex items-center gap-1.5 text-sm font-bold disabled:opacity-50 flex-shrink-0"
        >
          {adding ? <Loader2 size={15} className="animate-spin" /> : <UserPlus size={15} />}
          {adding ? "Adding…" : "Add"}
        </button>
      </form>
      <div className="space-y-2">
        {members.map((m) => (
          <div
            key={m.id}
            className="flex items-center justify-between gap-3 rounded-2xl bg-sc-surface border border-sc-border px-4 py-3"
          >
            <div className="flex items-center gap-2.5 min-w-0">
              <div className="size-8 rounded-xl bg-sc-blue/15 grid place-items-center text-sc-blue flex-shrink-0 text-xs font-black">
                {displayUser(m).charAt(0).toUpperCase()}
              </div>
              <div className="min-w-0">
                <p className="text-sm font-bold truncate">{displayUser(m)}</p>
                <p className="text-xs text-sc-muted">{m.is_virtual ? "Virtual" : "Telegram"}</p>
              </div>
            </div>
            {m.is_virtual && (
              <button
                type="button"
                disabled={busy}
                onClick={() => onRemove(m.id)}
                title="Remove"
                aria-label={`Remove ${displayUser(m)}`}
                className="size-8 rounded-xl bg-sc-surface hover:bg-sc-red/12 text-sc-muted hover:text-sc-red grid place-items-center transition flex-shrink-0 disabled:opacity-40"
              >
                {removingUserID === m.id ? (
                  <Loader2 size={14} className="animate-spin" />
                ) : (
                  <X size={14} />
                )}
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

// ─── Purchases panel ──────────────────────────────────────────────────────────

function PurchasesPanel({ purchases }: { purchases: Purchase[] }) {
  return (
    <div className="sc-card rounded-3xl p-5 space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-base font-bold">Expense history</h3>
          <p className="text-xs text-sc-muted mt-0.5">All recorded expenses.</p>
        </div>
        <span className="text-xs bg-sc-surface border border-sc-border rounded-xl px-2.5 py-1 font-bold text-sc-muted">
          {purchases.length}
        </span>
      </div>
      <div className="space-y-2">
        {purchases.length === 0 ? (
          <p className="text-center text-sm text-sc-muted py-4">No expenses yet</p>
        ) : (
          purchases.map((p) => (
            <div
              key={p.id}
              className="flex items-start justify-between gap-3 rounded-2xl bg-sc-surface border border-sc-border px-4 py-3"
            >
              <div className="min-w-0">
                <p className="text-sm font-bold truncate">{p.description}</p>
                <p className="text-xs text-sc-muted mt-0.5">
                  {displayUser(p.payer)} · {shortDate(p.created_at)}
                </p>
              </div>
              <span className="rounded-xl bg-sc-green/12 text-sc-green px-2.5 py-1 text-sm font-black flex-shrink-0">
                ${money(p.amount)}
              </span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

// ─── Empty state ──────────────────────────────────────────────────────────────

function EmptyState() {
  return (
    <div className="sc-card rounded-3xl min-h-[400px] grid place-items-center p-8 text-center">
      <div className="max-w-xs">
        <div className="size-16 rounded-3xl bg-gradient-to-br from-sc-green/20 to-sc-blue/20 border border-sc-green/20 grid place-items-center mx-auto mb-5">
          <Wallet size={26} className="text-sc-green" />
        </div>
        <h2 className="text-xl font-black mb-2">No fund selected</h2>
        <p className="text-sm text-sc-muted leading-relaxed">
          Create a fund or join one with an invite code to get started.
        </p>
      </div>
    </div>
  );
}

export default App;
