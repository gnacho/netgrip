import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  Activity, ArrowDown, ArrowUp, Cable, CloudOff, Cpu, Globe, HardDrive,
  Maximize2, MemoryStick, ShieldCheck, Smartphone, Wifi,
} from "lucide-react";
import { api } from "../../api";
import type {
  Board, Client, DriftProbe, EthPort,
  HistoryEntry,
  IfaceCounters, ModeProbe, SystemInfo, WanStatus, WirelessRadio,
  CPUProbe,
} from "../../types";
import type { HealthScore } from "../../hooks/useHealthScore";
import {
  AreaChart, Banner, Button, Card, EmptyState, Gauge,
  Modal, Pill, SegmentedControl, SkeletonChart, SkeletonRows,
} from "../ui";
import { IlluDevices } from "../ui/illustrations";
import { CpuDetailModal } from "../CpuDetailModal";
import { MemoryDetailModal } from "../MemoryDetailModal";
import { FlashDetailModal } from "../FlashDetailModal";
import { fmtBytes, fmtDate, fmtMB, fmtRate, fmtTime, fmtUptime } from "../../lib/format";

/** Título de tarjeta a una línea (design-rev2 §3): ellipsis + tooltip nativo. */
export function oneLine(text: string) {
  return <span className="block truncate" title={text}>{text}</span>;
}

/* ══════════════ Fila 0 — Salud del router (héroe) §2 ══════════════ */

function portLabel(p: EthPort) {
  return p.wan ? "WAN" : p.name.toUpperCase().replace(/^LAN(\d+)$/, "LAN $1");
}

function portSpeed(p: EthPort) {
  if (p.speed_mbps <= 0) return "";
  return p.speed_mbps >= 1000 ? `${p.speed_mbps / 1000} Gb/s` : `${p.speed_mbps} Mb/s`;
}

export function HealthHero({ health, board, system, onNavigate, wireless, ports, ap = false, role = "router" }: {
  health: HealthScore;
  board?: Board;
  system?: SystemInfo;
  onNavigate: (page: string, anchor?: string) => void;
  wireless?: WirelessRadio[];
  ports?: EthPort[];
  ap?: boolean;
  /** Rol detectado (#610): el título dice router, punto de acceso o switch. */
  role?: "router" | "ap" | "switch";
}) {
  const { t } = useTranslation();
  const go = (page: string, anchor?: string) => {
    if (page === "overview" && anchor) {
      document.getElementById(anchor)?.scrollIntoView({ behavior: "smooth", block: "start" });
    } else {
      onNavigate(page, anchor);
    }
  };

  const radios = (wireless ?? []).filter((r) => r.up);
  const ifaces = radios.flatMap((r) => r.interfaces).filter((i) => !i.disabled);
  const ssids = [...new Set(ifaces.map((i) => i.ssid))];
  const wifiClients = ifaces.reduce((n, i) => n + (i.clients?.length ?? 0), 0);
  const bands = [...new Set(radios.map((r) => t(r.band === "5g" ? "wifi.band5" : "wifi.band24")))];
  const emitting = radios.length > 0 && ifaces.length > 0;
  const uplink = !ports ? undefined : [...ports]
    .filter((p) => p.up)
    .sort((a, b) =>
      (Number(b.wan) - Number(a.wan)) ||
      (b.devices.length > 0 ? 1 : 0) - (a.devices.length > 0 ? 1 : 0) ||
      b.speed_mbps - a.speed_mbps)[0];
  const hasWifi = wireless !== undefined;
  const hasUplink = ports !== undefined;
  const extras = (hasWifi ? 1 : 0) + (hasUplink ? 1 : 0);
  const columns = extras === 0
    ? "md:grid-cols-[minmax(0,1fr)_auto]"
    : extras === 1 ? "md:grid-cols-2" : "md:grid-cols-3";

  return (
    <Card index={0} id="salud" className="md:col-span-12 order-1 md:order-none">
      <div className={`grid gap-4 ${columns}`}>
        <div className="flex min-w-0 flex-col items-start gap-3 sm:flex-row sm:items-center sm:gap-4">
          <Gauge value={health.score} size="lg" mode="health" ariaLabel={`${t(`health.title.${role}`)}: ${health.score}`} />
          <div className="min-w-0">
            <p className="text-eyebrow text-faint mb-1">{t(`health.title.${role}`)}</p>
            {/* Estado y resumen en la misma fila (#610). */}
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
              <Pill tone={health.tone}>{t(health.labelKey)}</Pill>
              <p className="text-body">
                {health.reasons.length === 0 ? t("health.allGood") : t("health.issues", { count: health.reasons.length })}
              </p>
            </div>
            {health.reasons.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-2">
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

        {hasWifi && (
          <div className="min-w-0 border-t border-border pt-4 md:border-l md:border-t-0 md:pl-5 md:pt-0">
            <div className="flex items-center justify-between gap-2">
              <span className="inline-flex items-center gap-1.5 text-small font-medium text-muted">
                <Wifi size={15} aria-hidden="true" />
                {t("overview.wifiStatus")}
              </span>
              <Pill tone={emitting ? "ok" : "muted"} live={emitting}>
                {emitting ? t("wifi.broadcasting") : t("wifi.bandOff")}
              </Pill>
            </div>
            <p className="stat-md mt-2 truncate">{ssids[0] ?? "—"}</p>
            <p className="text-caption text-muted truncate">
              {emitting ? t("overview.wifiBands", { bands: bands.join(" + ") }) : t("wifi.bandOffHint")}
            </p>
            <p className="mt-2 flex flex-wrap items-center gap-x-2 text-small">
              <span>{t("wifi.devices", { count: wifiClients })}</span>
              <button type="button" onClick={() => onNavigate("wifi")}
                className="text-accent hover:text-accent-hover ring-focus rounded-sm">
                {t("clients.open")} →
              </button>
            </p>
          </div>
        )}

        {hasUplink && (
          <div id="uplink" className="min-w-0 border-t border-border pt-4 md:border-l md:border-t-0 md:pl-5 md:pt-0">
            <div className="flex items-center justify-between gap-2">
              <span className="inline-flex items-center gap-1.5 text-small font-medium text-muted">
                <Cable size={15} aria-hidden="true" />
                {t("overview.uplink")}
              </span>
              <Pill tone={uplink ? "ok" : "danger"} live={!!uplink}>
                {uplink ? t("overview.connected") : t("overview.uplinkDown")}
              </Pill>
            </div>
            <p className="stat-md mt-2 truncate">
              {uplink ? `${portLabel(uplink)} · ${portSpeed(uplink) || "?"}` : "—"}
            </p>
            <p className="text-caption text-muted truncate">
              {ap ? t("overview.apNoNat") : t("overview.switchRole")}
            </p>
            {uplink && uplink.devices.length > 0 && (
              <p className="mt-2 truncate text-small">
                <span className="text-muted">{t("overview.uplinkDevice")}: </span>
                {uplink.devices[0].name || uplink.devices[0].mac}
              </p>
            )}
          </div>
        )}

        {/* Identidad del equipo (#706): columna propia a la derecha, una
            línea por dato. En móvil cae debajo del resto. */}
        <div id="equipo" className={`min-w-0 border-t border-border pt-4 md:border-l md:border-t-0 md:pl-5 md:pt-0 ${extras === 0 ? "md:text-right" : ""}`}>
          <div className="flex flex-col gap-1 text-caption text-muted">
            <span className="truncate">
              <span className="text-faint">{t("overview.model")}:</span>{" "}
              <span className="text-text">{board?.model ?? "…"}</span>
            </span>
            <span className="truncate">
              <span className="text-faint">{t("overview.firmware")}:</span>{" "}
              <span className="text-text">{board?.release ? `${board.release.distribution} ${board.release.version}` : "…"}</span>
            </span>
            <span className="truncate">
              <span className="text-faint">{t("system.uptime")}:</span>{" "}
              <span className="text-text">{system ? fmtUptime(t, system.uptime) : "…"}</span>
            </span>
          </div>
        </div>
      </div>
    </Card>
  );
}

/* ══════════════ Fila 1 — Internet / Memoria / Espacio §3 ══════════════ */

/** Etiqueta pequeña sobre el dato, dentro de la fila del enlace (#610). */
function WanItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <p className="text-eyebrow text-faint">{label}</p>
      <p className="text-body font-mono break-all">{value}</p>
    </div>
  );
}

export function InternetCard({ wan, mode }: { wan?: WanStatus; mode?: ModeProbe }) {
  const { t } = useTranslation();
  const ap = mode?.mode === "ap" || !wan?.present;
  return (
    <Card index={1} id="internet" className="md:col-span-4 order-2 md:order-none"
      title={oneLine(t("overview.internet"))} icon={Globe} help="internet"
      status={wan && (
        ap
          ? <Pill tone="muted">{t("overview.apMode")}</Pill>
          : <Pill tone={wan.up ? "ok" : "danger"}>{wan.up ? t("overview.connected") : t("overview.down")}</Pill>
      )}>
      {!wan ? <SkeletonRows rows={1} /> : ap ? (
        <p className="text-small text-muted">{t("wan.absent")}</p>
      ) : (
        <>
          {!wan.up && <Banner tone="danger" className="mb-3">{t("overview.wanDown")}</Banner>}
          {/* Una sola fila con todo lo del enlace (#610). En móvil envuelve. */}
          <div className="flex flex-wrap items-center gap-x-8 gap-y-3">
            {wan.up && wan.ipv4.length > 0 && (
              <div className="min-w-0">
                <p className="text-eyebrow text-faint">{t("wan.ip")}</p>
                <p className="stat-lg font-mono">{wan.ipv4[0]}</p>
              </div>
            )}
            <WanItem label={t("wan.gateway")} value={wan.gateway ?? "—"} />
            <WanItem label={t("wan.dns")} value={(wan.dns ?? []).join("  ") || "—"} />
            {wan.up && (
              <WanItem label={t("wan.uptimeLabel")} value={fmtUptime(t, wan.uptime)} />
            )}
          </div>
        </>
      )}
    </Card>
  );
}

/* ══════════════ Recursos: CPU, memoria y flash en una tarjeta ══════════════ */

/**
 * What the CPU is doing, core by core.
 *
 * A single average is the wrong number on a router: receive processing is
 * pinned to whichever core takes the NIC interrupt, so the box can be at
 * its routing ceiling with one core saturated and three idle. The panel
 * leads with the busiest core, and shows the counters that say whether the
 * network path is keeping up. Memory and storage sit beside it because they
 * describe the same device and belong to the same card.
 */
export function ResourcesCard({ system, index = 1 }: { system?: SystemInfo; index?: number }) {
  const { t } = useTranslation();
  const [cpu, setCpu] = useState<CPUProbe>();
  const [detail, setDetail] = useState<"cpu" | "memory" | "flash">();

  useEffect(() => {
    const load = () => api.cpu().then(setCpu).catch(() => {});
    load();
    const id = setInterval(load, 5000);
    return () => clearInterval(id);
  }, []);

  const ready = cpu && !cpu.warming && cpu.cores.length > 0;
  const losing = (cpu?.cores ?? []).filter((c) => c.dropped_rate > 0 || c.squeezed_rate > 0);
  const used = system ? system.memory.total - system.memory.available : 0;
  const memPct = system ? Math.round((used / system.memory.total) * 100) : 0;
  const freePct = system ? Math.round((system.root.free / system.root.total) * 100) : 0;
  const flashTone = freePct <= 10 ? "danger" : freePct <= 20 ? "warn" : "ok";
  const panelClass = "min-w-0 border-t border-border pt-4 first:border-t-0 first:pt-0 md:border-l md:border-t-0 md:pl-5 md:pt-0 md:first:border-l-0 md:first:pl-0";

  return (
    <Card index={index} id="recursos" title={oneLine(t("overview.resources"))} icon={Cpu} iconTone="muted">
      <div className="grid gap-4 md:grid-cols-3">
        <div className={panelClass}>
          <div className="flex items-center justify-between gap-2">
            <h3 className="inline-flex items-center gap-1.5 text-small font-medium text-muted">
              <Cpu size={15} aria-hidden="true" />
              {t("overview.cpu")}
            </h3>
            <button type="button" onClick={() => setDetail("cpu")} aria-label={t("overview.cpuExpand")}
              className="text-muted hover:text-text ring-focus rounded-sm">
              <Maximize2 size={14} />
            </button>
          </div>
          {!ready ? <SkeletonRows rows={1} /> : (
            <>
              <div className="mt-2 flex items-center gap-4">
                <Gauge value={Math.round(cpu!.busiest_pct)} size="sm" mode="consumption"
                  ariaLabel={`${t("overview.cpuBusiest")} ${cpu!.busiest_pct}%`} />
                <div className="min-w-0">
                  <p className="stat-md">{cpu!.busiest_pct}%</p>
                  <p className="text-caption text-muted mt-1">
                    {t("overview.cpuBusiest")} · {t("overview.cpuAverage", { pct: cpu!.usage_pct })}
                  </p>
                </div>
              </div>
              {losing.length > 0 && (
                <Banner tone="warn" className="mt-3">
                  {t("overview.cpuDropping", {
                    cores: losing.map((c) => c.idx).join(", "),
                    pps: Math.round(losing.reduce((a, c) => a + c.dropped_rate, 0)),
                  })}
                </Banner>
              )}
            </>
          )}
        </div>

        <div className={panelClass}>
          <div className="flex items-center justify-between gap-2">
            <h3 className="inline-flex items-center gap-1.5 text-small font-medium text-muted">
              <MemoryStick size={15} aria-hidden="true" />
              {t("overview.memory")}
            </h3>
            <button type="button" onClick={() => setDetail("memory")} aria-label={t("overview.memExpand")}
              className="text-muted hover:text-text ring-focus rounded-sm">
              <Maximize2 size={14} />
            </button>
          </div>
          {!system ? <SkeletonRows rows={1} /> : (
            <div className="mt-2 flex items-center gap-4">
              <Gauge value={memPct} size="sm" mode="consumption" ariaLabel={`${t("overview.memory")} ${memPct}%`} />
              <div className="min-w-0">
                <p className="stat-md">{fmtMB(used)} / {fmtMB(system.memory.total)}</p>
                <p className="text-caption text-muted mt-1">{t("overview.memByProc")}</p>
              </div>
            </div>
          )}
        </div>

        <div className={panelClass}>
          <div className="flex items-center justify-between gap-2">
            <h3 className="inline-flex items-center gap-1.5 text-small font-medium text-muted">
              <HardDrive size={15} aria-hidden="true" />
              {t("overview.flash")}
            </h3>
            <button type="button" onClick={() => setDetail("flash")} aria-label={t("overview.flashExpand")}
              className="text-muted hover:text-text ring-focus rounded-sm">
              <Maximize2 size={14} />
            </button>
          </div>
          {!system ? <SkeletonRows rows={1} /> : (
            <div className="mt-2 flex items-center gap-4">
              <Gauge value={freePct} size="sm" tone={flashTone} ariaLabel={`${t("overview.flash")} ${freePct}%`} />
              <p className="stat-md min-w-0">
                {t("overview.flashFree", {
                  free: fmtMB(system.root.free * 1024),
                  total: fmtMB(system.root.total * 1024),
                })}
              </p>
            </div>
          )}
        </div>
      </div>

      <CpuDetailModal open={detail === "cpu"} onClose={() => setDetail(undefined)} />
      <MemoryDetailModal system={system} open={detail === "memory"} onClose={() => setDetail(undefined)} />
      <FlashDetailModal system={system} open={detail === "flash"} onClose={() => setDetail(undefined)} />
    </Card>
  );
}

/* ══════════════ Fila 2 — Tráfico §4 ══════════════ */

type Sample = { ts: number; rates: Record<string, { rx: number; tx: number }> };
const MAX_SAMPLES = 60;

/** Ventanas de la tarjeta de tráfico: directo (counters) o histórico. */
type TrafficMode = "live" | "1h" | "24h";

function ifaceLabel(t: TFunction, name: string): string {
  if (name === "br-lan" || name.startsWith("br-")) return t("traffic.iface.lan");
  if (name === "eth0" || name === "wan" || name.startsWith("pppoe")) return t("traffic.iface.internet");
  if (name.startsWith("wlan") || name.startsWith("phy")) return t("traffic.iface.wifi");
  return name;
}

export function LiveTrafficCard() {
  const { t } = useTranslation();
  const [samples, setSamples] = useState<Sample[]>();
  const [selected, setSelected] = useState<string>();
  const [failed, setFailed] = useState(false);
  const prev = useRef<{ counters: IfaceCounters[]; ts: number }>(undefined);
  const [mode, setMode] = useState<TrafficMode>("live");
  const [entries, setEntries] = useState<HistoryEntry[]>();
  const [histFailed, setHistFailed] = useState(false);

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

  // Muestreo rápido al montar (600ms) hasta tener tres muestras; después el
  // ritmo tranquilo de 2s. Así la gráfica arranca enseguida en vez de
  // esperar dos intervalos completos. Se usa un timeout encadenado (no un
  // intervalo fijo) para no solapar polls si el router tarda en responder.
  const samplesRef = useRef<Sample[]>(undefined);
  useEffect(() => { samplesRef.current = samples; }, [samples]);
  useEffect(() => {
    let cancelled = false;
    let timer: number | undefined;
    const tick = async () => {
      await poll();
      if (cancelled) return;
      const n = samplesRef.current?.length ?? 0;
      timer = window.setTimeout(tick, n >= 3 ? 2000 : 600);
    };
    void tick();
    return () => { cancelled = true; clearTimeout(timer); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [poll]);

  const loadHistory = useCallback(() => {
    api.history()
      .then((r) => { setEntries(r.entries ?? []); setHistFailed(false); })
      .catch(() => setHistFailed(true));
  }, []);

  useEffect(() => {
    if (mode === "live") return;
    loadHistory();
    const id = setInterval(loadHistory, 60000);
    return () => clearInterval(id);
  }, [mode, loadHistory]);

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

  // Histórico: mismos deltas de bytes por muestra que antes en la tarjeta de
  // 24h, ahora recortado a la ventana elegida (1h o 24h completas).
  const historyModel = useMemo(() => {
    if (mode === "live" || !entries || entries.length < 2) return null;
    const cutoff = mode === "1h" ? entries[entries.length - 1].ts - 3600 : 0;
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
  }, [entries, mode]);

  return (
    <Card index={2} className="md:col-span-8 order-3 md:order-none"
      title={oneLine(t("overview.trafficLive"))} icon={Activity} iconTone="teal"
      action={mode === "live" && samples && samples.length > 1 ? <Pill tone="danger" live>{t("overview.live")}</Pill> : undefined}>
      {mode !== "live" ? (
        histFailed ? (
          <EmptyState small title={t("common.loadError")}
            illustration={<CloudOff size={24} />}
            action={<Button variant="secondary" size="sm" onClick={loadHistory}>{t("common.retry")}</Button>} />
        ) : !historyModel ? (
          <SkeletonChart height={200} />
        ) : (
          <>
            {/* Controles y cifras en una sola fila; debajo, la gráfica (#610). */}
            <div className="mb-2 flex flex-wrap items-center gap-x-4 gap-y-2">
              <SegmentedControl size="sm" ariaLabel={t("overview.trafficRange")}
                options={[
                  { value: "live" as const, label: t("overview.trafficModeLive") },
                  { value: "1h" as const, label: t("history.range1h") },
                  { value: "24h" as const, label: t("history.range24h") },
                ]}
                value={mode} onChange={setMode} />
              <div className="ml-auto flex flex-wrap items-center gap-x-4 gap-y-1">
                <span className="stat-md inline-flex items-center gap-1 text-ok">
                  <ArrowDown size={16} aria-hidden="true" /> {fmtBytes(historyModel.totalDown)}
                </span>
                <span className="stat-md inline-flex items-center gap-1 text-accent">
                  <ArrowUp size={16} aria-hidden="true" /> {fmtBytes(historyModel.totalUp)}
                </span>
                <span className="text-caption text-muted">
                  {t("history.peakAt", { rate: fmtRate(historyModel.peak.rx), time: fmtTime(historyModel.peak.ts) })}
                </span>
              </div>
            </div>
            <AreaChart
              rx={historyModel.deltas.map((d) => d.rx)}
              tx={historyModel.deltas.map((d) => d.tx)}
              height={200}
              xLabels={historyModel.deltas.map((d) => fmtTime(d.ts))}
              ariaLabel={`${t("overview.trafficLive")}: ${fmtBytes(historyModel.totalDown)} ↓, ${fmtBytes(historyModel.totalUp)} ↑`}
            />
          </>
        )
      ) : failed ? (
        <EmptyState small title={t("common.loadError")}
          illustration={<CloudOff size={24} />}
          action={<Button variant="secondary" size="sm" onClick={poll}>{t("common.retry")}</Button>} />
      ) : (
        <>
          {/* Controles, tasas y resumen en una sola fila; debajo, la gráfica (#610). */}
          <div className="mb-2 flex flex-wrap items-center gap-x-3 gap-y-2">
            <SegmentedControl size="sm" ariaLabel={t("overview.trafficRange")}
              options={[
                { value: "live" as const, label: t("overview.trafficModeLive") },
                { value: "1h" as const, label: t("history.range1h") },
                { value: "24h" as const, label: t("history.range24h") },
              ]}
              value={mode} onChange={setMode} />
            {samples && samples.length >= 2 && (
              <>
                <SegmentedControl
                  ariaLabel={t("traffic.title")}
                  options={ifaces.slice(0, 4).map((name) => ({ value: name, label: ifaceLabel(t, name) }))}
                  value={active}
                  onChange={setSelected}
                />
                <div className="ml-auto flex flex-wrap items-center gap-x-4 gap-y-1">
                  <span className="stat-md inline-flex items-center gap-1 text-ok">
                    <ArrowDown size={16} aria-hidden="true" /> {fmtRate(rxNow)}
                  </span>
                  <span className="stat-md inline-flex items-center gap-1 text-accent">
                    <ArrowUp size={16} aria-hidden="true" /> {fmtRate(txNow)}
                  </span>
                  <span className="text-caption text-muted">
                    {t("traffic.peak")} {fmtRate(peak)} · {t("traffic.avg")} {fmtRate(avg)} · {t("traffic.total")} {fmtBytes(total)}
                  </span>
                </div>
              </>
            )}
          </div>
          {!samples || samples.length < 2 ? (
            <div className="flex items-center justify-center gap-2 text-muted" style={{ height: 200 }}>
              <Activity size={14} aria-hidden="true" />
              <span className="text-caption">{t("overview.liveCollecting")}</span>
            </div>
          ) : (
            <AreaChart rx={rxSeries} tx={txSeries} height={200} live
              ariaLabel={`${t("overview.trafficLive")}: ${fmtRate(rxNow)} ↓, ${t("traffic.peak")} ${fmtRate(peak)}`} />
          )}
        </>
      )}
    </Card>
  );
}

/** Orden estable de las bandas; las desconocidas van al final. */
const BAND_ORDER = ["2g", "5g", "6g", "60g"];

function bandName(t: TFunction, band: string): string {
  if (band === "2g") return t("wifi.band24");
  if (band === "5g") return t("wifi.band5");
  if (band === "6g") return t("clients.band6g");
  if (band === "other") return t("overview.unknownBand");
  return band;
}

/**
 * Cuántos equipos hay por cada forma de llegar a la red (#610): cableados y
 * una cuenta por banda de radio. La banda de un cliente Wi-Fi se resuelve
 * por su interfaz contra el probe de radios, así que un 6 GHz o un 60 GHz
 * no se confunden con 2,4 GHz como hacía el tipo genérico del cliente.
 */
function countByBand(clients: Client[], wireless?: WirelessRadio[]): { wired: number; bands: { band: string; count: number }[] } {
  const bandOfIface = new Map<string, string>();
  const present = new Set<string>();
  for (const radio of wireless ?? []) {
    for (const iface of radio.interfaces) {
      bandOfIface.set(iface.ifname, radio.band);
      if (radio.up && !iface.disabled) present.add(radio.band);
    }
  }

  let wired = 0;
  const counts = new Map<string, number>();
  for (const c of clients) {
    if (c.type === "cable") {
      wired++;
      continue;
    }
    const band = (c.iface && bandOfIface.get(c.iface))
      || (c.type === "wifi5" ? "5g" : c.type === "wifi24" ? "2g" : "other");
    counts.set(band, (counts.get(band) ?? 0) + 1);
    present.add(band);
  }

  const bands = [...present].sort((a, b) => {
    const ia = BAND_ORDER.indexOf(a);
    const ib = BAND_ORDER.indexOf(b);
    if (ia !== -1 || ib !== -1) return (ia === -1 ? BAND_ORDER.length : ia) - (ib === -1 ? BAND_ORDER.length : ib);
    return a.localeCompare(b);
  });
  return { wired, bands: bands.map((band) => ({ band, count: counts.get(band) ?? 0 })) };
}

export function ClientsCard({ onNavigate, wireless, role = "router", className = "md:col-span-12" }: {
  onNavigate: (p: string) => void;
  wireless?: WirelessRadio[];
  /** Un AP no lista cableados (su FDB no es la del gateway): sin chip. */
  role?: "router" | "ap" | "switch";
  /** Clase de layout conservada para compatibilidad con las composiciones
   *  anteriores; el Resumen actual apila todas las tarjetas (#598). */
  className?: string;
}) {
  const { t } = useTranslation();
  const [clients, setClients] = useState<Client[]>();

  const load = useCallback(async () => {
    const r = await api.clients();
    setClients(r.clients);
  }, []);

  useEffect(() => {
    load().catch(() => {});
    const id = window.setInterval(() => load().catch(() => {}), 3000);
    return () => clearInterval(id);
  }, [load]);

  const online = clients?.length ?? 0;
  const counts = useMemo(() => countByBand(clients ?? [], wireless), [clients, wireless]);
  return (
    <Card index={4} className={`${className} order-4 md:order-none`}
      title={oneLine(t("overview.devices"))} icon={Smartphone} iconTone="teal"
      action={clients && (
        <span className="hidden items-center gap-3 sm:flex">
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
        <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
          {role !== "ap" && (
            <span className="inline-flex items-center gap-2">
              <Cable size={15} className="text-muted" aria-hidden="true" />
              <span className="stat-md">{counts.wired}</span>
              <span className="text-caption text-muted">{t("overview.wired", { count: counts.wired })}</span>
            </span>
          )}
          {counts.bands.map(({ band, count }) => (
            <span key={band} className="inline-flex items-center gap-2">
              <Wifi size={15} className="text-muted" aria-hidden="true" />
              <span className="stat-md">{count}</span>
              <span className="text-caption text-muted">
                {t("overview.onBand", { band: bandName(t, band) })}
              </span>
            </span>
          ))}
        </div>
      )}
    </Card>
  );
}

/* ══════════════ Fila 5 — Configuración protegida (drift) §7 ══════════════ */

export function DriftSection({ drift, onChange }: { drift?: DriftProbe; onChange: (d: DriftProbe) => void }) {
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
    <Card index={5} id="drift" className="md:col-span-12 order-11 md:order-none" help="drift" headerOnly
      title={oneLine(t("overview.drift"))} icon={ShieldCheck}
      iconTone={!drift.has_baseline ? "muted" : drift.changes === 0 ? "ok" : "warn"}
      action={
        <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-2">
          {!drift.has_baseline ? (
            <>
              <span className="text-small text-muted">{t("drift.noBaseline")}</span>
              <Button variant="secondary" size="sm" loading={loading} onClick={takeBaseline}>
                {t("drift.createFirst")}
              </Button>
            </>
          ) : drift.changes === 0 ? (
            <>
              <span className="text-small text-muted">{t("drift.clean")}</span>
              <span className="text-caption text-faint">{t("drift.sinceDate", { date: fmtDate(drift.snapshot_ts) })}</span>
              <Button variant="ghost" size="sm" loading={loading} onClick={takeBaseline}>{t("drift.createNow")}</Button>
            </>
          ) : (
            <>
              <span className="text-small">
                {t("drift.changes", { count: drift.changes, date: fmtDate(drift.snapshot_ts) })}
              </span>
              <Button variant="secondary" size="sm" onClick={() => setDiffOpen(true)}>{t("drift.viewChanges")}</Button>
              <Button variant="ghost" size="sm" loading={loading} onClick={takeBaseline}>{t("drift.createNow")}</Button>
            </>
          )}
        </div>
      }>
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
