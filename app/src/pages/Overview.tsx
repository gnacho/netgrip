import { useMode } from "../hooks/useMode";
import type { Board, DriftProbe, EthPort, ModeProbe, SystemInfo, WanStatus, WirelessRadio } from "../types";
import type { HealthScore } from "../hooks/useHealthScore";
import {
  ClientsCard, CpuCard, DriftSection, FlashCard, HealthHero, InternetCard,
  LiveTrafficCard, MemoryCard,
} from "../components/overview/cards";
import { GatewayOverview } from "../components/overview/GatewayOverview";
import { ApOverview } from "../components/overview/ApOverview";
import { SwitchOverview } from "../components/overview/SwitchOverview";

/**
 * Resumen (#487): composición distinta según el rol del equipo, como en el
 * mockup de rediseño - el gateway ve arriba su salud e Internet, el AP sus
 * radios y clientes, y el switch su chasis de bocas. Hasta que useMode no
 * confirma el rol (hidrata desde caché, #484) se pinta el layout genérico
 * histórico, sin parpadeo entre composiciones.
 */
export function Overview({ board, system, wan, drift, onDriftChange, isSwitch, health, mode, wireless, ethports, onNavigate }: {
  board?: Board;
  system?: SystemInfo;
  wan?: WanStatus;
  drift?: DriftProbe;
  onDriftChange: (d: DriftProbe) => void;
  isSwitch: boolean;
  health: HealthScore;
  mode?: ModeProbe;
  wireless?: WirelessRadio[];
  ethports?: EthPort[];
  onNavigate: (page: string) => void;
}) {
  const { mode: probe, modeReady } = useMode();
  const role = probe?.role ?? (probe?.mode === "ap" ? "ap" : "router");

  const props = {
    board, system, wan, drift, onDriftChange, health, mode, wireless, ethports, onNavigate,
  };

  // Layout genérico (el histórico) mientras el rol no es fiable todavía.
  if (!modeReady) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-12 gap-[var(--card-gap)]">
        <HealthHero health={health} board={board} system={system} onNavigate={onNavigate} />
        {/* El enlace y su tráfico en vivo: 4 + 8 = una fila completa. */}
        {!isSwitch && <InternetCard wan={wan} mode={mode} />}
        <LiveTrafficCard />
        {/* Recursos del equipo, los tres juntos: 4 + 4 + 4. */}
        <CpuCard />
        <MemoryCard system={system} />
        <FlashCard system={system} />
        {/* El consumo (quién gasta más) vive en la página Consumo (#385) y el
            chasis de puertos en Puertos ethernet (#384). */}
        <ClientsCard onNavigate={onNavigate} />
        <DriftSection drift={drift} onChange={onDriftChange} />
      </div>
    );
  }

  const Variant = role === "switch" ? SwitchOverview : role === "ap" ? ApOverview : GatewayOverview;
  return (
    <div className="grid grid-cols-1 md:grid-cols-12 gap-[var(--card-gap)]">
      <Variant {...props} />
    </div>
  );
}
