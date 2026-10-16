import type { PhysPortsProbe } from "../types";

export interface DegradedPort {
  name: string;
  /** velocidad negociada actual (Mbps) */
  speed: number;
  /** techo del puerto según los modos soportados que reporta ethtool (Mbps) */
  max: number;
}

/** fmtMbps (#522): velocidad con unidad para las frases de salud/banner:
 *  1000 -> "1 G", 2500 -> "2.5 G", 100 -> "100 Mbps". */
export function fmtMbps(mbps: number): string {
  if (mbps >= 1000) {
    const g = mbps / 1000;
    return `${Number.isInteger(g) ? g : g.toFixed(1)} G`;
  }
  return `${mbps} Mbps`;
}

/**
 * degradedPorts (#522): puertos enlazando POR DEBAJO de su capacidad
 * (autoneg activa, enlace arriba y la velocidad negociada por debajo del
 * máximo soportado). El caso típico es un cable dañado o de 4 hilos
 * (100 Mbps en un puerto de 1 G). Fuente: /api/physports (ethtool).
 * Un puerto caído reporta speed 0 ("Speed: Unknown") y no cuenta.
 */
export function degradedPorts(probe?: PhysPortsProbe): DegradedPort[] {
  if (!probe?.applicable) return [];
  const out: DegradedPort[] = [];
  for (const p of probe.ports) {
    if (!p.autoneg || p.speed_mbps <= 0) continue;
    const max = Math.max(0, ...p.supported.map((m) => m.speed_mbps));
    if (max > p.speed_mbps) out.push({ name: p.name, speed: p.speed_mbps, max });
  }
  return out;
}
