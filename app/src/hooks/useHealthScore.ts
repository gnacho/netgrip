import { useMemo } from "react";
import type {
  DriftProbe,
  ModeProbe,
  PhysPortsProbe,
  SystemInfo,
  WanStatus,
  WirelessRadio,
} from "../types";
import { degradedPorts, fmtMbps } from "../lib/degradedPorts";

export type HealthTone = "ok" | "warn" | "danger";

export interface HealthReason {
  /** clave i18n bajo health.reasons.* */
  key: string;
  /** interpolación para la clave (p.ej. { pct } o { port, speed, max }) */
  params?: Record<string, string | number>;
  /** página destino del chip (Page del Shell) */
  page: string;
  /** ancla dentro de la página (scrollIntoView) */
  anchor?: string;
}

export interface HealthScore {
  score: number; // 0–100 entero
  tone: HealthTone;
  /** clave i18n de la etiqueta: health.excellent | good | attention | critical */
  labelKey: string;
  reasons: HealthReason[];
}

export interface HealthInput {
  system?: SystemInfo;
  wan?: WanStatus;
  drift?: DriftProbe;
  mode?: ModeProbe;
  wireless?: WirelessRadio[];
  /** #522: probe de puertos físicos (ethtool) para detectar enlaces que
   *  negocian por debajo de su capacidad (cable dañado o de 4 hilos). */
  physports?: PhysPortsProbe;
}

/**
 * Health score §8.1 de design.md. Función pura (testeable) calculada en el
 * cliente solo con datos ya disponibles en el Shell.
 */
export function computeHealthScore({ system, wan, drift, mode, wireless, physports }: HealthInput): HealthScore {
  let score = 100;
  const reasons: HealthReason[] = [];

  const isRouter = !mode || mode.mode === "router";

  // −40 sin Internet (modo router, WAN presente y caída)
  if (isRouter && wan?.present && !wan.up) {
    score -= 40;
    reasons.push({ key: "noInternet", page: "overview", anchor: "internet" });
  }

  // RAM usada = (total − available) / total
  if (system && system.memory.total > 0) {
    const usedPct = Math.round(((system.memory.total - system.memory.available) / system.memory.total) * 100);
    if (usedPct >= 90) {
      score -= 20;
      reasons.push({ key: "memory", params: { pct: usedPct }, page: "overview", anchor: "recursos" });
    } else if (usedPct >= 75) {
      score -= 10;
      reasons.push({ key: "memory", params: { pct: usedPct }, page: "overview", anchor: "recursos" });
    }
  }

  // Flash libre = root.free / root.total
  if (system && system.root.total > 0) {
    const freePct = Math.round((system.root.free / system.root.total) * 100);
    if (freePct <= 10) {
      score -= 20;
      reasons.push({ key: "flash", params: { pct: freePct }, page: "overview", anchor: "recursos" });
    } else if (freePct <= 20) {
      score -= 10;
      reasons.push({ key: "flash", params: { pct: freePct }, page: "overview", anchor: "recursos" });
    }
  }

  // Carga
  const load1 = system?.load?.[0];
  if (load1 !== undefined) {
    if (load1 >= 4) {
      score -= 10;
      reasons.push({ key: "load", page: "overview", anchor: "recursos" });
    } else if (load1 >= 2) {
      score -= 5;
      reasons.push({ key: "load", page: "overview", anchor: "recursos" });
    }
  }

  // Drift de configuración
  if (drift && drift.changes > 0) {
    score -= 10;
    reasons.push({ key: "drift", page: "overview", anchor: "drift" });
  }

  // Mantenimiento pendiente: updates del sistema NO cuentan (política: NetGrip
  // no gestiona actualizaciones; esa superficie vive en LuCI/CLI).

  // Todas las radios apagadas
  if (mode?.has_wifi && wireless && wireless.length > 0 && wireless.every((r) => !r.up)) {
    score -= 10;
    reasons.push({ key: "wifiOff", page: "wifi" });
  }

  // #522: puerto negociando por debajo de su capacidad (cable dañado o de
  // 4 hilos). Una sola razón con el primer caso; el detalle completo vive
  // en el banner de la página Puertos.
  const degraded = degradedPorts(physports);
  if (degraded.length > 0) {
    score -= 10;
    const d = degraded[0];
    reasons.push({ key: "portSlow", params: { port: d.name, speed: fmtMbps(d.speed), max: fmtMbps(d.max) }, page: "ports" });
  }

  score = Math.max(0, Math.min(100, Math.round(score)));

  const tone: HealthTone = score >= 70 ? "ok" : score >= 50 ? "warn" : "danger";
  const labelKey = score >= 90 ? "health.excellent" : score >= 70 ? "health.good" : score >= 50 ? "health.attention" : "health.critical";

  return { score, tone, labelKey, reasons };
}

/** Hook §8: mismo cálculo, memoizado sobre los datos del Shell. */
export function useHealthScore(input: HealthInput): HealthScore {
  const { system, wan, drift, mode, wireless, physports } = input;
  return useMemo(
    () => computeHealthScore({ system, wan, drift, mode, wireless, physports }),
    [system, wan, drift, mode, wireless, physports],
  );
}
