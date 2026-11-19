import {
  ClientsCard, CpuCard, DriftSection, FlashCard, HealthHero, InternetCard,
  LiveTrafficCard, MemoryCard,
} from "./cards";
import { MiniChassis } from "./MiniChassis";
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
      {/* Recursos del equipo, uno por fila a todo el ancho (#598). */}
      <CpuCard />
      <MemoryCard system={system} />
      <FlashCard system={system} />
      {/* Cómo está cableado este equipo, de un vistazo (mockup #487). */}
      <MiniChassis ports={ethports} onNavigate={onNavigate} />
      <ClientsCard onNavigate={onNavigate} />
      <DriftSection drift={drift} onChange={onDriftChange} />
    </>
  );
}
