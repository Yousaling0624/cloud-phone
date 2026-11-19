import type { CloudPhone } from "../mygo";

/**
 * The command an operator runs on a machine that can reach the device's adb
 * server. Wireless devices need `adb connect` first; USB devices are selected
 * by serial.
 */
export function scrcpyCommand(
  phone?: Pick<CloudPhone, "external_id" | "ip">,
): string {
  if (!phone) return "";
  const address = phone.ip.includes(":") ? phone.ip : "";
  if (address) return `adb connect ${address} && scrcpy -s ${address}`;
  return phone.external_id ? `scrcpy -s ${phone.external_id}` : "";
}
