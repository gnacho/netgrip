import type { Board, DriftProbe, EthPort, ModeProbe, SystemInfo, WanStatus, WirelessRadio } from "../../types";
import type { HealthScore } from "../../hooks/useHealthScore";

/** Props comunes de las variantes por rol del Resumen (#487). */
export interface OverviewVariantProps {
  board?: Board;
  system?: SystemInfo;
  wan?: WanStatus;
  drift?: DriftProbe;
  onDriftChange: (d: DriftProbe) => void;
  health: HealthScore;
  mode?: ModeProbe;
  wireless?: WirelessRadio[];
  ethports?: EthPort[];
  onNavigate: (page: string) => void;
}
