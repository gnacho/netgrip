import { useEffect, useState } from "react";
import { api } from "../api";
import type { Client } from "../types";
import { TopConsumersCard } from "../components/overview/TopConsumersCard";

/**
 * Tráfico (#487): "qué dispositivos y apps gastan más" vive en el grupo "Tu
 * red" con id "traffic" (antes "usage" bajo Servicios); el resumen queda para
 * salud y estado en vivo.
 */
export function TrafficPage({ onNavigate }: { onNavigate: (p: string) => void }) {
  const [clients, setClients] = useState<Client[]>();

  useEffect(() => {
    const load = () => api.clients().then((r) => setClients(r.clients)).catch(() => {});
    load();
    const id = window.setInterval(load, 30000);
    return () => window.clearInterval(id);
  }, []);

  return (
    <div className="grid grid-cols-1 md:grid-cols-12 gap-[var(--card-gap)]">
      <TopConsumersCard clients={clients} onNavigate={onNavigate} />
    </div>
  );
}
