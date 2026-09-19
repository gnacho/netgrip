import { useTranslation } from "react-i18next";
import { Banner, Modal, SkeletonRows } from "./ui";
import type { SystemInfo } from "../types";
import { fmtBytes } from "../lib/format";

/** Expanded flash view: how much of the overlay filesystem is used. The
 *  card only carries the free-space gauge; the modal adds the used/free
 *  split and, when space runs low, says what actually fills a router:
 *  installed packages and their logs.
 */
export function FlashDetailModal({ system, open, onClose }: {
  system?: SystemInfo;
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const root = system?.root;
  const used = root ? root.total - root.free : 0;
  const usedPct = root && root.total > 0 ? (used / root.total) * 100 : 0;
  const low = root ? root.free / root.total <= 0.2 : false;

  return (
    <Modal open={open} onClose={onClose} title={t("overview.flashDetailTitle")}>
      {!root ? <SkeletonRows rows={3} /> : (
        <>
          <div className="flex items-center gap-3 text-caption">
            <span className="w-20 shrink-0 text-muted">{t("overview.flashUsed")}</span>
            <div className="h-2.5 flex-1 rounded-full bg-surface-2 overflow-hidden">
              <div
                className={`h-full rounded-full ${low ? "bg-warn" : "bg-accent"}`}
                style={{ width: `${Math.min(100, usedPct)}%` }}
              />
            </div>
            <span className="w-20 text-right" style={{ fontVariantNumeric: "tabular-nums" }}>
              {Math.round(usedPct)}%
            </span>
          </div>
          <p className="mt-3 text-caption text-muted">
            {t("overview.flashSplit", {
              used: fmtBytes(used * 1024),
              free: fmtBytes(root.free * 1024),
              total: fmtBytes(root.total * 1024),
            })}
          </p>
          {low && (
            <Banner tone="warn" className="mt-3">
              {t("overview.flashLow")}
            </Banner>
          )}
        </>
      )}
    </Modal>
  );
}
