import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Eye } from "lucide-react";
import { api } from "../api";
import type { DPIProbe } from "../types";
import { Card } from "./Card";

const categoryColors: Record<string, string> = {
  web: "#3b82f6",
  dns: "#8b5cf6",
  mail: "#ec4899",
  admin: "#f59e0b",
  file: "#10b981",
  streaming: "#ef4444",
  voip: "#06b6d4",
  chat: "#84cc16",
  iot: "#f97316",
  p2p: "#6366f1",
  database: "#14b8a6",
  other: "#6b7280",
};

function fmtBytes(b: number): string {
  if (b > 1073741824) return `${(b / 1073741824).toFixed(2)} GB`;
  if (b > 1048576) return `${(b / 1048576).toFixed(1)} MB`;
  if (b > 1024) return `${(b / 1024).toFixed(0)} KB`;
  return `${b} B`;
}

export function DPICard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<DPIProbe>();
  const [groupBy, setGroupBy] = useState<"protocol" | "category">("category");

  useEffect(() => {
    const load = () => api.dpi().then(setProbe).catch(() => {});
    load();
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, []);

  if (!probe?.applicable) return null;

  const protocols = probe.protocols ?? [];
  const grouped = groupBy === "category" ? groupByCategory({ ...probe, protocols }) : protocols;
  const maxBytes = Math.max(...grouped.map((p) => p.bytes), 1);
  const topN = grouped.slice(0, 12);

  return (
    <Card title={t("dpi.title")} icon={Eye}>
      <p className="text-xs text-muted mb-2">{t("dpi.intro")}</p>

      <div className="flex items-center justify-between mb-3 text-xs">
        <span className="text-muted">{t("dpi.total")}: {fmtBytes(probe.total_bytes)} ({probe.total_flows} {t("dpi.flows")})</span>
        <div className="flex gap-1">
          <button onClick={() => setGroupBy("category")}
            className={`px-2 py-0.5 rounded ${groupBy === "category" ? "bg-accent text-white" : "bg-bg border border-border text-muted"}`}>
            {t("dpi.byCategory")}
          </button>
          <button onClick={() => setGroupBy("protocol")}
            className={`px-2 py-0.5 rounded ${groupBy === "protocol" ? "bg-accent text-white" : "bg-bg border border-border text-muted"}`}>
            {t("dpi.byProtocol")}
          </button>
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        {topN.map((p) => {
          const pct = probe.total_bytes > 0 ? (p.bytes / probe.total_bytes) * 100 : 0;
          const barWidth = (p.bytes / maxBytes) * 100;
          const color = categoryColors[p.category] || categoryColors.other;
          return (
            <div key={p.name} className="flex items-center gap-2">
              <span className="text-xs w-24 truncate" title={p.name}>{p.name}</span>
              <div className="flex-1 h-4 bg-bg/50 rounded overflow-hidden relative">
                <div className="h-full rounded transition-all duration-500"
                  style={{ width: `${barWidth}%`, backgroundColor: color, opacity: 0.7 }} />
                <span className="absolute inset-0 flex items-center px-2 text-[10px] text-text/80">
                  {fmtBytes(p.bytes)}
                </span>
              </div>
              <span className="text-[10px] text-muted w-10 text-right">{pct.toFixed(1)}%</span>
              <span className="text-[10px] text-muted w-8 text-right">{p.flows}</span>
            </div>
          );
        })}
      </div>

      {topN.length === 0 && <p className="text-sm text-muted">{t("dpi.noData")}</p>}
    </Card>
  );
}

function groupByCategory(probe: DPIProbe): { name: string; bytes: number; flows: number; category: string }[] {
  const map = new Map<string, { bytes: number; flows: number; category: string }>();
  for (const p of probe.protocols) {
    const existing = map.get(p.category);
    if (existing) {
      existing.bytes += p.bytes;
      existing.flows += p.flows;
    } else {
      map.set(p.category, { bytes: p.bytes, flows: p.flows, category: p.category });
    }
  }
  const result = Array.from(map.entries()).map(([name, v]) => ({ name, ...v }));
  result.sort((a, b) => b.bytes - a.bytes);
  return result;
}
