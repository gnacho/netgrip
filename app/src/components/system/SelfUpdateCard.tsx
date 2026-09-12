import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Sparkles } from "lucide-react";
import { api } from "../../api";
import type { SelfUpdateCheck, SelfUpdateSchedule } from "../../types";
import { Button, Card, Pill, SkeletonRows, Toggle } from "../ui";
import { SelfUpdateDialog } from "./SelfUpdateDialog";

const fmtTime = (unix: number) =>
  unix > 0 ? new Date(unix * 1000).toLocaleString() : "-";

export function SelfUpdateCard({ index = 2 }: { index?: number }) {
  const { t } = useTranslation();
  const [check, setCheck] = useState<SelfUpdateCheck>();
  const [sched, setSched] = useState<SelfUpdateSchedule>();
  const [saving, setSaving] = useState(false);
  const [savedAt, setSavedAt] = useState(0);
  const [dialogOpen, setDialogOpen] = useState(false);

  useEffect(() => {
    api.selfUpdateCheck().then(setCheck).catch(() => {});
    api.selfUpdateSchedule().then(setSched).catch(() => {});
  }, []);

  const save = async (patch: Parameters<typeof api.selfUpdateScheduleSave>[0]) => {
    setSaving(true);
    try {
      await api.selfUpdateScheduleSave(patch);
      const fresh = await api.selfUpdateSchedule();
      setSched(fresh);
      setSavedAt(Date.now());
    } catch {
      /* the card keeps showing the previous config on failure */
    } finally {
      setSaving(false);
    }
  };

  const cfg = sched?.config;
  const st = sched?.state;

  return (
    <Card index={index} title={t("selfupdate.title")} icon={Sparkles}
      action={check && (check.available
        ? <Pill tone="warn">{t("selfupdate.available")}</Pill>
        : <Pill tone="ok">{t("selfupdate.upToDate")}</Pill>)}>
      {!check ? (
        <SkeletonRows rows={3} />
      ) : (
        <div className="flex flex-col gap-2.5">
          <div className="flex items-baseline justify-between gap-4">
            <span className="text-small text-muted">{t("selfupdate.current")}</span>
            <span className="font-mono text-small font-medium">{check.current}</span>
          </div>

          {check.available && (
            <>
              <div className="flex items-baseline justify-between gap-4">
                <span className="text-small text-muted">{t("selfupdate.latest")}</span>
                <span className="font-mono text-small font-semibold text-warn">{check.latest}</span>
              </div>
              <div>
                <Button size="sm" onClick={() => setDialogOpen(true)}>{t("selfupdate.update")}</Button>
              </div>
            </>
          )}

          {!cfg ? (
            <SkeletonRows rows={2} />
          ) : (
            <div className="mt-1 flex flex-col gap-2.5 border-t pt-2.5">
              <div className="flex items-center justify-between gap-4">
                <div className="flex flex-col">
                  <span className="text-small font-medium">{t("selfupdate.auto")}</span>
                  <span className="text-xs text-muted">{t("selfupdate.autoHint")}</span>
                </div>
                <Toggle
                  checked={cfg.enabled}
                  disabled={saving}
                  onChange={(v) => save({ enabled: v })}
                />
              </div>

              {cfg.enabled && (
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <label className="flex items-center gap-2">
                    <span className="text-small text-muted">{t("selfupdate.interval")}</span>
                    <select
                      className="rounded-lg border bg-elevated px-2 py-1 text-small"
                      value={cfg.intervalHours <= 24 ? 24 : 168}
                      disabled={saving}
                      onChange={(e) => save({ intervalHours: Number(e.target.value) })}
                    >
                      <option value={24}>{t("selfupdate.daily")}</option>
                      <option value={168}>{t("selfupdate.weekly")}</option>
                    </select>
                  </label>
                  <label className="flex items-center gap-2">
                    <span className="text-small text-muted">{t("selfupdate.window")}</span>
                    <select
                      className="rounded-lg border bg-elevated px-2 py-1 text-small"
                      value={cfg.windowStart}
                      disabled={saving}
                      onChange={(e) => {
                        const start = Number(e.target.value);
                        const end = start + 2 <= 24 ? start + 2 : start + 2 - 24;
                        save({ windowStart: start, windowEnd: end });
                      }}
                    >
                      {Array.from({ length: 24 }, (_, h) => (
                        <option key={h} value={h}>
                          {String(h).padStart(2, "0")}:00
                        </option>
                      ))}
                    </select>
                  </label>
                </div>
              )}

              {st && (
                <div className="flex flex-col gap-1">
                  <div className="flex items-baseline justify-between gap-4">
                    <span className="text-small text-muted">{t("selfupdate.lastCheck")}</span>
                    <span className="text-small">{fmtTime(st.lastCheck)}</span>
                  </div>
                  <div className="flex items-baseline justify-between gap-4">
                    <span className="text-small text-muted">{t("selfupdate.lastResult")}</span>
                    <span className="text-small">
                      {st.lastResult
                        ? t(`selfupdate.result_${st.lastResult.split(" ")[0]}`, {
                            version: st.lastResult.split(" ")[1] ?? "",
                          })
                        : "-"}
                    </span>
                  </div>
                </div>
              )}

              {savedAt > 0 && Date.now() - savedAt < 4000 && (
                <span className="text-xs text-ok">{t("selfupdate.saved")}</span>
              )}
            </div>
          )}
        </div>
      )}

      <SelfUpdateDialog open={dialogOpen} onClose={() => setDialogOpen(false)} initialCheck={check} />
    </Card>
  );
}
