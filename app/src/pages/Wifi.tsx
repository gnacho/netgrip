import { useTranslation } from "react-i18next";
import { Wifi } from "lucide-react";
import type { GuestProbe, IoTProbe, WirelessRadio } from "../types";
import { Card, Pill } from "../components/Card";
import { IotWifiCard } from "../components/IotWifiCard";
import { GuestWifiCard } from "../components/GuestWifiCard";

export function WifiPage({ radios, iot, onIotChange, guest, onGuestChange }: {
  radios: WirelessRadio[];
  iot: IoTProbe | undefined;
  onIotChange: (p: IoTProbe) => void;
  guest: GuestProbe | undefined;
  onGuestChange: (p: GuestProbe) => void;
}) {
  const { t } = useTranslation();

  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <Card title={t("wifi.title")} icon={Wifi} action={
        <Pill tone="muted">
          {t("wifi.clients", { count: radios.reduce((n, r) => n + r.interfaces.reduce((m, i) => m + i.clients.length, 0), 0) })}
        </Pill>
      }>
        {radios.map((radio) => (
          <div key={radio.name} className="mb-3 last:mb-0">
            <div className="flex items-center gap-2 text-sm font-medium mb-1">
              <span>{radio.band === "5g" ? t("wifi.band5") : t("wifi.band24")}</span>
              {!radio.up && <Pill tone="danger">{t("wifi.down")}</Pill>}
              <span className="text-muted text-xs">
                {t("wifi.channel")} {radio.channel} · {radio.htmode} · {radio.txpower} dBm
              </span>
            </div>
            {radio.interfaces.map((iface) => (
              <div key={iface.ifname} className="ml-2 text-sm">
                <div className="flex justify-between py-0.5">
                  <span>{iface.ssid}</span>
                  <span className="text-muted">{t("wifi.clients", { count: iface.clients.length })}</span>
                </div>
                {iface.clients.map((c) => (
                  <div key={c.mac} className="flex justify-between text-xs text-muted ml-2 py-0.5">
                    <span className="font-mono">{c.mac}</span>
                    <span>{c.signal} dBm</span>
                  </div>
                ))}
              </div>
            ))}
          </div>
        ))}
      </Card>

      <IotWifiCard probe={iot} onChange={onIotChange} />
      <GuestWifiCard probe={guest} onChange={onGuestChange} />
    </div>
  );
}
