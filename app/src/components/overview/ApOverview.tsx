import { ClientsCard, HealthHero, LiveTrafficCard } from "./cards";
import { UplinkCard } from "./UplinkCard";
import { WifiStatusCard } from "./WifiStatusCard";
import type { OverviewVariantProps } from "./props";

/**
 * Resumen modo punto de acceso (#487, mockup de rediseño): arriba la salud
 * y el uplink hacia el router; después el tráfico que mueve y, lo que de
 * verdad gestiona un AP, sus radios y sus clientes. Sin tarjetas de
 * gateway (Internet/WAN, dnsmasq) ni de switch (VLANs, STP).
 */
export function ApOverview(props: OverviewVariantProps) {
  const { board, system, health, wireless, ethports, onNavigate } = props;
  return (
    <>
      <HealthHero health={health} board={board} system={system} onNavigate={onNavigate} />
      {/* Cómo sale este equipo al resto de la red: 4 + 8 = una fila. */}
      <UplinkCard ports={ethports} ap />
      <LiveTrafficCard />
      {/* Lo que gestiona un AP: radios Wi-Fi y clientes asociados. */}
      <WifiStatusCard wireless={wireless} onNavigate={onNavigate} />
      <ClientsCard onNavigate={onNavigate} className="md:col-span-7" />
    </>
  );
}
