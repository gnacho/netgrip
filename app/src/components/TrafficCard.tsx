import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Activity } from "lucide-react";
import { api } from "../api";
import type { IfaceCounters } from "../types";
import { Card } from "./Card";

// Live traffic chart, GL.iNet-portal style (two smooth series filling the
// card) with NetPulse-style area gradients. Hand-rolled SVG, no chart lib.
type Sample = { ts: number; rates: Record<string, { rx: number; tx: number }> };

const MAX_SAMPLES = 40; // ~2 min at 3s poll
const VB_W = 600;
const VB_H = 160;
const PAD_X = 4;
const PAD_Y = 10;

function fmtRate(bytesPerSec: number): string {
  if (bytesPerSec > 1048576) return `${(bytesPerSec / 1048576).toFixed(1)} MB/s`;
  if (bytesPerSec > 1024) return `${(bytesPerSec / 1024).toFixed(0)} KB/s`;
  return `${Math.round(bytesPerSec)} B/s`;
}

function areaPath(points: { x: number; y: number }[], baseY: number): string {
  if (points.length < 2) return "";
  let d = `M ${points[0].x} ${baseY} L ${points[0].x} ${points[0].y}`;
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1];
    const curr = points[i];
    const cx = (prev.x + curr.x) / 2;
    d += ` C ${cx} ${prev.y}, ${cx} ${curr.y}, ${curr.x} ${curr.y}`;
  }
  d += ` L ${points[points.length - 1].x} ${baseY} Z`;
  return d;
}

function linePath(points: { x: number; y: number }[]): string {
  if (points.length < 2) return "";
  let d = `M ${points[0].x} ${points[0].y}`;
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1];
    const curr = points[i];
    const cx = (prev.x + curr.x) / 2;
    d += ` C ${cx} ${prev.y}, ${cx} ${curr.y}, ${curr.x} ${curr.y}`;
  }
  return d;
}

export function TrafficCard() {
  const { t } = useTranslation();
  const [samples, setSamples] = useState<Sample[]>([]);
  const [selected, setSelected] = useState<string>();
  const prev = useRef<{ counters: IfaceCounters[]; ts: number } | undefined>(undefined);

  useEffect(() => {
    let cancelled = false;
    const poll = async () => {
      try {
        const next = await api.netdev();
        if (cancelled) return;
        const before = prev.current;
        if (before) {
          const dt = (next.ts - before.ts) / 1000;
          const rates: Sample["rates"] = {};
          for (const c of next.counters) {
            const old = before.counters.find((o) => o.name === c.name);
            if (old && dt > 0) {
              rates[c.name] = {
                rx: Math.max(0, (c.rx_bytes - old.rx_bytes) / dt),
                tx: Math.max(0, (c.tx_bytes - old.tx_bytes) / dt),
              };
            }
          }
          setSamples((s) => [...s.slice(-MAX_SAMPLES + 1), { ts: next.ts, rates }]);
        }
        prev.current = next;
      } catch { /* keep previous */ }
    };
    poll();
    const id = setInterval(poll, 3000);
    return () => { cancelled = true; clearInterval(id); };
  }, []);

  const ifaces = useMemo(() => {
    const last = samples[samples.length - 1];
    return last ? Object.keys(last.rates) : [];
  }, [samples]);

  const active = selected && ifaces.includes(selected) ? selected
    : ifaces.includes("br-lan") ? "br-lan" : ifaces[0];

  const series = useMemo(() => {
    if (!active || samples.length < 2) return null;
    const rxPts = samples.map((s) => s.rates[active]?.rx ?? 0);
    const txPts = samples.map((s) => s.rates[active]?.tx ?? 0);
    const max = Math.max(...rxPts, ...txPts, 1024);
    const stepX = (VB_W - PAD_X * 2) / (samples.length - 1);
    const toPoint = (v: number, i: number) => ({
      x: PAD_X + i * stepX,
      y: VB_H - PAD_Y - (v / max) * (VB_H - PAD_Y * 2),
    });
    return {
      rx: rxPts.map(toPoint),
      tx: txPts.map(toPoint),
      rxNow: rxPts[rxPts.length - 1],
      txNow: txPts[txPts.length - 1],
      max,
    };
  }, [samples, active]);

  const baseY = VB_H - PAD_Y;

  return (
    <Card title={t("traffic.title")} icon={Activity}>
      <div className="flex flex-wrap gap-1 mb-2">
        {ifaces.map((name) => (
          <button key={name} onClick={() => setSelected(name)}
            className={`text-xs font-mono px-2 py-0.5 rounded-full transition-colors
              ${name === active ? "bg-accent/20 text-accent" : "bg-border/50 text-muted hover:text-text"}`}>
            {name}
          </button>
        ))}
      </div>

      {!series ? (
        <p className="text-sm text-muted">{t("traffic.measuring")}</p>
      ) : (
        <>
          <div className="flex gap-4 text-sm mb-1">
            <span className="text-ok">↓ {fmtRate(series.rxNow)}</span>
            <span className="text-accent">↑ {fmtRate(series.txNow)}</span>
            <span className="text-xs text-muted ml-auto self-center">{active}</span>
          </div>
          <svg viewBox={`0 0 ${VB_W} ${VB_H}`} className="w-full">
            <defs>
              <linearGradient id="gradRx" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#34d399" stopOpacity="0.45" />
                <stop offset="100%" stopColor="#34d399" stopOpacity="0.02" />
              </linearGradient>
              <linearGradient id="gradTx" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#2563eb" stopOpacity="0.45" />
                <stop offset="100%" stopColor="#2563eb" stopOpacity="0.02" />
              </linearGradient>
            </defs>
            {[0.25, 0.5, 0.75].map((f) => (
              <line key={f} x1={PAD_X} x2={VB_W - PAD_X}
                y1={PAD_Y + (VB_H - PAD_Y * 2) * f} y2={PAD_Y + (VB_H - PAD_Y * 2) * f}
                className="stroke-border/40" strokeDasharray="2 4" />
            ))}
            <path d={areaPath(series.rx, baseY)} fill="url(#gradRx)" />
            <path d={areaPath(series.tx, baseY)} fill="url(#gradTx)" />
            <path d={linePath(series.rx)} fill="none" stroke="#34d399" strokeWidth="1.8" />
            <path d={linePath(series.tx)} fill="none" stroke="#2563eb" strokeWidth="1.8" />
          </svg>
        </>
      )}
    </Card>
  );
}
