import { useApp } from "../lib/appctx";
import { fmtMoney, fmtRelative, fmtTime } from "../lib/format";
import { Icon, type IconName } from "../components/Icon";
import { Avatar, PageHead } from "../components/ui";

export function Dashboard() {
  const { snap, go, openAccountForm, openImport } = useApp();
  const online = snap.accounts.filter((a) => a.status === "online").length;
  const abnormal = snap.accounts.filter((a) => a.status === "abnormal").length;
  const running = snap.phones.filter((p) => p.state === "running").length;
  const idle = snap.phones.filter((p) => p.state === "idle").length;
  const unusedSms = snap.sms.filter((s) => !s.used).length;
  const today = new Date();
  const todaySms = snap.sms.filter((s) => {
    const d = new Date(s.received_at);
    return (
      d.getFullYear() === today.getFullYear() &&
      d.getMonth() === today.getMonth() &&
      d.getDate() === today.getDate()
    );
  }).length;
  const admin = snap.users.find((u) => u.username === "admin") ?? snap.users[0];
  const operator = admin?.nickname ?? "运营员";
  const hour = new Date().getHours();
  const greeting = hour < 11 ? "早上好" : hour < 18 ? "下午好" : "晚上好";
  const onlinePct = snap.accounts.length ? Math.round((online / snap.accounts.length) * 100) : 0;

  return (
    <div className="page">
      <PageHead
        title={`${greeting}，${operator}`}
        subtitle="这是当前账号池与云手机的运营概览，数据随登录与任务实时更新。"
      />

      <section className="kpis" aria-label="关键指标">
        <Kpi
          icon="accounts"
          label="账号总数"
          value={String(snap.accounts.length)}
          foot={`在线 ${online} · 异常 ${abnormal}`}
        />
        <Kpi
          icon="check-circle"
          tone="green"
          label="在线账号"
          value={String(online)}
          foot={`占全部账号 ${onlinePct}%`}
        />
        <Kpi
          icon="phone"
          tone="blue"
          label="云机运行"
          value={String(running)}
          foot={`共 ${snap.phones.length} 台 · 空闲 ${idle} 台`}
        />
        <Kpi
          icon="sms"
          tone="amber"
          label="待用验证码"
          value={String(unusedSms)}
          foot={`今天收到 ${todaySms} 条短信`}
        />
      </section>

      <section className="card">
        <div className="card-head">
          <div>
            <h2 className="card-title">快捷操作</h2>
            <p className="card-sub">常用入口，一步直达</p>
          </div>
        </div>
        <div className="quick">
          <QuickAction
            icon="plus"
            label="存号"
            hint="录入新的平台账号"
            onClick={() => openAccountForm()}
          />
          <QuickAction
            icon="upload"
            label="批量导入"
            hint="从 CSV 文本批量建档"
            onClick={openImport}
          />
          <QuickAction
            icon="wallet"
            label="系统充值"
            hint="为云机运营账户充值"
            onClick={() => go("wallet")}
          />
          <QuickAction
            icon="inbox"
            label="收信箱"
            hint="查看最近收到的验证码"
            onClick={() => go("sms")}
          />
        </div>
      </section>

      <div className="dash-grid">
        <section className="card">
          <div className="card-head">
            <div>
              <h2 className="card-title">最近短信</h2>
              <p className="card-sub">登录验证码会自动收进这里</p>
            </div>
            <button type="button" className="link-btn" onClick={() => go("sms")}>
              查看全部
              <Icon name="chevron-right" size={13} />
            </button>
          </div>
          {snap.sms.length === 0 ? (
            <div className="list-empty">还没有收到短信</div>
          ) : (
            <div className="list">
              {snap.sms.slice(0, 5).map((s) => (
                <button
                  type="button"
                  key={s.id}
                  className="list-row as-button"
                  onClick={() => go("sms")}
                >
                  <span className="list-ico">
                    <Icon name="sms" size={16} />
                  </span>
                  <span className="list-main">
                    <span className="list-title">{s.content || `验证码 ${s.code}`}</span>
                    <span className="list-sub">
                      {s.phone || "未知号码"} · {fmtRelative(s.received_at)}
                    </span>
                  </span>
                  <span className={`sms-mini-code${s.used ? " used" : ""}`}>{s.code}</span>
                </button>
              ))}
            </div>
          )}
        </section>

        <section className="card">
          <div className="card-head">
            <div>
              <h2 className="card-title">最近账单</h2>
              <p className="card-sub">充值、扣费与云机开销</p>
            </div>
            <button type="button" className="link-btn" onClick={() => go("wallet")}>
              查看全部
              <Icon name="chevron-right" size={13} />
            </button>
          </div>
          {snap.ledger.length === 0 ? (
            <div className="list-empty">还没有账单记录</div>
          ) : (
            <div className="list">
              {snap.ledger.slice(0, 5).map((l) => (
                <div key={l.id} className="list-row">
                  <span className={`list-ico${l.amount >= 0 ? " plus" : ""}`}>
                    <Icon name={l.amount >= 0 ? "upload" : "download"} size={15} />
                  </span>
                  <span className="list-main">
                    <span className="list-title">{l.reason || "账户变动"}</span>
                    <span className="list-sub">{fmtTime(l.at)}</span>
                  </span>
                  <span className={`list-amount${l.amount >= 0 ? " plus" : ""}`}>
                    {l.amount >= 0 ? "+" : "−"}¥{fmtMoney(Math.abs(l.amount))}
                  </span>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>

      <section className="operator-strip card">
        <div className="operator-strip-main">
          <Avatar name={operator} />
          <div>
            <div className="operator-strip-name">{operator}</div>
            <div className="operator-strip-hint">
              当前以 {admin?.role ?? "管理员"} 身份运营 · 数据由服务端统一保存
            </div>
          </div>
        </div>
        <span className="operator-strip-dot" />
        <span className="muted">运营台在线</span>
      </section>
    </div>
  );
}

function Kpi({
  icon,
  label,
  value,
  foot,
  tone = "",
}: {
  icon: IconName;
  label: string;
  value: string;
  foot: string;
  tone?: string;
}) {
  return (
    <article className={`card kpi${tone ? ` ${tone}` : ""}`}>
      <div className="kpi-top">
        <span className="kpi-label">{label}</span>
        <span className="kpi-ico">
          <Icon name={icon} size={17} />
        </span>
      </div>
      <div className="kpi-value">{value}</div>
      <div className="kpi-foot">{foot}</div>
    </article>
  );
}

function QuickAction({
  icon,
  label,
  hint,
  onClick,
}: {
  icon: IconName;
  label: string;
  hint: string;
  onClick: () => void;
}) {
  return (
    <button type="button" className="quick-btn" onClick={onClick}>
      <span className="quick-ico">
        <Icon name={icon} size={17} />
      </span>
      <span className="quick-text">
        <span className="quick-label">{label}</span>
        <span className="quick-hint">{hint}</span>
      </span>
      <Icon name="chevron-right" size={14} className="quick-arrow" />
    </button>
  );
}
