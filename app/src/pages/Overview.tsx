import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  Activity, ArrowDown, ArrowUp, CloudOff, Cpu, Globe, HardDrive,
  MemoryStick, ShieldCheck, Smartphone,
} from "lucide-react";
import { api } from "../api";
import type {
  Board, Client, DriftProbe,
  HistoryEntry,
  IfaceCounters, ModeProbe, SystemInfo, WanStatus,
  CPUProbe,
} from "../types";
import type { HealthScore } from "../hooks/useHealthScore";
import {
  AreaChart, Banner, Button, Card, EmptyState, Gauge,
  KeyValue, Modal, Pill, SegmentedControl, SkeletonChart, SkeletonRows,
} from "../components/ui";
import { IlluDevices } from "../components/ui/illustrations";
import { CpuDetailModal } from "../components/CpuDetailModal";
import { MemoryDetailModal } from "../components/MemoryDetailModal";
import { FlashDetailModal } from "../components/FlashDetailModal";
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
            { label: t("wan.dns"), value: (wan.dns ?? []).join("  ") || "—", mono: true },
            ...(wan.up ? [{ label: t("system.uptime"), value: t("overview.wanSince", { time: fmtUptime(t, wan.uptime) }) }] : []),
          ]} />
        </>
      )}
    </Card>
  );
}

/* ══════════════ CPU: por núcleo, no en promedio ══════════════ */

/**
 * What the CPU is doing, core by core.
 *
 * A single average is the wrong number on a router: receive processing is
 * pinned to whichever core takes the NIC interrupt, so the box can be at
 * its routing ceiling with one core saturated and three idle. The card
 * leads with the busiest core, and shows the two counters that say whether
 * the network path is keeping up — packets dropped for lack of backlog,
 * and softirq budget exhaustions.
 */
function CpuCard() {
  const { t } = useTranslation();
  const [cpu, setCpu] = useState<CPUProbe>();
  const [detail, setDetail] = useState(false);

  useEffect(() => {
    const load = () => api.cpu().then(setCpu).catch(() => {});
    load();
    const id = setInterval(load, 5000);
    return () => clearInterval(id);
  }, []);

  const ready = cpu && !cpu.warming && cpu.cores.length > 0;
  // Only worth surfacing on the card itself; everything else lives in the
  // expand view so the three resource cards stay one line tall.
  const losing = (cpu?.cores ?? []).filter((c) => c.dropped_rate > 0 || c.squeezed_rate > 0);

  return (
    <Card index={1} id="cpu" className="md:col-span-4 order-5 md:order-none"
      title={oneLine(t("overview.cpu"))} icon={Cpu} iconTone="muted" help="cpu"
      onExpand={() => setDetail(true)} expandLabel={t("overview.cpuExpand")}>
      {!ready ? <SkeletonRows rows={1} /> : (
        <>
          <div className="flex items-center gap-4">
            <Gauge value={Math.round(cpu!.busiest_pct)} size="sm" mode="consumption"
              ariaLabel={`${t("overview.cpuBusiest")} ${cpu!.busiest_pct}%`} />
            <div className="min-w-0">
              <p className="stat-md">{cpu!.busiest_pct}%</p>
              <p className="text-caption text-muted mt-1">
                {t("overview.cpuBusiest")} · {t("overview.cpuAverage", { pct: cpu!.usage_pct })}
              </p>
            </div>
          </div>

          {/* Packets the kernel threw away because a core could not keep
              up. Shown only while it is happening. */}
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
      <CpuDetailModal open={detail} onClose={() => setDetail(false)} />
    </Card>
  );
}

function MemoryCard({ system }: { system?: SystemInfo }) {
  const { t } = useTranslation();
  const [detail, setDetail] = useState(false);
  const used = system ? system.memory.total - system.memory.available : 0;
  const pct = system ? Math.round((used / system.memory.total) * 100) : 0;
  return (
    <Card index={1} id="recursos" className="md:col-span-4 order-6 md:order-none"
      title={oneLine(t("overview.memory"))} icon={MemoryStick} iconTone="muted" help="memory"
      onExpand={() => setDetail(true)} expandLabel={t("overview.memExpand")}>
      {!system ? <SkeletonRows rows={1} /> : (
        <div className="flex items-center gap-4">
          <Gauge value={pct} size="sm" mode="consumption" ariaLabel={`${t("overview.memory")} ${pct}%`} />
          <div className="min-w-0">
            <p className="stat-md">{fmtMB(used)} / {fmtMB(system.memory.total)}</p>
            <p className="text-caption text-muted mt-1">{t("overview.memByProc")}</p>
          </div>
        </div>
      )}
      <MemoryDetailModal system={system} open={detail} onClose={() => setDetail(false)} />
    </Card>
  );
}

function FlashCard({ system }: { system?: SystemInfo }) {
  const { t } = useTranslation();
  const [detail, setDetail] = useState(false);
  const freePct = system ? Math.round((system.root.free / system.root.total) * 100) : 0;
  const tone = freePct <= 10 ? "danger" : freePct <= 20 ? "warn" : "ok";
  return (
    <Card index={1} className="md:col-span-4 order-7 md:order-none"
      title={oneLine(t("overview.flash"))} icon={HardDrive} help="flash"
      onExpand={() => setDetail(true)} expandLabel={t("overview.flashExpand")}>
      {!system ? <SkeletonRows rows={1} /> : (
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
      <FlashDetailModal system={system} open={detail} onClose={() => setDetail(false)} />
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

function LiveTrafficCard() {
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
            <div className="flex flex-wrap items-center gap-3 mb-2">
              <SegmentedControl size="sm" ariaLabel={t("overview.trafficRange")}
                options={[
                  { value: "live" as const, label: t("overview.trafficModeLive") },
                  { value: "1h" as const, label: t("history.range1h") },
                  { value: "24h" as const, label: t("history.range24h") },
                ]}
                value={mode} onChange={setMode} />
              <div className="ml-auto flex items-center gap-4">
                <span className="stat-md inline-flex items-center gap-1 text-ok">
                  <ArrowDown size={16} aria-hidden="true" /> {fmtBytes(historyModel.totalDown)}
                </span>
                <span className="stat-md inline-flex items-center gap-1 text-accent">
                  <ArrowUp size={16} aria-hidden="true" /> {fmtBytes(historyModel.totalUp)}
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
            <p className="text-caption text-muted mt-2">
              {t("history.peakAt", { rate: fmtRate(historyModel.peak.rx), time: fmtTime(historyModel.peak.ts) })}
            </p>
          </>
        )
      ) : failed ? (
        <EmptyState small title={t("common.loadError")}
          illustration={<CloudOff size={24} />}
          action={<Button variant="secondary" size="sm" onClick={poll}>{t("common.retry")}</Button>} />
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-3 mb-2">
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
                <div className="ml-auto flex items-center gap-4">
                  <span className="stat-md inline-flex items-center gap-1 text-ok">
                    <ArrowDown size={16} aria-hidden="true" /> {fmtRate(rxNow)}
                  </span>
                  <span className="stat-md inline-flex items-center gap-1 text-accent">
                    <ArrowUp size={16} aria-hidden="true" /> {fmtRate(txNow)}
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
            <>
              <AreaChart rx={rxSeries} tx={txSeries} height={200} live
                ariaLabel={`${t("overview.trafficLive")}: ${fmtRate(rxNow)} ↓, ${t("traffic.peak")} ${fmtRate(peak)}`} />
              <p className="text-caption text-muted mt-2">
                {t("traffic.peak")} {fmtRate(peak)} · {t("traffic.avg")} {fmtRate(avg)} · {t("traffic.total")} {fmtBytes(total)}
              </p>
            </>
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

export function Overview({ board, system, wan, drift, onDriftChange, isSwitch, health, mode, onNavigate }: {
  board?: Board;
  system?: SystemInfo;
  wan?: WanStatus;
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
      {/* El enlace y su tráfico en vivo: 4 + 8 = una fila completa. */}
      {!isSwitch && <InternetCard wan={wan} mode={mode} />}
      <LiveTrafficCard />
      {/* Recursos del equipo, los tres juntos: 4 + 4 + 4. */}
      <CpuCard />
      <MemoryCard system={system} />
      <FlashCard system={system} />
      {/* El consumo (quién gasta más) vive en la página Consumo (#385) y el
          chasis de puertos en Puertos ethernet (#384). */}
      <ClientsCard clients={clients} onNavigate={onNavigate} />
      <DriftSection drift={drift} onChange={onDriftChange} />
    </div>
  );
}
