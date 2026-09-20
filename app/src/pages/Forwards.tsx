import type { FwdProbe } from "../types";
import { PortForwardCard } from "../components/ports/PortForwardCard";

/**
 * Puertos (#353): apertura de puertos a Internet. Vive como entrada propia
 * del menú ("Puertos") junto a "Puertos ethernet"; antes era una tarjeta al
 * final de la página WAN.
 */
export function ForwardsPage({ fwd, onFwdChange }: {
  fwd?: FwdProbe;
  onFwdChange: (p: FwdProbe) => void;
}) {
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <PortForwardCard probe={fwd} onChange={onFwdChange} />
    </div>
  );
}
