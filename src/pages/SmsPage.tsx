import { useState } from "react";
import { useApp } from "../lib/appctx";
import { copyText, fmtDateTime, fmtRelative } from "../lib/format";
import { Icon } from "../components/Icon";
import { Chip, EmptyState, PageHead } from "../components/ui";

type Tab = "all" | "unused" | "used";

export function SmsPage() {
  const { snap, notify } = useApp();
  const [tab, setTab] = useState<Tab>("all");

  const unused = snap.sms.filter((s) => !s.used).length;
  const used = snap.sms.length - unused;
  const rows = snap.sms.filter((s) => {
    if (tab === "unused") return !s.used;
    if (tab === "used") return s.used;
    return true;
  });

  async function copy(code: string) {
    if (!code) return;
    const ok = await copyText(code);
    notify(ok ? `验证码 ${code} 已复制` : "复制失败，请手动选择验证码", ok ? "success" : "error");
  }

  const tabs: { key: Tab; label: string; count: number }[] = [
    { key: "all", label: "全部", count: snap.sms.length },
    { key: "unused", label: "未使用", count: unused },
    { key: "used", label: "已使用", count: used },
  ];

  return (
    <div className="page">
      <PageHead
        title="短信管理"
        subtitle="登录验证码的收信箱。登扣成功收到验证码后，这里会实时出现新短信。"
      />

      <section className="card">
        <div className="card-head">
          <div className="tabs" role="tablist" aria-label="短信筛选">
            {tabs.map((t) => (
              <button
                key={t.key}
                type="button"
                role="tab"
                aria-selected={tab === t.key}
                className={`tab${tab === t.key ? " active" : ""}`}
                onClick={() => setTab(t.key)}
              >
                {t.label}
                <span className="tab-count">{t.count}</span>
              </button>
            ))}
          </div>
          <span className="muted small">按接收时间倒序</span>
        </div>

        {rows.length === 0 ? (
          <EmptyState
            icon="inbox"
            title={tab === "all" ? "收信箱还是空的" : tab === "unused" ? "没有未使用的验证码" : "没有已使用的验证码"}
            hint="在账号控制台点击「开始登扣」，收到的验证码会自动显示在这里。"
          />
        ) : (
          <div className="sms-list">
            {rows.map((s) => (
              <article key={s.id} className={`sms-row${s.used ? " used" : ""}`}>
                <span className="list-ico sms-ico">
                  <Icon name="sms" size={17} />
                </span>
                <div className="sms-body">
                  <div className="sms-content">{s.content || `验证码 ${s.code}`}</div>
                  <div className="sms-meta">
                    <span>{s.phone || "未知号码"}</span>
                    <span className="meta-dot" />
                    <span title={fmtDateTime(s.received_at)}>{fmtRelative(s.received_at)}</span>
                    <Chip
                      label={s.used ? "已使用" : "未使用"}
                      tone={s.used ? "mute" : "good"}
                      dot
                    />
                  </div>
                </div>
                <div className="sms-code">{s.code || "—"}</div>
                <button
                  type="button"
                  className="btn sm"
                  disabled={!s.code}
                  onClick={() => copy(s.code)}
                >
                  <Icon name="copy" size={14} />
                  复制
                </button>
              </article>
            ))}
          </div>
        )}

        <div className="table-foot">
          <span>共 {snap.sms.length} 条短信 · 未使用 {unused} 条</span>
          <span className="foot-hint">
            <Icon name="clock" size={13} />
            验证码确认登录成功后会自动标记为已使用
          </span>
        </div>
      </section>
    </div>
  );
}
