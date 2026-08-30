import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChartColumn, CloudOff, RefreshCw } from "lucide-react";
import { api } from "../api";
import type { DPIProbe } from "../types";
import { Button, Card, EmptyState, Pill, Skeleton } from "../components/ui";
import { fmtBytes } from "../lib/format";

function oneLine(text: string) {
  return <span className="block truncate" title={text}>{text}</span>;
}

export function DpiPage() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<DPIProbe>();
  const [error, setError] = useState(false);
  const [view, setView] = useState<"category" | "protocol">("category");

  const load = useCallback(async () => {
    setError(false);
    try {
      setProbe(await api.dpi());
    } catch {
      setError(true);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const byCategory = useMemo(() => {
    if (!probe?.protocols) return [];
    const map = new Map<string, { bytes: number; flows: number }>();
    for (const p of probe.protocols) {
      const cur = map.get(p.category) ?? { bytes: 0, flows: 0 };
      cur.bytes += p.bytes;
      cur.flows += p.flows;
      map.set(p.category, cur);
    }
    return [...map.entries()]
      .map(([cat, v]) => ({ category: cat, ...v }))
      .sort((a, b) => b.bytes - a.bytes);
  }, [probe]);

  const maxBytes = useMemo(() => {
    if (!probe?.protocols.length) return 1;
    if (view === "category") return Math.max(1, ...byCategory.map((c) => c.bytes));
    return Math.max(1, ...probe.protocols.map((p) => p.bytes));
  }, [probe, view, byCategory]);

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <Card index={0} title={oneLine(t("dpi.title"))} icon={ChartColumn} iconTone="teal">
        {error ? (
          <EmptyState
            small
            title={t("wifi.loadError")}
            illustration={<CloudOff size={24} />}
            action={<Button variant="secondary" size="sm" icon={RefreshCw} onClick={load}>{t("common.retry")}</Button>}
          />
        ) : !probe ? (
          <div className="space-y-3">
            <Skeleton className="h-3 w-64" />
            <Skeleton className="h-24" />
            <Skeleton className="h-24" />
          </div>
        ) : !probe.applicable ? (
          <EmptyState title={t("dpi.noData")} />
        ) : (
          <>
            <p className="text-small text-muted mb-4">{t("dpi.intro")}</p>
            <div className="flex flex-wrap items-center gap-4 mb-4">
              <div>
                <p className="text-caption text-muted">{t("dpi.total")}</p>
                <p className="text-h2 font-semibold" style={{ fontVariantNumeric: "tabular-nums" }}>{fmtBytes(probe.total_bytes)}</p>
              </div>
              <div>
                <p className="text-caption text-muted">{t("dpi.flows")}</p>
                <p className="text-h2 font-semibold" style={{ fontVariantNumeric: "tabular-nums" }}>{probe.total_flows.toLocaleString()}</p>
              </div>
              <div className="ml-auto flex gap-1 rounded-md border border-border/60 p-0.5">
                <button type="button" onClick={() => setView("category")}
                  className={`px-2.5 py-1 text-small rounded-sm ring-focus transition-colors ${view === "category" ? "bg-surface-2 text-text font-medium" : "text-muted hover:text-text"}`}>
                  {t("dpi.byCategory")}
                </button>
                <button type="button" onClick={() => setView("protocol")}
                  className={`px-2.5 py-1 text-small rounded-sm ring-focus transition-colors ${view === "protocol" ? "bg-surface-2 text-text font-medium" : "text-muted hover:text-text"}`}>
                  {t("dpi.byProtocol")}
                </button>
              </div>
            </div>
            {probe.protocols.length === 0 ? (
              <EmptyState small title={t("dpi.noData")} />
            ) : view === "category" ? (
              <ul className="flex flex-col gap-2.5">
                {byCategory.map((c) => (
                  <BarRow
                    key={c.category}
                    label={t(`dpi.category.${c.category}`)}
                    bytes={c.bytes}
                    flows={c.flows}
                    maxBytes={maxBytes}
                    tone="teal"
                  />
                ))}
              </ul>
            ) : (
              <ul className="flex flex-col gap-2.5">
                {probe.protocols.map((p) => (
                  <BarRow
                    key={p.name}
                    label={p.name}
                    bytes={p.bytes}
                    flows={p.flows}
                    maxBytes={maxBytes}
                    tone="accent"
                    extra={<Pill tone="muted">{t(`dpi.category.${p.category}`)}</Pill>}
                  />
                ))}
              </ul>
            )}
          </>
        )}
      </Card>
    </div>
  );
}

function BarRow({ label, bytes, flows, maxBytes, tone, extra }: {
  label: string;
  bytes: number;
  flows: number;
  maxBytes: number;
  tone: "teal" | "accent";
  extra?: React.ReactNode;
}) {
  const pct = Math.max(2, Math.round((bytes / maxBytes) * 100));
  const bg = tone === "teal" ? "bg-teal" : "bg-accent";
  return (
    <li className="flex flex-col gap-1">
      <div className="flex items-center gap-2 justify-between">
        <span className="text-small font-medium truncate">{label}</span>
        <span className="flex items-center gap-2 shrink-0">
          {extra}
          <span className="font-mono text-small" style={{ fontVariantNumeric: "tabular-nums" }}>{fmtBytes(bytes)}</span>
          <span className="text-caption text-muted" style={{ fontVariantNumeric: "tabular-nums" }}>{flows.toLocaleString()}</span>
        </span>
      </div>
      <div className="h-1.5 w-full rounded-full bg-surface-2 overflow-hidden">
        <div className={`h-full ${bg} rounded-full transition-all`} style={{ width: `${pct}%` }} />
      </div>
    </li>
  );
}
