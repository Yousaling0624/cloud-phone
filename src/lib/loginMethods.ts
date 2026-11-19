/** The login method keys are stored by the server; the labels live here. */
export type LoginMethodKey = "password" | "sms" | "scan";

export const LOGIN_METHODS: { key: LoginMethodKey; label: string }[] = [
  { key: "password", label: "账号密码登录" },
  { key: "sms", label: "短信验证登录" },
  { key: "scan", label: "扫码登录" },
];

export function normalizeLoginMethod(key: string): LoginMethodKey {
  return key === "sms" || key === "scan" ? key : "password";
}

export function loginMethodLabel(key: string): string {
  const found = LOGIN_METHODS.find((m) => m.key === key);
  return found?.label ?? LOGIN_METHODS[0]!.label;
}
