import type { Account, AccountStatus, LoginState, PhoneState } from "../mygo";

/** The text of an error crossing the Go bridge, without the IPC wrapper. */
export function errText(e: unknown): string {
  if (e instanceof Error && e.message) return e.message;
  if (typeof e === "string" && e) return e;
  return "操作失败，请稍后重试";
}

export function fmtMoney(n: number): string {
  return new Intl.NumberFormat("zh-CN", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(n);
}

function parseISO(iso: string): Date | null {
  if (!iso) return null;
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? null : d;
}

/** A short local time: today shows HH:mm, older shows MM-DD HH:mm. */
export function fmtTime(iso: string): string {
  const d = parseISO(iso);
  if (!d) return "—";
  const now = new Date();
  const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`;
  if (
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate()
  ) {
    return hm;
  }
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${hm}`;
}

export function fmtDateTime(iso: string): string {
  const d = parseISO(iso);
  if (!d) return "—";
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function fmtRelative(iso: string): string {
  const d = parseISO(iso);
  if (!d) return "—";
  const diff = Date.now() - d.getTime();
  if (diff < 60_000) return "刚刚";
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`;
  return fmtTime(iso);
}

function pad(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}

export interface StatusMeta {
  label: string;
  tone: string;
}

export function accountStatus(s: AccountStatus): StatusMeta {
  switch (s) {
    case "online":
      return { label: "在线", tone: "good" };
    case "logging_in":
      return { label: "登扣中", tone: "info" };
    case "abnormal":
      return { label: "异常", tone: "bad" };
    case "offline":
      return { label: "离线", tone: "mute" };
    default:
      return { label: s || "未知", tone: "mute" };
  }
}

export function loginState(s: LoginState): StatusMeta {
  switch (s) {
    case "waiting":
      return { label: "等待验证码", tone: "warn" };
    case "ready":
      return { label: "待确认", tone: "teal" };
    case "done":
      return { label: "登录成功", tone: "good" };
    case "fail":
      return { label: "登录失败", tone: "bad" };
    default:
      return { label: "未登录", tone: "mute" };
  }
}

export function phoneState(s: PhoneState): StatusMeta {
  switch (s) {
    case "running":
      return { label: "运行中", tone: "good" };
    case "starting":
      return { label: "启动中", tone: "info" };
    case "stopped":
      return { label: "已关机", tone: "mute" };
    case "idle":
      return { label: "空闲", tone: "teal" };
    case "error":
      return { label: "异常", tone: "bad" };
    default:
      return { label: s || "未知", tone: "mute" };
  }
}

export function simStatus(s: string): StatusMeta {
  switch (s) {
    case "占用":
      return { label: s, tone: "info" };
    case "空闲":
      return { label: s, tone: "good" };
    case "停机":
      return { label: s, tone: "bad" };
    default:
      return { label: s || "未知", tone: "mute" };
  }
}

export function accountTitle(a: Account): string {
  return a.display || a.number || `账号 ${a.id}`;
}

const AVATAR_TONES = ["a0", "a1", "a2", "a3", "a4", "a5"] as const;

export function avatarTone(name: string): string {
  let h = 0;
  for (const ch of name) h = (h * 31 + ch.codePointAt(0)!) % 997;
  return AVATAR_TONES[h % AVATAR_TONES.length] ?? "a0";
}

export function initial(name: string): string {
  const trimmed = name.trim();
  if (!trimmed) return "?";
  return Array.from(trimmed)[0] ?? "?";
}

/** Copies text, falling back to a hidden selection when the clipboard API
 * is unavailable (a web view without a secure origin, for example). */
export async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // fall through to the selection approach
  }
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand("copy");
    ta.remove();
    return ok;
  } catch {
    return false;
  }
}
