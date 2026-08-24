import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Wifi } from "lucide-react";
import { api } from "../api";
import type { IoTProbe } from "../types";
import { Card, Pill, Row } from "./Card";
import { Toggle } from "./Toggle";

export function IotWifiCard({ probe, onChange }: {
  probe: IoTProbe | undefined;
  onChange: (p: IoTProbe) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [ssid, setSsid] = useState("");
  const [key, setKey] = useState("");
  const [band, setBand] = useState("2g");

  useEffect(() => {
    if (probe) {
      setSsid(probe.ssid || "");
      if (probe.band) setBand(probe.band);
    }
  }, [probe]);

  const run = async (enabled: boolean) => {
    setBusy(true);
    setMsg(undefined);
    try {
      const result = await api.setIotwifi(
        enabled ? { enabled, ssid, key, band } : { enabled },
      );
      onChange(result.state);
      setKey("");
      setMsg(result.status === "applied"
        ? { tone: "ok", text: t("iot.applied") }
        : { tone: "danger", text: result.error || t("iot.rolledBack") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("iot.failed") });
      onChange(await api.iotwifi());
    } finally {
      setBusy(false);
    }
  };

  const canEnable = ssid.trim() !== "" && key.length >= 8;

  return (
    <Card title={t("iot.title")} icon={Wifi}>
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <span className="text-sm">{t("iot.toggle")}</span>
          {probe && (
            <Pill tone={probe.active ? "ok" : "muted"}>
              {probe.active ? t("iot.on") : t("iot.off")}
            </Pill>
          )}
        </div>
        <Toggle checked={probe?.active ?? false} busy={busy}
          disabled={!probe || (!probe.active && !canEnable)}
          onChange={run} />
      </div>

      {probe?.active && (
        <>
          <Row label="SSID" value={probe.ssid} />
          <Row label={t("iot.clients")} value={t("wifi.clients", { count: probe.clients })} />
          <Row label={t("iot.isolation")} value={probe.isolated ? t("ipv6.on") : t("ipv6.off")} />
        </>
      )}

      {!probe?.active && (
        <div className="mt-2 flex flex-col gap-2">
          <input value={ssid} onChange={(e) => setSsid(e.target.value)}
            placeholder="SSID"
            className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
          <input type="password" value={key} onChange={(e) => setKey(e.target.value)}
            placeholder={t("iot.key")} autoComplete="new-password"
            className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
          <select value={band} onChange={(e) => setBand(e.target.value)}
            className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent">
            <option value="2g">{t("wifi.band24")}</option>
            <option value="5g">{t("wifi.band5")}</option>
            <option value="both">{t("iot.both")}</option>
          </select>
          <p className="text-xs text-warn">{t("iot.reloadWarn")}</p>
        </div>
      )}
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
