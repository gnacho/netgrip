import type { EthPort } from "../types";
import { EthPortsCard } from "../components/ports/EthPortsCard";
import { PoECard } from "../components/ports/PoECard";
import { SwitchCard } from "../components/ports/SwitchCard";

/**
 * Puertos ethernet (#353, antes "Puertos"): PoE y bocas del switch; lo de
 * ingeniería (LAG, plantillas de puerto, perfiles, modos, VLANs,
 * estadísticas, IGMP, storm control, MAC ACL) vive en la página "Avanzadas"
 * (#441). El port-forwarding (abrir puertos a Internet) vive en la página
 * "Puertos" (Forwards.tsx). El chasis RJ45 de la instalación llega del
 * resumen (#384).
 */
export function Ports({ ethports }: { ethports?: EthPort[] }) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-[var(--card-gap)]">
      <EthPortsCard ports={ethports} index={0} />
      <PoECard index={1} />
      <SwitchCard index={2} />
    </div>
  );
}
