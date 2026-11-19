import { useEffect, useMemo, useRef, useState } from "react";
import { API, type CloudPhone } from "../mygo";
import { useApp } from "../lib/appctx";
import {
  accountStatus,
  accountTitle,
  errText,
  loginState,
} from "../lib/format";
import type { IconName } from "../components/Icon";
import { Icon } from "../components/Icon";
import { Avatar, Chip, EmptyState, PageHead, Spinner } from "../components/ui";

type StatusFilter = "all" | "online" | "logging" | "abnormal" | "offline";
type SortKey = "remark" | "number";

const STATUS_OPTIONS: { value: StatusFilter; label: string }[] = [
  { value: "all", label: "全部状态" },
  { value: "online", label: "在线" },
  { value: "logging", label: "登扣中" },
  { value: "abnormal", label: "异常" },
  { value: "offline", label: "离线" },
];

export function Accounts() {
  const {
    snap,
    refresh,
    notify,
    query,
    setQuery,
    openAccountForm,
    openImport,
    confirmDelete,
    openConsole,
  } = useApp();
  const [status, setStatus] = useState<StatusFilter>("all");
  const [phoneFilter, setPhoneFilter] = useState("all");
  const [sort, setSort] = useState<{ key: SortKey; dir: 1 | -1 }>({ key: "number", dir: 1 });
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [busyPhone, setBusyPhone] = useState<number | null>(null);
  const allRef = useRef<HTMLInputElement>(null);

  const rows = useMemo(() => {
    const q = query.trim().toLowerCase();
    return snap.accounts
      .filter((a) => {
        if (status === "logging") {
          const logging = a.status === "logging_in" || a.login === "waiting" || a.login === "ready";
          if (!logging) return false;
        } else if (status !== "all" && a.status !== status) {
          return false;
        }
        if (phoneFilter === "unbound") {
          if (a.bound_phone_id !== 0 || a.bound_phone !== "") return false;
        } else if (phoneFilter !== "all" && a.bound_phone !== phoneFilter) {
          return false;
        }
        if (!q) return true;
        return [a.remark, a.number, a.display, a.bound_phone].some((v) =>
          v.toLowerCase().includes(q),
        );
      })
      .sort((x, y) => {
        if (sort.key === "remark") {
          const xr = x.remark || "￿";
          const yr = y.remark || "￿";
          return xr.localeCompare(yr, "zh-CN") * sort.dir;
        }
        return x.number.localeCompare(y.number, "zh-CN", { numeric: true }) * sort.dir;
      });
  }, [snap.accounts, status, phoneFilter, query, sort]);

  const allSelected = rows.length > 0 && rows.every((r) => selected.has(r.id));
  const someSelected = selected.size > 0 && !allSelected;

  useEffect(() => {
    if (allRef.current) allRef.current.indeterminate = someSelected;
  }, [someSelected]);

  useEffect(() => {
    setSelected((prev) => {
      const ids = new Set(snap.accounts.map((a) => a.id));
      const next = new Set([...prev].filter((id) => ids.has(id)));
      return next.size === prev.size ? prev : next;
    });
  }, [snap.accounts]);

  const loggingCount = snap.accounts.filter(
    (a) => a.status === "logging_in" || a.login === "waiting" || a.login === "ready",
  ).length;
  const onlineCount = snap.accounts.filter((a) => a.status === "online").length;
  const abnormalCount = snap.accounts.filter((a) => a.status === "abnormal").length;

  function toggleRow(id: number) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAll() {
    setSelected((prev) => {
      if (rows.length > 0 && rows.every((r) => prev.has(r.id))) {
        return new Set();
      }
      return new Set(rows.map((r) => r.id));
    });
  }

  function clearFilters() {
    setStatus("all");
    setPhoneFilter("all");
    setQuery("");
  }

  async function exportIds(ids: number[]) {
    try {
      const r = await API.exportAccountsCSV(ids);
      if (!r.path) {
        notify("已取消导出", "info");
        return;
      }
      notify(`已导出 ${r.count} 个账号到 ${r.path}`);
    } catch (e) {
      notify(errText(e), "error");
    }
  }

  async function runPhoneAction(p: CloudPhone) {
    const action = p.state === "running" ? "restart" : "start";
    setBusyPhone(p.id);
    try {
      await API.setPhoneState(p.id, action);
      notify(action === "restart" ? `云机 ${p.name} 正在重启` : `云机 ${p.name} 正在开机`);
      await refresh();
    } catch (e) {
      notify(errText(e), "error");
    } finally {
      setBusyPhone(null);
    }
  }

  const filtered = rows.length !== snap.accounts.length;

  return (
    <div className="page">
      <PageHead
        title="账号管理"
        subtitle="管理平台账号、绑定云机与登扣状态，支持批量导入与导出。"
        actions={
          <>
            <button type="button" className="btn" onClick={openImport}>
              <Icon name="upload" size={15} />
              批量导入
            </button>
            <button
              type="button"
              className="btn"
              onClick={() => exportIds(rows.map((r) => r.id))}
              disabled={rows.length === 0}
            >
              <Icon name="download" size={15} />
              导出
            </button>
            <button type="button" className="btn primary" onClick={() => openAccountForm()}>
              <Icon name="plus" size={15} />
              存号
            </button>
          </>
        }
      />

      <section className="card filters">
        <label className="field">
          <span className="field-label">状态</span>
          <select
            className="select"
            value={status}
            onChange={(e) => setStatus(e.target.value as StatusFilter)}
          >
            {STATUS_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span className="field-label">绑定云机</span>
          <select
            className="select"
            value={phoneFilter}
            onChange={(e) => setPhoneFilter(e.target.value)}
          >
            <option value="all">全部云机</option>
            <option value="unbound">未绑定</option>
            {snap.phones.map((p) => (
              <option key={p.id} value={p.name}>
                {p.name} · {p.region}
              </option>
            ))}
          </select>
        </label>
        <div className="field grow">
          <span className="field-label">搜索</span>
          <div className="search inline">
            <Icon name="search" size={15} />
            <input
              value={query}
              placeholder="备注、账号、昵称"
              onChange={(e) => setQuery(e.target.value)}
            />
            {query && (
              <button
                type="button"
                className="search-clear"
                onClick={() => setQuery("")}
                aria-label="清空搜索"
              >
                <Icon name="x" size={13} />
              </button>
            )}
          </div>
        </div>
        <div className="filters-spacer" />
        <button type="button" className="btn ghost" onClick={clearFilters}>
          清空筛选
        </button>
      </section>

      <section className="mini-stats" aria-label="账号统计">
        <MiniStat icon="accounts" label="账号总数" value={snap.accounts.length} />
        <MiniStat icon="check-circle" label="在线" value={onlineCount} tone="green" />
        <MiniStat icon="clock" label="登扣中" value={loggingCount} tone="blue" />
        <MiniStat icon="alert" label="异常" value={abnormalCount} tone="red" />
      </section>

      <section className="card table-card">
        {selected.size > 0 && (
          <div className="bulkbar">
            <Icon name="check-circle" size={15} />
            <span>
              已选 <strong>{selected.size}</strong> 项
            </span>
            <span className="filters-spacer" />
            <button
              type="button"
              className="btn sm"
              onClick={() => exportIds([...selected])}
            >
              <Icon name="download" size={14} />
              批量导出
            </button>
            <button
              type="button"
              className="btn sm danger-ghost"
              onClick={() => confirmDelete([...selected])}
            >
              <Icon name="trash" size={14} />
              批量删除
            </button>
            <button type="button" className="btn sm ghost" onClick={() => setSelected(new Set())}>
              清空选择
            </button>
          </div>
        )}

        <div className="table-wrap">
          <table className="data">
            <thead>
              <tr>
                <th className="check-col">
                  <input
                    ref={allRef}
                    type="checkbox"
                    checked={allSelected}
                    onChange={toggleAll}
                    aria-label="全选当前行"
                  />
                </th>
                <th>
                  <button
                    type="button"
                    className="sort-btn"
                    onClick={() =>
                      setSort((s) =>
                        s.key === "remark" ? { key: "remark", dir: s.dir === 1 ? -1 : 1 } : { key: "remark", dir: 1 },
                      )
                    }
                  >
                    备注
                    {sort.key === "remark" && <SortArrow dir={sort.dir} />}
                  </button>
                </th>
                <th>
                  <button
                    type="button"
                    className="sort-btn"
                    onClick={() =>
                      setSort((s) =>
                        s.key === "number" ? { key: "number", dir: s.dir === 1 ? -1 : 1 } : { key: "number", dir: 1 },
                      )
                    }
                  >
                    账号
                    {sort.key === "number" && <SortArrow dir={sort.dir} />}
                  </button>
                </th>
                <th>绑定云机</th>
                <th>状态</th>
                <th>云机操作</th>
                <th className="right">操作</th>
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 ? (
                <tr>
                  <td colSpan={7}>
                    <EmptyState
                      icon="accounts"
                      title={filtered ? "没有匹配的账号" : "还没有账号"}
                      hint={
                        filtered
                          ? "换个关键词，或清空筛选条件看看全部账号。"
                          : "存一个账号，或从 CSV 文本批量导入，开始管理云机上的登录。"
                      }
                      action={
                        filtered ? (
                          <button type="button" className="btn" onClick={clearFilters}>
                            清空筛选
                          </button>
                        ) : (
                          <button
                            type="button"
                            className="btn primary"
                            onClick={() => openAccountForm()}
                          >
                            <Icon name="plus" size={15} />
                            存号
                          </button>
                        )
                      }
                    />
                  </td>
                </tr>
              ) : (
                rows.map((a) => {
                  const phone =
                    snap.phones.find((p) => p.id === a.bound_phone_id) ??
                    snap.phones.find((p) => p.name === a.bound_phone);
                  const boundName = phone?.name ?? a.bound_phone;
                  const selectedRow = selected.has(a.id);
                  return (
                    <tr key={a.id} className={selectedRow ? "selected" : ""}>
                      <td className="check-col">
                        <input
                          type="checkbox"
                          checked={selectedRow}
                          onChange={() => toggleRow(a.id)}
                          aria-label={`选择 ${accountTitle(a)}`}
                        />
                      </td>
                      <td>
                        {a.remark ? (
                          <span className="remark">{a.remark}</span>
                        ) : (
                          <span className="muted">—</span>
                        )}
                      </td>
                      <td>
                        <div className="cell-account">
                          <Avatar name={accountTitle(a)} size="sm" />
                          <div className="account-text">
                            <div className="account-name">{accountTitle(a)}</div>
                            <div className="account-num">{a.number}</div>
                          </div>
                        </div>
                      </td>
                      <td>
                        {boundName ? (
                          <div className="phone-text">
                            <div className="phone-name">{boundName}</div>
                            {phone && <div className="phone-region">{phone.region}</div>}
                          </div>
                        ) : (
                          <span className="muted">未绑定</span>
                        )}
                      </td>
                      <td>
                        <div className="status-cell">
                          <Chip {...accountStatus(a.status)} dot />
                          {(a.login === "waiting" || a.login === "ready") && (
                            <Chip {...loginState(a.login)} />
                          )}
                        </div>
                      </td>
                      <td>
                        {phone ? (
                          <button
                            type="button"
                            className="btn sm"
                            disabled={busyPhone === phone.id}
                            onClick={() => runPhoneAction(phone)}
                            title={`${phone.state === "running" ? "重启" : "开机"}云机 ${phone.name}`}
                          >
                            {busyPhone === phone.id ? (
                              <Spinner size="tiny" />
                            ) : (
                              <Icon name={phone.state === "running" ? "restart" : "power"} size={14} />
                            )}
                            {phone.state === "running" ? "重启" : "开机"}
                          </button>
                        ) : (
                          <span className="muted">—</span>
                        )}
                      </td>
                      <td>
                        <div className="row-actions">
                          <button
                            type="button"
                            className="iconbtn accent"
                            title="打开控制台"
                            aria-label={`打开 ${accountTitle(a)} 的控制台`}
                            onClick={() => openConsole(a.id)}
                          >
                            <Icon name="terminal" size={16} />
                          </button>
                          <button
                            type="button"
                            className="iconbtn"
                            title="编辑账号"
                            aria-label={`编辑 ${accountTitle(a)}`}
                            onClick={() => openAccountForm(a)}
                          >
                            <Icon name="edit" size={15} />
                          </button>
                          <button
                            type="button"
                            className="iconbtn danger"
                            title="删除账号"
                            aria-label={`删除 ${accountTitle(a)}`}
                            onClick={() => confirmDelete([a.id])}
                          >
                            <Icon name="trash" size={15} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>

        {snap.tasks.length > 0 && (
          <div className="taskbar">
            <Spinner size="tiny" />
            <strong>进行中任务</strong>
            <div className="task-pills">
              {snap.tasks.map((t) => (
                <span key={t.id} className="task-pill">
                  {t.label}
                </span>
              ))}
            </div>
          </div>
        )}

        <div className="table-foot">
          <span>
            {filtered
              ? `筛选出 ${rows.length} 个账号 · 共 ${snap.accounts.length} 个`
              : `共 ${snap.accounts.length} 个账号`}
          </span>
          <span className="foot-hint">
            <Icon name="shield" size={13} />
            账号数据由服务端统一保存，多个终端实时共享
          </span>
        </div>
      </section>
    </div>
  );
}

function SortArrow({ dir }: { dir: 1 | -1 }) {
  return <span className="sort-arrow">{dir === 1 ? "↑" : "↓"}</span>;
}

function MiniStat({
  icon,
  label,
  value,
  tone = "",
}: {
  icon: IconName;
  label: string;
  value: number;
  tone?: string;
}) {
  return (
    <div className={`card mini-stat${tone ? ` ${tone}` : ""}`}>
      <span className="mini-stat-ico">
        <Icon name={icon} size={16} />
      </span>
      <div className="mini-stat-text">
        <div className="mini-stat-value">{value}</div>
        <div className="mini-stat-label">{label}</div>
      </div>
    </div>
  );
}
