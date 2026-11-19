import type { ToastItem } from "../lib/types";
import { Icon } from "./Icon";

const ICONS = {
  success: "check-circle",
  error: "alert",
  info: "info",
} as const;

export function Toasts({
  toasts,
  dismiss,
}: {
  toasts: ToastItem[];
  dismiss: (id: number) => void;
}) {
  return (
    <div className="toasts" role="status" aria-live="polite">
      {toasts.map((t) => (
        <div key={t.id} className={`toast ${t.tone}`}>
          <span className="toast-ico">
            <Icon name={ICONS[t.tone]} size={16} />
          </span>
          <span className="toast-msg">{t.message}</span>
          <button
            type="button"
            className="toast-x"
            onClick={() => dismiss(t.id)}
            aria-label="关闭提示"
          >
            <Icon name="x" size={13} />
          </button>
        </div>
      ))}
    </div>
  );
}
