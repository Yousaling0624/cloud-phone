import { useState } from "react";
import { API } from "../mygo";
import { useApp } from "../lib/appctx";
import { errText, fmtMoney, fmtTime } from "../lib/format";
import { Icon } from "../components/Icon";
import { EmptyState, PageHead, Spinner } from "../components/ui";

const QUICK = [10, 50, 100, 500];

export function WalletPage() {
  const { snap, refresh, notify } = useApp();
  const [amount, setAmount] = useState("");
  const [reason, setReason] = useState("手动扣费");
  const [busy, setBusy] = useState<"topup" | "charge" | null>(null);

  const balance = snap.wallet.balance;

  async function submit(kind: "topup" | "charge") {
    const value = Number(amount);
    if (!Number.isFinite(value) || value <= 0) {
      notify("请输入大于 0 的金额", "error");
      return;
    }
    setBusy(kind);
    try {
      if (kind === "topup") {
        await API.topUp(value);
        notify(`已充值 ¥${fmtMoney(value)}`);
      } else {
        await API.charge(value, reason.trim() || "手动扣费");
        notify(`已扣费 ¥${fmtMoney(value)}`);
      }
      setAmount("");
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
        title="系统钱包"
        subtitle="云机开机、账号登扣等运营开销从系统钱包扣费，账单实时记录。"
      />

      <div className="wallet-grid">
        <section className="wallet-card">
          <div className="wallet-orb a" />
          <div className="wallet-orb b" />
          <div className="wallet-top">
            <span className="wallet-label">系统钱包余额</span>
            <span className="wallet-badge">
              <Icon name="shield" size={13} />
              平台账户
            </span>
          </div>
          <div className="wallet-value">
            <small>¥</small>
            {fmtMoney(balance)}
          </div>
          <div className="wallet-foot">
            <span>云币余额（元）</span>
            <span className="wallet-foot-dot" />
            <span>
              最近一笔{" "}
              {snap.ledger[0]
                ? `${snap.ledger[0].amount >= 0 ? "充值" : "扣费"} ¥${fmtMoney(
                    Math.abs(snap.ledger[0].amount),
                  )}`
                : "暂无"}
            </span>
          </div>
        </section>

        <section className="card card-pad wallet-actions">
          <div className="card-head bare">
            <div>
              <h2 className="card-title">资金操作</h2>
              <p className="card-sub">充值即时到账，扣费会写入账单流水</p>
            </div>
          </div>

          <label className="field">
            <span className="field-label">金额（元）</span>
            <div className="money-input">
              <span>¥</span>
              <input
                className="input"
                inputMode="decimal"
                placeholder="0.00"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
              />
            </div>
          </label>

          <div className="amount-chips">
            {QUICK.map((v) => (
              <button
                key={v}
                type="button"
                className={`amount-chip${amount === String(v) ? " active" : ""}`}
                onClick={() => setAmount(String(v))}
              >
                ¥{v}
              </button>
            ))}
          </div>

          <label className="field">
            <span className="field-label">扣费说明</span>
            <input
              className="input"
              value={reason}
              placeholder="例如：云机开机扣费"
              onChange={(e) => setReason(e.target.value)}
            />
          </label>

          <div className="wallet-btns">
            <button
              type="button"
              className="btn primary"
              disabled={busy !== null}
              onClick={() => submit("topup")}
            >
              {busy === "topup" ? <Spinner size="tiny" light /> : <Icon name="plus" size={15} />}
              在线充值
            </button>
            <button
              type="button"
              className="btn danger-ghost"
              disabled={busy !== null}
              onClick={() => submit("charge")}
            >
              {busy === "charge" ? <Spinner size="tiny" /> : <Icon name="download" size={15} />}
              手动扣费
            </button>
          </div>
        </section>
      </div>

      <section className="card table-card">
        <div className="card-head">
          <div>
            <h2 className="card-title">账单流水</h2>
            <p className="card-sub">最近 50 笔资金变动</p>
          </div>
          <span className="muted small">余额会自动随流水更新</span>
        </div>
        {snap.ledger.length === 0 ? (
          <EmptyState
            icon="wallet"
            title="还没有账单记录"
            hint="充值或扣费后，每一笔变动都会记录在这里。"
          />
        ) : (
          <div className="table-wrap">
            <table className="data">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>事项</th>
                  <th className="right">金额</th>
                </tr>
              </thead>
              <tbody>
                {snap.ledger.map((l) => (
                  <tr key={l.id}>
                    <td className="mono muted">{fmtTime(l.at)}</td>
                    <td>
                      <div className="cell-account">
                        <span className={`ledger-ico${l.amount >= 0 ? " plus" : ""}`}>
                          <Icon name={l.amount >= 0 ? "upload" : "download"} size={15} />
                        </span>
                        {l.reason || "账户变动"}
                      </div>
                    </td>
                    <td className={`right mono amount-cell${l.amount >= 0 ? " plus" : ""}`}>
                      {l.amount >= 0 ? "+" : "−"}¥{fmtMoney(Math.abs(l.amount))}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
