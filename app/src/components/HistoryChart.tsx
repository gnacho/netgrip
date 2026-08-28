import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { TrendingUp } from "lucide-react";
import { api } from "../api";
import { Card } from "./Card";
import type { HistoryEntry } from "../types";

export function HistoryChart() {
  const { t } = useTranslation();
  const [entries, setEntries] = useState<HistoryEntry[]>([]);

  useEffect(() => {
    api.history().then((r) => setEntries(r.entries ?? [])).catch(() => {});
    const interval = setInterval(() => {
      api.history().then((r) => setEntries(r.entries ?? [])).catch(() => {});
    }, 60000);
    return () => clearInterval(interval);
  }, []);

  if (entries.length < 2) return null;

  const deltas = entries.slice(1).map((e, i) => ({
    ts: e.ts,
    rx: Math.max(0, e.rx - entries[i].rx),
    tx: Math.max(0, e.tx - entries[i].tx),
  }));

  const maxVal = Math.max(...deltas.map((d) => Math.max(d.rx, d.tx)), 1);
  const w = 560;
  const h = 120;
  const padX = 0;
  const padY = 8;
  const stepX = (w - padX * 2) / (deltas.length - 1);

  const toY = (v: number) => h - padY - ((v / maxVal) * (h - padY * 2));

  const rxPoints = deltas.map((d, i) => `${padX + i * stepX},${toY(d.rx)}`).join(" ");
  const txPoints = deltas.map((d, i) => `${padX + i * stepX},${toY(d.tx)}`).join(" ");

  const rxArea = `${padX},${h - padY} ${rxPoints} ${padX + (deltas.length - 1) * stepX},${h - padY}`;
  const txArea = `${padX},${h - padY} ${txPoints} ${padX + (deltas.length - 1) * stepX},${h - padY}`;

  const fmtBytes = (b: number) => {
    if (b > 1048576) return `${(b / 1048576).toFixed(1)} MB`;
    if (b > 1024) return `${(b / 1024).toFixed(0)} KB`;
    return `${b} B`;
  };

  const fmtTime = (ts: number) => {
    const d = new Date(ts * 1000);
    return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  };

  const hours = Math.round((entries[entries.length - 1].ts - entries[0].ts) / 3600);

  return (
    <Card title={t("history.title")} icon={TrendingUp}>
      <svg viewBox={`0 0 ${w} ${h}`} className="w-full" preserveAspectRatio="none">
        <defs>
          <linearGradient id="rxGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="rgb(34 197 94 / 0.3)" />
            <stop offset="100%" stopColor="rgb(34 197 94 / 0.02)" />
          </linearGradient>
          <linearGradient id="txGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="rgb(59 130 246 / 0.3)" />
            <stop offset="100%" stopColor="rgb(59 130 246 / 0.02)" />
          </linearGradient>
        </defs>
        <polygon points={rxArea} fill="url(#rxGrad)" />
        <polygon points={txArea} fill="url(#txGrad)" />
        <polyline points={rxPoints} fill="none" stroke="rgb(34 197 94)" strokeWidth="1.5" />
        <polyline points={txPoints} fill="none" stroke="rgb(59 130 246)" strokeWidth="1.5" />
      </svg>
      <div className="flex items-center justify-between mt-1 text-xs text-muted">
        <span>{fmtTime(entries[0].ts)}</span>
        <div className="flex gap-3">
          <span className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-green-500" />
            RX {fmtBytes(deltas[deltas.length - 1]?.rx ?? 0)}
          </span>
          <span className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-blue-500" />
            TX {fmtBytes(deltas[deltas.length - 1]?.tx ?? 0)}
          </span>
        </div>
        <span>{fmtTime(entries[entries.length - 1].ts)}</span>
      </div>
      <p className="text-xs text-muted mt-1">{t("history.lastHours", { count: hours })}</p>
    </Card>
  );
}
