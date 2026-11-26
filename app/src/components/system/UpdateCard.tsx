import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Download } from "lucide-react";
import { api } from "../../api";
import type { Board, SysUpdateSchedule, UpdateCheck } from "../../types";
import { Banner, Button, Card, ConfirmDialog, HelpTip, Pill, SkeletonRows, Toggle, useToast } from "../ui";
import { WaitOverlay, useMeBack } from "./WaitOverlay";
import { InstallProgress, useInstallJob } from "../wizard/common";

const STEP_KEYS = ["update.stepDownload", "update.stepInstall", "update.stepReboot", "update.stepCheck"] as const;
/** Ms cosméticos por paso; la verdad la da el sondeo a /api/me. */
const STEP_TIMERS = [2500, 5000] as const;
const MIN_ELAPSED_MS = 6000; // el recorrido demo dura ~6 s en total

/** Warnings de owut reexpresados en llano (system.md §2). */
function plainWarning(w: string, t: (k: string) => string): string {
  if (/netgrip/i.test(w)) return t("update.warnReinstall");
  return w;
}

export function UpdateCard({ board, update, onChange }: {
  board: Board | undefined;
  update: UpdateCheck | undefined;
  onChange: (u: UpdateCheck) => void;
}) {
  const { t } = useTranslation();
  const { push } = useToast();
  const { begin, running: installing, job } = useInstallJob();
  const [checking, setChecking] = useState(false);
  const [confirm, setConfirm] = useState(false);
  /** fase del flujo de actualización con overlay a pantalla completa */
  const [phase, setPhase] = useState<"idle" | "overlay" | "background">("idle");
  const [step, setStep] = useState(0);
  const [sched, setSched] = useState<SysUpdateSchedule>();
  const [schedSaving, setSchedSaving] = useState(false);
  const timersRef = useRef<ReturnType<typeof setTimeout>[]>([]);

  useEffect(() => () => timersRef.current.forEach(clearTimeout), []);

  useEffect(() => { api.sysUpdateSchedule().then(setSched).catch(() => {}); }, []);

  const saveSched = async (patch: { enabled?: boolean; intervalHours?: number; windowStart?: number; windowEnd?: number }) => {
    setSchedSaving(true);
    try {
      setSched(await api.sysUpdateScheduleSave(patch));
    } catch { /* se conserva lo anterior */ } finally {
      setSchedSaving(false);
    }
  };

  const recheck = async () => {
    setChecking(true);
    try { onChange(await api.updateCheck()); } catch { /* se conserva el estado anterior */ }
    setChecking(false);
  };

  const installOwut = async () => {
    try {
      const j = await begin(() => api.wizardPackages(["owut"]));
      if (j.phase === "done") {
        push({ tone: "ok", text: t("update.owutInstalled") });
        recheck();
      } else {
        push({ tone: "danger", text: j.error || t("update.failed") });
      }
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : t("update.failed") });
    }
  };

  const finish = () => {
    setPhase("idle");
    push({ tone: "ok", text: t("update.doneToast") });
    // refresco para que el badge del Shell quede coherente
    api.updateCheck().then(onChange).catch(() => {});
  };

  // sondeo /api/me cada 5 s (sigue aunque el overlay pase a segundo plano)
  useMeBack(phase !== "idle", MIN_ELAPSED_MS, finish);

  const start = async () => {
    setConfirm(false);
    setPhase("overlay");
    setStep(0);
    try {
      await api.startUpdate();
      setStep(1);
      timersRef.current = STEP_TIMERS.map((ms, i) => setTimeout(() => setStep(i + 2), ms));
    } catch (e) {
      setPhase("idle");
      push({ tone: "danger", text: e instanceof Error ? e.message : t("update.failed") });
    }
  };

  const currentCaption = board?.release
    ? `${board.release.distribution} ${board.release.version} (${board.release.revision})`
    : update ? `OpenWrt ${update.version_from}` : "…";

  const blocked = update ? !update.safe_to_proceed && !update.safe_with_reinstall : false;
  // Firmware upgrade only (#155): the packages branch (same_version) is
  // intentionally hidden - package management belongs to LuCI / CLI.
  const showUpgrade = update !== undefined && update.available && !update.same_version;

  return (
    <Card index={0} title={t("update.title")} icon={Download}
      iconTone={showUpgrade ? "warn" : "ok"} help="firmware"
      status={update === undefined ? undefined
        : update.owut_present === false ? <Pill tone="warn">{t("update.pillNoOwut")}</Pill>
        : showUpgrade ? <Pill tone="warn">{t("update.pillNew")}</Pill>
        : <Pill tone="ok">{t("update.upToDate")}</Pill>}>
      {update === undefined ? (
        <SkeletonRows rows={1} />
      ) : (
        <>
          {/* #722: una sola fila, como la tarjeta de NetGrip. */}
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
            <span className="inline-flex items-center gap-2">
              <span className="text-small text-muted">{t("update.current")}</span>
              <span className="font-mono text-small font-medium">{currentCaption}</span>
              {showUpgrade && (
                <>
                  <span className="text-faint" aria-hidden="true">→</span>
                  <span className="font-mono text-small font-semibold text-warn">{update.version_to}</span>
                </>
              )}
            </span>

            <span className="inline-flex items-center gap-2">
              {update.owut_present === false ? (
                <Button variant="secondary" size="sm" loading={installing} onClick={installOwut}>
                  <Download size={14} aria-hidden="true" /> {t("update.installOwut")}
                </Button>
              ) : (
                showUpgrade && (
                  <Button onClick={() => setConfirm(true)} disabled={blocked || phase !== "idle"}>
                    {t("update.updateNow")}
                  </Button>
                )
              )}
              {update.owut_present !== false && (
                <Button variant="secondary" size="sm" onClick={recheck} loading={checking}>
                  {t("update.checkNow")}
                </Button>
              )}
            </span>

            {/* Programación del firmware (#640): solo tiene sentido con owut. */}
            {update.owut_present !== false && sched && (
              <span className="ml-auto inline-flex flex-wrap items-center gap-x-3 gap-y-2">
                <span className="inline-flex items-center gap-1.5">
                  <span className="text-small">{t("update.scheduleTitle")}</span>
                  <HelpTip title={t("update.scheduleTitle")} body={t("update.scheduleHint")} />
                </span>
                {sched.config.enabled && (
                  <>
                    <select
                      className="rounded-lg border bg-elevated px-2 py-1 text-small"
                      aria-label={t("selfupdate.interval")}
                      value={sched.config.intervalHours <= 24 ? 24 : 168}
                      disabled={schedSaving}
                      onChange={(e) => saveSched({ intervalHours: Number(e.target.value) })}
                    >
                      <option value={24}>{t("selfupdate.daily")}</option>
                      <option value={168}>{t("selfupdate.weekly")}</option>
                    </select>
                    <select
                      className="rounded-lg border bg-elevated px-2 py-1 text-small"
                      aria-label={t("selfupdate.window")}
                      value={sched.config.windowStart}
                      disabled={schedSaving}
                      onChange={(e) => {
                        const start = Number(e.target.value);
                        const end = start + 2 <= 24 ? start + 2 : start + 2 - 24;
                        saveSched({ windowStart: start, windowEnd: end });
                      }}
                    >
                      {Array.from({ length: 24 }, (_, h) => (
                        <option key={h} value={h}>{String(h).padStart(2, "0")}:00</option>
                      ))}
                    </select>
                  </>
                )}
                <Toggle
                  checked={sched.config.enabled}
                  busy={schedSaving}
                  onChange={(v) => saveSched({ enabled: v })}
                  label={t("update.scheduleTitle")}
                />
              </span>
            )}
          </div>

          {update.owut_present === false && (
            <Banner tone="warn" className="mt-3">{t("update.noOwut")}</Banner>
          )}
          {installing && <p className="mt-3 text-small text-muted">{t("update.installingOwut")}</p>}
          <InstallProgress job={installing ? job : null} />

          {showUpgrade && update.warnings.length > 0 && (
            <ul className="mt-3 flex flex-col gap-1">
              {update.warnings.map((w) => (
                <li key={w} className="text-small text-muted">· {plainWarning(w, t)}</li>
              ))}
            </ul>
          )}
          {showUpgrade && !update.safe_to_proceed && (
            <Banner tone="warn" className="mt-3">
              {update.missing_packages.length > 0
                ? t("update.missingPkgs", { list: update.missing_packages.join(", ") })
                : t("update.unsafe")}
              {update.safe_with_reinstall && <> {t("update.warnReinstall")}</>}
            </Banner>
          )}
        </>
      )}

      {phase === "background" && (
        <Banner tone="info" className="mt-3">{t("update.runningBanner")}</Banner>
      )}

      <ConfirmDialog
        open={confirm}
        onClose={() => setConfirm(false)}
        onConfirm={start}
        title={t("update.confirmTitle")}
        consequence={t("update.confirmConsequence")}
        confirmLabel={t("update.updateNow")}
      />

      <WaitOverlay
        open={phase === "overlay"}
        title={t("update.overlayTitle")}
        steps={STEP_KEYS.map((k) => t(k))}
        step={step}
        onBackground={() => setPhase("background")}
      />
    </Card>
  );
}
