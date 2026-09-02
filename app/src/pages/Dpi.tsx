import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Activity, ChartColumn, CloudOff, Package, RefreshCw, Search } from "lucide-react";
import { api } from "../api";
import type { DPIProbe, NetifydApp, NetifydProbe, NetifydTimeline } from "../types";
import { ActionBanner, Button, Card, EmptyState, Pill, Skeleton, Toggle } from "../components/ui";
import { Input } from "../components/ui/Field";
import { MultiSeriesChart, type MultiSeries } from "../components/ui/charts";
import { fmtBytes } from "../lib/format";

function oneLine(text: string) {
  return <span className="block truncate" title={text}>{text}</span>;
}

export function DpiPage() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<DPIProbe>();
  const [netifyd, setNetifyd] = useState<NetifydProbe>();
  const [apps, setApps] = useState<NetifydApp[]>();
  const [timeline, setTimeline] = useState<NetifydTimeline>();
  const [error, setError] = useState(false);
  const [netifydError, setNetifydError] = useState("");
  const [view, setView] = useState<"category" | "protocol">("category");
  const [busy, setBusy] = useState(false);
  const [banner, setBanner] = useState<{ phase: "applying" | "done" | "failed"; text?: string; detail?: string } | null>(null);
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<{ key: "name" | "local_bytes" | "other_bytes" | "bytes"; dir: "asc" | "desc" }>({ key: "bytes", dir: "desc" });

  const loadLegacy = useCallback(async () => {
    try {
      setProbe(await api.dpi());
    } catch {
      setError(true);
    }
  }, []);

  const loadNetifyd = useCallback(async () => {
    try {
      const [state, appsRes, timelineRes] = await Promise.all([
        api.netifyd(),
        api.dpiApps(),
        api.dpiTimeline(),
      ]);
      setNetifyd(state);
      setApps(appsRes.apps);
      setTimeline(timelineRes);
      setNetifydError("");
    } catch (e) {
      setNetifydError((e as Error).message);
    }
  }, []);

  const load = useCallback(async () => {
    setError(false);
    await Promise.all([loadLegacy(), loadNetifyd()]);
  }, [loadLegacy, loadNetifyd]);

  useEffect(() => { load(); }, [load]);

  useEffect(() => {
    if (!netifyd?.enabled || !netifyd?.running) return;
    const id = setInterval(() => { loadNetifyd().catch(() => {}); }, 3000);
    return () => clearInterval(id);
  }, [netifyd?.enabled, netifyd?.running, loadNetifyd]);

  const handleToggle = async (enabled: boolean) => {
    setBusy(true);
    setBanner({ phase: "applying" });
    try {
      const res = await api.setNetifyd(enabled);
      setNetifyd(res.state);
      if (res.rolled_back || res.status === "failed") {
        setBanner({ phase: "failed", text: t("dpi.applyFailed"), detail: res.error });
      } else {
        setBanner({ phase: "done", text: enabled ? t("dpi.enabled") : t("dpi.disabled") });
      }
    } catch (e) {
      setBanner({ phase: "failed", text: t("dpi.applyFailed"), detail: (e as Error).message });
    } finally {
      setBusy(false);
      load();
    }
  };

  const installHint = !netifyd?.installed && !netifyd?.low_end;

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

  const chartData = useMemo<MultiSeries[]>(() => {
    if (!timeline?.buckets.length || !timeline.top.length) return [];
    const palette = [
      "var(--color-chart-rx)",
      "var(--color-chart-tx)",
      "var(--color-teal)",
      "var(--color-accent)",
      "#f59e0b", "#ef4444", "#8b5cf6", "#ec4899", "#10b981", "#6366f1",
    ];
    const appNames = timeline.top.map((a) => a.name);
    return appNames.map((name, idx) => ({
      key: name,
      label: name,
      color: palette[idx % palette.length],
      points: timeline.buckets.map((b) => b.apps[name]?.total ?? 0),
    }));
  }, [timeline]);

  const xLabels = useMemo(() => {
    if (!timeline?.buckets.length) return [];
    return timeline.buckets.map((b) => {
      const d = new Date(b.time);
      return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    });
  }, [timeline]);

  const filteredApps = useMemo(() => {
    const list = timeline?.top ?? apps ?? [];
    const term = search.trim().toLowerCase();
    let rows = term ? list.filter((a) => a.name.toLowerCase().includes(term)) : [...list];
    rows.sort((a, b) => {
      const va = a[sort.key];
      const vb = b[sort.key];
      if (typeof va === "string" && typeof vb === "string") {
        return sort.dir === "asc" ? va.localeCompare(vb) : vb.localeCompare(va);
      }
      return sort.dir === "asc" ? (va as number) - (vb as number) : (vb as number) - (va as number);
    });
    return rows;
  }, [timeline, apps, search, sort]);

  const maxBytes = useMemo(() => {
    if (timeline?.top.length) return Math.max(1, ...timeline.top.map((a) => a.bytes));
    if (byCategory.length) return Math.max(1, ...byCategory.map((c) => c.bytes));
    if (probe?.protocols.length) return Math.max(1, ...probe.protocols.map((p) => p.bytes));
    return 1;
  }, [timeline, byCategory, probe]);

  const showNetifyd = netifyd?.installed && netifyd?.enabled && netifyd?.running;

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {banner && (
        <ActionBanner
          phase={banner.phase}
          text={banner.text}
          detail={banner.detail}
          onDone={() => setBanner(null)}
        />
      )}

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
        ) : (
          <>
            <div className="flex flex-wrap items-center gap-4 mb-4">
              <div>
                <p className="text-caption text-muted">{t("dpi.total")}</p>
                <p className="text-h2 font-semibold" style={{ fontVariantNumeric: "tabular-nums" }}>{fmtBytes(probe.total_bytes)}</p>
              </div>
              <div>
                <p className="text-caption text-muted">{t("dpi.flows")}</p>
                <p className="text-h2 font-semibold" style={{ fontVariantNumeric: "tabular-nums" }}>{probe.total_flows.toLocaleString()}</p>
              </div>
              <div className="ml-auto flex items-center gap-4">
                <div className="flex items-center gap-2">
                  <span className="text-small text-muted">{t("dpi.netifyd")}</span>
                  <Toggle
                    checked={!!netifyd?.enabled}
                    disabled={busy || netifyd?.low_end || !netifyd?.installed || installHint}
                    busy={busy}
                    onChange={handleToggle}
                    label={t("dpi.netifyd")}
                  />
                </div>
                <div className="flex gap-1 rounded-md border border-border/60 p-0.5">
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
            </div>

            {netifyd?.low_end && (
              <p className="text-small text-muted mb-4">{t("dpi.lowEnd")}</p>
            )}

            {installHint && (
              <div className="rounded-md bg-surface-2 border border-border p-4 mb-4 flex flex-col gap-2">
                <div className="flex items-center gap-2">
                  <Package size={18} className="text-muted" />
                  <p className="text-small font-medium">{t("dpi.notInstalled")}</p>
                </div>
                <p className="text-caption text-muted">{t("dpi.installHint")}</p>
              </div>
            )}

            {netifydError && (
              <p className="text-small text-danger mb-4">{netifydError}</p>
            )}

            {showNetifyd && timeline ? (
              <DpiNetifydView
                timeline={timeline}
                chartData={chartData}
                xLabels={xLabels}
                filteredApps={filteredApps}
                search={search}
                onSearch={setSearch}
                sort={sort}
                onSort={setSort}
              />
            ) : !probe.applicable ? (
              <EmptyState title={t("dpi.noData")} />
            ) : probe.protocols.length === 0 ? (
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

function DpiNetifydView({
  timeline,
  chartData,
  xLabels,
  filteredApps,
  search,
  onSearch,
  sort,
  onSort,
}: {
  timeline: NetifydTimeline;
  chartData: MultiSeries[];
  xLabels: string[];
  filteredApps: NetifydApp[];
  search: string;
  onSearch: (v: string) => void;
  sort: { key: "name" | "local_bytes" | "other_bytes" | "bytes"; dir: "asc" | "desc" };
  onSort: (s: { key: "name" | "local_bytes" | "other_bytes" | "bytes"; dir: "asc" | "desc" }) => void;
}) {
  const { t } = useTranslation();
  const total = timeline.totals.total || 1;
  const totalLocal = timeline.totals.local || 1;
  const totalOther = timeline.totals.other || 1;

  const sortHeader = (key: typeof sort.key) => {
    const nextDir = sort.key === key && sort.dir === "desc" ? "asc" : "desc";
    return (
      <button
        type="button"
        onClick={() => onSort({ key, dir: nextDir })}
        className="flex items-center gap-1 text-caption font-medium text-muted hover:text-text uppercase tracking-wide"
      >
        {t(`dpi.${key === "local_bytes" ? "download" : key === "other_bytes" ? "upload" : key === "bytes" ? "colTotal" : "colApp"}`)}
        {sort.key === key && <span className="text-[10px]">{sort.dir === "desc" ? "▼" : "▲"}</span>}
      </button>
    );
  };

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <Card index={1} title={oneLine(t("dpi.timeline"))} icon={Activity} iconTone="accent">
        {chartData.length > 1 ? (
          <MultiSeriesChart series={chartData} xLabels={xLabels} height={260} ariaLabel={t("dpi.timeline")} />
        ) : (
          <EmptyState small title={t("dpi.noData")} />
        )}
      </Card>

      <Card index={2} title={oneLine(t("dpi.tableTitle"))}>
        <div className="mb-4">
          <Input
            icon={Search}
            value={search}
            onChange={(e) => onSearch(e.target.value)}
            placeholder={t("dpi.searchPlaceholder")}
          />
        </div>

        <div className="overflow-x-auto">
          <table className="w-full border-collapse">
            <thead>
              <tr className="border-b border-border/60 text-left">
                <th className="pb-2 pr-4">{sortHeader("name")}</th>
                <th className="pb-2 pr-4 text-right">{sortHeader("other_bytes")}</th>
                <th className="pb-2 pr-4 text-right">{sortHeader("local_bytes")}</th>
                <th className="pb-2 pr-4 text-right">{sortHeader("bytes")}</th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-b border-border/40 bg-surface-2/40 font-medium">
                <td className="py-3 pr-4">
                  <span className="flex items-center gap-2 text-small">
                    <Activity size={16} className="text-accent shrink-0" />
                    {t("dpi.totalTraffic")}
                  </span>
                </td>
                <td className="py-3 pr-4 text-right font-mono text-small" style={{ fontVariantNumeric: "tabular-nums" }}>
                  {fmtBytes(timeline.totals.other)}<br />
                  <span className="text-caption text-muted">100 %</span>
                </td>
                <td className="py-3 pr-4 text-right font-mono text-small" style={{ fontVariantNumeric: "tabular-nums" }}>
                  {fmtBytes(timeline.totals.local)}<br />
                  <span className="text-caption text-muted">100 %</span>
                </td>
                <td className="py-3 pr-4 text-right font-mono text-small" style={{ fontVariantNumeric: "tabular-nums" }}>
                  {fmtBytes(timeline.totals.total)}<br />
                  <span className="text-caption text-muted">100 %</span>
                </td>
              </tr>
              {filteredApps.map((app) => {
                const pctTotal = Math.min(100, Math.round((app.bytes / total) * 1000) / 10);
                const pctLocal = Math.min(100, Math.round((app.local_bytes / totalLocal) * 1000) / 10);
                const pctOther = Math.min(100, Math.round((app.other_bytes / totalOther) * 1000) / 10);
                return (
                  <tr key={app.name} className="border-b border-border/40 last:border-0">
                    <td className="py-3 pr-4">
                      <span className="flex items-center gap-2 text-small">
                        <span className="inline-flex h-6 w-6 items-center justify-center rounded bg-surface-2 text-caption font-medium shrink-0">
                          {app.name.slice(0, 2).toUpperCase()}
                        </span>
                        {app.name}
                      </span>
                    </td>
                    <td className="py-3 pr-4 text-right font-mono text-small" style={{ fontVariantNumeric: "tabular-nums" }}>
                      {fmtBytes(app.local_bytes)}<br />
                      <span className="text-caption text-muted">{pctLocal.toFixed(1)} %</span>
                    </td>
                    <td className="py-3 pr-4 text-right font-mono text-small" style={{ fontVariantNumeric: "tabular-nums" }}>
                      {fmtBytes(app.other_bytes)}<br />
                      <span className="text-caption text-muted">{pctOther.toFixed(1)} %</span>
                    </td>
                    <td className="py-3 pr-4 text-right font-mono text-small" style={{ fontVariantNumeric: "tabular-nums" }}>
                      {fmtBytes(app.bytes)}<br />
                      <span className="text-caption text-muted">{pctTotal.toFixed(1)} %</span>
                    </td>
                  </tr>
                );
              })}
              {filteredApps.length === 0 && (
                <tr>
                  <td colSpan={4} className="py-6 text-center text-small text-muted">{t("dpi.noMatches")}</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
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
