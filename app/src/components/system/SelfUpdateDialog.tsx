import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowRight, Check, Circle, DownloadCloud, Loader2, AlertTriangle } from "lucide-react";
import { api } from "../../api";
import type { SelfUpdateCheck, SelfUpdateStatus } from "../../types";
import { Button, Modal, useToast } from "../ui";

const STEP_ORDER = ["downloading", "installing", "restarting"] as const;

export function SelfUpdateDialog({ open, onClose, initialCheck }: {
  open: boolean;
  onClose: () => void;
  initialCheck?: SelfUpdateCheck;
}) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [check, setCheck] = useState<SelfUpdateCheck | undefined>(initialCheck);
  const [status, setStatus] = useState<SelfUpdateStatus>({ phase: "idle", progress: 0 });
  const [ack, setAck] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);
  const reloadRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);

  const active = status.phase === "downloading" || status.phase === "installing" || status.phase === "restarting";

  const clearTimers = useCallback(() => {
    if (pollRef.current) clearInterval(pollRef.current);
    if (reloadRef.current) clearInterval(reloadRef.current);
    pollRef.current = undefined;
    reloadRef.current = undefined;
  }, []);

  useEffect(() => {
    if (initialCheck) setCheck(initialCheck);
  }, [initialCheck]);

  useEffect(() => {
    if (!open) {
      clearTimers();
      setError(null);
      setAck(false);
      return;
    }
    setError(null);
    setAck(false);
    if (!check) {
      api.selfUpdateCheck().then(setCheck).catch(() => {});
    }
  }, [open, check, clearTimers]);

  useEffect(() => {
    if (!active) return;
    pollRef.current = setInterval(() => {
      api.selfUpdateStatus().then((s) => {
        setStatus(s);
        if (s.phase === "restarting") {
          // backend ya reinició el servicio: esperamos a que vuelva
          if (pollRef.current) clearInterval(pollRef.current);
          reloadRef.current = setInterval(() => {
            api.me().then(() => {
              if (reloadRef.current) clearInterval(reloadRef.current);
              window.location.reload();
            }).catch(() => {});
          }, 2000);
        } else if (s.phase === "error") {
          if (pollRef.current) clearInterval(pollRef.current);
          setError(s.message || t("selfupdate.failed"));
        } else if (s.phase === "idle") {
          // terminó sin reiniciar (no debería pasar en self-update)
          if (pollRef.current) clearInterval(pollRef.current);
          push({ tone: "ok", text: t("selfupdate.doneToast", { version: check?.latest ?? "" }) });
          onClose();
          api.selfUpdateCheck().then(setCheck).catch(() => {});
        }
      }).catch(() => {});
    }, 800);
    return clearTimers;
  }, [active, clearTimers, t, push, check?.latest, onClose]);

  const apply = async () => {
    setError(null);
    try {
      await api.selfUpdateApply();
      setStatus({ phase: "downloading", progress: 0 });
    } catch (e) {
      setError(e instanceof Error ? e.message : t("selfupdate.failed"));
    }
  };

  const changelogLines = useMemo(() =>
    (check?.notes ?? "")
      .split("\n")
      .map((l) => l.trim())
      .filter((l) => l && !/^(co-authored-by|signed-off-by|reviewed-by):/i.test(l)),
    [check?.notes],
  );

  const stepIndex = STEP_ORDER.indexOf(status.phase as typeof STEP_ORDER[number]);

  return (
    <Modal open={open} onClose={() => { if (!active) onClose(); }} wide>
      <div className="flex flex-col gap-4">
        <div className="flex items-center gap-2">
          <DownloadCloud size={22} className="text-accent" />
          <h2 className="text-h2 flex-1">{t("selfupdate.dialogTitle")}</h2>
        </div>

        {!active && !error && (
          <>
            <p className="text-body text-muted">{t("selfupdate.dialogDesc")}</p>

            <div className="flex items-center justify-center gap-3 rounded-lg border border-border bg-surface-2 px-4 py-3">
              <span className="font-mono text-small text-muted">{check?.current ?? "—"}</span>
              <ArrowRight size={16} className="text-muted" />
              <span className="font-mono text-small font-semibold text-accent">{check?.latest ?? "—"}</span>
            </div>

            {changelogLines.length > 0 && (
              <div className="flex flex-col gap-2">
                <p className="text-eyebrow text-faint">{t("selfupdate.changelogTitle")}</p>
                <div className="max-h-44 overflow-y-auto rounded-lg border border-border bg-surface-2 px-3.5 py-2.5">
                  <ul className="flex flex-col gap-1.5">
                    {changelogLines.map((line, i) => (
                      <li key={i} className="flex items-start gap-2 text-small text-muted">
                        <span className="mt-1.5 h-1 w-1 shrink-0 rounded-full bg-text-muted/60" />
                        {line.replace(/^[-*]\s+/, "")}
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            )}

            <label className="flex cursor-pointer items-start gap-2.5 rounded-lg bg-warn-soft px-3.5 py-2.5 text-small leading-snug text-warn">
              <input
                type="checkbox"
                checked={ack}
                onChange={(e) => setAck(e.target.checked)}
                className="mt-0.5 h-4 w-4 accent-warn shrink-0"
              />
              <span>{t("selfupdate.downtimeNotice")}</span>
            </label>
          </>
        )}

        {active && (
          <div className="flex flex-col gap-4" role="status">
            <ul className="flex flex-col gap-2.5">
              {STEP_ORDER.map((s, i) => {
                const done = stepIndex > i;
                const current = stepIndex === i;
                return (
                  <li key={s} className="flex items-center gap-2.5 text-body">
                    {done ? (
                      <Check size={18} className="shrink-0 text-ok" />
                    ) : current ? (
                      <Loader2 size={18} className="shrink-0 animate-spin text-accent" />
                    ) : (
                      <Circle size={18} className="shrink-0 text-muted/40" />
                    )}
                    <span className={done ? "text-muted" : current ? "font-medium text-text" : "text-faint"}>
                      {t(`selfupdate.step${s.charAt(0).toUpperCase() + s.slice(1)}`)}
                    </span>
                  </li>
                );
              })}
            </ul>
            <div className="flex flex-col gap-1.5">
              <div className="w-full bg-border/60 rounded-full h-1.5">
                <div className="bg-accent h-1.5 rounded-full transition-all duration-300" style={{ width: `${status.progress}%` }} />
              </div>
              <div className="flex justify-between text-caption text-muted">
                <span>{t(`selfupdate.step${status.phase.charAt(0).toUpperCase() + status.phase.slice(1)}`)}…</span>
                <span className="font-mono tabular-nums">{status.progress}%</span>
              </div>
            </div>
            <p className="text-center text-caption text-muted">{t("selfupdate.reloadSoon")}</p>
          </div>
        )}

        {error && !active && (
          <div className="flex items-start gap-3 rounded-lg bg-danger/10 px-4 py-3">
            <AlertTriangle size={20} className="shrink-0 text-danger mt-0.5" />
            <div>
              <p className="text-body text-danger">{t("selfupdate.failed")}</p>
              {error && <p className="text-caption text-muted mt-1">{error}</p>}
            </div>
          </div>
        )}

        <div className="flex justify-end gap-2 mt-2">
          {!active && !error && (
            <>
              <Button variant="ghost" onClick={onClose}>{t("selfupdate.cancel")}</Button>
              <Button onClick={apply} disabled={!ack}>{t("selfupdate.start")}</Button>
            </>
          )}
          {error && !active && (
            <Button variant="ghost" onClick={onClose}>{t("selfupdate.close")}</Button>
          )}
        </div>
      </div>
    </Modal>
  );
}
