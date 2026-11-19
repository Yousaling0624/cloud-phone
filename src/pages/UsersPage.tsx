import { useState } from "react";
import { API, type User } from "../mygo";
import { useApp } from "../lib/appctx";
import { errText } from "../lib/format";
import { Icon } from "../components/Icon";
import { Avatar, Chip, PageHead, Switch } from "../components/ui";

function roleTone(role: string): string {
  switch (role) {
    case "管理员":
      return "info";
    case "运营":
      return "teal";
    default:
      return "mute";
  }
}

export function UsersPage() {
  const { snap, refresh, notify } = useApp();
  const [busy, setBusy] = useState<number | null>(null);

  async function toggle(u: User, next: boolean) {
    setBusy(u.id);
    try {
      await API.setUserEnabled(u.id, next);
      notify(next ? `已启用 ${u.nickname}` : `已停用 ${u.nickname}`, next ? "success" : "info");
      await refresh();
    } catch (e) {
      notify(errText(e), "error");
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="page">
      <PageHead
        title="用户信息"
        subtitle="管理可登录本控制台的操作员。停用后该账号无法登录，但历史数据保留。"
      />

      <section className="user-grid">
        {snap.users.map((u) => (
          <article key={u.id} className="card user-card">
            <div className="user-top">
              <Avatar name={u.nickname || u.username} size="lg" />
              <div className="user-main">
                <div className="user-name">{u.nickname || u.username}</div>
                <div className="user-username">@{u.username}</div>
              </div>
              <Chip label={u.role} tone={roleTone(u.role)} />
            </div>
            <div className="user-divider" />
            <div className="user-foot">
              <div>
                <div className="user-state">{u.enabled ? "允许登录控制台" : "已停用登录"}</div>
                <div className="user-state-hint">
                  {u.enabled ? "该操作员可以正常访问运营功能" : "开启后恢复访问权限"}
                </div>
              </div>
              <Switch
                checked={u.enabled}
                disabled={busy === u.id}
                label={`${u.enabled ? "停用" : "启用"} ${u.nickname}`}
                onChange={(next) => toggle(u, next)}
              />
            </div>
          </article>
        ))}
      </section>

      <section className="card user-note">
        <span className="note-ico">
          <Icon name="shield" size={16} />
        </span>
        <div>
          <div className="note-title">权限说明</div>
          <p className="note-text">
            管理员拥有全部运营权限；运营可管理账号、云机与短信；只读账号仅可查看数据。开关状态会立即保存到服务端数据库。
          </p>
        </div>
      </section>
    </div>
  );
}
