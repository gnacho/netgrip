import {
  ClientsCard, DriftSection, HealthHero, InternetCard, LiveTrafficCard, ResourcesCard,
} from "./cards";
import { EthPortsCard } from "../ports/EthPortsCard";
import type { OverviewVariantProps } from "./props";

/**
 * Resumen modo gateway (#487, mockup de rediseño): arriba la salud y la
 * conexión a Internet; después el tráfico en vivo, los recursos, un
 * mini-chasis con el cableado del equipo y, al final, dispositivos y
 * configuración protegida. Es la variante más completa: la puerta de enlace
 * es la que más superficie gestiona.
 */
export function GatewayOverview(props: OverviewVariantProps) {
  const { board, system, wan, drift, onDriftChange, health, mode, ethports, onNavigate } = props;
  return (
    <>
      <HealthHero health={health} board={board} system={system} onNavigate={onNavigate} />
      {/* El enlace y su tráfico en vivo, cada uno a todo el ancho (#598). */}
      <InternetCard wan={wan} mode={mode} />
      <LiveTrafficCard />
      {/* CPU, memoria y almacenamiento comparten tarjeta (#601). */}
      <ResourcesCard system={system} />
      {/* Las bocas usan el mismo modelo que el Resumen de AP (#602). */}
      <EthPortsCard ports={ethports} index={3} />
      <ClientsCard onNavigate={onNavigate} />
      <DriftSection drift={drift} onChange={onDriftChange} />
    </>
  );
}
