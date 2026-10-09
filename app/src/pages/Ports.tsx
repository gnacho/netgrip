import type { EthPort } from "../types";
import { EthPortsCard } from "../components/ports/EthPortsCard";
import { PoECard } from "../components/ports/PoECard";

/**
 * Puertos ethernet (#353, antes "Puertos"): el chasis RJ45 es el
 * protagonista (ancho completo) y cada boca abre su detalle al pulsarla
 * (nota, admin y PoE; #484). La tarjeta "Las bocas del switch" se
 * eliminó: su funcionalidad vive en esa banda de detalle. Lo de
 * ingeniería (LAG, plantillas de puerto, perfiles, modos, VLANs,
 * estadísticas, IGMP, storm control, MAC ACL) vive en la página
 * "Avanzadas" (#441). El port-forwarding (abrir puertos a Internet) vive
 * en la página "Puertos" (Forwards.tsx). El chasis RJ45 de la
 * instalación llega del resumen (#384).
 */
export function Ports({ ethports }: { ethports?: EthPort[] }) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-[var(--card-gap)]">
      <EthPortsCard ports={ethports} index={0} />
      <PoECard index={1} />
    </div>
  );
}
