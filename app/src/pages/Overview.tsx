import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  Activity, ArrowDown, ArrowUp, Cable, ChartColumn, CloudOff, Globe, HardDrive,
  History, MemoryStick, ShieldCheck, Smartphone,
} from "lucide-react";
import { api, isDemo } from "../api";
import type {
  Board, Client, DPIProbe, DriftProbe, EthPort, HistoryEntry,
  IfaceCounters, ModeProbe, SystemInfo, WanStatus,
} from "../types";
import type { HealthScore } from "../hooks/useHealthScore";
import {
  AreaChart, Banner, Button, Card, EmptyState, Gauge,
  KeyValue, Modal, MultiSeriesChart, Pill, SegmentedControl, SkeletonChart, SkeletonRows, type MultiSeries,
} from "../components/ui";
import { IlluDevices, IlluPlug } from "../components/ui/illustrations";
import { fmtBytes, fmtDate, fmtMB, fmtRate, fmtTime, fmtUptime } from "../lib/format";

/** Título de tarjeta a una línea (design-rev2 §3): ellipsis + tooltip nativo. */
function oneLine(text: string) {
  return <span className="block truncate" title={text}>{text}</span>;
}

/* ══════════════ Fila 0 — Salud del router (héroe) §2 ══════════════ */

function HealthHero({ health, board, system, onNavigate }: {
  health: HealthScore;
  board?: Board;
  system?: SystemInfo;
  onNavigate: (page: string, anchor?: string) => void;
}) {
  const { t } = useTranslation();
  const go = (page: string, anchor?: string) => {
    if (page === "overview" && anchor) {
      document.getElementById(anchor)?.scrollIntoView({ behavior: "smooth", block: "start" });
    } else {
      onNavigate(page, anchor);
    }
  };
  return (
    <Card index={0} id="salud" className="md:col-span-12 order-1 md:order-none">
      <div className="flex flex-col md:flex-row md:items-center gap-5">
        <div className="flex items-center gap-5 flex-1 min-w-0">
          <Gauge value={health.score} size="lg" mode="health" ariaLabel={`${t("health.title")}: ${health.score}`} />
          <div className="min-w-0">
            <p className="text-eyebrow text-faint mb-1">{t("health.title")}</p>
            <Pill tone={health.tone}>{t(health.labelKey)}</Pill>
            <p className="text-body mt-2">
              {health.reasons.length === 0 ? t("health.allGood") : t("health.issues", { count: health.reasons.length })}
            </p>
            {health.reasons.length > 0 && (
              <div className="mt-2.5 flex flex-wrap gap-2">
                {health.reasons.slice(0, 3).map((r, i) => (
                  <button
                    key={r.key}
                    type="button"
                    onClick={() => go(r.page, r.anchor)}
                    style={{ "--i": i + 2 } as React.CSSProperties}
                    className="animate-fade-up inline-flex items-center gap-1.5 rounded-full border border-border bg-surface-2 px-3 py-1 text-small hover:border-accent hover:text-accent transition-colors ring-focus"
                  >
                    {t(`health.reasons.${r.key}`, r.params)}
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
        {/* micro-stats del sistema */}
        <KeyValue
          className="md:w-64 shrink-0"
          items={[
            { label: t("overview.model"), value: board?.model ?? "…" },
            { label: t("overview.firmware"), value: board?.release ? `${board.release.distribution} ${board.release.version}` : "…" },
            { label: t("system.uptime"), value: system ? fmtUptime(t, system.uptime) : "…" },
          ]}
        />
      </div>
    </Card>
  );
}

/* ══════════════ Fila 1 — Internet / Memoria / Espacio §3 ══════════════ */

function InternetCard({ wan, mode }: { wan?: WanStatus; mode?: ModeProbe }) {
  const { t } = useTranslation();
  const ap = mode?.mode === "ap" || !wan?.present;
  return (
    <Card index={1} id="internet" className="md:col-span-4 order-2 md:order-none"
      title={oneLine(t("overview.internet"))} icon={Globe} help="internet"
      action={wan && (
        ap
          ? <Pill tone="muted">{t("overview.apMode")}</Pill>
          : <Pill tone={wan.up ? "ok" : "danger"}>{wan.up ? t("overview.connected") : t("overview.down")}</Pill>
      )}>
      {!wan ? <SkeletonRows rows={2} /> : ap ? (
        <p className="text-small text-muted">{t("wan.absent")}</p>
      ) : (
        <>
          {wan.up && wan.ipv4.length > 0 && (
            <p className="stat-lg font-mono mb-2">{wan.ipv4[0]}</p>
          )}
          {!wan.up && <Banner tone="danger" className="mb-2">{t("overview.wanDown")}</Banner>}
          <KeyValue items={[
            { label: t("wan.gateway"), value: wan.gateway ?? "—", mono: true },
            { label: t("wan.dns"), value: wan.dns.join("  ") || "—", mono: true },
            ...(wan.up ? [{ label: t("system.uptime"), value: t("overview.wanSince", { time: fmtUptime(t, wan.uptime) }) }] : []),
          ]} />
        </>
      )}
    </Card>
  );
}

function MemoryCard({ system }: { system?: SystemInfo }) {
  const { t } = useTranslation();
  const used = system ? system.memory.total - system.memory.available : 0;
  const pct = system ? Math.round((used / system.memory.total) * 100) : 0;
  return (
    <Card index={1} id="recursos" className="md:col-span-4 order-5 md:order-none"
      title={oneLine(t("overview.memory"))} icon={MemoryStick} iconTone="muted" help="memory">
      {!system ? <SkeletonRows rows={2} /> : (
        <div className="flex items-center gap-4">
          <Gauge value={pct} size="sm" mode="consumption" ariaLabel={`${t("overview.memory")} ${pct}%`} />
          <div className="min-w-0">
            <p className="stat-md">{fmtMB(used)} / {fmtMB(system.memory.total)}</p>
            <p className="text-caption text-muted mt-1">
              {t("overview.load")}: {system.load.map((l) => l.toFixed(2)).join(" · ")}
            </p>
          </div>
        </div>
      )}
    </Card>
  );
}

function FlashCard({ system }: { system?: SystemInfo }) {
  const { t } = useTranslation();
  const freePct = system ? Math.round((system.root.free / system.root.total) * 100) : 0;
  const tone = freePct <= 10 ? "danger" : freePct <= 20 ? "warn" : "ok";
  return (
    <Card index={1} className="md:col-span-4 order-6 md:order-none"
      title={oneLine(t("overview.flash"))} icon={HardDrive} help="flash">
      {!system ? <SkeletonRows rows={2} /> : (
        <div className="flex items-center gap-4">
          <Gauge value={freePct} size="sm" tone={tone} ariaLabel={`${t("overview.flash")} ${freePct}%`} />
          <p className="stat-md min-w-0">
            {t("overview.flashFree", {
              free: fmtMB(system.root.free * 1024),
              total: fmtMB(system.root.total * 1024),
            })}
          </p>
        </div>
      )}
    </Card>
  );
}

/* ══════════════ Fila 2 — Tráfico §4 ══════════════ */

type Sample = { ts: number; rates: Record<string, { rx: number; tx: number }> };
const MAX_SAMPLES = 60;

function ifaceLabel(t: TFunction, name: string): string {
  if (name === "br-lan" || name.startsWith("br-")) return t("traffic.iface.lan");
  if (name === "eth0" || name === "wan" || name.startsWith("pppoe")) return t("traffic.iface.internet");
  if (name.startsWith("wlan") || name.startsWith("phy")) return t("traffic.iface.wifi");
  return name;
}

function LiveTrafficCard() {
  const { t } = useTranslation();
  const [samples, setSamples] = useState<Sample[]>();
  const [selected, setSelected] = useState<string>();
  const [failed, setFailed] = useState(false);
  const prev = useRef<{ counters: IfaceCounters[]; ts: number }>(undefined);

  const poll = useCallback(async () => {
    try {
      const next = await api.netdev();
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
        setSamples((s) => [...(s ?? []).slice(-MAX_SAMPLES + 1), { ts: next.ts, rates }]);
      }
      prev.current = next;
      setFailed(false);
    } catch {
      // error parcial §8: solo si nunca llegaron datos; con datos, se conservan
      if (!prev.current) setFailed(true);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    poll();
    const id = setInterval(poll, 2000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const ifaces = useMemo(() => {
    const last = samples?.[samples.length - 1];
    return last ? Object.keys(last.rates) : [];
  }, [samples]);

  const active = selected && ifaces.includes(selected) ? selected
    : ifaces.includes("br-lan") ? "br-lan" : ifaces[0];

  const rxSeries = samples?.map((s) => s.rates[active]?.rx ?? 0) ?? [];
  const txSeries = samples?.map((s) => s.rates[active]?.tx ?? 0) ?? [];
  const rxNow = rxSeries[rxSeries.length - 1] ?? 0;
  const txNow = txSeries[txSeries.length - 1] ?? 0;
  const peak = Math.max(...rxSeries, ...txSeries, 0);
  const avg = rxSeries.length ? rxSeries.reduce((a, b) => a + b, 0) / rxSeries.length : 0;
  const total = rxSeries.reduce((a, b) => a + b, 0) * 2 + txSeries.reduce((a, b) => a + b, 0) * 2; // ×dt(2s)

  return (
    <Card index={2} className="md:col-span-8 order-3 md:order-none"
      title={oneLine(t("overview.trafficLive"))} icon={Activity} iconTone="teal"
      action={samples && samples.length > 1 ? <Pill tone="danger" live>{t("overview.live")}</Pill> : undefined}>
      {failed ? (
        <EmptyState small title={t("common.loadError")}
          illustration={<CloudOff size={24} />}
          action={<Button variant="secondary" size="sm" onClick={poll}>{t("common.retry")}</Button>} />
      ) : !samples || samples.length < 2 ? (
        <SkeletonChart height={200} />
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-3 mb-2">
            <SegmentedControl
              ariaLabel={t("traffic.title")}
              options={ifaces.slice(0, 4).map((name) => ({ value: name, label: ifaceLabel(t, name) }))}
              value={active}
              onChange={setSelected}
            />
            <div className="ml-auto flex items-center gap-4">
              <span className="stat-md inline-flex items-center gap-1 text-ok">
                <ArrowDown size={16} aria-hidden="true" /> {fmtRate(rxNow)}
              </span>
              <span className="stat-md inline-flex items-center gap-1 text-accent">
                <ArrowUp size={16} aria-hidden="true" /> {fmtRate(txNow)}
              </span>
            </div>
          </div>
          <AreaChart rx={rxSeries} tx={txSeries} height={200} live
            ariaLabel={`${t("overview.trafficLive")}: ${fmtRate(rxNow)} ↓, ${t("traffic.peak")} ${fmtRate(peak)}`} />
          <p className="text-caption text-muted mt-2">
            {t("traffic.peak")} {fmtRate(peak)} · {t("traffic.avg")} {fmtRate(avg)} · {t("traffic.total")} {fmtBytes(total)}
          </p>
        </>
      )}
    </Card>
  );
}

function HistoryCard() {
  const { t } = useTranslation();
  const [entries, setEntries] = useState<HistoryEntry[]>();
  const [failed, setFailed] = useState(false);
  const [range, setRange] = useState<"1h" | "24h">("24h");

  const load = useCallback(() => {
    api.history()
      .then((r) => { setEntries(r.entries ?? []); setFailed(false); })
      .catch(() => setFailed(true));
  }, []);

  useEffect(() => {
    load();
    const id = setInterval(load, 60000);
    return () => clearInterval(id);
  }, [load]);

  const model = useMemo(() => {
    if (!entries || entries.length < 2) return null;
    const cutoff = range === "1h" ? entries[entries.length - 1].ts - 3600 : 0;
    const slice = entries.filter((e) => e.ts >= cutoff);
    if (slice.length < 2) return null;
    const deltas = slice.slice(1).map((e, i) => {
      const dt = Math.max(1, e.ts - slice[i].ts);
      return { ts: e.ts, rx: Math.max(0, (e.rx - slice[i].rx) / dt), tx: Math.max(0, (e.tx - slice[i].tx) / dt), rxBytes: Math.max(0, e.rx - slice[i].rx), txBytes: Math.max(0, e.tx - slice[i].tx) };
    });
    const totalDown = deltas.reduce((a, d) => a + d.rxBytes, 0);
    const totalUp = deltas.reduce((a, d) => a + d.txBytes, 0);
    const peak = deltas.reduce((m, d) => (d.rx > m.rx ? m = d : m), deltas[0]);
    return { deltas, totalDown, totalUp, peak };
  }, [entries, range]);

  const hasHour = !!entries && entries.length >= 2 && entries[entries.length - 1].ts - entries[0].ts > 3600;

  return (
    <Card index={2} className="md:col-span-4 order-9 md:order-none"
      title={oneLine(t("overview.last24"))} icon={History} iconTone="teal">
      {failed ? (
        <EmptyState small title={t("common.loadError")} illustration={<CloudOff size={24} />}
          action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retry")}</Button>} />
      ) : !model ? (
        <SkeletonChart height={120} />
      ) : (
        <>
          {hasHour && (
            <SegmentedControl size="sm" ariaLabel={t("overview.last24")}
              options={[{ value: "1h" as const, label: t("history.range1h") }, { value: "24h" as const, label: t("history.range24h") }]}
              value={range} onChange={setRange} />
          )}
          <div className="mt-2">
            <AreaChart
              rx={model.deltas.map((d) => d.rx)}
              tx={model.deltas.map((d) => d.tx)}
              height={120}
              xLabels={model.deltas.map((d) => fmtTime(d.ts))}
              ariaLabel={t("overview.last24")}
            />
          </div>
          <div className="mt-2 space-y-1 text-caption text-muted">
            <p>{t("history.totalDown")}: <span className="text-text font-medium">{fmtBytes(model.totalDown)}</span>
              {" · "}{t("history.totalUp")}: <span className="text-text font-medium">{fmtBytes(model.totalUp)}</span></p>
            <p>{t("history.peakAt", { rate: fmtRate(model.peak.rx), time: fmtTime(model.peak.ts) })}</p>
          </div>
        </>
      )}
    </Card>
  );
}

/* ══════════════ Fila 3 — Quién gasta más / Puertos / Mesh §5 ══════════════ */

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

function TopConsumersCard({ clients, onNavigate }: { clients?: Client[]; onNavigate: (p: string) => void }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<DPIProbe>();
  const snapshots = useAppSeries();

  useEffect(() => {
    if (!isDemo()) return;
    api.dpi().then(setProbe).catch(() => {});
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
    if (chart || !clients) return null;
    const withBytes = clients.filter((c) => c.rx_bytes + c.tx_bytes > 0);
    if (withBytes.length === 0) return null;
    return [...withBytes].sort((a, b) => b.rx_bytes + b.tx_bytes - (a.rx_bytes + a.tx_bytes)).slice(0, 5);
  }, [chart, clients]);

  const multiSeries = chart
    ? chart.series.map((s, i) => ({
        key: s.name, label: s.name,
        color: SERIES_PALETTE[i % SERIES_PALETTE.length],
        points: s.bytes,
      }))
    : null;

  return (
    <Card index={3} className="md:col-span-12 order-7 md:order-none"
      title={oneLine(t("overview.topConsumers"))} icon={ChartColumn} iconTone="teal" help="dpi">
      {multiSeries ? (
        <>
          {/* Leyenda */}
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
          {/* Ranking compacto */}
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
        </>
      ) : clientTop ? (
        <div className="space-y-2.5">
          {clientTop.map((c) => {
            const bytes = c.rx_bytes + c.tx_bytes;
            const max = clientTop[0].rx_bytes + clientTop[0].tx_bytes || 1;
            return (
              <div key={c.mac}>
                <div className="flex justify-between text-small mb-1">
                  <span className="font-medium truncate">{c.name || c.mac}</span>
                  <span className="text-muted" style={{ fontVariantNumeric: "tabular-nums" }}>{fmtBytes(bytes)}</span>
                </div>
                <div className="h-1.5 rounded-full bg-surface-2 overflow-hidden">
                  <div className="h-full rounded-full bg-accent transition-[width] duration-500" style={{ width: `${(bytes / max) * 100}%` }} />
                </div>
              </div>
            );
          })}
        </div>
      ) : probe || clients ? (
        <EmptyState small
          illustration={<IlluDevices size={120} />}
          title={t("overview.dpiEmpty")}
          body={t("overview.dpiEmptyBody")}
          action={<Button variant="secondary" size="sm" onClick={() => onNavigate("services")}>{t("overview.dpiGoServices")}</Button>}
        />
      ) : (
        <SkeletonChart height={220} />
      )}
    </Card>
  );
}

const SERIES_PALETTE = [
  "#22d3ee", "#3b82f6", "#facc15", "#4ade80", "#fb7185", "#fb923c",
  "#a78bfa", "#38bdf8", "#fbbf24", "#f472b6",
];

function PortsCard({ ports, onNavigate }: { ports?: EthPort[]; onNavigate: (p: string) => void }) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<string>();

  const sorted = useMemo(() => !ports ? [] : [...ports].sort((a, b) => {
    if (a.name === "wan") return -1;
    if (b.name === "wan") return 1;
    return a.name.localeCompare(b.name, undefined, { numeric: true });
  }), [ports]);

  const selectedPort = sorted.find((p) => p.name === selected);

  return (
    <Card index={3} className="md:col-span-12 order-10 md:order-none"
      title={oneLine(t("overview.ports"))} icon={Cable} iconTone="teal">
      {!ports ? <SkeletonRows rows={3} /> : sorted.length === 0 ? (
        <EmptyState small illustration={<IlluPlug size={120} />} title={t("overview.portsEmpty")} />
      ) : (
        <>
          <p className="text-caption text-muted mb-3">
            {t("ports.inUse", { used: sorted.filter((p) => p.up).length, total: sorted.length })}
          </p>
          {/* chasis RJ45 redibujado: boca + LED + etiqueta + dispositivo */}
          <div className="flex flex-wrap gap-x-3 gap-y-4 rounded-md border border-border bg-surface-2 px-3 py-3">
            {sorted.map((p) => {
              const label = p.wan ? "WAN" : p.name.toUpperCase().replace(/^LAN(\d+)$/, "LAN $1");
              const device = !p.up ? t("ports.free")
                : p.devices.length === 1 ? p.devices[0].name || p.devices[0].mac
                : p.devices.length > 1 ? t("ports.unmanagedN", { count: p.devices.length })
                : t("ports.busy");
              const led = !p.up ? "bg-border-strong" : p.speed_mbps >= 1000 ? "bg-ok" : "bg-warn";
              return (
                <button key={p.name} type="button"
                  onClick={() => setSelected(selected === p.name ? undefined : p.name)}
                  title={p.up ? `${label} · ${p.speed_mbps >= 1000 ? `${p.speed_mbps / 1000} Gb/s` : `${p.speed_mbps} Mb/s`} · ${device}` : label}
                  className={`flex w-[64px] flex-col items-center gap-1 rounded-sm p-1 ring-focus transition-colors hover:bg-surface
                    ${selected === p.name ? "bg-surface shadow-card" : ""}`}>
                  <span className="flex w-8 justify-between" aria-hidden="true">
                    <span className={`h-1.5 w-1.5 rounded-full ${led}`} />
                    <span className={`h-1.5 w-1.5 rounded-full ${led} ${p.up ? "animate-pulse-dot" : ""}`} />
                  </span>
                  <span className={`relative h-10 w-10 rounded-sm border-2 ${p.up ? "border-border-strong bg-surface" : "border-border bg-surface-2"}`} aria-hidden="true">
                    <span className="absolute inset-x-[6px] top-[4px] flex justify-between">
                      {Array.from({ length: 6 }).map((_, i) => (
                        <span key={i} className={`h-2 w-[2px] rounded-full ${p.up ? "bg-warn/70" : "bg-border"}`} />
                      ))}
                    </span>
                    <span className="absolute inset-x-[5px] bottom-[4px] h-[14px] rounded-[3px] border border-border bg-bg" />
                  </span>
                  <span className={`text-[10px] font-semibold font-mono tracking-wide ${p.wan ? "text-accent" : "text-muted"}`}>{label}</span>
                  <span className="w-full truncate text-center text-[11px] text-text">{device}</span>
                </button>
              );
            })}
          </div>
          {selectedPort && (
            <div className="mt-3 flex items-center justify-between gap-2 text-small">
              <span className="text-muted font-mono text-caption">
                {selectedPort.devices[0]?.mac ?? ""}
                {selectedPort.up && selectedPort.speed_mbps > 0 && ` · ${selectedPort.speed_mbps >= 1000 ? `${selectedPort.speed_mbps / 1000} Gb/s` : `${selectedPort.speed_mbps} Mb/s`}`}
              </span>
              <button type="button" onClick={() => onNavigate("ports")}
                className="text-accent hover:text-accent-hover ring-focus rounded-sm shrink-0">
                {t("overview.ports")} →
              </button>
            </div>
          )}
        </>
      )}
    </Card>
  );
}

/** Mini-grafo radial mesh §5: este router al centro, satélites alrededor. */
function ClientsCard({ clients, onNavigate }: { clients?: Client[]; onNavigate: (p: string) => void }) {
  const { t } = useTranslation();
  const online = clients?.length ?? 0;
  return (
    <Card index={4} className="md:col-span-12 order-4 md:order-none"
      title={oneLine(t("overview.devices"))} icon={Smartphone} iconTone="teal"
      action={clients && (
        <span className="flex items-center gap-3">
          <Pill tone="muted" live>{t("overview.online", { count: online })}</Pill>
          <button type="button" onClick={() => onNavigate("clients")}
            className="text-small text-accent hover:text-accent-hover ring-focus rounded-sm">
            {t("clients.open")} →
          </button>
        </span>
      )}>
      {!clients ? <SkeletonRows rows={2} /> : clients.length === 0 ? (
        <EmptyState small illustration={<IlluDevices size={120} />} title={t("overview.devicesEmpty")} />
      ) : (
        <div className="flex flex-wrap gap-x-4 gap-y-1">
          {clients.slice(0, 8).map((c) => (
            <span key={c.mac} className="inline-flex items-center gap-1.5 text-small">
              <span className="inline-block h-1.5 w-1.5 rounded-full bg-ok" />
              <span className="max-w-32 truncate">{c.name || c.mac}</span>
            </span>
          ))}
          {clients.length > 8 && <span className="text-small text-muted">+{clients.length - 8}</span>}
        </div>
      )}
    </Card>
  );
}

/* ══════════════ Fila 5 — Configuración protegida (drift) §7 ══════════════ */

function DriftSection({ drift, onChange }: { drift?: DriftProbe; onChange: (d: DriftProbe) => void }) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [diffOpen, setDiffOpen] = useState(false);

  const takeBaseline = async () => {
    setLoading(true);
    try {
      await api.createSnapshot();
      onChange(await api.drift());
    } catch { /* ignorado */ } finally {
      setLoading(false);
    }
  };

  if (!drift) {
    return (
      <Card index={5} id="drift" className="md:col-span-12 order-11 md:order-none"
        title={oneLine(t("overview.drift"))} icon={ShieldCheck} iconTone="muted" help="drift">
        <SkeletonRows rows={1} />
      </Card>
    );
  }

  return (
    <Card index={5} id="drift" className="md:col-span-12 order-11 md:order-none" help="drift"
      title={oneLine(t("overview.drift"))} icon={ShieldCheck}
      iconTone={!drift.has_baseline ? "muted" : drift.changes === 0 ? "ok" : "warn"}>
      {!drift.has_baseline ? (
        <div className="flex flex-wrap items-center gap-3">
          <p className="text-small text-muted flex-1 min-w-52">{t("drift.noBaseline")}</p>
          <Button variant="secondary" size="sm" loading={loading} onClick={takeBaseline}>
            {t("drift.createFirst")}
          </Button>
        </div>
      ) : drift.changes === 0 ? (
        <div>
          <p className="text-body">{t("drift.clean")}</p>
          <p className="text-caption text-muted mt-0.5">{t("drift.sinceDate", { date: fmtDate(drift.snapshot_ts) })}</p>
        </div>
      ) : (
        <div className="flex flex-wrap items-center gap-3">
          <p className="text-body flex-1 min-w-52">
            {t("drift.changes", { count: drift.changes, date: fmtDate(drift.snapshot_ts) })}
          </p>
          <Button variant="secondary" size="sm" onClick={() => setDiffOpen(true)}>{t("drift.viewChanges")}</Button>
          <Button variant="ghost" size="sm" loading={loading} onClick={takeBaseline}>{t("drift.createNow")}</Button>
        </div>
      )}

      <Modal open={diffOpen} onClose={() => setDiffOpen(false)} title={t("drift.viewChanges")} wide>
        {(drift.configs ?? []).map((cfg) => (
          <div key={cfg.config} className="mb-3 last:mb-0">
            <p className="text-small font-semibold mb-1">{cfg.config}</p>
            <div className="rounded-sm border border-border bg-surface-2 p-2 font-mono text-caption max-h-56 overflow-y-auto">
              {cfg.lines.map((line, i) => (
                <div key={i} className={line.kind === "added" ? "text-ok" : "text-danger"}>
                  {line.kind === "added" ? "+" : "-"} {line.text}
                </div>
              ))}
            </div>
          </div>
        ))}
      </Modal>
    </Card>
  );
}

/* ══════════════ Página ══════════════ */

export function Overview({ board, system, wan, ethports, drift, onDriftChange, isSwitch, health, mode, onNavigate }: {
  board?: Board;
  system?: SystemInfo;
  wan?: WanStatus;
  ethports?: EthPort[];
  drift?: DriftProbe;
  onDriftChange: (d: DriftProbe) => void;
  isSwitch: boolean;
  health: HealthScore;
  mode?: ModeProbe;
  onNavigate: (page: string) => void;
}) {
  const [clients, setClients] = useState<Client[]>();
  const pollRef = useRef<number>(undefined);

  const loadClients = useCallback(async () => {
    const r = await api.clients();
    setClients(r.clients);
  }, []);

  useEffect(() => {
    loadClients().catch(() => {});
    pollRef.current = window.setInterval(() => loadClients().catch(() => {}), 3000);
    return () => clearInterval(pollRef.current);
  }, [loadClients]);

  return (
    <div className="grid grid-cols-1 md:grid-cols-12 gap-[var(--card-gap)]">
      <HealthHero health={health} board={board} system={system} onNavigate={onNavigate} />
      {!isSwitch && <InternetCard wan={wan} mode={mode} />}
      <MemoryCard system={system} />
      <FlashCard system={system} />
      <LiveTrafficCard />
      <HistoryCard />
      {!isSwitch && <TopConsumersCard clients={clients} onNavigate={onNavigate} />}
      <PortsCard ports={ethports} onNavigate={onNavigate} />
      <ClientsCard clients={clients} onNavigate={onNavigate} />
      <DriftSection drift={drift} onChange={onDriftChange} />
    </div>
  );
}
