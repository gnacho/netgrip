import { useEffect, useMemo, useRef, useState } from "react";
import { fmtBytes, fmtRate } from "../../lib/format";

const RX = "var(--color-chart-rx)";
const TX = "var(--color-chart-tx)";
const GRID = "var(--color-chart-grid)";

function smoothLine(pts: { x: number; y: number }[]): string {
  if (pts.length < 2) return "";
  let d = `M ${pts[0].x} ${pts[0].y}`;
  for (let i = 1; i < pts.length; i++) {
    const p = pts[i - 1];
    const c = pts[i];
    const cx = (p.x + c.x) / 2;
    d += ` C ${cx} ${p.y}, ${cx} ${c.y}, ${c.x} ${c.y}`;
  }
  return d;
}

function smoothArea(pts: { x: number; y: number }[], baseY: number): string {
  if (pts.length < 2) return "";
  return `${smoothLine(pts)} L ${pts[pts.length - 1].x} ${baseY} L ${pts[0].x} ${baseY} Z`;
}

/**
 * AreaChart §6.9: dos series (RX/TX) con relleno plano al 12 %, rejilla,
 * etiquetas Y auto-unidad y tooltip por índice de muestra. SVG a medida.
 */
export function AreaChart({ rx, tx, height = 200, xLabels, formatY = fmtRate, ariaLabel, live = false }: {
  rx: number[];
  tx?: number[];
  height?: number;
  /** etiquetas del eje X (primera/medio/última visibles) */
  xLabels?: string[];
  formatY?: (v: number) => string;
  ariaLabel?: string;
  live?: boolean;
}) {
  const W = 600;
  const H = 200;
  const PAD_X = 6;
  const PAD_TOP = 14;
  const PAD_BOTTOM = 18;
  const svgRef = useRef<SVGSVGElement>(null);
  const [hover, setHover] = useState<number>();

  const n = rx.length;
  const model = useMemo(() => {
    if (n < 2) return null;
    const max = Math.max(...rx, ...(tx ?? []), 1);
    const stepX = (W - PAD_X * 2) / (n - 1);
    const toPt = (v: number, i: number) => ({
      x: PAD_X + i * stepX,
      y: PAD_TOP + (1 - v / max) * (H - PAD_TOP - PAD_BOTTOM),
    });
    return {
      max,
      stepX,
      rxPts: rx.map(toPt),
      txPts: tx?.map(toPt),
    };
  }, [rx, tx, n]);

  const onPointerMove = (e: React.PointerEvent<SVGSVGElement>) => {
    if (!model || !svgRef.current) return;
    const rect = svgRef.current.getBoundingClientRect();
    const x = ((e.clientX - rect.left) / rect.width) * W;
    const idx = Math.round((x - PAD_X) / model.stepX);
    setHover(Math.max(0, Math.min(n - 1, idx)));
  };

  const baseY = H - PAD_BOTTOM;

  return (
    <div className="relative">
      <svg
        ref={svgRef}
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="none"
        style={{ height }}
        className="w-full touch-none select-none"
        role="img"
        aria-label={ariaLabel}
        onPointerMove={onPointerMove}
        onPointerLeave={() => setHover(undefined)}
      >
        {/* rejilla */}
        {[0.25, 0.5, 0.75, 1].map((f) => {
          const y = PAD_TOP + (1 - f) * (H - PAD_TOP - PAD_BOTTOM);
          return <line key={f} x1={PAD_X} x2={W - PAD_X} y1={y} y2={y} stroke={GRID} strokeWidth="1" />;
        })}
        {model && (
          <g style={live ? { transition: "transform 300ms linear" } : undefined}>
            {model.txPts && (
              <>
                <path d={smoothArea(model.txPts, baseY)} fill={TX} fillOpacity={0.12} />
                <path d={smoothLine(model.txPts)} fill="none" stroke={TX} strokeWidth="2" />
              </>
            )}
            <path d={smoothArea(model.rxPts, baseY)} fill={RX} fillOpacity={0.12} />
            <path d={smoothLine(model.rxPts)} fill="none" stroke={RX} strokeWidth="2" />
            {hover !== undefined && (
              <g>
                <line x1={model.rxPts[hover].x} x2={model.rxPts[hover].x} y1={PAD_TOP} y2={baseY} stroke={GRID} strokeWidth="1.5" />
                <circle cx={model.rxPts[hover].x} cy={model.rxPts[hover].y} r="3.5" fill={RX} stroke="var(--color-surface)" strokeWidth="1.5" />
                {model.txPts && <circle cx={model.txPts[hover].x} cy={model.txPts[hover].y} r="3.5" fill={TX} stroke="var(--color-surface)" strokeWidth="1.5" />}
              </g>
            )}
          </g>
        )}
        {/* etiquetas Y */}
        {model && [0.5, 1].map((f) => (
          <text key={f} x={PAD_X + 2} y={PAD_TOP + (1 - f) * (H - PAD_TOP - PAD_BOTTOM) - 3}
            fontSize="10" fill="var(--color-faint)" style={{ fontVariantNumeric: "tabular-nums" }}>
            {formatY(model.max * f)}
          </text>
        ))}
      </svg>

      {/* tooltip */}
      {model && hover !== undefined && (
        <div
          className="pointer-events-none absolute -top-1 z-10 -translate-x-1/2 rounded-md border border-border bg-surface px-2.5 py-1.5 shadow-elevated text-caption whitespace-nowrap"
          style={{ left: `${((model.rxPts[hover].x) / W) * 100}%` }}
        >
          {xLabels?.[hover] && <div className="text-muted mb-0.5">{xLabels[hover]}</div>}
          <div className="flex items-center gap-1.5" style={{ color: RX }}>↓ {formatY(rx[hover])}</div>
          {tx && <div className="flex items-center gap-1.5" style={{ color: TX }}>↑ {formatY(tx[hover])}</div>}
        </div>
      )}

      {/* etiquetas X */}
      {xLabels && xLabels.length >= 2 && (
        <div className="flex justify-between text-caption text-faint mt-1">
          <span>{xLabels[0]}</span>
          {xLabels.length > 2 && <span>{xLabels[Math.floor(xLabels.length / 2)]}</span>}
          <span>{xLabels[xLabels.length - 1]}</span>
        </div>
      )}
    </div>
  );
}

export interface MultiSeries {
  key: string;
  label: string;
  color: string;
  points: number[];
}

/**
 * MultiSeriesChart: varias series de línea/área sobre un eje temporal
 * compartido. Leyenda superior (chips), rejilla, eje Y auto-unidad, y
 * tooltip de hover con el desglose de todas las series en el punto.
 */
export function MultiSeriesChart({ series, xLabels, height = 220, formatY = fmtBytes, ariaLabel }: {
  series: MultiSeries[];
  xLabels?: string[];
  height?: number;
  formatY?: (v: number) => string;
  ariaLabel?: string;
}) {
  const W = 700;
  const H = 220;
  const PAD_X = 8;
  const PAD_TOP = 16;
  const PAD_BOTTOM = 20;
  const svgRef = useRef<SVGSVGElement>(null);
  const [hover, setHover] = useState<number>();

  const n = series[0]?.points.length ?? 0;
  const model = useMemo(() => {
    if (n < 2 || series.length === 0) return null;
    const max = Math.max(...series.flatMap((s) => s.points), 1);
    const stepX = (W - PAD_X * 2) / (n - 1);
    const toPt = (v: number, i: number) => ({
      x: PAD_X + i * stepX,
      y: PAD_TOP + (1 - v / max) * (H - PAD_TOP - PAD_BOTTOM),
    });
    return { max, stepX, pts: series.map((s) => s.points.map(toPt)) };
  }, [series, n]);

  const onPointerMove = (e: React.PointerEvent<SVGSVGElement>) => {
    if (!model || !svgRef.current) return;
    const rect = svgRef.current.getBoundingClientRect();
    const x = ((e.clientX - rect.left) / rect.width) * W;
    const idx = Math.round((x - PAD_X) / model.stepX);
    setHover(Math.max(0, Math.min(n - 1, idx)));
  };

  const baseY = H - PAD_BOTTOM;

  return (
    <div className="relative">
      <svg
        ref={svgRef}
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="none"
        style={{ height }}
        className="w-full touch-none select-none"
        role="img"
        aria-label={ariaLabel}
        onPointerMove={onPointerMove}
        onPointerLeave={() => setHover(undefined)}
      >
        {[0.25, 0.5, 0.75, 1].map((f) => {
          const y = PAD_TOP + (1 - f) * (H - PAD_TOP - PAD_BOTTOM);
          return <line key={f} x1={PAD_X} x2={W - PAD_X} y1={y} y2={y} stroke={GRID} strokeWidth="1" />;
        })}
        {model && series.map((s, si) => (
          <g key={s.key}>
            <path d={smoothArea(model.pts[si], baseY)} fill={s.color} fillOpacity={0.08} />
            <path d={smoothLine(model.pts[si])} fill="none" stroke={s.color} strokeWidth="2" />
          </g>
        ))}
        {model && hover !== undefined && (
          <g>
            <line x1={model.pts[0][hover].x} x2={model.pts[0][hover].x} y1={PAD_TOP} y2={baseY} stroke={GRID} strokeWidth="1.5" />
            {series.map((s, si) => (
              <circle key={s.key} cx={model.pts[si][hover].x} cy={model.pts[si][hover].y} r="3" fill={s.color} stroke="var(--color-surface)" strokeWidth="1.5" />
            ))}
          </g>
        )}
        {model && [0.5, 1].map((f) => (
          <text key={f} x={PAD_X + 2} y={PAD_TOP + (1 - f) * (H - PAD_TOP - PAD_BOTTOM) - 4}
            fontSize="10" fill="var(--color-faint)" style={{ fontVariantNumeric: "tabular-nums" }}>
            {formatY(model.max * f)}
          </text>
        ))}
      </svg>

      {model && hover !== undefined && (
        <div
          className="pointer-events-none absolute -top-1 z-10 -translate-x-1/2 rounded-md border border-border bg-surface px-3 py-2 shadow-elevated text-caption whitespace-nowrap"
          style={{ left: `${(model.pts[0][hover].x / W) * 100}%` }}
        >
          {xLabels?.[hover] && <div className="text-muted mb-1 font-medium">{xLabels[hover]}</div>}
          <div className="flex flex-col gap-0.5">
            {series.map((s) => (
              <div key={s.key} className="flex items-center gap-1.5">
                <span className="inline-block h-1.5 w-1.5 rounded-full" style={{ background: s.color }} />
                <span className="text-muted">{s.label}</span>
                <span className="ml-auto font-mono" style={{ fontVariantNumeric: "tabular-nums" }}>{formatY(s.points[hover])}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {xLabels && xLabels.length >= 2 && (
        <div className="flex justify-between text-caption text-faint mt-1">
          <span>{xLabels[0]}</span>
          {xLabels.length > 2 && <span>{xLabels[Math.floor(xLabels.length / 2)]}</span>}
          <span>{xLabels[xLabels.length - 1]}</span>
        </div>
      )}
    </div>
  );
}

/** Sparkline §6.10: línea 96×28 sin ejes, draw-in al montar. */
export function Sparkline({ values, stroke = "var(--color-accent)", width = 96, height = 28 }: {
  values: number[];
  stroke?: string;
  width?: number;
  height?: number;
}) {
  const [offset, setOffset] = useState(300);
  useEffect(() => {
    const id = requestAnimationFrame(() => setOffset(0));
    return () => cancelAnimationFrame(id);
  }, []);
  if (values.length < 2) return null;
  const max = Math.max(...values);
  const min = Math.min(...values);
  const span = max - min || 1;
  const pts = values.map((v, i) => ({
    x: 1 + (i / (values.length - 1)) * (width - 2),
    y: 2 + (1 - (v - min) / span) * (height - 4),
  }));
  const d = smoothLine(pts);
  const len = 300; // aprox para draw-in
  return (
    <svg viewBox={`0 0 ${width} ${height}`} width={width} height={height} aria-hidden="true">
      <path d={d} fill="none" stroke={stroke} strokeWidth="1.5" strokeLinecap="round"
        strokeDasharray={len} strokeDashoffset={offset} className="draw-in" />
    </svg>
  );
}
