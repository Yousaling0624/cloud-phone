import { useEffect, useState } from "react";
import { API, type ConnectionInfo } from "../mygo";
import { errText } from "../lib/format";
import { Icon } from "./Icon";
import { Spinner } from "./ui";

type Busy = "test" | "save" | null;
type Message = { tone: "ok" | "bad" | "info"; text: string } | null;

/**
 * ConnectionPanel edits client.json: which server this desktop talks to and
 * with which token. It never needs a backend, so the same panel works on the
 * startup-error screen. Changes apply after a restart.
 */
export function ConnectionPanel({ onSaved }: { onSaved?: () => void }) {
  const [info, setInfo] = useState<ConnectionInfo | null>(null);
  const [url, setUrl] = useState("");
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState<Busy>(null);
  const [message, setMessage] = useState<Message>(null);

  useEffect(() => {
    let alive = true;
    API.connection()
      .then((next) => {
        if (!alive) return;
        setInfo(next);
        setUrl(next.mode === "remote" ? next.server_url : "");
      })
      .catch((e) => {
        if (alive) setMessage({ tone: "bad", text: errText(e) });
      });
    return () => {
      alive = false;
    };
  }, []);

  async function test() {
    setBusy("test");
    setMessage(null);
    try {
      if (!url.trim()) {
        setMessage({ tone: "info", text: "内嵌模式无需测试：保存后重启即使用本机服务。" });
        return;
      }
      const result = await API.testConnection(url, token);
      setMessage({ tone: "ok", text: result });
    } catch (e) {
      setMessage({ tone: "bad", text: errText(e) });
    } finally {
      setBusy(null);
    }
  }

  async function save() {
    setBusy("save");
    setMessage(null);
    try {
      await API.saveConnection(url, token);
      setMessage({ tone: "ok", text: "连接设置已保存，重启应用后生效。" });
      onSaved?.();
    } catch (e) {
      setMessage({ tone: "bad", text: errText(e) });
    } finally {
      setBusy(null);
    }
  }

  const mode = url.trim() ? "remote" : "embedded";

  return (
    <div className="connection-panel">
      <div className="connection-status">
        <span className={`mode-tag ${info?.mode ?? mode}`}>
          <Icon name={info?.mode === "remote" ? "system" : "phone"} size={13} />
          {info?.mode === "remote" ? "远程服务" : "本机内嵌服务"}
        </span>
        {info?.server_url && <span className="mono muted connection-url">{info.server_url}</span>}
      </div>
      {info?.startup_error && (
        <div className="notice bad">
          <Icon name="alert" size={15} />
          <span>上次启动连接失败：{info.startup_error}</span>
        </div>
      )}

      <div className="form-grid">
        <label className="field full">
          <span className="field-label">服务端地址</span>
          <input
            className="input mono"
            value={url}
            placeholder="留空使用本机内嵌服务，例如 http://127.0.0.1:8787"
            spellCheck={false}
            onChange={(e) => setUrl(e.target.value)}
          />
          <span className="hint">
            留空表示在本机运行内嵌服务；填写其他终端可访问的地址即可共享同一份数据。
          </span>
        </label>
        <label className="field full">
          <span className="field-label">访问令牌</span>
          <input
            className="input mono"
            type="password"
            value={token}
            autoComplete="off"
            placeholder="留空保持当前令牌；首次连接请填写服务端令牌"
            onChange={(e) => setToken(e.target.value)}
          />
          <span className="hint">令牌保存在本机 client.json（仅当前用户可读），不会同步到其他终端；留空表示保持已保存的令牌。</span>
        </label>
      </div>

      {info && (
        <div className="connection-meta">
          <span>配置文件</span>
          <span className="mono">{info.config_path || "—"}</span>
        </div>
      )}

      {message && (
        <div className={`notice ${message.tone === "ok" ? "ok" : message.tone === "bad" ? "bad" : "info"}`}>
          <span>{message.text}</span>
        </div>
      )}

      <div className="connection-actions">
        <button type="button" className="btn" disabled={busy !== null} onClick={test}>
          {busy === "test" ? <Spinner size="tiny" /> : <Icon name="refresh" size={14} />}
          测试连接
        </button>
        <button type="button" className="btn primary" disabled={busy !== null} onClick={save}>
          {busy === "save" ? <Spinner size="tiny" light /> : <Icon name="check" size={14} />}
          保存连接
        </button>
      </div>
    </div>
  );
}
