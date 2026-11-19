import { ClientsCard, HealthHero, LiveTrafficCard } from "./cards";
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
      <HealthHero health={health} board={board} system={system} onNavigate={onNavigate}
        wireless={wireless} ports={ethports} ap />
      <LiveTrafficCard />
      {/* Lo que gestiona un AP: sus bocas y clientes asociados. */}
      <EthPortsCard ports={ethports} />
      <ClientsCard onNavigate={onNavigate} className="md:col-span-7" />
    </>
  );
}
