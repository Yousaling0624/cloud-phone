export type PageKey =
  | "dashboard"
  | "accounts"
  | "sms"
  | "phones"
  | "sims"
  | "wallet"
  | "users"
  | "settings"
  | "stock"
  | "extract"
  | "merchant";

export type Tone = "success" | "error" | "info";

export interface ToastItem {
  id: number;
  message: string;
  tone: Tone;
}
