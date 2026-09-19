import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Modal, SkeletonRows } from "./ui";
import { api } from "../api";
import type { CPUProbe, SystemInfo } from "../types";
import { fmtBytes } from "../lib/format";

/** Expanded memory view: how the RAM is split (processes, cache, buffers,
 *  free) and which processes hold the most of it. The card only carries the
 *  headline gauge; the split is where the answer to "where did the RAM go"
 *  actually lives.
 *
 *  Polls both /api/system and /api/cpu while open (3s): the RSS ranking
 *  comes from the CPU probe, which already reads every /proc/<pid>/stat.
 */
export function MemoryDetailModal({ system, open, onClose }: {
  system?: SystemInfo;
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [info, setInfo] = useState<SystemInfo | undefined>(system);
  const [cpu, setCpu] = useState<CPUProbe>();

  useEffect(() => {
    if (!open) return;
    let alive = true;
    const load = () => {
      api.system().then((s) => { if (alive) setInfo(s); }).catch(() => {});
      api.cpu().then((c) => { if (alive) setCpu(c); }).catch(() => {});
    };
    load();
    const id = setInterval(load, 3000);
    return () => { alive = false; clearInterval(id); };
  }, [open]);

  const m = info?.memory;
  const total = m?.total ?? 0;
  const memProcs = cpu?.mem_procs ?? [];
  const rows = m ? [
    { key: "memUsedProcs", bytes: m.total - m.available, cls: "bg-accent" },
    { key: "memCached", bytes: m.cached, cls: "bg-teal" },
    { key: "memBuffered", bytes: m.buffered, cls: "bg-warn" },
    { key: "memFree", bytes: m.free, cls: "bg-faint" },
  ] : [];

  return (
    <Modal open={open} onClose={onClose} wide title={t("overview.memDetailTitle")}>
      {!m ? <SkeletonRows rows={4} /> : (
        <>
          <div className="space-y-2">
            {rows.map((r) => {
              const pct = total > 0 ? (r.bytes / total) * 100 : 0;
              return (
                <div key={r.key} className="flex items-center gap-3 text-caption">
                  <span className="w-36 shrink-0 text-muted">{t(`overview.${r.key}`)}</span>
                  <div className="h-2.5 flex-1 rounded-full bg-surface-2 overflow-hidden">
                    <div className={`h-full rounded-full ${r.cls}`} style={{ width: `${Math.min(100, pct)}%` }} />
                  </div>
                  <span className="w-20 text-right" style={{ fontVariantNumeric: "tabular-nums" }}>
                    {fmtBytes(r.bytes)}
                  </span>
                  <span className="w-12 text-right text-faint" style={{ fontVariantNumeric: "tabular-nums" }}>
                    {Math.round(pct)}%
                  </span>
                </div>
              );
            })}
          </div>
          <p className="mt-2 text-caption text-faint">
            {t("overview.memTotal", { total: fmtBytes(total) })}
          </p>

          {memProcs.length > 0 && (
            <div className="mt-4 border-t border-border/50 pt-3">
              <div className="text-caption text-muted mb-2">{t("overview.memTop")}</div>
              <div className="space-y-1">
                {memProcs.map((pr) => (
                  <div key={pr.pid} className="flex items-center gap-3 text-caption">
                    <span className="flex-1 truncate font-medium" translate="no" title={pr.name}>{pr.name}</span>
                    <span className="w-14 text-right text-faint" style={{ fontVariantNumeric: "tabular-nums" }}>
                      #{pr.pid}
                    </span>
                    <span className="w-20 text-right" style={{ fontVariantNumeric: "tabular-nums" }}>
                      {pr.rss_bytes ? fmtBytes(pr.rss_bytes) : "—"}
                    </span>
                    <span className="w-14 text-right text-faint" style={{ fontVariantNumeric: "tabular-nums" }}>
                      {total > 0 && pr.rss_bytes ? `${((pr.rss_bytes / total) * 100).toFixed(1)}%` : "—"}
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
