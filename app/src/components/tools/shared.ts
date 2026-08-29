import type { TFunction } from "i18next";
import { fmtDate, fmtTime } from "../../lib/format";

/**
 * El backend real devuelve status "applied" y el modo demo "ok"; el ciclo de
 * acción (useActionCycle) solo entiende ModuleResult, así que normalizamos.
 */
export function asApplied<T extends { status: string }>(r: T): T {
  return r.status === "ok" ? { ...r, status: "applied" } : r;
}

/** Fecha de copia en lenguaje humano: "hoy a las 09:12", "ayer, 21:40" o fecha corta. */
export function fmtRelDate(t: TFunction, ts: number): string {
  const d = new Date(ts * 1000);
  const now = new Date();
  const time = fmtTime(ts);
  const startOfDay = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const days = Math.round((startOfDay(now) - startOfDay(d)) / 86400000);
  if (days === 0) return t("tools.todayAt", { time });
  if (days === 1) return t("tools.yesterdayAt", { time });
  return fmtDate(ts);
}

export type BackupAge = { kind: "fresh" } | { kind: "mid"; days: number } | { kind: "old" };

/** Antigüedad de una copia (design-rev2 §5): <24h fresh, >7 días old. */
export function backupAge(ts: number): BackupAge {
  const days = (Date.now() - ts * 1000) / 86400000;
  if (days < 1) return { kind: "fresh" };
  if (days > 7) return { kind: "old" };
  return { kind: "mid", days: Math.floor(days) };
}

export interface DiffLine {
  kind: "added" | "removed";
  text: string;
}

/** Diff de líneas sencillo (LCS) entre dos configs UCI, para el modal "Comparar". */
export function lineDiff(before: string, after: string): DiffLine[] {
  const a = before.split("\n").filter((l) => l.trim() !== "");
  const b = after.split("\n").filter((l) => l.trim() !== "");
  const n = a.length;
  const m = b.length;
  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array<number>(m + 1).fill(0));
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }
  const out: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (a[i] === b[j]) { i++; j++; }
    else if (dp[i + 1][j] >= dp[i][j + 1]) out.push({ kind: "removed", text: a[i++] });
    else out.push({ kind: "added", text: b[j++] });
  }
  while (i < n) out.push({ kind: "removed", text: a[i++] });
  while (j < m) out.push({ kind: "added", text: b[j++] });
  return out;
}
