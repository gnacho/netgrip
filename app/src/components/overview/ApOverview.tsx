import { ClientsCard, HealthHero, LiveTrafficCard } from "./cards";
import { UplinkCard } from "./UplinkCard";
import { WifiStatusCard } from "./WifiStatusCard";
import { EthPortsCard } from "../ports/EthPortsCard";
import type { OverviewVariantProps } from "./props";

/**
 * Resumen modo punto de acceso (#487, mockup de rediseño): arriba la salud
 * y el uplink hacia el router; después el tráfico que mueve y, lo que de
 * verdad gestiona un AP, sus radios, sus bocas (#500: la página Puertos
 * desaparece en APs y su detalle vive aquí) y sus clientes. Sin tarjetas
 * de gateway (Internet/WAN, dnsmasq) ni de switch (VLANs, STP).
 */
export function ApOverview(props: OverviewVariantProps) {
  const { board, system, health, wireless, ethports, onNavigate } = props;
  return (
    <>
      <HealthHero health={health} board={board} system={system} onNavigate={onNavigate} />
      {/* Cómo sale este equipo al resto de la red: 4 + 8 = una fila. */}
      <UplinkCard ports={ethports} ap />
      <LiveTrafficCard />
      {/* Lo que gestiona un AP: radios Wi-Fi, bocas y clientes asociados. */}
      <WifiStatusCard wireless={wireless} onNavigate={onNavigate} />
      <EthPortsCard ports={ethports} />
      <ClientsCard onNavigate={onNavigate} className="md:col-span-7" />
    </>
  );
}
