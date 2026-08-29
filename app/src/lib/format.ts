import type { TFunction } from "i18next";
import i18n from "../i18n";

/** Locale de la app (el del selector ES/EN), NO el del navegador. */
const appLocale = (): string => i18n.language || "en";

/** Bytes/s → "5,2 MB/s" (auto-unidad, tabular-nums vía clases del caller). */
export function fmtRate(bytesPerSec: number): string {
  if (bytesPerSec > 1048576) return `${(bytesPerSec / 1048576).toFixed(1)} MB/s`;
  if (bytesPerSec > 1024) return `${(bytesPerSec / 1024).toFixed(1)} KB/s`;
  return `${Math.round(bytesPerSec)} B/s`;
}

/** Bytes → "2,1 GB" (auto-unidad). */
export function fmtBytes(bytes: number): string {
  if (bytes > 1073741824) return `${(bytes / 1073741824).toFixed(2)} GB`;
  if (bytes > 1048576) return `${(bytes / 1048576).toFixed(1)} MB`;
  if (bytes > 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${Math.round(bytes)} B`;
}

/** Uptime en segundos → "12 días 4 h" / "12d 4h" según las claves time.* */
export function fmtUptime(t: TFunction, secs: number): string {
  const d = Math.floor(secs / 86400);
  const h = Math.floor((secs % 86400) / 3600);
  const m = Math.floor((secs % 3600) / 60);
  if (d > 0) return `${t("time.days", { count: d })} ${t("time.hours", { count: h })}`;
  if (h > 0) return `${t("time.hours", { count: h })} ${t("time.minutes", { count: m })}`;
  return t("time.minutes", { count: m });
}

export function fmtMB(bytes: number): string {
  return `${Math.round(bytes / 1048576)} MB`;
}

/** ts (segundos unix) → "21:15" */
export function fmtTime(ts: number): string {
  return new Date(ts * 1000).toLocaleTimeString(appLocale(), { hour: "2-digit", minute: "2-digit" });
}

/** ts (segundos unix) → "28 ago, 21:15" */
export function fmtDate(ts: number): string {
  return new Date(ts * 1000).toLocaleDateString(appLocale(), { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}
