import { useTranslation } from "react-i18next";
import { Cable } from "lucide-react";
import type { EthPort } from "../../types";
import { Card, KeyValue, Pill, SkeletonRows } from "../ui";
import { oneLine } from "./cards";

/**
 * Uplink del Resumen en modo AP y switch (#487, mockup de rediseño): como
 * estos equipos no son la puerta de enlace, el probe WAN no aplica y lo que
 * de verdad importa es por qué boca salen al resto de la red y a qué
 * velocidad. La IP de gestión y su puerta de enlace no las expone ningún
 * probe: el mockup las muestra, pero sin backend se omiten.
 */
export function UplinkCard({ ports, ap, index = 1 }: {
  ports?: EthPort[];
  ap: boolean;
  index?: number;
}) {
  const { t } = useTranslation();

  // Boca de salida: la marcada como WAN si la hay (AP con uplink dedicado);
  // si no, la boca activa más rápida con algo conectado detrás.
  const uplink = !ports ? undefined : [...ports]
    .filter((p) => p.up)
    .sort((a, b) =>
      (Number(b.wan) - Number(a.wan)) ||
      (b.devices.length > 0 ? 1 : 0) - (a.devices.length > 0 ? 1 : 0) ||
      b.speed_mbps - a.speed_mbps)[0];

  const label = (p: EthPort) =>
    p.wan ? "WAN" : p.name.toUpperCase().replace(/^LAN(\d+)$/, "LAN $1");
  const speed = (p: EthPort) =>
    p.speed_mbps >= 1000 ? `${p.speed_mbps / 1000} Gb/s` : `${p.speed_mbps} Mb/s`;

  return (
    <Card index={index} id="uplink" className="md:col-span-4 order-2 md:order-none"
      title={oneLine(t("overview.uplink"))} icon={Cable}
      action={ports && (
        <Pill tone={uplink ? "ok" : "danger"} live={!!uplink}>
          {uplink ? t("overview.connected") : t("overview.uplinkDown")}
        </Pill>
      )}>
      {!ports ? <SkeletonRows rows={2} /> : (
        <>
          {uplink && (
            <p className="text-body mb-2">
              {t("overview.uplinkVia", {
                port: label(uplink),
                speed: uplink.speed_mbps > 0 ? speed(uplink) : "?",
              })}
            </p>
          )}
          <KeyValue items={[
            { label: t("overview.uplinkMode"), value: ap ? t("overview.apNoNat") : t("overview.switchRole") },
            ...(uplink && uplink.devices.length > 0
              ? [{ label: t("overview.uplinkDevice"), value: uplink.devices[0].name || uplink.devices[0].mac }]
              : []),
          ]} />
        </>
      )}
    </Card>
  );
}
