import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Download } from "lucide-react";
import { api } from "../api";
import { Card, Pill } from "../components/Card";
import type { SelfUpdateCheck, SelfUpdateStatus } from "../types";

export function SelfUpdateCard() {
  const { t } = useTranslation();
  const [check, setCheck] = useState<SelfUpdateCheck>();
  const [status, setStatus] = useState<SelfUpdateStatus>({ phase: "idle", progress: 0 });
  const [confirming, setConfirming] = useState(false);
  const pollRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);

  useEffect(() => { api.selfUpdateCheck().then(setCheck).catch(() => {}); }, []);

  useEffect(() => {
    if (status.phase === "downloading" || status.phase === "installing" || status.phase === "restarting") {
      pollRef.current = setInterval(() => {
        api.selfUpdateStatus().then((s) => {
          setStatus(s);
          if (s.phase === "idle" || s.phase === "error") {
            if (pollRef.current) clearInterval(pollRef.current);
          }
          if (s.phase === "restarting") {
            if (pollRef.current) clearInterval(pollRef.current);
            setTimeout(() => window.location.reload(), 5000);
          }
        }).catch(() => {});
      }, 500);
      return () => { if (pollRef.current) clearInterval(pollRef.current); };
    }
  }, [status.phase]);

  const apply = async () => {
    setConfirming(false);
    setStatus({ phase: "downloading", progress: 0 });
    try {
      await api.selfUpdateApply();
    } catch (e: any) {
      setStatus({ phase: "error", progress: 0, message: e.message });
    }
  };

  if (!check) return null;

  const isActive = status.phase === "downloading" || status.phase === "installing" || status.phase === "restarting";

  return (
    <Card title={t("selfupdate.title")} icon={Download}>
      <div className="flex flex-col gap-2 text-sm">
        <div className="flex justify-between gap-4 py-1 border-b border-border/50">
          <span className="text-muted">{t("selfupdate.current")}</span>
          <span className="font-mono">{check.current}</span>
        </div>
        {check.latest && (
          <div className="flex justify-between gap-4 py-1 border-b border-border/50">
            <span className="text-muted">{t("selfupdate.latest")}</span>
            <span className="font-mono">{check.latest}</span>
          </div>
        )}
        {!check.available && check.current !== "dev" && !isActive && (
          <Pill tone="ok">{t("selfupdate.upToDate")}</Pill>
        )}
        {check.available && !isActive && status.phase !== "error" && (
          <>
            <Pill tone="warn">{t("selfupdate.available")}</Pill>
            {check.notes && (
              <details className="mt-1">
                <summary className="text-xs cursor-pointer text-muted">{t("selfupdate.available")}</summary>
                <pre className="text-xs mt-1 whitespace-pre-wrap max-h-32 overflow-auto">{check.notes}</pre>
              </details>
            )}
            {confirming ? (
              <div className="flex gap-2 mt-2">
                <button onClick={apply}
                  className="text-xs bg-accent hover:bg-accent/85 rounded-lg px-3 py-1.5 font-medium">
                  {t("selfupdate.update")}
                </button>
                <button onClick={() => setConfirming(false)} className="text-xs text-muted">x</button>
              </div>
            ) : (
              <button onClick={() => setConfirming(true)}
                className="text-xs bg-accent hover:bg-accent/85 rounded-lg px-3 py-1.5 font-medium mt-2">
                {t("selfupdate.update")}
              </button>
            )}
            <p className="text-xs text-muted mt-1">{t("selfupdate.confirmMsg")}</p>
          </>
        )}

        {isActive && (
          <div className="mt-2">
            <div className="flex justify-between text-xs mb-1">
              <span>
                {status.phase === "downloading" && t("selfupdate.downloading", { pct: status.progress })}
                {status.phase === "installing" && t("selfupdate.installing")}
                {status.phase === "restarting" && t("selfupdate.restarting")}
              </span>
              <span className="text-muted">{status.progress}%</span>
            </div>
            <div className="w-full bg-border/50 rounded-full h-2">
              <div className="bg-accent h-2 rounded-full transition-all duration-300"
                style={{ width: `${status.progress}%` }} />
            </div>
          </div>
        )}

        {status.phase === "error" && (
          <p className="text-xs text-danger mt-2">{t("selfupdate.error", { msg: status.message })}</p>
        )}
      </div>
    </Card>
  );
}
