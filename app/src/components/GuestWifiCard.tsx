import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { UserX } from "lucide-react";
import { api } from "../api";
import type { GuestProbe } from "../types";
import { Card, Pill, Row } from "./Card";
import { Toggle } from "./Toggle";

export function GuestWifiCard({ probe, onChange }: {
  probe: GuestProbe | undefined;
  onChange: (p: GuestProbe) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [ssid, setSsid] = useState("");
  const [key, setKey] = useState("");
  const [band, setBand] = useState("2g");

  useEffect(() => {
    if (probe?.ssid) setSsid(probe.ssid);
  }, [probe]);

  const run = async (enabled: boolean) => {
    setBusy(true);
    setMsg(undefined);
    try {
      const result = await api.setGuestwifi(
        enabled ? { enabled, ssid, key, band } : { enabled },
      );
      onChange(result.state);
      setKey("");
      setMsg(result.status === "applied"
        ? { tone: "ok", text: t("guest.applied") }
        : { tone: "danger", text: result.error || t("guest.rolledBack") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("guest.failed") });
      onChange(await api.guestwifi());
    } finally {
      setBusy(false);
    }
  };

  const canEnable = ssid.trim() !== "" && key.length >= 8;

  if (!probe || !probe.gateway) return null;
  return (
    <Card title={t("guest.title")} icon={UserX}>
      <>
        <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <span className="text-sm">{t("guest.toggle")}</span>
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

          {probe.gl_conflict && (
            <p className="text-xs text-warn mb-2">{t("guest.glConflict")}</p>
          )}

          {probe?.active && (
            <>
              <Row label="SSID" value={probe.ssid} />
              <Row label={t("guest.subnet")} value={probe.subnet} />
              <Row label={t("iot.clients")} value={t("wifi.clients", { count: probe.clients })} />
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
              <p className="text-xs text-muted">{t("guest.scope")}</p>
            </div>
          )}
      </>
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
