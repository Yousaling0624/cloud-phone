import { useEffect, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { avatarTone, initial } from "../lib/format";
import type { IconName } from "./Icon";
import { Icon } from "./Icon";

export function Chip({
  label,
  tone = "mute",
  dot = false,
}: {
  label: string;
  tone?: string;
  dot?: boolean;
}) {
  return (
    <span className={`chip ${tone}`}>
      {dot && <span className="dot" />}
      {label}
    </span>
  );
}

export function Avatar({
  name,
  size = "md",
}: {
  name: string;
  size?: "sm" | "md" | "lg";
}) {
  const sizeCls = size === "md" ? "" : ` ${size}`;
  return <span className={`avatar ${avatarTone(name)}${sizeCls}`}>{initial(name)}</span>;
}

export function Spinner({ size = "md", light = false }: { size?: "tiny" | "md"; light?: boolean }) {
  const cls = ["spinner", size === "tiny" ? "tiny" : "", light ? "light" : ""]
    .filter(Boolean)
    .join(" ");
  return <span className={cls} aria-hidden="true" />;
}

export function PageHead({
  title,
  subtitle,
  actions,
}: {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
}) {
  return (
    <header className="page-head">
      <div>
        <h1 className="page-title">{title}</h1>
        {subtitle && <p className="page-sub">{subtitle}</p>}
      </div>
      {actions && <div className="page-actions">{actions}</div>}
    </header>
  );
}

export function EmptyState({
  icon = "inbox",
  title,
  hint,
  action,
}: {
  icon?: IconName;
  title: string;
  hint?: string;
  action?: ReactNode;
}) {
  return (
    <div className="empty">
      <div className="empty-ico">
        <Icon name={icon} size={24} />
      </div>
      <div className="empty-title">{title}</div>
      {hint && <div className="empty-hint">{hint}</div>}
      {action && <div className="empty-action">{action}</div>}
    </div>
  );
}

export function Switch({
  checked,
  onChange,
  disabled = false,
  label,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  disabled?: boolean;
  label: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      className={`switch${checked ? " on" : ""}`}
      disabled={disabled}
      onClick={() => onChange(!checked)}
    />
  );
}

export function Modal({
  title,
  subtitle,
  width,
  onClose,
  children,
  footer,
  className = "",
}: {
  title: string;
  subtitle?: string;
  width?: number;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  className?: string;
}) {
  const panel = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    panel.current?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        onClose();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      previous?.focus?.();
    };
  }, [onClose]);

  return createPortal(
    <div
      className="modal-overlay"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={panel}
        className={`modal ${className}`}
        style={width ? { width } : undefined}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        tabIndex={-1}
      >
        <div className="modal-head">
          <div>
            <h2 className="modal-title">{title}</h2>
            {subtitle && <p className="modal-sub">{subtitle}</p>}
          </div>
          <button type="button" className="iconbtn close" onClick={onClose} aria-label="关闭">
            <Icon name="x" size={18} />
          </button>
        </div>
        <div className="modal-body">{children}</div>
        {footer && <div className="modal-footer">{footer}</div>}
      </div>
    </div>,
    document.body,
  );
}
