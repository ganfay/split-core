import {
  ArrowRight,
  BadgeDollarSign,
  BanknoteArrowUp,
  Calendar,
  CheckCircle,
  ChevronRight,
  Copy,
  Crown,
  LogOut,
  Moon,
  Plus,
  ReceiptText,
  RefreshCw,
  Send,
  ShieldCheck,
  Sparkles,
  Sun,
  Trash2,
  User as UserIcon,
  UserPlus,
  Users,
  Wallet,
  X,
  XCircle,
} from "lucide-react";
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { api, authStore } from "./api/client";
import type { Debt, Fund, Purchase, Settlement, User } from "./api/types";
import { displayUser, fullDateTime, money, shortDate } from "./lib/format";

const botName = import.meta.env.VITE_BOT_NAME || "SplitCoreBot";

type FundDetails = {
  fund?: Fund;
  members: User[];
  settlement?: Settlement;
  purchases: Purchase[];
};

type Toast = {
  id: number;
  type: "success" | "error";
  message: string;
};

// ─── Utility ────────────────────────────────────────────────────────────────

function parseJwtUserId(jwtToken: string | null): number | null {
  if (!jwtToken) return null;
  try {
    const parts = jwtToken.split(".");
    if (parts.length < 2) return null;
    const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const jsonPayload = decodeURIComponent(
      atob(base64)
        .split("")
        .map((c) => "%" + ("00" + c.charCodeAt(0).toString(16)).slice(-2))
        .join("")
    );
    const data = JSON.parse(jsonPayload);
    return typeof data.user_id === "number" ? data.user_id : null;
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
    const saved = localStorage.getItem("splitcore.theme");
    return saved === "light" ? "light" : "dark"; // default is dark
  });
  const [funds, setFunds] = useState<Fund[]>([]);
  const [activeFundID, setActiveFundID] = useState<number | null>(null);
  const [details, setDetails] = useState<FundDetails>({ members: [], purchases: [] });
  const [sessionID, setSessionID] = useState("");
  const [status, setStatus] = useState("idle");
  const [busy, setBusy] = useState(false);
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [deleteConfirm, setDeleteConfirm] = useState<number | null>(null);

  // Sync theme class to documentElement
  useEffect(() => {
    if (theme === "light") {
      document.documentElement.classList.add("light");
      document.documentElement.classList.remove("dark");
    } else {
      document.documentElement.classList.remove("light");
      document.documentElement.classList.add("dark");
    }
    localStorage.setItem("splitcore.theme", theme);
  }, [theme]);

  const toggleTheme = useCallback(() => {
    setTheme((prev) => (prev === "dark" ? "light" : "dark"));
  }, []);

  // Fetch current user info whenever token changes
  useEffect(() => {
    if (!token) {
      setCurrentUser(null);
      return;
    }
    api.getMe(token)
      .then((u) => {
        if (u) {
          setCurrentUser(u);
          authStore.setUser(u);
        }
      })
      .catch(() => {});
  }, [token]);

  const currentUserID = useMemo(() => parseJwtUserId(token), [token]);
  const activeFund = details.fund ?? funds.find((f) => f.id === activeFundID);
  const isOwnerOfActiveFund = Boolean(
    activeFund && currentUserID !== null && activeFund.author_id === currentUserID
  );
  const activeFundOwner = useMemo(() => {
    if (!activeFund) return undefined;
    return details.members.find((m) => m.id === activeFund.author_id);
  }, [activeFund, details.members]);

  const resolvedCurrentUser = useMemo(() => {
    if (currentUser) return currentUser;
    if (currentUserID !== null) {
      const fromMembers = details.members.find((m) => m.id === currentUserID);
      if (fromMembers) return fromMembers;
    }
    return null;
  }, [currentUser, currentUserID, details.members]);
  const totalMembers = details.members.length;
  const totalSpent = details.settlement?.total_amount ?? 0;
  const average = details.settlement?.average ?? 0;

  const debtRows = useMemo(() => {
    const byID = new Map(details.members.map((m) => [m.id, m]));
    return (details.settlement?.debts ?? []).map((debt) => ({
      ...debt,
      from: displayUser(byID.get(debt.from_id)),
      to: displayUser(byID.get(debt.to_id)),
    }));
  }, [details.members, details.settlement]);

  function toast(type: "success" | "error", message: string) {
    const id = ++toastCounter;
    setToasts((prev) => [...prev, { id, type, message }]);
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 4000);
  }

  const loadFunds = useCallback(
    async (nextToken = token) => {
      if (!nextToken) return;
      const list = await api.listFunds(nextToken);
      const safeList = list || [];
      setFunds(safeList);
      if (!activeFundID && safeList.length > 0) {
        setActiveFundID(safeList[0].id);
      }
    },
    [activeFundID, token]
  );

  const loadDetails = useCallback(
    async (fundID: number, nextToken = token) => {
      if (!nextToken) return;
      const [fund, members, settlement, purchases] = await Promise.all([
        api.getFundInfo(nextToken, fundID),
        api.getMembers(nextToken, fundID),
        api.getBalance(nextToken, fundID),
        api.getPurchases(nextToken, fundID),
      ]);
      setDetails({ fund, members: members || [], settlement, purchases: purchases || [] });
    },
    [token]
  );

  useEffect(() => {
    if (!token) return;
    loadFunds().catch((err) => {
      toast("error", err.message);
      authStore.clear();
      setToken(null);
    });
  }, [loadFunds, token]);

  useEffect(() => {
    if (!activeFundID || !token) return;
    loadDetails(activeFundID).catch((err) => toast("error", err.message));
  }, [activeFundID, loadDetails, token]);

  // ── Listen for silent token refresh / forced logout from client.ts ────────
  useEffect(() => {
    const onRefreshed = (e: Event) => {
      const newToken = (e as CustomEvent<string>).detail;
      authStore.setToken(newToken);
      setToken(newToken);
    };
    const onLogout = () => {
      authStore.clear();
      setToken(null);
      setCurrentUser(null);
      setFunds([]);
      setActiveFundID(null);
      setDetails({ members: [], purchases: [] });
    };
    window.addEventListener("splitcore:tokenrefreshed", onRefreshed);
    window.addEventListener("splitcore:logout", onLogout);
    return () => {
      window.removeEventListener("splitcore:tokenrefreshed", onRefreshed);
      window.removeEventListener("splitcore:logout", onLogout);
    };
  }, []);

  useEffect(() => {
    if (!sessionID || status !== "pending") return;
    const timer = window.setInterval(async () => {
      try {
        const nextStatus = await api.checkSession(sessionID);
        setStatus(nextStatus);
        if (nextStatus === "authenticated") {
          const tokens = await api.generateTokens(sessionID);
          authStore.setToken(tokens.access_token);
          setToken(tokens.access_token);
          api.getMe(tokens.access_token)
            .then((u) => {
              if (u) {
                setCurrentUser(u);
                authStore.setUser(u);
              }
            })
            .catch(() => {});
          toast("success", "Telegram connected!");
          window.clearInterval(timer);
        }
      } catch (err) {
        toast("error", err instanceof Error ? err.message : "Auth check failed");
      }
    }, 2200);
    return () => window.clearInterval(timer);
  }, [sessionID, status]);

  async function startTelegramAuth() {
    setBusy(true);
    try {
      const session = await api.createSession();
      setSessionID(session.uuid);
      setStatus(session.status);
      window.open(telegramLink(session.uuid), "_blank", "noopener,noreferrer");
    } catch (err) {
      toast("error", err instanceof Error ? err.message : "Failed to start auth");
    } finally {
      setBusy(false);
    }
  }

  async function submitCreateFund(event: FormEvent<HTMLFormElement>) {
      event.preventDefault();
      if (!token) return;
      const formEl = event.currentTarget as HTMLFormElement;
      const form = new FormData(formEl);
      const name = String(form.get("name") ?? "").trim();
      if (!name) return;
      setBusy(true);
      try {
        const fund = await api.createFund(token, name);
        formEl.reset();
        setActiveFundID(fund.id);
        await loadFunds(token);
        await loadDetails(fund.id, token);
        toast("success", `Fund "${fund.name}" created!`);
      } catch (err) {
        toast("error", err instanceof Error ? err.message : "Failed to create fund");
      } finally {
        setBusy(false);
      }
  }

  async function submitJoinFund(event: FormEvent<HTMLFormElement>) {
      event.preventDefault();
      if (!token) return;
      const formEl = event.currentTarget as HTMLFormElement;
      const form = new FormData(formEl);
      const code = String(form.get("invite") ?? "").trim();
      if (!code) return;
      setBusy(true);
      try {
        const fund = await api.joinFund(token, code);
        formEl.reset();
        setActiveFundID(fund.id);
        await loadFunds(token);
        await loadDetails(fund.id, token);
        toast("success", `Joined "${fund.name}"!`);
      } catch (err) {
        toast("error", err instanceof Error ? err.message : "Failed to join fund");
      } finally {
        setBusy(false);
      }
  }

  async function confirmDeleteFund(fundID: number) {
    if (!token) return;
    setBusy(true);
    try {
      await api.deleteFund(token, fundID);
      setDeleteConfirm(null);
      const newFunds = funds.filter((f) => f.id !== fundID);
      setFunds(newFunds);
      if (activeFundID === fundID) {
        const next = newFunds[0] ?? null;
        setActiveFundID(next?.id ?? null);
        setDetails({ members: [], purchases: [] });
        if (next) await loadDetails(next.id, token);
      }
      toast("success", "Fund deleted");
    } catch (err) {
      toast("error", err instanceof Error ? err.message : "Failed to delete fund");
    } finally {
      setBusy(false);
    }
  }

  async function submitExpense(event: FormEvent<HTMLFormElement>) {
      event.preventDefault();
      if (!token || !activeFundID) return;
      const formEl = event.currentTarget as HTMLFormElement;
      const form = new FormData(formEl);
      const cost = Number(form.get("cost"));
      const description = String(form.get("description") ?? "").trim();
      if (!description) {
        toast("error", "Description is required");
        return;
      }
      if (isNaN(cost) || cost < 1) {
        toast("error", "Expense amount must be at least 1");
        return;
      }
      setBusy(true);
      try {
        await api.addExpense(token, activeFundID, cost, description);
        formEl.reset();
        await loadDetails(activeFundID, token);
        toast("success", "Expense added!");
      } catch (err) {
        toast("error", err instanceof Error ? err.message : "Failed to add expense");
      } finally {
        setBusy(false);
      }
  }

  async function submitVirtualUser(event: FormEvent<HTMLFormElement>) {
      event.preventDefault();
      if (!token || !activeFundID) return;
      const formEl = event.currentTarget as HTMLFormElement;
      const form = new FormData(formEl);
      const firstName = String(form.get("firstName") ?? "").trim();
      if (!firstName) return;
      setBusy(true);
      try {
        await api.addVirtualUser(token, activeFundID, firstName);
        formEl.reset();
        await loadDetails(activeFundID, token);
        toast("success", "Member added!");
      } catch (err) {
        toast("error", err instanceof Error ? err.message : "Failed to add member");
      } finally {
        setBusy(false);
      }
  }

  async function removeMember(userID: number) {
    if (!token || !activeFundID) return;
    setBusy(true);
    try {
      await api.removeMember(token, activeFundID, userID);
      await loadDetails(activeFundID, token);
      toast("success", "Member removed");
    } catch (err) {
      toast("error", err instanceof Error ? err.message : "Failed to remove member");
    } finally {
      setBusy(false);
    }
  }

  function logout() {
    authStore.clear();
    setToken(null);
    setFunds([]);
    setActiveFundID(null);
    setDetails({ members: [], purchases: [] });
    setSessionID("");
    setStatus("idle");
  }

  return (
    <div className="min-h-screen bg-sc-bg text-sc-text font-sans antialiased">
      {/* Toast stack */}
      <div className="fixed bottom-6 right-4 left-4 sm:left-auto sm:right-6 z-50 flex flex-col gap-2 items-end pointer-events-none">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`pointer-events-auto flex items-center gap-3 rounded-2xl px-4 py-3 text-sm font-semibold shadow-2xl backdrop-blur-xl border max-w-sm w-full sm:w-auto transition-all
              ${t.type === "success"
                ? "bg-sc-green/15 border-sc-green/30 text-sc-green"
                : "bg-sc-red/15 border-sc-red/30 text-sc-red"
              }`}
          >
            {t.type === "success" ? <CheckCircle size={16} /> : <XCircle size={16} />}
            {t.message}
          </div>
        ))}
      </div>

      {/* Delete confirmation modal */}
      {deleteConfirm !== null && (
        <div className="fixed inset-0 z-40 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
          <div className="sc-card rounded-3xl p-6 max-w-sm w-full">
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
                onClick={() => setDeleteConfirm(null)}
                className="flex-1 sc-btn-soft rounded-2xl py-2.5 text-sm font-bold"
              >
                Cancel
              </button>
              <button
                onClick={() => confirmDeleteFund(deleteConfirm)}
                disabled={busy}
                className="flex-1 bg-sc-red/90 hover:bg-sc-red text-white rounded-2xl py-2.5 text-sm font-bold transition disabled:opacity-50"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}

      {!token ? (
        <AuthPage busy={busy} status={status} sessionID={sessionID} onStart={startTelegramAuth} />
      ) : (
        <div className="flex flex-col min-h-screen">
          {/* Navbar */}
          <header className="sticky top-0 z-30 border-b border-sc-border bg-sc-bg/80 backdrop-blur-xl">
            <div className="mx-auto max-w-7xl px-4 sm:px-6 h-16 flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <img src="/logo.png" alt="SplitCore" className="size-9 object-contain flex-shrink-0" />
                <span className="font-black text-lg tracking-tight">SplitCore</span>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => token && loadFunds(token)}
                  className="sc-btn-ghost rounded-xl p-2.5"
                  title="Refresh"
                >
                  <RefreshCw size={17} />
                </button>
                <button
                  onClick={toggleTheme}
                  className="sc-btn-ghost rounded-xl p-2.5"
                  title={theme === "dark" ? "Switch to Light mode" : "Switch to Dark mode"}
                >
                  {theme === "dark" ? <Moon size={17} /> : <Sun size={17} />}
                </button>
                <button
                  onClick={logout}
                  className="sc-btn-ghost rounded-xl p-2.5 text-sc-muted hover:text-sc-red"
                  title="Logout"
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
                    <label className="block text-xs font-bold uppercase tracking-wider text-sc-muted">New fund</label>
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
                      <Plus size={16} /> Create
                    </button>
                  </form>

                  <div className="border-t border-sc-border" />

                  <form onSubmit={submitJoinFund} className="space-y-3">
                    <label className="block text-xs font-bold uppercase tracking-wider text-sc-muted">Join by code</label>
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
                      <ArrowRight size={16} /> Join
                    </button>
                  </form>
                </div>

                {/* Fund list */}
                <div className="sc-card rounded-3xl p-4">
                  <div className="flex items-center justify-between px-1 mb-3">
                    <h2 className="text-sm font-bold uppercase tracking-wider text-sc-muted">My funds</h2>
                    <span className="text-xs bg-sc-surface rounded-lg px-2 py-0.5 font-bold text-sc-muted">{funds.length}</span>
                  </div>
                  <div className="space-y-1 max-h-[380px] overflow-y-auto pr-1">
                    {funds.map((fund) => (
                      <div
                        key={fund.id}
                        className={`group relative flex items-center rounded-2xl transition cursor-pointer
                          ${activeFundID === fund.id
                            ? "bg-sc-green/12 text-sc-text"
                            : "hover:bg-sc-surface text-sc-text/80 hover:text-sc-text"
                          }`}
                      >
                        <button
                          className="flex-1 flex items-center gap-3 p-3 text-left min-w-0"
                          onClick={() => setActiveFundID(fund.id)}
                        >
                          <div className={`size-8 rounded-xl grid place-items-center flex-shrink-0 text-sm font-black
                            ${activeFundID === fund.id ? "bg-sc-green/20 text-sc-green" : "bg-sc-surface text-sc-muted"}`}>
                            <Wallet size={14} />
                          </div>
                          <div className="min-w-0">
                            <p className="truncate text-sm font-bold">{fund.name}</p>
                            <p className="text-xs text-sc-muted font-mono">{fund.invite_code}</p>
                          </div>
                          {activeFundID === fund.id && <ChevronRight size={14} className="flex-shrink-0 text-sc-green ml-auto" />}
                        </button>
                        {currentUserID !== null && fund.author_id === currentUserID && (
                          <button
                            onClick={(e) => { e.stopPropagation(); setDeleteConfirm(fund.id); }}
                            className="opacity-0 group-hover:opacity-100 p-2 mr-1 rounded-xl text-sc-muted hover:text-sc-red hover:bg-sc-red/10 transition"
                            title="Delete fund"
                          >
                            <Trash2 size={14} />
                          </button>
                        )}
                      </div>
                    ))}
                    {funds.length === 0 && (
                      <div className="text-center py-8 text-sc-muted text-sm">
                        <Wallet size={28} className="mx-auto mb-2 opacity-30" />
                        No funds yet
                      </div>
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
                              <p className="text-xs font-bold uppercase tracking-wider text-sc-green">Active fund</p>
                              {isOwnerOfActiveFund ? (
                                <span className="flex items-center gap-1 text-xs font-bold px-2 py-0.5 rounded-lg bg-sc-amber/15 text-sc-amber">
                                  <Crown size={10} /> Owner
                                </span>
                              ) : (
                                <span className="text-xs font-bold px-2 py-0.5 rounded-lg bg-sc-surface text-sc-muted">
                                  Member
                                </span>
                              )}
                            </div>
                            <h1 className="text-2xl sm:text-3xl font-black tracking-tight">{activeFund.name}</h1>
                          </div>
                          <div className="flex items-center gap-2">
                            <button
                              onClick={() => navigator.clipboard.writeText(activeFund.invite_code).then(() => toast("success", "Invite code copied!"))}
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
                                onClick={() => setDeleteConfirm(activeFund.id)}
                                className="flex items-center gap-2 rounded-2xl border border-sc-red/30 bg-sc-red/10 hover:bg-sc-red/20 px-3.5 py-3.5 text-sc-red transition"
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
                            <span className="text-sc-text font-semibold">{fullDateTime(activeFund.created_at)}</span>
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

                    {/* Metrics */}
                    <div className="grid grid-cols-2 xl:grid-cols-4 gap-3">
                      <MetricCard label="Total spent" value={`$${money(totalSpent)}`} icon={<BadgeDollarSign size={18} />} color="green" />
                      <MetricCard label="Average" value={`$${money(average)}`} icon={<BanknoteArrowUp size={18} />} color="amber" />
                      <MetricCard label="Members" value={String(totalMembers)} icon={<Users size={18} />} color="blue" />
                      <MetricCard label="Transfers" value={String(debtRows.length)} icon={<ReceiptText size={18} />} color="purple" />
                    </div>

                    {/* Expense + Settlement */}
                    <div className="grid gap-5 xl:grid-cols-2">
                      <ExpensePanel busy={busy} onSubmit={submitExpense} />
                      <SettlementPanel debts={debtRows} />
                    </div>

                    {/* Members + Purchases */}
                    <div className="grid gap-5 xl:grid-cols-2">
                      <MembersPanel busy={busy} members={details.members} onAdd={submitVirtualUser} onRemove={removeMember} />
                      <PurchasesPanel purchases={details.purchases} />
                    </div>
                  </>
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

function AuthPage({ busy, status, sessionID, onStart }: {
  busy: boolean; status: string; sessionID: string; onStart: () => void;
}) {
  const link = sessionID ? telegramLink(sessionID) : "";
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

          <button
            onClick={onStart}
            disabled={busy}
            className="w-full sc-btn-primary rounded-2xl py-3.5 text-sm font-bold flex items-center justify-center gap-2.5 disabled:opacity-50"
          >
            <Send size={17} />
            {busy ? "Connecting…" : "Connect with Telegram"}
          </button>

          {link && (
            <a
              href={link}
              target="_blank"
              rel="noreferrer"
              className="block text-center text-sm text-sc-muted hover:text-sc-text underline underline-offset-2 transition"
            >
              Open auth link manually
            </a>
          )}

          {/* Status steps */}
          {sessionID && (
            <div className="rounded-2xl bg-sc-surface border border-sc-border p-4 space-y-3">
              <p className="text-xs font-bold uppercase tracking-wider text-sc-muted">Auth status</p>
              <div className="flex items-center gap-2">
                <div className={`size-2 rounded-full flex-shrink-0 ${status === "authenticated" ? "bg-sc-green" : status === "pending" ? "bg-sc-amber animate-pulse" : "bg-sc-surface"}`} />
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

// ─── Metric card ─────────────────────────────────────────────────────────────

const colorMap = {
  green:  { bg: "bg-sc-green/12",  text: "text-sc-green",  icon: "bg-sc-green/20 text-sc-green" },
  amber:  { bg: "bg-sc-amber/12",  text: "text-sc-amber",  icon: "bg-sc-amber/20 text-sc-amber" },
  blue:   { bg: "bg-sc-blue/12",   text: "text-sc-blue",   icon: "bg-sc-blue/20 text-sc-blue" },
  purple: { bg: "bg-sc-purple/12", text: "text-sc-purple", icon: "bg-sc-purple/20 text-sc-purple" },
};

function MetricCard({ label, value, icon, color }: {
  label: string; value: string; icon: React.ReactNode; color: keyof typeof colorMap;
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

function ExpensePanel({ busy, onSubmit }: { busy: boolean; onSubmit: (e: FormEvent<HTMLFormElement>) => void }) {
  return (
    <form onSubmit={onSubmit} className="sc-card rounded-3xl p-5 space-y-4">
      <div>
        <h3 className="text-base font-bold">Log expense</h3>
        <p className="text-xs text-sc-muted mt-0.5">Add an amount with a description.</p>
      </div>
      <div className="flex gap-3">
        <input name="cost" type="number" min="1" step="any" placeholder="1.00" required
          className="sc-input w-28 rounded-2xl px-3 py-3 text-sm flex-shrink-0 text-center font-mono" />
        <input name="description" placeholder="Dinner, taxi, groceries…" required
          className="sc-input flex-1 rounded-2xl px-4 py-3 text-sm" />
      </div>
      <button type="submit" disabled={busy}
        className="sc-btn-primary w-full rounded-2xl py-2.5 text-sm font-bold flex items-center justify-center gap-2 disabled:opacity-50">
        <ReceiptText size={16} /> Add expense
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
            <div key={`${debt.from_id}-${debt.to_id}-${i}`} className="flex items-center justify-between gap-3 rounded-2xl bg-sc-surface border border-sc-border px-4 py-3">
              <span className="text-sm font-bold truncate">{debt.from}</span>
              <div className="flex items-center gap-2 flex-shrink-0">
                <span className="text-xs text-sc-muted">owes</span>
                <span className="rounded-xl bg-sc-red/12 text-sc-red px-2.5 py-1 text-xs font-black">${money(debt.amount)}</span>
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

function MembersPanel({ busy, members, onAdd, onRemove }: {
  busy: boolean; members: User[]; onAdd: (e: FormEvent<HTMLFormElement>) => void; onRemove: (id: number) => void;
}) {
  return (
    <div className="sc-card rounded-3xl p-5 space-y-4">
      <div>
        <h3 className="text-base font-bold">Members</h3>
        <p className="text-xs text-sc-muted mt-0.5">Real and virtual participants.</p>
      </div>
      <form onSubmit={onAdd} className="flex gap-2">
        <input name="firstName" placeholder="Add virtual member…"
          className="sc-input flex-1 rounded-2xl px-4 py-2.5 text-sm" />
        <button type="submit" disabled={busy}
          className="sc-btn-soft rounded-2xl px-3 py-2.5 flex items-center gap-1.5 text-sm font-bold disabled:opacity-50 flex-shrink-0">
          <UserPlus size={15} /> Add
        </button>
      </form>
      <div className="space-y-2">
        {members.map((m) => (
          <div key={m.id} className="flex items-center justify-between gap-3 rounded-2xl bg-sc-surface border border-sc-border px-4 py-3">
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
              <button onClick={() => onRemove(m.id)} title="Remove"
                className="size-8 rounded-xl bg-sc-surface hover:bg-sc-red/12 text-sc-muted hover:text-sc-red grid place-items-center transition flex-shrink-0">
                <X size={14} />
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
        <span className="text-xs bg-sc-surface border border-sc-border rounded-xl px-2.5 py-1 font-bold text-sc-muted">{purchases.length}</span>
      </div>
      <div className="space-y-2">
        {purchases.length === 0 ? (
          <p className="text-center text-sm text-sc-muted py-4">No expenses yet</p>
        ) : (
          purchases.map((p) => (
            <div key={p.id} className="flex items-start justify-between gap-3 rounded-2xl bg-sc-surface border border-sc-border px-4 py-3">
              <div className="min-w-0">
                <p className="text-sm font-bold truncate">{p.description}</p>
                <p className="text-xs text-sc-muted mt-0.5">{displayUser(p.payer)} · {shortDate(p.created_at)}</p>
              </div>
              <span className="rounded-xl bg-sc-green/12 text-sc-green px-2.5 py-1 text-sm font-black flex-shrink-0">${money(p.amount)}</span>
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
