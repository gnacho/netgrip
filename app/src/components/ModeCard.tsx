import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Route } from "lucide-react";
import { api } from "../api";
import type { ModeProbe } from "../types";
import { Card, Pill, Row } from "./Card";

export function ModeCard() {
  const { t } = useTranslation();
  const [mode, setMode] = useState<ModeProbe>();

  useEffect(() => {
    api.mode().then(setMode).catch(() => {});
  }, []);

  return (
    <Card title={t("mode.title")} icon={Route}>
      {!mode ? (
        <p className="text-sm text-muted">…</p>
      ) : (
        <>
          <Row label={t("mode.title")} value={
            <Pill tone={mode.mode === "router" ? "ok" : "warn"}>
              {mode.mode === "router" ? t("mode.router") : t("mode.ap")}
            </Pill>
          } />
          {mode.mode === "ap" && <p className="text-xs text-warn mt-1">{t("mode.wanBridge")}</p>}
          <Row label="dnsmasq" value={mode.dnsmasq_on ? t("mode.on") : t("mode.off")} />
          <Row label="Firewall" value={mode.firewall_on ? t("mode.on") : t("mode.off")} />
          <p className="text-xs text-muted mt-2">{t("mode.hint")}</p>
        </>
      )}
    </Card>
  );
}
