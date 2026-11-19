import { useState } from "react";
import { API, type CloudPhone } from "../mygo";
import { useApp } from "../lib/appctx";
import { accountTitle, copyText, errText, phoneState } from "../lib/format";
import { scrcpyCommand } from "../lib/phones";
import { Icon } from "../components/Icon";
import { Chip, EmptyState, PageHead, Spinner } from "../components/ui";

export function PhonesPage() {
  const { snap, refresh, notify } = useApp();
  const [busy, setBusy] = useState<number | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [q, setQ] = useState("");

  const rows = snap.phones.filter((p) => {
    const needle = q.trim().toLowerCase();
    if (!needle) return true;
    return [p.name, p.external_id, p.region, p.image, p.ip].some((v) =>
      v.toLowerCase().includes(needle),
    );
  });

  async function act(p: CloudPhone, action: "start" | "stop" | "restart") {
    setBusy(p.id);
    try {
      await API.setPhoneState(p.id, action);
      const verb = action === "start" ? "唤醒" : action === "stop" ? "熄屏" : "重启";
      notify(`云机 ${p.name} 已执行${verb}`, "success");
      await refresh();
    } catch (e) {
      notify(errText(e), "error");
    } finally {
      setBusy(null);
    }
  }

  async function sync() {
    setSyncing(true);
    try {
      const res = await API.syncPhones();
      notify(
        res.total === 0
          ? "网关没有发现设备，请检查 ADB 连接与设备配置"
          : `设备同步完成：新增 ${res.added} 台，更新 ${res.updated} 台`,
        res.total === 0 ? "info" : "success",
      );
      await refresh();
    } catch (e) {
      notify(errText(e), "error");
    } finally {
      setSyncing(false);
    }
  }

  async function copyCommand(p: CloudPhone) {
    const command = scrcpyCommand(p);
    if (!command) {
      notify("该云机没有设备号，无法生成 scrcpy 命令", "error");
      return;
    }
    const ok = await copyText(command);
    notify(ok ? `已复制：${command}` : "复制失败，请手动复制", ok ? "success" : "error");
  }

  const running = snap.phones.filter((p) => p.state === "running").length;
  const idle = snap.phones.filter((p) => p.state === "idle").length;
  const canSync = !!snap.settings.provider_base_url && !syncing;

  return (
    <div className="page">
      <PageHead
        title="云手机管理"
        subtitle={`共 ${snap.phones.length} 台云机，${running} 台运行中，${idle} 台空闲待分配。`}
        actions={
          <>
            <button
              type="button"
              className="btn"
              disabled={!canSync}
              title={
                snap.settings.provider_base_url
                  ? "从设备网关读取 ADB 设备并新增/更新云机"
                  : "请先在系统设置中填写云机网关地址"
              }
              onClick={sync}
            >
              {syncing ? <Spinner size="tiny" /> : <Icon name="refresh" size={14} />}
              同步设备
            </button>
            <div className="search inline">
              <Icon name="search" size={15} />
              <input
                value={q}
                placeholder="搜索云机、设备号、区域、IP"
                onChange={(e) => setQ(e.target.value)}
              />
            </div>
          </>
        }
      />

      <section className="card table-card">
        <div className="table-wrap">
          <table className="data">
            <thead>
              <tr>
                <th>云机</th>
                <th>区域</th>
                <th>镜像</th>
                <th>ADB 地址</th>
                <th>绑定账号</th>
                <th>状态</th>
                <th className="right">操作</th>
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 ? (
                <tr>
                  <td colSpan={7}>
                    <EmptyState
                      icon="phone"
                      title="没有匹配的云机"
                      hint={
                        q.trim()
                          ? "换个关键词试试，或清空搜索。"
                          : snap.settings.provider_base_url
                            ? "连接 Android 设备并开启 USB 调试后，点「同步设备」。"
                            : "请先在系统设置中填写设备网关地址，再回来同步设备。"
                      }
                    />
                  </td>
                </tr>
              ) : (
                rows.map((p) => {
                  const acc = snap.accounts.find((a) => a.id === p.account_id);
                  return (
                    <tr key={p.id}>
                      <td>
                        <div className="cell-account">
                          <span className="device-ico">
                            <Icon name="phone" size={16} />
                          </span>
                          <div className="account-text">
                            <div className="account-name">{p.name}</div>
                            <div className="account-num mono">
                              {p.external_id ? `设备号 ${p.external_id}` : `云机 #${p.id}`}
                            </div>
                          </div>
                        </div>
                      </td>
                      <td>{p.region}</td>
                      <td>
                        <span className="soft-tag">{p.image}</span>
                      </td>
                      <td className="mono">{p.ip || "—"}</td>
                      <td>
                        {acc ? (
                          <div className="phone-text">
                            <div className="phone-name">{accountTitle(acc)}</div>
                            <div className="phone-region">{acc.number}</div>
                          </div>
                        ) : (
                          <span className="muted">未绑定</span>
                        )}
                      </td>
                      <td>
                        <Chip {...phoneState(p.state)} dot />
                      </td>
                      <td>
                        <div className="row-actions">
                          <button
                            type="button"
                            className="btn sm ghost"
                            title="复制 scrcpy 控制命令"
                            onClick={() => copyCommand(p)}
                          >
                            <Icon name="copy" size={13} />
                          </button>
                          {p.state === "running" ? (
                            <>
                              <button
                                type="button"
                                className="btn sm"
                                disabled={busy === p.id}
                                title="执行 adb reboot，设备将重启"
                                onClick={() => act(p, "restart")}
                              >
                                {busy === p.id ? (
                                  <Spinner size="tiny" />
                                ) : (
                                  <Icon name="restart" size={14} />
                                )}
                                重启
                              </button>
                              <button
                                type="button"
                                className="btn sm danger-ghost"
                                disabled={busy === p.id}
                                title="物理设备：只熄屏，不是断电"
                                onClick={() => act(p, "stop")}
                              >
                                <Icon name="stop" size={13} />
                                关机
                              </button>
                            </>
                          ) : (
                            <button
                              type="button"
                              className="btn sm primary"
                              disabled={busy === p.id}
                              title="物理设备：唤醒屏幕；断电设备无法远程开机"
                              onClick={() => act(p, "start")}
                            >
                              {busy === p.id ? (
                                <Spinner size="tiny" light />
                              ) : (
                                <Icon name="power" size={14} />
                              )}
                              开机
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
        <div className="table-foot">
          <span>共 {snap.phones.length} 台云机</span>
          <span className="foot-hint">
            <Icon name="info" size={13} />
            物理设备：开机=唤醒、关机=熄屏；断电设备无法远程开机
          </span>
        </div>
      </section>
    </div>
  );
}
