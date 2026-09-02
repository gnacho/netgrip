import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Activity, ChartColumn, CloudOff, Download, Package, RefreshCw, Upload } from "lucide-react";
import { api } from "../api";
import type { DPIProbe, NetifydApp, NetifydProbe } from "../types";
import { ActionBanner, Button, Card, EmptyState, Pill, Skeleton, Toggle } from "../components/ui";
import { fmtBytes } from "../lib/format";

function oneLine(text: string) {
  return <span className="block truncate" title={text}>{text}</span>;
}

export function DpiPage() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<DPIProbe>();
  const [netifyd, setNetifyd] = useState<NetifydProbe>();
  const [apps, setApps] = useState<NetifydApp[]>();
  const [error, setError] = useState(false);
  const [netifydError, setNetifydError] = useState("");
  const [view, setView] = useState<"category" | "protocol">("category");
  const [busy, setBusy] = useState(false);
  const [banner, setBanner] = useState<{ phase: "applying" | "done" | "failed"; text?: string; detail?: string } | null>(null);

  const loadLegacy = useCallback(async () => {
    try {
      setProbe(await api.dpi());
    } catch {
      setError(true);
    }
  }, []);

  const loadNetifyd = useCallback(async () => {
    try {
      setNetifyd(await api.netifyd());
      setApps((await api.dpiApps()).apps);
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

  // Poll netifyd apps every 3 seconds when enabled and running.
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
  const showApps = netifyd?.installed && netifyd?.enabled && netifyd?.running && apps && apps.length > 0;

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
    if (showApps && apps) return Math.max(1, ...apps.map((a) => a.bytes));
    if (!probe?.protocols.length) return 1;
    if (view === "category") return Math.max(1, ...byCategory.map((c) => c.bytes));
    return Math.max(1, ...probe.protocols.map((p) => p.bytes));
  }, [probe, view, byCategory, apps, showApps]);

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

            {showApps ? (
              <>
                <p className="text-small text-muted mb-4">{t("dpi.appsIntro")}</p>
                <ul className="flex flex-col gap-2.5">
                  {apps.map((a) => (
                    <AppRow
                      key={a.name}
                      app={a}
                      maxBytes={maxBytes}
                    />
                  ))}
                </ul>
              </>
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

function AppRow({ app, maxBytes }: { app: NetifydApp; maxBytes: number }) {
  const { t } = useTranslation();
  const pct = Math.max(2, Math.round((app.bytes / maxBytes) * 100));
  return (
    <li className="flex flex-col gap-1">
      <div className="flex items-center gap-2 justify-between">
        <span className="flex items-center gap-2 text-small font-medium truncate">
          <Activity size={16} className="text-accent shrink-0" />
          {app.name}
        </span>
        <span className="flex items-center gap-2 shrink-0">
          <span className="flex items-center gap-1 text-caption text-muted" title={t("dpi.download")}>
            <Download size={12} /> {fmtBytes(app.other_bytes)}
          </span>
          <span className="flex items-center gap-1 text-caption text-muted" title={t("dpi.upload")}>
            <Upload size={12} /> {fmtBytes(app.local_bytes)}
          </span>
          <span className="font-mono text-small" style={{ fontVariantNumeric: "tabular-nums" }}>{fmtBytes(app.bytes)}</span>
          <span className="text-caption text-muted" style={{ fontVariantNumeric: "tabular-nums" }}>{app.flows.toLocaleString()}</span>
        </span>
      </div>
      <div className="h-1.5 w-full rounded-full bg-surface-2 overflow-hidden">
        <div className="h-full bg-accent rounded-full transition-all" style={{ width: `${pct}%` }} />
      </div>
    </li>
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
