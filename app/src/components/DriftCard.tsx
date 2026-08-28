import { useState } from "react";
import { useTranslation } from "react-i18next";
import { GitCompare, Plus, Minus, Camera, ChevronDown, ChevronRight } from "lucide-react";
import { api } from "../api";
import type { DriftProbe } from "../types";
import { Card } from "./Card";

export function DriftCard({ drift, onChange }: { drift?: DriftProbe; onChange: (d: DriftProbe) => void }) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const takeBaseline = async () => {
    setLoading(true);
    try {
      await api.createSnapshot();
      const d = await api.drift();
      onChange(d);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  };

  const formatDate = (ts: number) => {
    const d = new Date(ts * 1000);
    return d.toLocaleDateString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
  };

  if (!drift) return null;

  if (!drift.has_baseline) {
    return (
      <Card icon={GitCompare} title={t("drift.title")}>
        <p className="text-sm text-muted mb-3">{t("drift.noBaseline")}</p>
        <button onClick={takeBaseline} disabled={loading}
          className="flex items-center gap-2 text-sm bg-accent/15 text-accent px-3 py-1.5 rounded-lg hover:bg-accent/25 disabled:opacity-50 transition-colors">
          <Camera size={14} /> {loading ? t("drift.taking") : t("drift.takeBaseline")}
        </button>
      </Card>
    );
  }

  if (drift.changes === 0) {
    return (
      <Card icon={GitCompare} title={t("drift.title")}>
        <p className="text-sm text-green-400">{t("drift.clean")}</p>
        <p className="text-xs text-muted mt-1">{t("drift.sinceDate", { date: formatDate(drift.snapshot_ts) })}</p>
      </Card>
    );
  }

  return (
    <Card icon={GitCompare} title={t("drift.title")}>
      <div className="mb-2">
        <span className="text-sm font-medium text-amber-400">
          {t("drift.changesCount", { count: drift.changes })}
        </span>
        <span className="text-xs text-muted ml-2">{t("drift.sinceDate", { date: formatDate(drift.snapshot_ts) })}</span>
      </div>
      <div className="space-y-1">
        {drift.configs.map((cfg) => (
          <div key={cfg.config} className="border border-border rounded-lg">
            <button
              onClick={() => setExpanded(expanded === cfg.config ? null : cfg.config)}
              className="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-bg/50 transition-colors"
            >
              {expanded === cfg.config ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
              <span className="font-medium">{cfg.config}</span>
              <span className="text-xs text-muted ml-auto">
                {cfg.lines.filter((l) => l.kind === "added").length} <Plus size={10} className="inline text-green-400" />
                {" "}
                {cfg.lines.filter((l) => l.kind === "removed").length} <Minus size={10} className="inline text-red-400" />
              </span>
            </button>
            {expanded === cfg.config && (
              <div className="px-3 pb-2 text-xs font-mono max-h-48 overflow-y-auto">
                {cfg.lines.map((line, i) => (
                  <div key={i} className={line.kind === "added" ? "text-green-400" : "text-red-400"}>
                    {line.kind === "added" ? "+" : "-"} {line.text}
                  </div>
                ))}
              </div>
            )}
          </div>
        ))}
      </div>
      <button onClick={takeBaseline} disabled={loading}
        className="mt-3 flex items-center gap-2 text-sm text-muted hover:text-text transition-colors">
        <Camera size={14} /> {loading ? t("drift.taking") : t("drift.updateBaseline")}
      </button>
    </Card>
  );
}
