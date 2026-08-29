import type { LucideIcon } from "lucide-react";

/**
 * Tonos del sistema de acentos (design-rev2 §1): color pleno sobre fondo soft
 * del mismo acento. `ok` se mantiene como alias de `success` por compat.
 */
export type Tone = "accent" | "teal" | "success" | "warn" | "danger" | "violet" | "muted" | "ok";

export const TILE_TONES: Record<Tone, string> = {
  accent: "bg-accent-soft text-accent",
  teal: "bg-teal-soft text-teal",
  success: "bg-success-soft text-success",
  warn: "bg-warn-soft text-warn",
  danger: "bg-danger-soft text-danger",
  violet: "bg-violet-soft text-violet",
  muted: "bg-fill text-muted",
  ok: "bg-success-soft text-success",
};

/** Cuadrado 36×36, r-md, fondo *-soft e icono 18px semántico (§6.2). */
export function IconTile({ icon: Icon, tone = "accent", size = 36 }: {
  icon: LucideIcon;
  tone?: Tone;
  size?: number;
}) {
  const iconSize = size >= 36 ? 18 : 16;
  return (
    <span
      aria-hidden="true"
      className={`inline-flex shrink-0 items-center justify-center rounded-md ${TILE_TONES[tone]}`}
      style={{ width: size, height: size }}
    >
      <Icon size={iconSize} />
    </span>
  );
}
