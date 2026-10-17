import { TrafficRulesCard } from "../components/firewall/TrafficRulesCard";

/**
 * Reglas de tráfico (#444): editor de políticas de firewall estilo UniFi
 * (allow/deny/reject con origen IP/CIDR o cualquiera, destino + rango de
 * puertos, protocolo y horario). Reemplaza a la antigua card de cortafuegos
 * de Servicios: este es el único escritor de firewall.rule del panel.
 */
export function TrafficRulesPage() {
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <TrafficRulesCard />
    </div>
  );
}
