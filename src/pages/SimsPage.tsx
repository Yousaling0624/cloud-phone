import { useApp } from "../lib/appctx";
import { accountTitle, simStatus } from "../lib/format";
import { Icon } from "../components/Icon";
import { Chip, EmptyState, PageHead } from "../components/ui";

export function SimsPage() {
  const { snap } = useApp();
  const occupied = snap.sims.filter((s) => s.status === "占用").length;
  const free = snap.sims.filter((s) => s.status === "空闲").length;

  return (
    <div className="page">
      <PageHead
        title="卡位管理"
        subtitle={`共 ${snap.sims.length} 个卡位，${occupied} 个占用，${free} 个空闲。`}
      />

      <section className="card table-card">
        <div className="table-wrap">
          <table className="data">
            <thead>
              <tr>
                <th>卡位</th>
                <th>号码</th>
                <th>运营商</th>
                <th>绑定账号</th>
                <th>状态</th>
              </tr>
            </thead>
            <tbody>
              {snap.sims.length === 0 ? (
                <tr>
                  <td colSpan={5}>
                    <EmptyState icon="slot" title="还没有卡位" hint="接入 SIM 卡池后，卡位会显示在这里。" />
                  </td>
                </tr>
              ) : (
                snap.sims.map((s, i) => {
                  const acc = snap.accounts.find((a) => a.id === s.bound_account);
                  return (
                    <tr key={s.id}>
                      <td>
                        <div className="cell-account">
                          <span className="device-ico">
                            <Icon name="slot" size={16} />
                          </span>
                          <div className="account-text">
                            <div className="account-name">卡位 {i + 1}</div>
                            <div className="account-num">SIM #{s.id}</div>
                          </div>
                        </div>
                      </td>
                      <td className="mono">{s.phone}</td>
                      <td>
                        <span className="soft-tag">{s.carrier}</span>
                      </td>
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
                        <Chip {...simStatus(s.status)} dot />
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
        <div className="table-foot">
          <span>共 {snap.sims.length} 个卡位</span>
          <span className="foot-hint">
            <Icon name="info" size={13} />
            卡位与账号的绑定关系来自短信驱动
          </span>
        </div>
      </section>
    </div>
  );
}
