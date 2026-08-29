import type { ReactNode } from "react";

export type PillTone = "ok" | "warn" | "danger" | "muted" | "accent";

/* Colores del sistema de acentos (design-rev2 §1); "ok" = success. */
const PILL_TONES: Record<PillTone, string> = {
  ok: "bg-success-soft text-success",
  warn: "bg-warn-soft text-warn",
  danger: "bg-danger-soft text-danger",
  muted: "bg-fill text-muted",
  accent: "bg-accent-soft text-accent",
};

const DOT_TONES: Record<PillTone, string> = {
  ok: "bg-success",
  warn: "bg-warn",
  danger: "bg-danger",
  muted: "bg-faint",
  accent: "bg-accent",
};

/** Pill de estado §6.5: caption 600 + punto 6px. `live` = pulse-dot.
 *  `className` permite, p. ej., `max-w-28` para acotar pills con texto largo
 *  (dominios); el texto interno hace ellipsis vía el span envolvente. */
export function Pill({ tone = "muted", live = false, className = "", children }: {
  tone?: PillTone;
  live?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-caption font-semibold whitespace-nowrap ${PILL_TONES[tone]} ${className}`}>
      <span aria-hidden="true" className={`h-1.5 w-1.5 rounded-full shrink-0 ${DOT_TONES[tone]} ${live ? "animate-pulse-dot" : ""}`} />
      <span className="min-w-0 overflow-hidden text-ellipsis">{children}</span>
    </span>
  );
}

/** StatusDot §6.6: semáforo puro de 8px para listas densas. */
export function StatusDot({ tone = "muted", label, live = false }: {
  tone?: "ok" | "warn" | "danger" | "muted";
  label?: string;
  live?: boolean;
}) {
  return (
    <span
      role={label ? "img" : undefined}
      aria-label={label}
      title={label}
      className={`inline-block h-2 w-2 rounded-full shrink-0 ${DOT_TONES[tone]} ${live ? "animate-pulse-dot" : ""}`}
    />
  );
}

/** Badge de conteo para la nav §6.21. */
export function Badge({ tone = "accent", children }: {
  tone?: "accent" | "warn" | "danger";
  children: ReactNode;
}) {
  const tones = {
    accent: "bg-accent-soft text-accent",
    warn: "bg-warn-soft text-warn",
    danger: "bg-danger-soft text-danger",
  } as const;
  return (
    <span className={`inline-flex items-center justify-center min-w-[18px] h-[18px] px-1 rounded-full text-caption font-semibold ${tones[tone]}`}>
      {children}
    </span>
  );
}
