import {
  ArrowRight,
  BadgeDollarSign,
  BanknoteArrowUp,
  Copy,
  LogOut,
  Plus,
  ReceiptText,
  RefreshCw,
  Send,
  Sparkles,
  Trash2,
  UserPlus,
  Users
} from "lucide-react";
import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { api, authStore } from "./api/client";
import type { Debt, Fund, Purchase, Settlement, User } from "./api/types";
import { Button } from "./components/Button";
import { Metric } from "./components/Metric";
import { TextInput } from "./components/TextInput";
import { displayUser, money, shortDate } from "./lib/format";

const botName = import.meta.env.VITE_BOT_NAME || "SplitCoreBot";

type FundDetails = {
  fund?: Fund;
  members: User[];
  settlement?: Settlement;
  purchases: Purchase[];
};

function App() {
  const [token, setToken] = useState(() => authStore.getToken());
  const [funds, setFunds] = useState<Fund[]>([]);
  const [activeFundID, setActiveFundID] = useState<number | null>(null);
  const [details, setDetails] = useState<FundDetails>({
    members: [],
    purchases: []
  });
  const [sessionID, setSessionID] = useState("");
  const [status, setStatus] = useState("idle");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");

  const activeFund = details.fund ?? funds.find((fund) => fund.id === activeFundID);
  const totalMembers = details.members.length;
  const totalSpent = details.settlement?.total_amount ?? 0;
  const average = details.settlement?.average ?? 0;

  const debtRows = useMemo(() => {
    const byID = new Map(details.members.map((member) => [member.id, member]));
    return (details.settlement?.debts ?? []).map((debt) => ({
      ...debt,
      from: displayUser(byID.get(debt.from_id)),
      to: displayUser(byID.get(debt.to_id))
    }));
  }, [details.members, details.settlement]);

  const loadFunds = useCallback(
    async (nextToken = token) => {
      if (!nextToken) {
        return;
      }
      const list = await api.listFunds(nextToken);
      setFunds(list);
      if (!activeFundID && list.length > 0) {
        setActiveFundID(list[0].id);
      }
    },
    [activeFundID, token]
  );

  const loadDetails = useCallback(
    async (fundID: number, nextToken = token) => {
      if (!nextToken) {
        return;
      }
      const [fund, members, settlement, purchases] = await Promise.all([
        api.getFundInfo(nextToken, fundID),
        api.getMembers(nextToken, fundID),
        api.getBalance(nextToken, fundID),
        api.getPurchases(nextToken, fundID)
      ]);
      setDetails({ fund, members, settlement, purchases });
    },
    [token]
  );

  useEffect(() => {
    if (!token) {
      return;
    }
    loadFunds().catch((error) => {
      setNotice(error.message);
      authStore.clear();
      setToken(null);
    });
  }, [loadFunds, token]);

  useEffect(() => {
    if (!activeFundID || !token) {
      return;
    }
    loadDetails(activeFundID).catch((error) => setNotice(error.message));
  }, [activeFundID, loadDetails, token]);

  useEffect(() => {
    if (!sessionID || status !== "pending") {
      return;
    }
    const timer = window.setInterval(async () => {
      try {
        const nextStatus = await api.checkSession(sessionID);
        setStatus(nextStatus);
        if (nextStatus === "authenticated") {
          const tokens = await api.generateTokens(sessionID);
          authStore.setToken(tokens.access_token);
          setToken(tokens.access_token);
          setNotice("Telegram account connected");
          window.clearInterval(timer);
        }
      } catch (error) {
        setNotice(error instanceof Error ? error.message : "Auth check failed");
      }
    }, 2200);

    return () => window.clearInterval(timer);
  }, [sessionID, status]);

  async function startTelegramAuth() {
    setBusy(true);
    setNotice("");
    try {
      const session = await api.createSession();
      setSessionID(session.uuid);
      setStatus(session.status);
      window.open(telegramLink(session.uuid), "_blank", "noopener,noreferrer");
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Failed to start auth");
    } finally {
      setBusy(false);
    }
  }

  async function submitCreateFund(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token) {
      return;
    }
    const form = new FormData(event.currentTarget);
    const name = String(form.get("name") ?? "").trim();
    if (!name) {
      return;
    }
    setBusy(true);
    try {
      const fund = await api.createFund(token, name);
      event.currentTarget.reset();
      setActiveFundID(fund.id);
      await loadFunds(token);
      await loadDetails(fund.id, token);
      setNotice("Fund created");
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Failed to create fund");
    } finally {
      setBusy(false);
    }
  }

  async function submitJoinFund(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token) {
      return;
    }
    const form = new FormData(event.currentTarget);
    const code = String(form.get("invite") ?? "").trim();
    if (!code) {
      return;
    }
    setBusy(true);
    try {
      const fund = await api.joinFund(token, code);
      event.currentTarget.reset();
      setActiveFundID(fund.id);
      await loadFunds(token);
      await loadDetails(fund.id, token);
      setNotice("Joined fund");
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Failed to join fund");
    } finally {
      setBusy(false);
    }
  }

  async function submitExpense(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || !activeFundID) {
      return;
    }
    const form = new FormData(event.currentTarget);
    const cost = Number(form.get("cost"));
    const description = String(form.get("description") ?? "").trim();
    if (!cost || !description) {
      return;
    }
    setBusy(true);
    try {
      await api.addExpense(token, activeFundID, cost, description);
      event.currentTarget.reset();
      await loadDetails(activeFundID, token);
      setNotice("Expense added");
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Failed to add expense");
    } finally {
      setBusy(false);
    }
  }

  async function submitVirtualUser(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || !activeFundID) {
      return;
    }
    const form = new FormData(event.currentTarget);
    const firstName = String(form.get("firstName") ?? "").trim();
    if (!firstName) {
      return;
    }
    setBusy(true);
    try {
      await api.addVirtualUser(token, activeFundID, firstName);
      event.currentTarget.reset();
      await loadDetails(activeFundID, token);
      setNotice("Member added");
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Failed to add member");
    } finally {
      setBusy(false);
    }
  }

  async function removeMember(userID: number) {
    if (!token || !activeFundID) {
      return;
    }
    setBusy(true);
    try {
      await api.removeMember(token, activeFundID, userID);
      await loadDetails(activeFundID, token);
      setNotice("Member removed");
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Failed to remove member");
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
    <main className="app-shell px-4 py-4 text-paper sm:px-6 lg:px-8">
      <div className="mx-auto flex min-h-[calc(100vh-2rem)] w-full max-w-7xl flex-col gap-4">
        <Header token={token} onLogout={logout} onRefresh={() => token && loadFunds(token)} />

        {notice ? (
          <div className="panel rounded-lg px-4 py-3 text-sm font-semibold text-paper/80">
            {notice}
          </div>
        ) : null}

        {!token ? (
          <AuthPanel
            busy={busy}
            status={status}
            sessionID={sessionID}
            onStart={startTelegramAuth}
          />
        ) : (
          <div className="grid flex-1 gap-4 lg:grid-cols-[340px_minmax(0,1fr)]">
            <aside className="grid gap-4 lg:content-start">
              <FundForms
                busy={busy}
                onCreate={submitCreateFund}
                onJoin={submitJoinFund}
              />
              <FundList
                funds={funds}
                activeFundID={activeFundID}
                onSelect={setActiveFundID}
              />
            </aside>

            <section className="grid min-w-0 gap-4">
              {activeFund ? (
                <>
                  <FundHero fund={activeFund} />
                  <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
                    <Metric
                      label="Total spent"
                      value={money(totalSpent)}
                      icon={<BadgeDollarSign size={20} />}
                      accent="mint"
                    />
                    <Metric
                      label="Average"
                      value={money(average)}
                      icon={<BanknoteArrowUp size={20} />}
                      accent="amber"
                    />
                    <Metric
                      label="Members"
                      value={String(totalMembers)}
                      icon={<Users size={20} />}
                      accent="aqua"
                    />
                    <Metric
                      label="Transfers"
                      value={String(debtRows.length)}
                      icon={<ReceiptText size={20} />}
                      accent="coral"
                    />
                  </div>

                  <div className="grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(320px,0.95fr)]">
                    <ExpensePanel busy={busy} onSubmit={submitExpense} />
                    <SettlementPanel debts={debtRows} />
                  </div>

                  <div className="grid gap-4 xl:grid-cols-[minmax(320px,0.88fr)_minmax(0,1.12fr)]">
                    <MembersPanel
                      busy={busy}
                      members={details.members}
                      onAdd={submitVirtualUser}
                      onRemove={removeMember}
                    />
                    <PurchasesPanel purchases={details.purchases} />
                  </div>
                </>
              ) : (
                <EmptyState />
              )}
            </section>
          </div>
        )}
      </div>
    </main>
  );
}

function telegramLink(uuid: string) {
  return `https://t.me/${botName}?start=auth_${uuid}`;
}

function Header({
  token,
  onLogout,
  onRefresh
}: {
  token: string | null;
  onLogout: () => void;
  onRefresh: () => void;
}) {
  return (
    <header className="panel flex flex-wrap items-center justify-between gap-3 rounded-lg px-4 py-3">
      <div className="flex min-w-0 items-center gap-3">
        <div className="grid size-11 shrink-0 place-items-center rounded-lg bg-mint text-ink shadow-glow">
          <Sparkles size={21} />
        </div>
        <div className="min-w-0">
          <h1 className="truncate text-xl font-black tracking-normal text-paper sm:text-2xl">
            SplitCore
          </h1>
          <p className="truncate text-sm font-semibold text-paper/52">
            Funds, members, expenses, settlement
          </p>
        </div>
      </div>
      {token ? (
        <div className="flex w-full gap-2 sm:w-auto">
          <Button
            tone="soft"
            className="flex-1 sm:flex-none"
            icon={<RefreshCw size={17} />}
            onClick={onRefresh}
          >
            Refresh
          </Button>
          <Button
            tone="ghost"
            className="flex-1 sm:flex-none"
            icon={<LogOut size={17} />}
            onClick={onLogout}
          >
            Logout
          </Button>
        </div>
      ) : null}
    </header>
  );
}

function AuthPanel({
  busy,
  status,
  sessionID,
  onStart
}: {
  busy: boolean;
  status: string;
  sessionID: string;
  onStart: () => void;
}) {
  const link = sessionID ? telegramLink(sessionID) : "";

  return (
    <section className="grid flex-1 place-items-center py-8">
      <div className="panel grid w-full max-w-5xl gap-6 overflow-hidden rounded-lg p-5 sm:p-7 lg:grid-cols-[1.05fr_0.95fr]">
        <div className="grid content-center gap-6">
          <div className="max-w-2xl">
            <p className="mb-3 inline-flex rounded-lg border border-mint/24 bg-mint/12 px-3 py-1 text-sm font-bold text-mint">
              Telegram-powered expense control
            </p>
            <h2 className="text-4xl font-black tracking-normal text-paper sm:text-5xl lg:text-6xl">
              Shared money without the awkward math.
            </h2>
            <p className="mt-4 max-w-xl text-base font-medium leading-7 text-paper/64 sm:text-lg">
              Open your SplitCore dashboard, connect through Telegram, and manage funds,
              invite codes, spend history, members, and settlement in one responsive cockpit.
            </p>
          </div>

          <div className="flex flex-col gap-3 sm:flex-row">
            <Button
              className="w-full sm:w-auto"
              icon={<Send size={18} />}
              disabled={busy}
              onClick={onStart}
            >
              Connect Telegram
            </Button>
            {link ? (
              <a
                className="focus-ring inline-flex min-h-11 items-center justify-center rounded-lg border border-paper/10 bg-paper/10 px-4 py-2.5 text-sm font-bold text-paper transition hover:bg-paper/16"
                href={link}
                target="_blank"
                rel="noreferrer"
              >
                Open auth link
              </a>
            ) : null}
          </div>
        </div>

        <div className="surface grid gap-4 rounded-lg p-4">
          <div className="rounded-lg bg-ink/70 p-4">
            <p className="text-sm font-bold uppercase text-paper/48">Session</p>
            <p className="mt-2 break-all font-mono text-sm text-mint">
              {sessionID || "Waiting for login"}
            </p>
          </div>
          <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
            {["init", status, "token"].map((step, index) => (
              <div key={`${step}-${index}`} className="rounded-lg border border-paper/10 bg-paper/6 p-4">
                <p className="text-xs font-black uppercase text-paper/44">Step {index + 1}</p>
                <p className="mt-2 text-lg font-black capitalize text-paper">{step}</p>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function FundForms({
  busy,
  onCreate,
  onJoin
}: {
  busy: boolean;
  onCreate: (event: FormEvent<HTMLFormElement>) => void;
  onJoin: (event: FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <div className="panel grid gap-4 rounded-lg p-4">
      <form className="grid gap-3" onSubmit={onCreate}>
        <TextInput name="name" label="New fund" placeholder="Roommates, Trip, BBQ" />
        <Button disabled={busy} icon={<Plus size={17} />}>
          Create fund
        </Button>
      </form>
      <div className="h-px bg-paper/10" />
      <form className="grid gap-3" onSubmit={onJoin}>
        <TextInput name="invite" label="Invite code" placeholder="aB12cD" />
        <Button tone="soft" disabled={busy} icon={<ArrowRight size={17} />}>
          Join fund
        </Button>
      </form>
    </div>
  );
}

function FundList({
  funds,
  activeFundID,
  onSelect
}: {
  funds: Fund[];
  activeFundID: number | null;
  onSelect: (id: number) => void;
}) {
  return (
    <div className="panel rounded-lg p-3">
      <div className="mb-3 flex items-center justify-between px-1">
        <h2 className="text-lg font-black">My funds</h2>
        <span className="rounded-lg bg-paper/10 px-2 py-1 text-xs font-bold text-paper/62">
          {funds.length}
        </span>
      </div>
      <div className="grid max-h-[420px] gap-2 overflow-auto pr-1">
        {funds.map((fund) => (
          <button
            key={fund.id}
            className={`focus-ring rounded-lg border p-3 text-left transition ${
              activeFundID === fund.id
                ? "border-mint/40 bg-mint/13"
                : "border-paper/8 bg-paper/5 hover:bg-paper/9"
            }`}
            onClick={() => onSelect(fund.id)}
          >
            <p className="truncate text-base font-black text-paper">{fund.name}</p>
            <p className="mt-1 truncate text-sm font-semibold text-paper/48">
              code {fund.invite_code}
            </p>
          </button>
        ))}
        {funds.length === 0 ? (
          <div className="rounded-lg border border-dashed border-paper/14 p-5 text-sm font-semibold text-paper/46">
            No funds yet.
          </div>
        ) : null}
      </div>
    </div>
  );
}

function FundHero({ fund }: { fund: Fund }) {
  async function copyInvite() {
    await navigator.clipboard.writeText(fund.invite_code);
  }

  return (
    <div className="panel overflow-hidden rounded-lg">
      <div className="grid gap-4 p-5 sm:grid-cols-[minmax(0,1fr)_auto] sm:p-6">
        <div className="min-w-0">
          <p className="text-sm font-bold uppercase text-mint">Active fund</p>
          <h2 className="mt-2 break-words text-3xl font-black tracking-normal text-paper sm:text-4xl">
            {fund.name}
          </h2>
          <p className="mt-2 text-sm font-semibold text-paper/50">
            Created {shortDate(fund.created_at)}
          </p>
        </div>
        <button
          className="focus-ring flex min-h-16 items-center justify-between gap-4 rounded-lg border border-amber/24 bg-amber/12 px-4 text-left text-amber transition hover:bg-amber/18 sm:min-w-56"
          onClick={copyInvite}
          title="Copy invite code"
        >
          <span>
            <span className="block text-xs font-black uppercase opacity-70">Invite</span>
            <span className="font-mono text-xl font-black">{fund.invite_code}</span>
          </span>
          <Copy size={18} />
        </button>
      </div>
    </div>
  );
}

function ExpensePanel({
  busy,
  onSubmit
}: {
  busy: boolean;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  return (
    <form className="panel grid gap-4 rounded-lg p-4" onSubmit={onSubmit}>
      <div>
        <h3 className="text-xl font-black">Log expense</h3>
        <p className="mt-1 text-sm font-semibold text-paper/50">Amount and a short note.</p>
      </div>
      <div className="grid gap-3 sm:grid-cols-[160px_minmax(0,1fr)]">
        <TextInput name="cost" label="Cost" min="0.01" step="0.01" type="number" placeholder="150.50" />
        <TextInput name="description" label="Description" placeholder="Taxi to hotel" />
      </div>
      <Button disabled={busy} icon={<ReceiptText size={17} />}>
        Add expense
      </Button>
    </form>
  );
}

function SettlementPanel({ debts }: { debts: Array<Debt & { from: string; to: string }> }) {
  return (
    <div className="panel rounded-lg p-4">
      <h3 className="text-xl font-black">Settlement</h3>
      <div className="mt-4 grid gap-3">
        {debts.map((debt, index) => (
          <div key={`${debt.from_id}-${debt.to_id}-${index}`} className="surface rounded-lg p-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <p className="font-black text-paper">{debt.from}</p>
              <span className="rounded-lg bg-coral/16 px-3 py-1 text-sm font-black text-coral">
                {money(debt.amount)}
              </span>
              <p className="font-black text-mint">{debt.to}</p>
            </div>
          </div>
        ))}
        {debts.length === 0 ? (
          <div className="rounded-lg border border-mint/18 bg-mint/10 p-4 text-sm font-bold text-mint">
            All settled up.
          </div>
        ) : null}
      </div>
    </div>
  );
}

function MembersPanel({
  busy,
  members,
  onAdd,
  onRemove
}: {
  busy: boolean;
  members: User[];
  onAdd: (event: FormEvent<HTMLFormElement>) => void;
  onRemove: (userID: number) => void;
}) {
  return (
    <div className="panel grid gap-4 rounded-lg p-4">
      <div>
        <h3 className="text-xl font-black">Members</h3>
        <p className="mt-1 text-sm font-semibold text-paper/50">Real and virtual participants.</p>
      </div>
      <form className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]" onSubmit={onAdd}>
        <TextInput name="firstName" label="Virtual member" placeholder="Alex" />
        <Button className="self-end" disabled={busy} icon={<UserPlus size={17} />}>
          Add
        </Button>
      </form>
      <div className="grid gap-2">
        {members.map((member) => (
          <div
            key={member.id}
            className="surface flex items-center justify-between gap-3 rounded-lg p-3"
          >
            <div className="min-w-0">
              <p className="truncate font-black text-paper">{displayUser(member)}</p>
              <p className="truncate text-sm font-semibold text-paper/46">
                {member.is_virtual ? "Virtual" : "Telegram"} member
              </p>
            </div>
            {member.is_virtual ? (
              <button
                className="focus-ring grid size-10 shrink-0 place-items-center rounded-lg bg-coral/14 text-coral transition hover:bg-coral/22"
                onClick={() => onRemove(member.id)}
                title="Remove member"
              >
                <Trash2 size={17} />
              </button>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  );
}

function PurchasesPanel({ purchases }: { purchases: Purchase[] }) {
  return (
    <div className="panel rounded-lg p-4">
      <div className="mb-4 flex items-center justify-between gap-3">
        <h3 className="text-xl font-black">Expense history</h3>
        <span className="rounded-lg bg-paper/10 px-2 py-1 text-xs font-bold text-paper/62">
          {purchases.length}
        </span>
      </div>
      <div className="grid gap-3">
        {purchases.map((purchase) => (
          <div key={purchase.id} className="surface rounded-lg p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <p className="break-words font-black text-paper">{purchase.description}</p>
                <p className="mt-1 text-sm font-semibold text-paper/48">
                  {displayUser(purchase.payer)} · {shortDate(purchase.created_at)}
                </p>
              </div>
              <p className="rounded-lg bg-mint/12 px-3 py-1 font-black text-mint">
                {money(purchase.amount)}
              </p>
            </div>
          </div>
        ))}
        {purchases.length === 0 ? (
          <div className="rounded-lg border border-dashed border-paper/14 p-5 text-sm font-semibold text-paper/46">
            No expenses yet.
          </div>
        ) : null}
      </div>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="panel grid min-h-[420px] place-items-center rounded-lg p-6 text-center">
      <div className="max-w-md">
        <div className="mx-auto grid size-14 place-items-center rounded-lg bg-mint text-ink shadow-glow">
          <Plus size={24} />
        </div>
        <h2 className="mt-5 text-3xl font-black">Create your first fund</h2>
        <p className="mt-2 text-base font-semibold leading-7 text-paper/58">
          Your dashboard will show invite codes, members, spending history, and the optimized
          settlement plan.
        </p>
      </div>
    </div>
  );
}

export default App;
