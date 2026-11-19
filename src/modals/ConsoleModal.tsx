import { useState } from "react";
import { API } from "../mygo";
import { useApp } from "../lib/appctx";
import {
  accountStatus,
  accountTitle,
  copyText,
  errText,
  loginState,
} from "../lib/format";
import {
  LOGIN_METHODS,
  normalizeLoginMethod,
  type LoginMethodKey,
} from "../lib/loginMethods";
import { scrcpyCommand } from "../lib/phones";
import type { IconName } from "../components/Icon";
import { Icon } from "../components/Icon";
import { DevicePreview } from "../components/DevicePreview";
import { Avatar, Chip, Modal, Spinner, Switch } from "../components/ui";

type ConsoleTab = "login" | "scan" | "query" | "extract";

const TABS: { key: ConsoleTab; label: string }[] = [
  { key: "login", label: "登扣" },
  { key: "scan", label: "快速扫码" },
  { key: "query", label: "快速查询" },
  { key: "extract", label: "提号" },
];

const TAB_EMPTY: Record<Exclude<ConsoleTab, "login">, { icon: IconName; title: string; hint: string }> = {
  scan: {
    icon: "phone",
    title: "扫码登录通道正在接入",
    hint: "接入云机实时画面后，这里会显示二维码并支持一键确认。",
  },
  query: {
    icon: "search",
    title: "快速查询即将开放",
    hint: "之后可以在这里查询账号资料、绑定关系与风控状态。",
  },
  extract: {
    icon: "extract",
    title: "提号流程即将开放",
    hint: "提号会校验登录状态并自动分配云机，当前仍使用登扣页签。",
  },
};

export function ConsoleModal({
  accountId,
  onClose,
}: {
  accountId: number;
  onClose: () => void;
}) {
  const { snap, refresh, notify, go } = useApp();
  const [tab, setTab] = useState<ConsoleTab>("login");
  const [method, setMethod] = useState<LoginMethodKey>(
    normalizeLoginMethod(snap.settings.login_method),
  );
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [autoFill, setAutoFill] = useState(snap.settings.auto_fill);
  const [autoBind, setAutoBind] = useState(snap.settings.auto_bind);
  const [saveLog, setSaveLog] = useState(snap.settings.save_log);
  const [starting, setStarting] = useState(false);
  const [confirming, setConfirming] = useState<"ok" | "fail" | null>(null);

  const account = snap.accounts.find((a) => a.id === accountId);
  if (!account) return null;

  const login = account.login;
  const waiting = login === "waiting";
  const ready = login === "ready";
  const done = login === "done";
  const canStart = !starting && !waiting && !ready;
  const canConfirmOk = !confirming && ready;
  const canConfirmFail = !confirming && (waiting || ready);
  const startLabel = starting
    ? "正在启动…"
    : waiting
      ? "等待登录流程"
      : ready
        ? "等待确认结果"
        : done || login === "fail"
          ? "重新登扣"
          : "开始登扣";

  async function start() {
    if (!canStart) return;
    setStarting(true);
    try {
      await API.startLogin(account!.id, {
        method,
        auto_fill: autoFill,
        auto_bind: autoBind,
        save_log: saveLog,
      });
      notify("已开始登录流程，请按控制台提示完成");
      await refresh();
    } catch (e) {
      notify(errText(e), "error");
    } finally {
      setStarting(false);
    }
  }

  async function confirm(ok: boolean) {
    if (ok ? !canConfirmOk : !canConfirmFail) return;
    setConfirming(ok ? "ok" : "fail");
    try {
      await API.confirmLogin(account!.id, ok);
      notify(ok ? `${accountTitle(account!)} 已登录上线` : "已标记本次登录失败", ok ? "success" : "info");
      await refresh();
      onClose();
    } catch (e) {
      notify(errText(e), "error");
    } finally {
      setConfirming(null);
    }
  }

  async function copyCode() {
    if (!account!.verify_code) return;
    const ok = await copyText(account!.verify_code);
    notify(ok ? `验证码 ${account!.verify_code} 已复制` : "复制失败，请手动选择验证码", ok ? "success" : "error");
  }

  const boundPhone =
    snap.phones.find((p) => p.id === account.bound_phone_id) ??
    snap.phones.find((p) => p.name === account.bound_phone);
  const command = scrcpyCommand(boundPhone);

  async function copyCommand() {
    if (!command) return;
    const ok = await copyText(command);
    notify(ok ? `已复制：${command}` : "复制失败，请手动选择命令", ok ? "success" : "error");
  }

  return (
    <Modal
      title="账号控制台"
      subtitle="执行登录流程；物理设备请使用外部 scrcpy / Escrcpy 人工操作"
      width={1180}
      onClose={onClose}
      className="console-modal"
      footer={
        <>
          <button
            type="button"
            className="btn"
            disabled={!account.verify_code}
            onClick={copyCode}
          >
            <Icon name="copy" size={14} />
            复制验证码
          </button>
          <button
            type="button"
            className="btn ghost"
            onClick={() => {
              onClose();
              go("sms");
            }}
          >
            <Icon name="inbox" size={15} />
            打开收信箱
          </button>
          <span className="footer-spacer" />
          <button type="button" className="btn ghost" onClick={onClose}>
            关闭
          </button>
        </>
      }
    >
      <div className="console-body">
        <aside className="console-side">
          <div className="console-account">
            <Avatar name={accountTitle(account)} size="lg" />
            <div className="console-account-text">
              <div className="console-account-name">{accountTitle(account)}</div>
              <div className="console-account-num">{account.number}</div>
            </div>
          </div>
          <div className="console-chips">
            <Chip {...accountStatus(account.status)} dot />
            <Chip {...loginState(login)} />
          </div>

          <div className="side-divider" />

          <div className="kv">
            <KvRow label="登录规则" value={account.login_rule || "默认登录规则"} mono />
            <KvRow
              label="绑定云机"
              value={
                account.bound_phone
                  ? `${account.bound_phone}${boundPhone ? ` · ${boundPhone.region}` : ""}`
                  : "未绑定"
              }
            />
            <KvRow label="设备号" value={boundPhone?.external_id || "—"} mono />
            <KvRow label="ADB 地址" value={boundPhone?.ip || "—"} mono />
            <KvRow label="验证手机" value={account.verify_phone || "未设置"} mono />
            <KvRow label="最近验证码" value={account.verify_code || "—"} mono />
          </div>

          <div className="side-divider" />

          <div className="side-title">账号安全</div>
          <div className="perks">
            <Perk
              icon="shield"
              label="QQ 关联"
              value={account.linked_qq ? account.linked_qid || "已开启" : "未开启"}
              ok={account.linked_qq}
            />
            <Perk
              icon="key"
              label="登录密码"
              value={account.has_password ? "已设置" : "未设置"}
              ok={account.has_password}
            />
            <Perk
              icon="check-circle"
              label="登录状态"
              value={loginState(login).label}
              ok={login === "done"}
            />
          </div>

          <div className="console-side-foot">
            <span className="muted small">账号编号 #{account.id}</span>
          </div>
        </aside>

        <div className="console-main">
          <div className="ctabs" role="tablist" aria-label="控制台功能">
            {TABS.map((t) => (
              <button
                key={t.key}
                type="button"
                role="tab"
                aria-selected={tab === t.key}
                className={`ctab${tab === t.key ? " active" : ""}`}
                onClick={() => setTab(t.key)}
              >
                {t.label}
              </button>
            ))}
          </div>

          {tab !== "login" ? (
            <div className="console-tab-empty">
              <span className="empty-ico">
                <Icon name={TAB_EMPTY[tab].icon} size={24} />
              </span>
              <div className="empty-title">{TAB_EMPTY[tab].title}</div>
              <div className="empty-hint">{TAB_EMPTY[tab].hint}</div>
            </div>
          ) : (
            <>
              <div className="console-section-head">
                <div>
                  <h3 className="console-h3">账号登录</h3>
                  <p className="console-sub">自动执行登录规则，短信到达后确认结果</p>
                </div>
                <label className="inline-field">
                  <span>登录方式</span>
                  <select
                    className="select"
                    value={method}
                    disabled={waiting || ready}
                    onChange={(e) => setMethod(normalizeLoginMethod(e.target.value))}
                  >
                    {LOGIN_METHODS.map((m) => (
                      <option key={m.key} value={m.key}>
                        {m.label}
                      </option>
                    ))}
                  </select>
                </label>
              </div>

              <div className="settings">
                <button
                  type="button"
                  className="settings-head"
                  aria-expanded={settingsOpen}
                  onClick={() => setSettingsOpen((v) => !v)}
                >
                  <Icon name={settingsOpen ? "chevron-down" : "chevron-right"} size={14} />
                  高级设置
                  <span className="settings-hint">本次登扣生效，默认值可在系统设置修改</span>
                </button>
                {settingsOpen && (
                  <div className="settings-body">
                    <SettingRow
                      label="自动填写账号"
                      hint="在云机上自动输入账号与密码"
                      checked={autoFill}
                      onChange={setAutoFill}
                    />
                    <SettingRow
                      label="登录后自动绑定云机"
                      hint="成功后把账号绑定到当前云机"
                      checked={autoBind}
                      onChange={setAutoBind}
                    />
                    <SettingRow
                      label="保存登录日志"
                      hint="保留本次登录的执行步骤，便于排查"
                      checked={saveLog}
                      onChange={setSaveLog}
                    />
                  </div>
                )}
              </div>

              {done ? (
                <div className="code-card ready">
                  <span className="code-big-ico">
                    <Icon name="check-circle" size={26} />
                  </span>
                  <div className="code-label">登录完成</div>
                  <div className="code-done-title">账号已成功上线</div>
                  <div className="code-hint">
                    {account.verify_code
                      ? `验证码 ${account.verify_code} 已使用，可在收信箱查看`
                      : "本次登录结果已保存"}
                  </div>
                </div>
              ) : ready && account.verify_code ? (
                <div className="code-card ready">
                  <div className="code-label">短信验证码</div>
                  <div className="code-value">{account.verify_code}</div>
                  <div className="code-hint">
                    来自 {account.verify_phone || "绑定手机"} · 确认无误后点击「登录成功」
                  </div>
                  <div className="code-actions">
                    <button type="button" className="btn sm" onClick={copyCode}>
                      <Icon name="copy" size={14} />
                      复制验证码
                    </button>
                    <button
                      type="button"
                      className="btn sm ghost"
                      onClick={() => {
                        onClose();
                        go("sms");
                      }}
                    >
                      查看收信箱
                    </button>
                  </div>
                </div>
              ) : ready ? (
                <div className="code-card ready">
                  <span className="code-big-ico">
                    <Icon name="phone" size={24} />
                  </span>
                  <div className="code-label">人工登录已就绪</div>
                  <div className="code-done-title">
                    {command
                      ? "请在 scrcpy / Escrcpy 中完成 QQ 登录"
                      : boundPhone
                        ? "云机没有设备号，无法生成控制命令"
                        : "账号还没有绑定云机"}
                  </div>
                  {command ? (
                    <div className="cmd-line">
                      <code className="mono">{command}</code>
                      <button type="button" className="btn sm" onClick={copyCommand}>
                        <Icon name="copy" size={13} />
                        复制命令
                      </button>
                    </div>
                  ) : null}
                  <div className="code-hint">
                    {command
                      ? "命令需在能访问该设备 ADB 的机器上执行；登录完成后点击「登录成功」，失败请点「登录失败」。"
                      : "请先在账号编辑中绑定云机或到云机页同步设备，然后重新开始登扣。"}
                  </div>
                </div>
              ) : waiting ? (
                <div className="code-card waiting">
                  <Spinner />
                  <div className="code-label">正在等待登录流程</div>
                  <div className="waiting-title">等待网关返回结果</div>
                  <div className="code-hint">
                    自动网关会在这里显示验证码；物理设备网关会直接进入人工登录。
                  </div>
                  <span className="waiting-dots" aria-hidden="true">
                    <i />
                    <i />
                    <i />
                  </span>
                </div>
              ) : login === "fail" ? (
                <div className="code-card failed">
                  <span className="code-big-ico bad">
                    <Icon name="alert" size={24} />
                  </span>
                  <div className="code-label">本次登录未完成</div>
                  <div className="notice bad console-notice">
                    请检查账号绑定、网关连接与设备 ADB 状态后重新登扣。
                  </div>
                </div>
              ) : (
                <div className="code-card idle">
                  <span className="code-big-ico">
                    <Icon name="key" size={24} />
                  </span>
                  <div className="code-label">尚未开始登扣</div>
                  <div className="code-hint">
                    点击「开始登扣」，系统会执行登录流程；物理设备会在外部 scrcpy 中人工完成 QQ 登录。
                  </div>
                </div>
              )}

              <div className="console-actions">
                <button
                  type="button"
                  className="btn primary"
                  disabled={!canStart}
                  onClick={start}
                >
                  {starting ? (
                    <Spinner size="tiny" light />
                  ) : (
                    <Icon name="play" size={14} />
                  )}
                  {startLabel}
                </button>
                <span className="footer-spacer" />
                <button
                  type="button"
                  className="btn success"
                  disabled={!canConfirmOk}
                  onClick={() => confirm(true)}
                >
                  {confirming === "ok" ? <Spinner size="tiny" light /> : <Icon name="check" size={15} />}
                  登录成功
                </button>
                <button
                  type="button"
                  className="btn danger-ghost"
                  disabled={!canConfirmFail}
                  onClick={() => confirm(false)}
                >
                  {confirming === "fail" ? <Spinner size="tiny" /> : <Icon name="x" size={15} />}
                  登录失败
                </button>
              </div>
            </>
          )}
        </div>

        <DevicePreview accountId={account.id} phone={boundPhone} enabled />
      </div>
    </Modal>
  );
}

function KvRow({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="kv-row">
      <span className="kv-key">{label}</span>
      <span className={`kv-val${mono ? " mono" : ""}`}>{value}</span>
    </div>
  );
}

function Perk({
  icon,
  label,
  value,
  ok,
}: {
  icon: IconName;
  label: string;
  value: string;
  ok: boolean;
}) {
  return (
    <div className="perk">
      <span className={`perk-ico${ok ? " ok" : ""}`}>
        <Icon name={icon} size={14} />
      </span>
      <span className="perk-label">{label}</span>
      <span className={`perk-value${ok ? " ok" : ""}`}>{value}</span>
    </div>
  );
}

function SettingRow({
  label,
  hint,
  checked,
  onChange,
}: {
  label: string;
  hint: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <div className="setting-row">
      <div>
        <div className="setting-label">{label}</div>
        <div className="setting-hint">{hint}</div>
      </div>
      <Switch checked={checked} label={label} onChange={onChange} />
    </div>
  );
}
