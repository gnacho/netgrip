import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Zap } from "lucide-react";
import { api } from "../api";
import type { OffloadProbe } from "../types";
import { Card, Pill } from "./Card";

export function OffloadCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<OffloadProbe>();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string } | undefined>();

  useEffect(() => {
    api.offload().then(setProbe).catch(() => {});
  }, []);

  const toggle = async (enabled: boolean) => {
    setBusy(true); setMsg(undefined);
    try {
      await api.setOffload(enabled);
      setProbe(await api.offload());
      setMsg({ tone: "ok", text: t("access.saved") });
    } catch (err) {
      setMsg({ tone: "danger", text: err instanceof Error ? err.message : String(err) });
    } finally {
      setBusy(false);
    }
  };

  if (!probe || !probe.applicable) return null;

  return (
    <Card title={t("offload.title")} icon={Zap}>
      <div className="flex flex-col gap-3 text-sm">
        <div className="flex items-center justify-between gap-2">
          <div className="flex-1">
            <div className="flex items-center gap-2">
              <span>{t("offload.software")}</span>
              <Pill tone={probe.software ? "ok" : "muted"}>
                {probe.software ? t("offload.on") : t("offload.off")}
              </Pill>
            </div>
            <div className="text-xs text-muted">{t("offload.softwareHint")}</div>
          </div>
          <input
            type="checkbox" checked={probe.software} disabled={busy}
            onChange={(e) => toggle(e.target.checked)} className="accent-accent"
          />
        </div>
        <div className="text-xs text-muted border-t border-border/50 pt-2">
          <span className="font-medium">{t("offload.hardwareNote")}</span> {t("offload.hardwareHint")}
        </div>
        {msg && <p className={`text-xs ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
      </div>
    </Card>
  );
}
