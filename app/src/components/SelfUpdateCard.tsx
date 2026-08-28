import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Download } from "lucide-react";
import { api } from "../api";
import { Card, Pill } from "../components/Card";
import type { SelfUpdateCheck } from "../types";

export function SelfUpdateCard() {
  const { t } = useTranslation();
  const [check, setCheck] = useState<SelfUpdateCheck>();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ text: string; tone: "ok" | "danger" }>();
  const [confirming, setConfirming] = useState(false);

  useEffect(() => { api.selfUpdateCheck().then(setCheck).catch(() => {}); }, []);

  const apply = async () => {
    setBusy(true);
    setConfirming(false);
    try {
      await api.selfUpdateApply();
      setMsg({ text: t("selfupdate.done"), tone: "ok" });
      setTimeout(() => window.location.reload(), 5000);
    } catch (e: any) {
      setMsg({ text: e.message, tone: "danger" });
      setBusy(false);
    }
  };

  if (!check) return null;

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
        {!check.available && check.current !== "dev" && (
          <Pill tone="ok">{t("selfupdate.upToDate")}</Pill>
        )}
        {check.available && (
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
                <button onClick={apply} disabled={busy}
                  className="text-xs bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium">
                  {busy ? t("selfupdate.updating") : t("selfupdate.update")}
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
      </div>
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
