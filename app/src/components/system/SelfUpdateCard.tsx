import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Sparkles } from "lucide-react";
import { api } from "../../api";
import type { SelfUpdateCheck, SelfUpdateStatus } from "../../types";
import { Button, Card, ConfirmDialog, Pill, SkeletonRows, useToast } from "../ui";

/** Tras el reinicio del panel la página se recarga: el toast de éxito
 *  se muestra al volver gracias a esta marca en sessionStorage. */
const DONE_FLAG = "netgrip:selfupdated";

export function SelfUpdateCard({ index = 2 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [check, setCheck] = useState<SelfUpdateCheck>();
  const [status, setStatus] = useState<SelfUpdateStatus>({ phase: "idle", progress: 0 });
  const [confirming, setConfirming] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);
  const updatingRef = useRef(false);

  useEffect(() => {
    api.selfUpdateCheck().then(setCheck).catch(() => {});
    try {
      const v = sessionStorage.getItem(DONE_FLAG);
      if (v) {
        sessionStorage.removeItem(DONE_FLAG);
        push({ tone: "ok", text: t("selfupdate.doneToast", { version: v }) });
      }
    } catch { /* sin persistencia */ }
  }, [push, t]);

  const active = status.phase === "downloading" || status.phase === "installing" || status.phase === "restarting";

  useEffect(() => {
    if (!active) return;
    pollRef.current = setInterval(() => {
      api.selfUpdateStatus().then((s) => {
        setStatus(s);
        if (s.phase === "restarting") {
          // el panel se reinicia: recargamos cuando vuelva
          if (pollRef.current) clearInterval(pollRef.current);
          try { sessionStorage.setItem(DONE_FLAG, check?.latest || ""); } catch { /* sin persistencia */ }
          setTimeout(() => window.location.reload(), 5000);
        } else if (s.phase === "idle" || s.phase === "error") {
          if (pollRef.current) clearInterval(pollRef.current);
          if (s.phase === "idle" && updatingRef.current) {
            updatingRef.current = false;
            push({ tone: "ok", text: t("selfupdate.doneToast", { version: check?.latest ?? "" }) });
            api.selfUpdateCheck().then(setCheck).catch(() => {});
          }
        }
      }).catch(() => {});
    }, 500);
    return () => { if (pollRef.current) clearInterval(pollRef.current); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, status.phase]);

  const apply = async () => {
    setConfirming(false);
    updatingRef.current = true;
    setStatus({ phase: "downloading", progress: 0 });
    try {
      await api.selfUpdateApply();
    } catch (e) {
      updatingRef.current = false;
      setStatus({ phase: "error", progress: 0, message: e instanceof Error ? e.message : String(e) });
    }
  };

  const phaseText = status.phase === "downloading"
    ? t("selfupdate.downloading", { pct: status.progress })
    : status.phase === "installing"
      ? t("selfupdate.installing")
      : t("selfupdate.restarting");

  return (
    <Card index={index} title={t("selfupdate.title")} icon={Sparkles}
      action={check && (active
        ? <Pill tone="accent">{status.progress}%</Pill>
        : check.available
          ? <Pill tone="warn">{t("selfupdate.available")}</Pill>
          : <Pill tone="ok">{t("selfupdate.upToDate")}</Pill>)}
    >
      {!check ? (
        <SkeletonRows rows={3} />
      ) : (
        <div className="flex flex-col gap-2.5">
          <div className="flex items-baseline justify-between gap-4">
            <span className="text-small text-muted">{t("selfupdate.current")}</span>
            <span className="font-mono text-small font-medium">{check.current}</span>
          </div>

          {check.available && !active && status.phase !== "error" && (
            <>
              <p className="text-body font-medium">{t("selfupdate.newVersion", { version: check.latest })}</p>
              {check.notes && <p className="text-small text-muted whitespace-pre-wrap">{check.notes}</p>}
              <div>
                <Button size="sm" onClick={() => setConfirming(true)}>{t("selfupdate.update")}</Button>
              </div>
            </>
          )}

          {active && (
            <div>
              <div className="flex justify-between text-caption text-muted mb-1.5">
                <span>{phaseText}</span>
                <span className="tabular-nums">{status.progress}%</span>
              </div>
              {/* barra fina real según SelfUpdateStatus.progress */}
              <div className="w-full bg-border/60 rounded-full h-1">
                <div className="bg-accent h-1 rounded-full transition-all duration-300" style={{ width: `${status.progress}%` }} />
              </div>
              <p className="text-caption text-muted mt-2">{t("selfupdate.confirmMsg")}</p>
            </div>
          )}

          {status.phase === "error" && (
            <p className="text-small text-danger">{t("selfupdate.error", { msg: status.message })}</p>
          )}
        </div>
      )}

      <ConfirmDialog
        open={confirming}
        onClose={() => setConfirming(false)}
        onConfirm={apply}
        title={t("selfupdate.update")}
        consequence={t("selfupdate.confirmMsg")}
        confirmLabel={t("selfupdate.update")}
      />
    </Card>
  );
}
