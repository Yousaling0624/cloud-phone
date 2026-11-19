import { useApp } from "../lib/appctx";
import type { IconName } from "../components/Icon";
import { Icon } from "../components/Icon";
import { PageHead } from "../components/ui";
import type { PageKey } from "../lib/types";

export function Placeholder({ page }: { page: PageKey }) {
  const { go } = useApp();
  const config = PLACEHOLDERS[page] ?? FALLBACK;
  return (
    <div className="page">
      <PageHead title={config.title} subtitle={config.desc} />
      <section className="placeholder">
        <span className="placeholder-ico">
          <Icon name={config.icon} size={28} />
        </span>
        <span className="chip teal placeholder-chip">功能建设中</span>
        <h2 className="placeholder-title">{config.headline}</h2>
        <p className="placeholder-desc">{config.detail}</p>
        <button type="button" className="btn" onClick={() => go("dashboard")}>
          <Icon name="dashboard" size={15} />
          返回工作台
        </button>
      </section>
    </div>
  );
}

interface PlaceholderConfig {
  title: string;
  desc: string;
  icon: IconName;
  headline: string;
  detail: string;
}

const FALLBACK: PlaceholderConfig = {
  title: "账号库存管理",
  desc: "查看未分配账号的库存水位、来源批次与占用情况。",
  icon: "stock",
  headline: "库存视图正在搭建",
  detail:
    "上线后可以按批次查看账号余量、被占用比例与补货提醒，避免账号池断档。当前可先在账号管理中查看全部账号。",
};

const PLACEHOLDERS: Partial<Record<PageKey, PlaceholderConfig>> = {
  stock: FALLBACK,
  extract: {
    title: "提号管理",
    desc: "按批次提取账号，自动分配云机并跟踪交付状态。",
    icon: "extract",
    headline: "提号流程即将开放",
    detail:
      "上线后支持按标签筛选可提账号、一键分配云机、生成交付清单，并记录每一次提取操作。",
  },
  merchant: {
    title: "商家管理",
    desc: "维护上游商家、结算方式与供货质量评分。",
    icon: "merchant",
    headline: "商家档案还在路上",
    detail:
      "上线后可以登记商家联系人、结算周期与历史供货质量，让账号来源与成本一目了然。",
  },
};
