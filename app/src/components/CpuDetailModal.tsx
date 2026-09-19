import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Modal, SkeletonRows } from "./ui";
import { api } from "../api";
import type { CPUProbe } from "../types";
import { fmtBytes } from "../lib/format";

/** Expanded CPU view (#346): everything the card compresses, in one place.
 *  Per-core load with the loss counters, load average, temperature, and
 *  the process list with CPU (per interval) and resident memory.
 *
 *  Reuses /api/cpu — the probe already computes per-process deltas, there
 *  is nothing extra to sample. Polls faster than the card (2s) while open
 *  because this is where someone is actively looking.
 */
export function CpuDetailModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const [cpu, setCpu] = useState<CPUProbe>();

  useEffect(() => {
    if (!open) return;
    let alive = true;
    const load = () => api.cpu().then((c) => { if (alive) setCpu(c); }).catch(() => {});
    load();
    const id = setInterval(load, 2000);
    return () => { alive = false; clearInterval(id); };
  }, [open]);

  const ready = cpu && !cpu.warming && cpu.cores.length > 0;

  return (
    <Modal open={open} onClose={onClose} wide title={t("overview.cpuDetailTitle")}>
      {!ready ? <SkeletonRows rows={5} /> : (
        <>
          {/* Per core, same story as the card but with room for the
              counters that explain a saturated core. */}
          <div className="space-y-2">
            {cpu!.cores.map((c) => (
              <div key={c.idx} className="flex items-center gap-3 text-caption">
                <span className="w-14 shrink-0 text-muted">{t("overview.cpuCore", { n: c.idx })}</span>
                <div className="h-2.5 flex-1 rounded-full bg-surface-2 overflow-hidden">
                  <div
                    className={`h-full rounded-full transition-[width] duration-500 ${
                      c.usage_pct >= 85 ? "bg-danger" : c.usage_pct >= 60 ? "bg-warn" : "bg-accent"
                    }`}
                    style={{ width: `${Math.min(100, c.usage_pct)}%` }}
                  />
                </div>
                <span className="w-12 text-right text-muted" style={{ fontVariantNumeric: "tabular-nums" }}>
                  {Math.round(c.usage_pct)}%
                </span>
                <span className="w-16 text-right text-faint" style={{ fontVariantNumeric: "tabular-nums" }}>
                  {c.freq_mhz ? `${c.freq_mhz} MHz` : "—"}
                </span>
                <span className="w-24 text-right" style={{ fontVariantNumeric: "tabular-nums" }}>
                  {c.dropped_rate > 0 || c.squeezed_rate > 0 ? (
                    <span className="text-warn" title={t("overview.cpuDroppedTotal", { n: c.dropped.toLocaleString() })}>
                      ↓{fmtCount(c.dropped_rate)}/s · s{fmtCount(c.squeezed_rate)}/s
                    </span>
                  ) : (
                    <span className="text-faint">·</span>
                  )}
                </span>
              </div>
            ))}
          </div>

          <div className="mt-4 flex flex-wrap gap-x-4 gap-y-1 text-caption text-muted">
            {cpu!.load.length > 0 && (
              <span>{t("overview.load")}: {cpu!.load.map((l) => l.toFixed(2)).join(" · ")}</span>
            )}
            {cpu!.temp_c != null && (
              <span title={cpu!.temp_source}>
                {Math.round(cpu!.temp_c)} °C{cpu!.temp_source ? ` (${cpu!.temp_source})` : ""}
              </span>
            )}
            {cpu!.governor && <span>{cpu!.governor}</span>}
          </div>

          {cpu!.procs.length > 0 && (
            <div className="mt-4 border-t border-border/50 pt-3">
              <div className="text-caption text-muted mb-2">{t("overview.cpuProcesses")}</div>
              <div className="space-y-1">
                {cpu!.procs.map((pr) => (
                  <div key={pr.pid} className="flex items-center gap-3 text-caption">
                    <span className="flex-1 truncate font-medium" translate="no" title={pr.name}>{pr.name}</span>
                    <span className="w-14 text-right text-faint" style={{ fontVariantNumeric: "tabular-nums" }}>
                      #{pr.pid}
                    </span>
                    <span className="w-14 text-right" style={{ fontVariantNumeric: "tabular-nums" }}>
                      {pr.usage_pct}%
                    </span>
                    <span className="w-20 text-right text-muted" style={{ fontVariantNumeric: "tabular-nums" }}>
                      {pr.rss_bytes ? fmtBytes(pr.rss_bytes) : "—"}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </>
      )}
    </Modal>
  );
}

function fmtCount(v: number): string {
  if (v >= 1000) return `${(v / 1000).toFixed(1)}k`;
  return `${Math.round(v)}`;
}
