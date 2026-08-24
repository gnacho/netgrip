import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Network } from "lucide-react";
import { api } from "../api";
import type { IPv6Probe } from "../types";
import { Card, Pill } from "./Card";
import { Toggle } from "./Toggle";

export function Ipv6Card({ probe, onChange }: {
  probe: IPv6Probe | undefined;
  onChange: (p: IPv6Probe) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  const toggle = async (enabled: boolean) => {
    setBusy(true);
    setMsg(undefined);
    try {
      const result = await api.setIpv6(enabled);
      onChange(result.state);
      if (result.status === "applied") {
        setMsg({ tone: "ok", text: t("ipv6.applied") });
      } else if (result.status === "rolled_back") {
        setMsg({ tone: "danger", text: t("ipv6.rolledBack") });
      }
    } catch {
      setMsg({ tone: "danger", text: t("ipv6.failed") });
      onChange(await api.ipv6());
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card title={t("ipv6.title")} icon={Network}>
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <span className="text-sm">{t("ipv6.toggle")}</span>
          {probe && (
            <Pill tone={probe.state === "enabled" ? "ok" : probe.state === "disabled" ? "muted" : "warn"}>
              {t(`ipv6.${probe.state}`)}
            </Pill>
          )}
        </div>
        <Toggle checked={probe?.state === "enabled"} busy={busy} disabled={!probe} onChange={toggle} />
      </div>
      {probe && (
        <p className="text-xs text-muted">
          {t("ipv6.details", {
            odhcpd: probe.odhcpd_enabled ? t("ipv6.on") : t("ipv6.off"),
            ra: probe.ra_mode || "-",
            dhcpv6: probe.dhcpv6_mode || "-",
          })}
        </p>
      )}
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
