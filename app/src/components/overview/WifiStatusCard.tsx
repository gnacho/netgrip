import { useTranslation } from "react-i18next";
import { Wifi } from "lucide-react";
import type { WirelessRadio } from "../../types";
import { Card, Pill, SkeletonRows } from "../ui";
import { oneLine } from "./cards";

/**
 * Wi-Fi del Resumen en modo AP (#487, mockup de rediseño): lo que gestiona
 * un AP son sus radios. Cabecera con el SSID principal, bandas que emiten
 * y recuento de estaciones, todo derivado del probe wireless que ya carga
 * el Shell (sin endpoints nuevos).
 */
export function WifiStatusCard({ wireless, onNavigate, index = 3 }: {
  wireless?: WirelessRadio[];
  onNavigate: (p: string) => void;
  index?: number;
}) {
  const { t } = useTranslation();

  const radios = (wireless ?? []).filter((r) => r.up);
  const ifaces = radios.flatMap((r) => r.interfaces).filter((i) => !i.disabled);
  const ssids = [...new Set(ifaces.map((i) => i.ssid))];
  const clients = ifaces.reduce((n, i) => n + (i.clients?.length ?? 0), 0);
  const bands = [...new Set(radios.map((r) => t(r.band === "5g" ? "wifi.band5" : "wifi.band24")))];
  const emitting = radios.length > 0 && ifaces.length > 0;

  return (
    <Card index={index} className="md:col-span-5 order-4 md:order-none"
      title={oneLine(t("overview.wifiStatus"))} icon={Wifi} iconTone="teal"
      action={wireless && (
        <Pill tone={emitting ? "ok" : "muted"} live={emitting}>
          {emitting ? t("wifi.broadcasting") : t("wifi.bandOff")}
        </Pill>
      )}>
      {!wireless ? <SkeletonRows rows={2} /> : (
        <>
          <p className="stat-md truncate">{ssids[0] ?? "—"}</p>
          <p className="text-caption text-muted mt-1">
            {emitting
              ? t("overview.wifiBands", { bands: bands.join(" + ") })
              : t("wifi.bandOffHint")}
          </p>
          <div className="mt-3 flex items-center gap-3">
            <span className="text-body">{t("wifi.devices", { count: clients })}</span>
            <button type="button" onClick={() => onNavigate("wifi")}
              className="text-small text-accent hover:text-accent-hover ring-focus rounded-sm">
              {t("clients.open")} →
            </button>
          </div>
        </>
      )}
    </Card>
  );
}
