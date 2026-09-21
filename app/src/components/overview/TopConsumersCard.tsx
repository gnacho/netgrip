import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChartColumn } from "lucide-react";
import { api, isDemo } from "../../api";
import type { Client, DPIProbe, NlbwmonTop } from "../../types";
import { Button, Card, EmptyState, MultiSeriesChart, SkeletonChart, type MultiSeries } from "../ui";
import { IlluDevices } from "../ui/illustrations";
import { fmtBytes, fmtDate } from "../../lib/format";

/** Título de tarjeta a una línea (design-rev2 §3): ellipsis + tooltip nativo. */
function oneLine(text: string) {
  return <span className="block truncate" title={text}>{text}</span>;
}

const SERIES_PALETTE = [
  "#22d3ee", "#3b82f6", "#facc15", "#4ade80", "#fb7185", "#fb923c",
  "#a78bfa", "#38bdf8", "#fbbf24", "#f472b6",
];

// Serie de datos de una aplicación a lo largo de la ventana de muestreo.
interface AppSeries {
  name: string;
  bytes: number[];
}

interface Snapshot { ts: number; bytes: Record<string, number>; }

/** Buffer circular que acumula snapshots de DPI a lo largo del tiempo. */
const SAMPLE_WINDOW = 40;

function useAppSeries() {
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);

  useEffect(() => {
    if (isDemo()) return;
    const load = () => api.dpi().then((p) => {
      if (!p?.applicable || !p.protocols?.length) return;
      const bytes: Record<string, number> = {};
      for (const pr of p.protocols) bytes[pr.name] = (bytes[pr.name] ?? 0) + pr.bytes;
      setSnapshots((prev) => [...prev.slice(-SAMPLE_WINDOW + 1), { ts: Math.floor(Date.now() / 1000), bytes }]);
    }).catch(() => {});
    load();
    const id = setInterval(load, 30000);
    return () => clearInterval(id);
  }, []);

  return snapshots;
}

/** En demo, genera series realistas (nombres + picos) sin router. */
function seededAppSeries(): { series: MultiSeries[]; xLabels: string[] } {
  const names = ["YouTube", "QUIC", "Netflix", "WhatsApp", "Zoom", "HTTP", "BitTorrent", "DNS", "MQTT"];
  const n = 24;
  const base = [160, 90, 120, 40, 60, 70, 30, 10, 6];
  const colors = ["#22d3ee", "#3b82f6", "#facc15", "#4ade80", "#fb7185", "#fb923c", "#a78bfa", "#38bdf8", "#fbbf24"];
  const series: MultiSeries[] = names.map((name, i) => ({
    key: name,
    label: name,
    color: colors[i % colors.length],
    points: Array.from({ length: n }, (_, k) => Math.max(0, Math.round(
      base[i] * (0.5 + 0.5 * Math.sin((k / (n - 1) * 2 + i) * Math.PI * 1.3) + 0.25 * (i % 3) * Math.sin(k * 0.7 + i))
    ))),
  }));
  const now = Math.floor(Date.now() / 1000);
  const xLabels = Array.from({ length: n }, (_, k) => fmtDate(now - (n - 1 - k) * 3600));
  return { series, xLabels };
}

/** Quién gasta más (#385): vive en la página Consumo, no en el resumen. */
export function TopConsumersCard({ clients, onNavigate, index = 0 }: { clients?: Client[]; onNavigate: (p: string) => void; index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<DPIProbe>();
  const snapshots = useAppSeries();
  // Accounting per device and per protocol from nlbwmon. It is the only
  // source that covers wired clients and anything behind a switch: the
  // counters in Client come from this router's own wireless stations, so a
  // gateway whose clients all sit behind a switch has none of them.
  const [top, setTop] = useState<NlbwmonTop>();

  useEffect(() => {
    if (!isDemo()) return;
    api.dpi().then(setProbe).catch(() => {});
  }, []);

  useEffect(() => {
    const load = () => api.nlbwmonTop()
      .then((r) => setTop(r?.devices?.length || r?.apps?.length ? r : undefined))
      .catch(() => {});
    load();
    const id = setInterval(load, 60000);
    return () => clearInterval(id);
  }, []);

  const demo = isDemo();
  const seeded = demo ? seededAppSeries() : null;

  // Series finales (demo generadas / reales acumuladas).
  const chart = useMemo<{ series: AppSeries[]; xLabels: string[] } | null>(() => {
    if (seeded) return { series: seeded.series.map((s) => ({ name: s.label, bytes: s.points })), xLabels: seeded.xLabels };
    if (snapshots.length < 2) return null;
    const byName = new Map<string, number[]>();
    const xLabels: string[] = [];
    const recent = snapshots.slice(-SAMPLE_WINDOW);
    for (const snap of recent) {
      const entries = Object.entries(snap.bytes).sort((a, b) => b[1] - a[1]).slice(0, 6);
      for (const [name] of entries) if (!byName.has(name)) byName.set(name, []);
    }
    for (const snap of recent) {
      xLabels.push(fmtDate(snap.ts));
      for (const [name, arr] of byName) arr.push(snap.bytes[name] ?? 0);
    }
    const series = [...byName.entries()]
      .map(([name, bytes]) => ({ name, bytes, total: bytes.reduce((a, b) => a + b, 0) }))
      .sort((a, b) => b.total - a.total)
      .slice(0, 6)
      .map(({ name, bytes }) => ({ name, bytes }));
    return { series, xLabels };
  }, [seeded, snapshots]);

  const totalAll = chart?.series.reduce((acc, s) => acc + s.bytes[s.bytes.length - 1], 0) ?? 0;

  const clientTop = useMemo(() => {
    if (!clients) return null;
    const withBytes = clients.filter((c) => c.rx_bytes + c.tx_bytes > 0);
    if (withBytes.length === 0) return null;
    return [...withBytes].sort((a, b) => b.rx_bytes + b.tx_bytes - (a.rx_bytes + a.tx_bytes)).slice(0, 5);
  }, [clients]);

  /** Devices as nlbwmon accounts them, named from the client list when the
   *  MAC is one we know. Preferred over the wireless counters because it
   *  covers every client, not just the stations of this router. */
  const usageTop = useMemo(() => {
    if (!top?.devices?.length) return null;
    // A client with nothing better carries its own MAC as its name, so a
    // hit here can still be a MAC. Drop those: an address reads better.
    const nameOf = new Map(
      (clients ?? [])
        .filter((c) => c.name && c.name.toLowerCase() !== c.mac.toLowerCase())
        .map((c) => [c.mac.toLowerCase(), c.name]),
    );
    const max = top.devices[0].down_bytes + top.devices[0].up_bytes || 1;
    return top.devices.slice(0, 5).map((d) => ({
      key: d.key,
      // || and not ??: an empty ip must fall through to the MAC, and ??
      // only catches null/undefined, which would leave the row blank.
      label: nameOf.get(d.key.toLowerCase()) || d.ip || d.key,
      bytes: d.down_bytes + d.up_bytes,
      pct: ((d.down_bytes + d.up_bytes) / max) * 100,
    }));
  }, [top, clients]);

  const multiSeries = chart
    ? chart.series.map((s, i) => ({
        key: s.name, label: s.name,
        color: SERIES_PALETTE[i % SERIES_PALETTE.length],
        points: s.bytes,
      }))
    : null;

  /** The device half of the card. nlbwmon covers every client; the
   *  wireless counters only this router's own stations, so they are the
   *  fallback for a router without nlbwmon. */
  const deviceRows = useMemo(() => {
    if (usageTop) return usageTop;
    if (!clientTop) return null;
    const max = clientTop[0].rx_bytes + clientTop[0].tx_bytes || 1;
    return clientTop.map((c) => {
      const bytes = c.rx_bytes + c.tx_bytes;
      return { key: c.mac, label: c.name || c.mac, bytes, pct: (bytes / max) * 100 };
    });
  }, [usageTop, clientTop]);

  const appRows = top?.apps?.length ? top.apps.slice(0, 6) : null;
  const hasAnything = deviceRows || multiSeries || appRows;

  return (
    <Card index={index} className="md:col-span-12"
      title={oneLine(t("overview.topConsumers"))} icon={ChartColumn} iconTone="teal" help="dpi">
      {/* Devices — the half the title promises and the port chart can never
          show. nlbwmon covers wired clients and anything behind a switch;
          the wireless counters only this router's own stations. */}
      {deviceRows && (
        <div className="space-y-2.5">
          {deviceRows.map((d) => (
            <div key={d.key}>
              <div className="flex justify-between text-small mb-1">
                <span className="font-medium truncate" translate="no">{d.label}</span>
                <span className="text-muted" style={{ fontVariantNumeric: "tabular-nums" }}>{fmtBytes(d.bytes)}</span>
              </div>
              <div className="h-1.5 rounded-full bg-surface-2 overflow-hidden">
                <div className="h-full rounded-full bg-accent transition-[width] duration-500" style={{ width: `${d.pct}%` }} />
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Apps — the conntrack series when there is one, otherwise the
          protocols nlbwmon named. Both are kept: the series shows change
          over time, nlbwmon shows the totals with recognisable names. */}
      {multiSeries ? (
        <div className={deviceRows ? "mt-4 border-t border-border/50 pt-3" : ""}>
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 mb-2">
            {multiSeries.map((s) => (
              <span key={s.key} className="inline-flex items-center gap-1.5 text-caption">
                <span className="inline-block h-1.5 w-1.5 rounded-full" style={{ background: s.color }} />
                {s.label}
              </span>
            ))}
          </div>
          <MultiSeriesChart series={multiSeries} xLabels={chart!.xLabels} height={220}
            ariaLabel={t("overview.topConsumers")} />
          <div className="mt-4 grid gap-x-4 gap-y-2 sm:grid-cols-2">
            {chart!.series.map((s, i) => {
              const val = s.bytes[s.bytes.length - 1];
              const pct = totalAll > 0 ? (val / totalAll) * 100 : 0;
              return (
                <div key={s.name} className="flex items-center gap-2 text-small">
                  <span className="inline-block h-2 w-2 rounded-full shrink-0" style={{ background: multiSeries![i].color }} />
                  <span className="flex-1 truncate font-medium">{s.name}</span>
                  <span className="text-muted" style={{ fontVariantNumeric: "tabular-nums" }}>{fmtBytes(val)}</span>
                  <span className="text-faint w-10 text-right" style={{ fontVariantNumeric: "tabular-nums" }}>{pct.toFixed(0)}%</span>
                </div>
              );
            })}
          </div>
        </div>
      ) : appRows ? (
        <div className={deviceRows ? "mt-4 border-t border-border/50 pt-3" : ""}>
          <div className="text-caption text-muted mb-2">{t("overview.byProtocol")}</div>
          <div className="grid gap-x-4 gap-y-1.5 sm:grid-cols-2">
            {appRows.map((a) => (
              <div key={a.key} className="flex items-center gap-2 text-small">
                <span className="flex-1 truncate font-medium">{a.key}</span>
                <span className="text-muted" style={{ fontVariantNumeric: "tabular-nums" }}>
                  {fmtBytes(a.down_bytes + a.up_bytes)}
                </span>
              </div>
            ))}
          </div>
        </div>
      ) : null}

      {usageTop && <p className="mt-3 text-caption text-faint">{t("overview.usagePeriod")}</p>}

      {!hasAnything && (probe || clients ? (
        <EmptyState small
          illustration={<IlluDevices size={120} />}
          title={t("overview.dpiEmpty")}
          body={t("overview.dpiEmptyBody")}
          action={<Button variant="secondary" size="sm" onClick={() => onNavigate("services")}>{t("overview.dpiGoServices")}</Button>}
        />
      ) : (
        <SkeletonChart height={220} />
      ))}
    </Card>
  );
}
