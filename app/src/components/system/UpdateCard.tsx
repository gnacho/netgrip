import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { CircleCheck, Download } from "lucide-react";
import { api } from "../../api";
import type { Board, PkgUpgrade, UpdateCheck } from "../../types";
import { Banner, Button, Card, ConfirmDialog, IconTile, SkeletonRows, useToast } from "../ui";
import { WaitOverlay, useMeBack } from "./WaitOverlay";

const STEP_KEYS = ["update.stepDownload", "update.stepInstall", "update.stepReboot", "update.stepCheck"] as const;
/** Ms cosméticos por paso; la verdad la da el sondeo a /api/me. */
const STEP_TIMERS = [2500, 5000] as const;
const MIN_ELAPSED_MS = 6000; // el recorrido demo dura ~6 s en total

/** Warnings de owut reexpresados en llano (system.md §2). */
function plainWarning(w: string, t: (k: string) => string): string {
  if (/netgrip/i.test(w)) return t("update.warnReinstall");
  return w;
}

export function UpdateCard({ board, update, onChange, onPackagesChange }: {
  board: Board | undefined;
  update: UpdateCheck | undefined;
  onChange: (u: UpdateCheck) => void;
  onPackagesChange: (p: PkgUpgrade[]) => void;
}) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [checking, setChecking] = useState(false);
  const [confirm, setConfirm] = useState(false);
  /** fase del flujo de actualización con overlay a pantalla completa */
  const [phase, setPhase] = useState<"idle" | "overlay" | "background">("idle");
  const [step, setStep] = useState(0);
  const timersRef = useRef<ReturnType<typeof setTimeout>[]>([]);

  useEffect(() => () => timersRef.current.forEach(clearTimeout), []);

  const recheck = async () => {
    setChecking(true);
    try { onChange(await api.updateCheck()); } catch { /* se conserva el estado anterior */ }
    setChecking(false);
  };

  const finish = () => {
    setPhase("idle");
    push({ tone: "ok", text: t("update.doneToast") });
    // refresco para que el badge del Shell quede coherente
    api.updateCheck().then(onChange).catch(() => {});
    api.packages().then((r) => onPackagesChange(r.upgradable)).catch(() => {});
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
      iconTone={showUpgrade ? "warn" : "ok"} help="firmware">
      {update === undefined ? (
        <SkeletonRows rows={3} />
      ) : update.owut_present === false ? (
        <Banner tone="warn">{t("update.noOwut")}</Banner>
      ) : !showUpgrade ? (
        /* al día (firmware-wise) */
        <div className="flex flex-col items-start gap-2.5">
          <div className="flex items-center gap-3">
            <IconTile icon={CircleCheck} tone="ok" size={44} />
            <div>
              <p className="text-h2">{t("update.heroUpToDate")}</p>
            <p className="text-caption text-muted mt-0.5">{currentCaption}</p>
          </div>
        </div>
        <Button variant="ghost" size="sm" onClick={recheck} loading={checking}>{t("update.checkNow")}</Button>
        </div>
      ) : (
        /* hay actualización: héroe warn (design-rev2 §5) + detalles */
        <div className="flex flex-col gap-3">
          <div className="flex items-center gap-3">
            <IconTile icon={Download} tone="warn" size={44} />
            <div className="min-w-0">
              <p className="text-h2">{t("update.availableTitle", { version: update.version_to })}</p>
              <p className="text-caption text-muted mt-0.5">{currentCaption}</p>
            </div>
          </div>
          {update.warnings.length > 0 && (
            <ul className="flex flex-col gap-1">
              {update.warnings.map((w) => (
                <li key={w} className="text-small text-muted">· {plainWarning(w, t)}</li>
              ))}
            </ul>
          )}
          {!update.safe_to_proceed && (
            <Banner tone="warn">
              {update.missing_packages.length > 0
                ? t("update.missingPkgs", { list: update.missing_packages.join(", ") })
                : t("update.unsafe")}
              {update.safe_with_reinstall && <> {t("update.warnReinstall")}</>}
            </Banner>
          )}
          <div className="flex items-center gap-2">
            <Button onClick={() => setConfirm(true)} disabled={blocked || phase !== "idle"}>
              {t("update.updateNow")}
            </Button>
            <Button variant="ghost" size="sm" onClick={recheck} loading={checking}>{t("update.checkNow")}</Button>
          </div>
        </div>
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
