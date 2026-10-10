import { HealthHero, LiveTrafficCard } from "./cards";
import { UplinkCard } from "./UplinkCard";
import { VlansCard } from "./VlansCard";
import { StpStatusCard } from "./StpStatusCard";
import { EthPortsCard } from "../ports/EthPortsCard";
import { PortStatsCard } from "../ports/PortStatsCard";
import type { OverviewVariantProps } from "./props";

/**
 * Resumen modo switch (#487, mockup de rediseño): arriba la salud y el
 * uplink; el protagonista es el chasis de bocas a ancho completo, seguido
 * de las VLANs que atraviesan el equipo, el estado STP y el tráfico por
 * boca. Sin tarjetas de gateway (Internet/WAN, Wi-Fi) ni de recursos: un
 * switch L2 no las gestiona.
 */
export function SwitchOverview(props: OverviewVariantProps) {
  const { board, system, health, ethports, onNavigate } = props;
  return (
    <>
      <HealthHero health={health} board={board} system={system} onNavigate={onNavigate} />
      {/* Cómo se conecta este equipo al resto de la red: 4 + 8 = una fila. */}
      <UplinkCard ports={ethports} ap={false} />
      <LiveTrafficCard />
      {/* El protagonista del switch: el chasis de bocas, a ancho completo. */}
      <EthPortsCard ports={ethports} index={3} className="md:col-span-12 order-5 md:order-none" />
      {/* Qué redes separadas pasan por el chasis y quién vigila los bucles. */}
      <VlansCard index={4} />
      <StpStatusCard index={4} />
      {/* Tráfico por boca en vivo (probe portstats ya existente). */}
      <div className="md:col-span-12 order-9 md:order-none">
        <PortStatsCard />
      </div>
    </>
  );
}
