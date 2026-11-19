import { useState, type FormEvent } from "react";
import { API, type Account } from "../mygo";
import { useApp } from "../lib/appctx";
import { errText } from "../lib/format";
import { Modal, Spinner } from "../components/ui";

export function AccountFormModal({
  account,
  onClose,
}: {
  account?: Account;
  onClose: () => void;
}) {
  const { snap, refresh, notify } = useApp();
  const editing = !!account;
  const [remark, setRemark] = useState(account?.remark ?? "");
  const [number, setNumber] = useState(account?.number ?? "");
  const [display, setDisplay] = useState(account?.display ?? "");
  const [boundPhoneID, setBoundPhoneID] = useState(account?.bound_phone_id ?? 0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(e: FormEvent) {
    e.preventDefault();
    const trimmed = number.trim();
    if (!trimmed) {
      setError("账号不能为空");
      return;
    }
    setBusy(true);
    setError("");
    try {
      if (account) {
        await API.updateAccount(account.id, remark, display, boundPhoneID);
        notify(`已保存 ${trimmed} 的资料`);
      } else {
        await API.importAccount(remark, trimmed, display, boundPhoneID);
        notify(`已存入账号 ${trimmed}`);
      }
      await refresh();
      onClose();
    } catch (err) {
      setError(errText(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      title={editing ? "编辑账号" : "存号"}
      subtitle={editing ? "修改备注、昵称与绑定的云机" : "录入一个平台账号，稍后可在控制台登扣"}
      width={520}
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn ghost" onClick={onClose}>
            取消
          </button>
          <button type="submit" form="account-form" className="btn primary" disabled={busy}>
            {busy && <Spinner size="tiny" light />}
            {editing ? "保存修改" : "存入账号"}
          </button>
        </>
      }
    >
      <form id="account-form" onSubmit={submit}>
        <div className="form-grid">
          <label className="field full">
            <span className="field-label">账号</span>
            <input
              className="input"
              value={number}
              disabled={editing}
              autoFocus={!editing}
              placeholder="例如 2441864785"
              onChange={(e) => setNumber(e.target.value)}
            />
            <span className="hint">
              {editing
                ? "账号是登录身份，创建后不可修改；如需更换请新建账号。"
                : "平台登录 ID，同一账号不会重复导入。"}
            </span>
          </label>
          <label className="field">
            <span className="field-label">备注</span>
            <input
              className="input"
              value={remark}
              autoFocus={editing}
              placeholder="例如 游戏小号A"
              onChange={(e) => setRemark(e.target.value)}
            />
          </label>
          <label className="field">
            <span className="field-label">昵称</span>
            <input
              className="input"
              value={display}
              placeholder="账号显示的昵称"
              onChange={(e) => setDisplay(e.target.value)}
            />
          </label>
          <label className="field full">
            <span className="field-label">绑定云机</span>
            <select
              className="select"
              value={String(boundPhoneID)}
              onChange={(e) => setBoundPhoneID(Number(e.target.value))}
            >
              <option value="0">不绑定</option>
              {snap.phones.map((p) => {
                const takenByOther = p.account_id !== 0 && p.account_id !== account?.id;
                return (
                  <option key={p.id} value={String(p.id)} disabled={takenByOther}>
                    {p.name} · {p.region} · {p.image}
                    {takenByOther ? "（已被其他账号绑定）" : ""}
                  </option>
                );
              })}
            </select>
          </label>
        </div>
        {error && <div className="notice bad form-notice">{error}</div>}
      </form>
    </Modal>
  );
}

export function ImportCsvModal({ onClose }: { onClose: () => void }) {
  const { refresh, notify } = useApp();
  const [text, setText] = useState(
    "# 每行一个账号：备注,账号\n# 示例：\n# 游戏小号B,2441865001\n# 备用号,2441865002\n",
  );
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<{ added: number; errors: string[] } | null>(null);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!text.trim()) return;
    setBusy(true);
    try {
      const r = await API.importCSV(text);
      setResult(r);
      if (r.added > 0) notify(`已导入 ${r.added} 个账号`);
      await refresh();
      if (r.errors.length === 0) {
        if (r.added === 0) notify("没有新增账号（可能都已存在）", "info");
        onClose();
      }
    } catch (err) {
      notify(errText(err), "error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      title="批量导入账号"
      subtitle="粘贴 CSV 文本，每行一个账号，重复账号会自动跳过"
      width={600}
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn ghost" onClick={onClose}>
            关闭
          </button>
          <button type="submit" form="import-form" className="btn primary" disabled={busy || !text.trim()}>
            {busy && <Spinner size="tiny" light />}
            开始导入
          </button>
        </>
      }
    >
      <form id="import-form" onSubmit={submit}>
        <label className="field">
          <span className="field-label">CSV 内容</span>
          <textarea
            className="input area"
            value={text}
            spellCheck={false}
            onChange={(e) => setText(e.target.value)}
          />
          <span className="hint">
            格式：<code>备注,账号</code>；只有一列时按账号处理。以 # 开头的行会被忽略。
          </span>
        </label>
        {result && result.errors.length > 0 && (
          <div className="notice bad form-notice import-errors">
            <div>
              <strong>部分行未导入：</strong>
              <ul>
                {result.errors.map((e) => (
                  <li key={e}>{e}</li>
                ))}
              </ul>
            </div>
          </div>
        )}
      </form>
    </Modal>
  );
}

export function ConfirmDeleteModal({
  ids,
  onClose,
}: {
  ids: number[];
  onClose: () => void;
}) {
  const { snap, refresh, notify } = useApp();
  const [busy, setBusy] = useState(false);
  const accounts = snap.accounts.filter((a) => ids.includes(a.id));
  const names = accounts.slice(0, 3).map((a) => a.display || a.number);
  const more = accounts.length > 3 ? ` 等 ${accounts.length} 个账号` : "";
  const summary = accounts.length
    ? `将删除 ${names.join("、")}${more}。`
    : "所选账号已不存在，可能已被删除。";

  async function remove() {
    setBusy(true);
    try {
      if (ids.length === 1) {
        const id = ids[0];
        if (id !== undefined) await API.deleteAccount(id);
      } else {
        await API.deleteAccounts(ids);
      }
      notify(`已删除 ${ids.length} 个账号`, "info");
      await refresh();
      onClose();
    } catch (e) {
      notify(errText(e), "error");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      title={ids.length > 1 ? `删除 ${ids.length} 个账号` : "删除账号"}
      width={460}
      onClose={onClose}
      footer={
        <>
          <button type="button" className="btn ghost" onClick={onClose}>
            取消
          </button>
          <button type="button" className="btn danger" disabled={busy || accounts.length === 0} onClick={remove}>
            {busy && <Spinner size="tiny" light />}
            确认删除
          </button>
        </>
      }
    >
      <div className="notice bad">
        <span>删除后无法恢复。账号资料、登录状态与验证码记录会从服务端移除，绑定的云机会自动释放。</span>
      </div>
      <p className="confirm-line">{summary}</p>
    </Modal>
  );
}
