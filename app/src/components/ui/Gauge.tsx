import { useEffect, useState } from "react";

type GaugeMode = "consumption" | "health";
type GaugeTone = "accent" | "ok" | "warn" | "danger";

function toneFor(value: number, mode: GaugeMode): GaugeTone {
  if (mode === "health") {
    if (value >= 90) return "ok";
    if (value >= 70) return "warn";
    return "danger";
  }
  // consumo (RAM/flash): umbrales §6.8
  if (value >= 90) return "danger";
  if (value >= 70) return "warn";
  return "accent";
}

const STROKES: Record<GaugeTone, string> = {
  accent: "var(--color-accent)",
  ok: "var(--color-ok)",
  warn: "var(--color-warn)",
  danger: "var(--color-danger)",
};

const R = 50;
const C = 2 * Math.PI * R; // ≈ 314.16

/**
 * Gauge circular SVG §6.8. 0–100, pista completa + arco con draw-in.
 * `tone` fuerza el color (p.ej. % libre, donde el umbral es inverso).
 */
export function Gauge({ value, size = "lg", mode = "consumption", tone, label, sub, ariaLabel }: {
  value: number; // 0–100
  size?: "lg" | "sm";
  mode?: GaugeMode;
  tone?: GaugeTone;
  /** texto central bajo el valor */
  label?: string;
  /** línea extra bajo label (p.ej. "/100") */
  sub?: string;
  ariaLabel?: string;
}) {
  const v = Math.max(0, Math.min(100, value));
  const t = tone ?? toneFor(v, mode);
  const px = size === "lg" ? 120 : 72;
  // draw-in al montar: arranca vacío y transiciona al valor
  const [offset, setOffset] = useState(C);
  useEffect(() => {
    const id = requestAnimationFrame(() => setOffset(C * (1 - v / 100)));
    return () => cancelAnimationFrame(id);
  }, [v]);

  return (
    <div role="img" aria-label={ariaLabel ?? `${Math.round(v)}`} className="relative inline-flex items-center justify-center shrink-0" style={{ width: px, height: px }}>
      <svg viewBox="0 0 120 120" width={px} height={px}>
        <circle cx="60" cy="60" r={R} fill="none" stroke="var(--color-border)" strokeWidth="10" strokeLinecap="round" />
        <circle
          cx="60" cy="60" r={R} fill="none"
          stroke={STROKES[t]}
          strokeWidth="10" strokeLinecap="round"
          strokeDasharray={C}
          strokeDashoffset={offset}
          transform="rotate(-90 60 60)"
          className="draw-in-stroke"
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className={size === "lg" ? "stat-xl" : "stat-md"}>{Math.round(v)}</span>
        {(label || sub) && (
          <span className="text-caption text-muted leading-tight text-center px-1">
            {label}{sub ? ` ${sub}` : ""}
          </span>
        )}
      </div>
    </div>
  );
}
