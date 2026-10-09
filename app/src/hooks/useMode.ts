import { useEffect, useState } from "react";
import { api } from "../api";
import type { ModeProbe } from "../types";

/**
 * Rol del equipo con hidratación instantánea (#484): el primer paint usa el
 * último probe cacheado en localStorage y el probe real se refetcha en
 * segundo plano. Sin esto, hasta que llegaba /api/mode el nav se quedaba
 * permisivo y las entradas gateadas por rol parpadeaban al recargar.
 * En demo api.mode() delega en demoApi (escenario García), así que la ruta
 * es la misma y el menú completo del demo no cambia.
 */
const CACHE_KEY = "netgrip.mode.v1";

/** Validación defensiva del shape mínimo cacheado: mode/has_wifi/port_count
 *  siempre presentes; role solo se acepta si es un rol conocido. */
function readCache(): ModeProbe | undefined {
  try {
    const raw = localStorage.getItem(CACHE_KEY);
    if (!raw) return undefined;
    const v = JSON.parse(raw) as Record<string, unknown>;
    if (!v || typeof v !== "object") return undefined;
    if (v.mode !== "router" && v.mode !== "ap") return undefined;
    if (typeof v.has_wifi !== "boolean") return undefined;
    if (typeof v.port_count !== "number" || v.port_count < 0) return undefined;
    if (v.role !== undefined && v.role !== "router" && v.role !== "ap" && v.role !== "switch") {
      return undefined;
    }
    return v as unknown as ModeProbe;
  } catch {
    return undefined;
  }
}

/** { mode, modeReady }: modeReady = hay un valor fiable (caché o probe).
 *  Hasta entonces los consumidores deben tratar el rol como desconocido y
 *  no pintar nada gateado por rol. */
export function useMode(): { mode: ModeProbe | undefined; modeReady: boolean } {
  // Estado inicial desde la caché: en una recarga el nav sale ya recortado
  // al rol real, sin esperar al probe.
  const [mode, setMode] = useState<ModeProbe | undefined>(() => readCache());

  useEffect(() => {
    let alive = true;
    api.mode()
      .then((m) => {
        if (!alive) return;
        setMode(m);
        try {
          localStorage.setItem(CACHE_KEY, JSON.stringify(m));
        } catch {
          // Sin localStorage (modo privado) el gating sigue funcionando
          // con el probe; solo se pierde la hidratación instantánea.
        }
      })
      .catch(() => {
        // Probe caído: se conserva la caché si la hay. Sin caché el rol
        // queda desconocido y el nav se queda en lo no gateado (overview).
      });
    return () => {
      alive = false;
    };
  }, []);

  return { mode, modeReady: mode !== undefined };
}
