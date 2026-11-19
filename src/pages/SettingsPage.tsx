import { useState } from "react";
import { API } from "../mygo";
import { useApp } from "../lib/appctx";
import { errText } from "../lib/format";
import { LOGIN_METHODS, normalizeLoginMethod, type LoginMethodKey } from "../lib/loginMethods";
import { Icon } from "../components/Icon";
import { ConnectionPanel } from "../components/ConnectionPanel";
import { PageHead, Spinner, Switch } from "../components/ui";

export function SettingsPage() {
  const { snap, refresh, notify } = useApp();
  const [baseURL, setBaseURL] = useState(snap.settings.provider_base_url);
  const [token, setToken] = useState(snap.settings.provider_token);
  const [method, setMethod] = useState<LoginMethodKey>(normalizeLoginMethod(snap.settings.login_method));
  const [autoFill, setAutoFill] = useState(snap.settings.auto_fill);
  const [autoBind, setAutoBind] = useState(snap.settings.auto_bind);
  const [saveLog, setSaveLog] = useState(snap.settings.save_log);
  const [saving, setSaving] = useState(false);

  async function save() {
    setSaving(true);
    try {
      await API.saveSettings({
        provider_base_url: baseURL.trim(),
        provider_token: token.trim(),
        login_method: method,
        auto_fill: autoFill,
        auto_bind: autoBind,
        save_log: saveLog,
      });
      notify("系统设置已保存");
      await refresh();
    } catch (e) {
      notify(errText(e), "error");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="page">
      <PageHead
        title="系统设置"
        subtitle="云端网关由服务端共享；桌面端连接只影响本机"
        actions={
          <button type="button" className="btn primary" disabled={saving} onClick={save}>
            {saving ? <Spinner size="tiny" light /> : <Icon name="check" size={15} />}
            保存设置
          </button>
        }
      />

      <section className="card settings-card">
        <div className="card-head">
          <div>
            <div className="card-title">云机网关</div>
            <div className="card-sub">
              云机开关机、实时画面与登扣都通过这个 HTTP 网关；物理设备由 ADB 网关管理
            </div>
          </div>
          <span className={`mode-tag ${baseURL.trim() ? "remote" : "embedded"}`}>
            {baseURL.trim() ? "外部网关" : "内置演示网关"}
          </span>
        </div>
        <div className="settings-card-body">
          <div className="form-grid">
            <label className="field full">
              <span className="field-label">网关地址</span>
              <input
                className="input mono"
                value={baseURL}
                placeholder="留空使用内置演示网关，例如 http://127.0.0.1:8791"
                spellCheck={false}
                onChange={(e) => setBaseURL(e.target.value)}
              />
              <span className="hint">
                留空使用演示网关；物理 Android 设备填 device-gateway 地址（默认 8791），保存后立即生效。
              </span>
            </label>
            <label className="field full">
              <span className="field-label">网关令牌</span>
              <input
                className="input mono"
                type="password"
                value={token}
                autoComplete="off"
                placeholder="Bearer token"
                onChange={(e) => setToken(e.target.value)}
              />
              <span className="hint">
                device-gateway 启动时传入的令牌，不是平台服务端的访问令牌；保存在服务端数据库中。
              </span>
            </label>
          </div>
        </div>
      </section>

      <section className="card settings-card">
        <div className="card-head">
          <div>
            <div className="card-title">登录默认值</div>
            <div className="card-sub">控制台打开时采用的初始选项，可在每次登扣时临时修改</div>
          </div>
        </div>
        <div className="settings-card-body">
          <div className="form-grid">
            <label className="field">
              <span className="field-label">登录方式</span>
              <select
                className="select"
                value={method}
                onChange={(e) => setMethod(normalizeLoginMethod(e.target.value))}
              >
                {LOGIN_METHODS.map((m) => (
                  <option key={m.key} value={m.key}>
                    {m.label}
                  </option>
                ))}
              </select>
            </label>
            <div className="settings-switches">
              <div className="setting-row">
                <div>
                  <div className="setting-label">自动填写账号</div>
                  <div className="setting-hint">自动网关使用；物理 ADB 网关忽略此项，登录由人工完成</div>
                </div>
                <Switch checked={autoFill} label="自动填写账号" onChange={setAutoFill} />
              </div>
              <div className="setting-row">
                <div>
                  <div className="setting-label">登录后自动绑定云机</div>
                  <div className="setting-hint">成功后把账号绑定到当前云机</div>
                </div>
                <Switch checked={autoBind} label="登录后自动绑定云机" onChange={setAutoBind} />
              </div>
              <div className="setting-row">
                <div>
                  <div className="setting-label">保存登录日志</div>
                  <div className="setting-hint">在服务端保留本次登录的执行摘要，便于排查</div>
                </div>
                <Switch checked={saveLog} label="保存登录日志" onChange={setSaveLog} />
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="card settings-card">
        <div className="card-head">
          <div>
            <div className="card-title">桌面端连接</div>
            <div className="card-sub">本机通过 HTTP 连接服务端，多个终端共享同一份数据</div>
          </div>
        </div>
        <div className="settings-card-body">
          <ConnectionPanel />
        </div>
      </section>
    </div>
  );
}
