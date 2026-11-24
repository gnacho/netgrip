import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Sparkles } from "lucide-react";
import { api } from "../../api";
import type { SelfUpdateCheck, SelfUpdateSchedule } from "../../types";
import { Button, Card, HelpTip, Pill, SkeletonRows, Toggle } from "../ui";
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
  const [checking, setChecking] = useState(false);

  useEffect(() => {
    api.selfUpdateCheck().then(setCheck).catch(() => {});
    api.selfUpdateSchedule().then(setSched).catch(() => {});
  }, []);

  const recheck = async () => {
    setChecking(true);
    try { setCheck(await api.selfUpdateCheck()); } catch { /* se conserva el estado anterior */ }
    setChecking(false);
  };

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
      action={check && (check.check_error
        ? <Pill tone="danger">{t("selfupdate.checkFailed")}</Pill>
        : check.available
          ? <Pill tone="warn">{t("selfupdate.available")}</Pill>
          : <Pill tone="ok">{t("selfupdate.upToDate")}</Pill>)}>
      {!check ? (
        <SkeletonRows rows={1} />
      ) : (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          {/* Versión, comprobación y actualización */}
          <span className="inline-flex items-center gap-2">
            <span className="text-small text-muted">{t("selfupdate.current")}</span>
            <span className="font-mono text-small font-medium">{check.current}</span>
            {check.available && (
              <>
                <span className="text-faint" aria-hidden="true">→</span>
                <span className="font-mono text-small font-semibold text-warn">{check.latest}</span>
              </>
            )}
          </span>

          <span className="inline-flex items-center gap-2">
            {check.available && (
              <Button size="sm" onClick={() => setDialogOpen(true)}>{t("selfupdate.update")}</Button>
            )}
            <Button size="sm" variant="secondary" loading={checking} onClick={recheck}>
              {t("selfupdate.checkNow")}
            </Button>
          </span>

          {check.check_error && (
            <span className="text-small text-danger">{t("selfupdate.checkFailedHint")}</span>
          )}

          {/* Actualización automática, en la misma línea (#639) */}
          {cfg && (
            <span className="ml-auto inline-flex flex-wrap items-center gap-x-3 gap-y-2">
              <span className="inline-flex items-center gap-1.5">
                <span className="text-small">{t("selfupdate.auto")}</span>
                <HelpTip
                  title={t("selfupdate.auto")}
                  body={`${t("selfupdate.autoHint")}${st ? ` · ${t("selfupdate.lastCheck")}: ${fmtTime(st.lastCheck)} · ${t("selfupdate.lastResult")}: ${st.lastResult ? t(`selfupdate.result_${st.lastResult.split(" ")[0]}`, { version: st.lastResult.split(" ")[1] ?? "" }) : "-"}` : ""}`}
                />
              </span>
              {cfg.enabled && (
                <>
                  <select
                    className="rounded-lg border bg-elevated px-2 py-1 text-small"
                    aria-label={t("selfupdate.interval")}
                    value={cfg.intervalHours <= 24 ? 24 : 168}
                    disabled={saving}
                    onChange={(e) => save({ intervalHours: Number(e.target.value) })}
                  >
                    <option value={24}>{t("selfupdate.daily")}</option>
                    <option value={168}>{t("selfupdate.weekly")}</option>
                  </select>
                  <select
                    className="rounded-lg border bg-elevated px-2 py-1 text-small"
                    aria-label={t("selfupdate.window")}
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
                </>
              )}
              <Toggle
                checked={cfg.enabled}
                disabled={saving}
                onChange={(v) => save({ enabled: v })}
              />
              {savedAt > 0 && Date.now() - savedAt < 4000 && (
                <span className="text-xs text-ok">{t("selfupdate.saved")}</span>
              )}
            </span>
          )}
        </div>
      )}

      <SelfUpdateDialog open={dialogOpen} onClose={() => setDialogOpen(false)} initialCheck={check} />
    </Card>
  );
}
