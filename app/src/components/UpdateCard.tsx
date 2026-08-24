import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowUpCircle } from "lucide-react";
import { api } from "../api";
import type { Board, UpdateCheck } from "../types";
import { Card, Pill, Row } from "./Card";

export function UpdateCard({ board, update, onChange }: {
  board: Board | undefined;
  update: UpdateCheck | undefined;
  onChange: (u: UpdateCheck) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  const recheck = async () => {
    setBusy(true);
    try { onChange(await api.updateCheck()); } catch { /* keep previous */ }
    setBusy(false);
  };

  const start = async () => {
    setConfirm(false);
    setBusy(true);
    setMsg(undefined);
    try {
      await api.startUpdate();
      setMsg({ tone: "ok", text: t("update.started") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("update.failed") });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card title={t("update.title")} icon={ArrowUpCircle} action={
      <button onClick={recheck} disabled={busy} className="text-xs text-muted hover:text-text">
        {busy ? t("update.checking") : t("update.check")}
      </button>
    }>
      <Row label={t("update.current")} value={board?.release && `${board.release.version} (${board.release.revision})`} />
      {update?.owut_present === false ? (
        <p className="text-sm text-muted mt-2">{t("update.noOwut")}</p>
      ) : update ? (
        <>
          {!update.same_version && <Row label={t("update.available")} value={update.version_to} />}
          <Row label="" value={
            !update.same_version
              ? <Pill tone="warn">{t("update.newVersion")}</Pill>
              : update.out_of_date_packages > 0
                ? <Pill tone="warn">{t("update.outOfDate", { count: update.out_of_date_packages })}</Pill>
                : <Pill tone="ok">{t("update.upToDate")}</Pill>
          } />
          {update.warnings.map((w) => <p key={w} className="text-xs text-warn mt-1">{w}</p>)}
          {!update.safe_to_proceed && update.safe_with_reinstall && (
            <p className="text-xs text-warn mt-2">{t("update.reinstallNote")}</p>
          )}
          {!update.safe_to_proceed && !update.safe_with_reinstall && (
            <p className="text-xs text-danger mt-2">{t("update.unsafe")}</p>
          )}
          {confirm ? (
            <div className="mt-3 border border-warn/40 rounded-lg p-3">
              <p className="text-xs font-medium mb-1">{t("update.confirmTitle")}</p>
              <p className="text-xs text-muted mb-3">
                {update.same_version ? t("update.confirmBodyPackages") : t("update.confirmBodyFirmware")}
              </p>
              <div className="flex gap-2">
                <button onClick={start} className="text-xs bg-danger/80 hover:bg-danger rounded-lg px-3 py-1.5 font-medium">
                  {t("update.confirmYes")}
                </button>
                <button onClick={() => setConfirm(false)} className="text-xs bg-border hover:bg-border/70 rounded-lg px-3 py-1.5">
                  {t("update.confirmNo")}
                </button>
              </div>
            </div>
          ) : update.available && (
            <button
              onClick={() => setConfirm(true)}
              disabled={(!update.safe_to_proceed && !update.safe_with_reinstall) || busy}
              className="mt-3 text-sm bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium"
            >
              {update.same_version
                ? t("update.upgradePackages", { count: update.out_of_date_packages })
                : t("update.upgradeFirmware")}
            </button>
          )}
        </>
      ) : (
        <p className="text-sm text-muted mt-2">{t("update.checking")}</p>
      )}
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
