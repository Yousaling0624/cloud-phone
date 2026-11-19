import { useCallback, useEffect, useRef, useState } from "react";
import { API, events, type Account, type Snapshot } from "./mygo";
import { AppContext } from "./lib/appctx";
import { errText } from "./lib/format";
import type { PageKey, Tone, ToastItem } from "./lib/types";
import { Icon, type IconName } from "./components/Icon";
import { Avatar, Spinner } from "./components/ui";
import { Toasts } from "./components/Toast";
import { Dashboard } from "./pages/Dashboard";
import { Accounts } from "./pages/Accounts";
import { SmsPage } from "./pages/SmsPage";
import { PhonesPage } from "./pages/PhonesPage";
import { SimsPage } from "./pages/SimsPage";
import { WalletPage } from "./pages/WalletPage";
import { UsersPage } from "./pages/UsersPage";
import { SettingsPage } from "./pages/SettingsPage";
import { Placeholder } from "./pages/Placeholder";
import { AccountFormModal, ConfirmDeleteModal, ImportCsvModal } from "./modals/AccountModals";
import { ConsoleModal } from "./modals/ConsoleModal";
import { ConnectionModal } from "./modals/ConnectionModal";

const TOP_TABS: { key: PageKey; label: string; icon: IconName }[] = [
  { key: "accounts", label: "账号管理", icon: "accounts" },
  { key: "users", label: "用户信息", icon: "user" },
  { key: "sms", label: "收信箱", icon: "sms" },
  { key: "phones", label: "云手机管理", icon: "phone" },
];

// The context exists before the first snapshot arrives; pages are only
// rendered once snap is set, so this placeholder never reaches them.
const EMPTY_SNAPSHOT: Snapshot = {
  accounts: [],
  phones: [],
  sms: [],
  sims: [],
  users: [],
  wallet: { balance: 0 },
  ledger: [],
  tasks: [],
  settings: {
    provider_base_url: "",
    provider_token: "",
    login_method: "password",
    auto_fill: true,
    auto_bind: true,
    save_log: false,
  },
  now: new Date().toISOString(),
};

export function App() {
  const [snap, setSnap] = useState<Snapshot | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState<PageKey>("dashboard");
  const [query, setQuery] = useState("");
  const [refreshing, setRefreshing] = useState(false);
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [accountsOpen, setAccountsOpen] = useState(true);
  const [accountForm, setAccountForm] = useState<{ account: Account | null } | null>(null);
  const [importOpen, setImportOpen] = useState(false);
  const [deleteIds, setDeleteIds] = useState<number[] | null>(null);
  const [consoleId, setConsoleId] = useState<number | null>(null);
  const [connectionOpen, setConnectionOpen] = useState(false);

  const toastSeq = useRef(0);
  const refreshTimer = useRef<number | null>(null);
  const mainRef = useRef<HTMLElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  const refresh = useCallback(async () => {
    setRefreshing(true);
    try {
      const next = await API.snapshot();
      setSnap(next);
      setError(null);
    } catch (e) {
      setError(errText(e));
    } finally {
      setRefreshing(false);
    }
  }, []);

  const notify = useCallback((message: string, tone: Tone = "success") => {
    const id = ++toastSeq.current;
    setToasts((list) => [...list, { id, message, tone }]);
    window.setTimeout(() => {
      setToasts((list) => list.filter((t) => t.id !== id));
    }, 3600);
  }, []);

  const dismissToast = useCallback((id: number) => {
    setToasts((list) => list.filter((t) => t.id !== id));
  }, []);

  const go = useCallback((next: PageKey) => {
    setPage(next);
  }, []);

  useEffect(() => {
    void refresh();
    const off = events.stateChanged.on(() => {
      if (refreshTimer.current !== null) window.clearTimeout(refreshTimer.current);
      refreshTimer.current = window.setTimeout(() => {
        void refresh();
      }, 120);
    });
    return () => {
      off();
      if (refreshTimer.current !== null) window.clearTimeout(refreshTimer.current);
    };
  }, [refresh]);

  // A visible-page fallback in case the event stream was interrupted; SSE
  // normally refreshes sooner.
  useEffect(() => {
    const id = window.setInterval(() => {
      if (document.visibilityState === "visible") void refresh();
    }, 15000);
    return () => window.clearInterval(id);
  }, [refresh]);

  useEffect(() => {
    mainRef.current?.scrollTo({ top: 0 });
  }, [page]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        searchRef.current?.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const admin = snap?.users.find((u) => u.username === "admin") ?? snap?.users[0];
  const operatorName = admin?.nickname ?? "喜相逢";
  const operatorRole = admin?.role ?? "管理员";

  function onSearch(value: string) {
    setQuery(value);
    if (value && page !== "accounts") setPage("accounts");
  }

  const accountsSection = page === "accounts" || page === "stock" || page === "extract";

  return (
    <AppContext.Provider
      value={{
        snap: snap ?? EMPTY_SNAPSHOT,
        refresh,
        notify,
        go,
        query,
        setQuery: onSearch,
        openAccountForm: (account) => setAccountForm({ account: account ?? null }),
        openImport: () => setImportOpen(true),
        confirmDelete: (ids) => setDeleteIds(ids),
        openConsole: (id) => setConsoleId(id),
      }}
    >
      <div className="app">
        <aside className="sidebar">
          <div className="brand">
            <span className="brand-mark">存</span>
            <div className="brand-text">
              <div className="brand-name">存号网络</div>
              <div className="brand-sub">账号运营平台</div>
            </div>
          </div>

          <nav className="nav" aria-label="主导航">
            <NavItem
              icon="dashboard"
              label="工作台"
              active={page === "dashboard"}
              onClick={() => go("dashboard")}
            />
            <NavItem
              icon="sms"
              label="短信管理"
              active={page === "sms"}
              onClick={() => go("sms")}
            />
            <div className={`nav-group${accountsOpen ? " open" : ""}`}>
              <button
                type="button"
                className={`nav-item nav-group-head${accountsSection ? " parent-active" : ""}`}
                aria-expanded={accountsOpen}
                onClick={() => setAccountsOpen((v) => !v)}
              >
                <span className="nav-ico">
                  <Icon name="accounts" size={17} />
                </span>
                <span className="nav-label">账号管理</span>
                <Icon
                  name={accountsOpen ? "chevron-down" : "chevron-right"}
                  size={14}
                  className="nav-chev"
                />
              </button>
              {accountsOpen && (
                <div className="nav-sub">
                  <SubNavItem
                    label="账号管理"
                    active={page === "accounts"}
                    onClick={() => go("accounts")}
                  />
                  <SubNavItem
                    label="账号库存管理"
                    active={page === "stock"}
                    onClick={() => go("stock")}
                  />
                  <SubNavItem
                    label="提号管理"
                    active={page === "extract"}
                    onClick={() => go("extract")}
                  />
                </div>
              )}
            </div>
            <NavItem
              icon="slot"
              label="卡位管理"
              active={page === "sims"}
              onClick={() => go("sims")}
            />
            <NavItem
              icon="phone"
              label="云手机管理"
              active={page === "phones"}
              onClick={() => go("phones")}
            />
            <NavItem
              icon="merchant"
              label="商家管理"
              active={page === "merchant"}
              onClick={() => go("merchant")}
            />
            <NavItem
              icon="wallet"
              label="系统钱包"
              active={page === "wallet"}
              onClick={() => go("wallet")}
            />
            <NavItem
              icon="system"
              label="系统管理"
              active={page === "users"}
              onClick={() => go("users")}
            />
            <NavItem
              icon="gear"
              label="系统设置"
              active={page === "settings"}
              onClick={() => go("settings")}
            />
          </nav>

          <div className="operator">
            <Avatar name={operatorName} size="sm" />
            <div className="op-text">
              <div className="op-name">{operatorName}</div>
              <div className="op-role">{operatorRole}</div>
            </div>
            <span className="op-dot" title="在线" />
          </div>
        </aside>

        <div className="content">
          <header className="topbar">
            <nav className="top-tabs" aria-label="快捷页签">
              {TOP_TABS.map((t) => (
                <button
                  key={t.key}
                  type="button"
                  className={`top-tab${page === t.key ? " active" : ""}`}
                  onClick={() => go(t.key)}
                >
                  <Icon name={t.icon} size={15} />
                  {t.label}
                </button>
              ))}
            </nav>
            <div className="topbar-spacer" />
            <div className="search">
              <Icon name="search" size={15} />
              <input
                ref={searchRef}
                value={query}
                placeholder="搜索账号、备注、昵称"
                onChange={(e) => onSearch(e.target.value)}
              />
              {query ? (
                <button
                  type="button"
                  className="search-clear"
                  onClick={() => setQuery("")}
                  aria-label="清空搜索"
                >
                  <Icon name="x" size={13} />
                </button>
              ) : (
                <span className="search-kbd">Ctrl K</span>
              )}
            </div>
            <button
              type="button"
              className={`iconbtn refresh-btn${refreshing ? " spin" : ""}`}
              title="刷新数据"
              aria-label="刷新数据"
              onClick={() => void refresh()}
            >
              <Icon name="refresh" size={17} />
            </button>
            <span className="mode-pill">
              <span className="mode-dot" />
              运营模式
            </span>
          </header>

          <main className="main" ref={mainRef}>
            {error && snap && (
              <div className="notice bad page-error">
                <Icon name="alert" size={15} />
                <span>{error}</span>
                <button type="button" className="link-btn" onClick={() => void refresh()}>
                  重试
                </button>
              </div>
            )}

            {!snap ? (
              error ? (
                <div className="page-loading">
                  <span className="empty-ico">
                    <Icon name="alert" size={24} />
                  </span>
                  <strong>无法读取运营数据</strong>
                  <span className="muted">{error}</span>
                  <div className="page-loading-actions">
                    <button type="button" className="btn" onClick={() => setConnectionOpen(true)}>
                      <Icon name="gear" size={15} />
                      连接设置
                    </button>
                    <button type="button" className="btn primary" onClick={() => void refresh()}>
                      <Icon name="refresh" size={15} />
                      重新载入
                    </button>
                  </div>
                </div>
              ) : (
                <div className="page-loading">
                  <span className="brand-mark loading-mark">存</span>
                  <Spinner />
                  <span>正在载入运营数据…</span>
                </div>
              )
            ) : (
              renderPage(page)
            )}
          </main>
        </div>
      </div>

      {snap && accountForm && (
        <AccountFormModal
          key={accountForm.account?.id ?? "new"}
          account={accountForm.account ?? undefined}
          onClose={() => setAccountForm(null)}
        />
      )}
      {snap && importOpen && <ImportCsvModal onClose={() => setImportOpen(false)} />}
      {snap && deleteIds && (
        <ConfirmDeleteModal ids={deleteIds} onClose={() => setDeleteIds(null)} />
      )}
      {snap && consoleId !== null && (
        <ConsoleModal accountId={consoleId} onClose={() => setConsoleId(null)} />
      )}
      {connectionOpen && <ConnectionModal onClose={() => setConnectionOpen(false)} />}

      <Toasts toasts={toasts} dismiss={dismissToast} />
    </AppContext.Provider>
  );
}

function renderPage(page: PageKey) {
  switch (page) {
    case "dashboard":
      return <Dashboard />;
    case "accounts":
      return <Accounts />;
    case "sms":
      return <SmsPage />;
    case "phones":
      return <PhonesPage />;
    case "sims":
      return <SimsPage />;
    case "wallet":
      return <WalletPage />;
    case "users":
      return <UsersPage />;
    case "settings":
      return <SettingsPage />;
    default:
      return <Placeholder page={page} />;
  }
}

function NavItem({
  icon,
  label,
  active,
  onClick,
}: {
  icon: IconName;
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={`nav-item${active ? " active" : ""}`}
      aria-current={active ? "page" : undefined}
      onClick={onClick}
    >
      <span className="nav-ico">
        <Icon name={icon} size={17} />
      </span>
      <span className="nav-label">{label}</span>
    </button>
  );
}

function SubNavItem({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={`nav-item sub${active ? " active" : ""}`}
      aria-current={active ? "page" : undefined}
      onClick={onClick}
    >
      <span className="nav-label">{label}</span>
    </button>
  );
}
