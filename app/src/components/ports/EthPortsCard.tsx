import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Cable, Pencil, Zap } from "lucide-react";
import { api } from "../../api";
import type { EthPort, PhysPortsProbe, SwitchPort, SwitchProbe } from "../../types";
import {
  ActionBanner, Button, Card, ConfirmDialog, EmptyState, Input, Pill, SkeletonRows, Toggle,
} from "../ui";
import { IlluPlug } from "../ui/illustrations";
import { useActionCycle } from "../wifi/action";
import { PhysConfigSection, SfpModuleSection } from "./PhysPortDetail";

/** Título de tarjeta a una línea (design-rev2 §3): ellipsis + tooltip nativo. */
function oneLine(text: string) {
  return <span className="block truncate" title={text}>{text}</span>;
}

/**
 * Chasis RJ45 de la página Puertos ethernet (#384). Cada boca es un botón:
 * al pulsarla se selecciona y bajo el chasis aparece una banda de detalle
 * con nota, estado admin y PoE (del SwitchProbe, fusionado por nombre de
 * puerto; en routers/AP sin switch aplicable solo se muestra MAC/velocidad).
 * La antigua tarjeta "Las bocas del switch" (#484) desaparece: su
 * funcionalidad vive en esta banda.
 */
export function EthPortsCard({ ports, index = 0, className = "md:col-span-2" }: {
  ports?: EthPort[];
  index?: number;
  /** Ancho de la retícula: por defecto media página (2/2); el Resumen de
   *  switch (#487) lo pasa a 12/12 para el chasis a ancho completo. */
  className?: string;
}) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<string>();
  const [probe, setProbe] = useState<SwitchProbe>();
  const [busyPort, setBusyPort] = useState<string>();
  const [editDesc, setEditDesc] = useState<{ name: string; value: string }>();
  const [confirmOff, setConfirmOff] = useState<SwitchPort>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const { phase, detail, busy, run, clear } = useActionCycle();

  // Config física y SFP (#485): el modo avanzado se consulta una vez; el
  // probe de puertos fisicos se carga la primera vez que se abre el detalle
  // de una boca (52 ethtool tardan un par de segundos).
  const [phys, setPhys] = useState<PhysPortsProbe>();
  const physRequested = useRef(false);

  const load = useCallback(async () => {
    try {
      setProbe(await api.switchPorts());
    } catch {
      // Sin datos de switch (router/AP) el chasis funciona igual, sin detalle.
    }
  }, []);

  useEffect(() => { load(); }, [load]);
  useEffect(() => {
  }, []);
  const loadPhys = useCallback(async () => {
    try { setPhys(await api.physPorts()); } catch { /* sin probe, sin bloque */ }
  }, []);
  useEffect(() => {
    if (selected && !physRequested.current) {
      physRequested.current = true;
      void loadPhys();
    }
  }, [selected, loadPhys]);

  const physByName = useMemo(() => {
    const m = new Map<string, PhysPortsProbe["ports"][number]>();
    // #534: supported/ports pueden llegar nulos desde un probe viejo o
    // incompleto; el chasis no debe caerse por un array ausente.
    if (phys?.applicable) for (const p of phys.ports ?? []) m.set(p.name, p);
    return m;
  }, [phys]);

  const sorted = useMemo(() => !ports ? [] : [...ports].sort((a, b) => {
    if (a.wan !== b.wan) return a.wan ? -1 : 1;
    return a.name.localeCompare(b.name, undefined, { numeric: true });
  }), [ports]);

  // Fusion defensiva por nombre: solo enriquece bocas que existen en ambos.
  const swByName = useMemo(() => {
    const m = new Map<string, SwitchPort>();
    if (probe?.applicable) for (const p of probe.ports) m.set(p.name, p);
    return m;
  }, [probe]);

  const selectedPort = sorted.find((p) => p.name === selected);
  const selectedSw = selectedPort ? swByName.get(selectedPort.name) : undefined;
  const selectedPhys = selectedPort ? physByName.get(selectedPort.name) : undefined;

  const setAdmin = (port: SwitchPort, up: boolean) => {
    setBusyPort(port.name);
    run(() => api.setSwitchPort({ name: port.name, admin_up: up })).then((res) => {
      setBusyPort(undefined);
      if (res?.status === "applied") setProbe(res.state);
    });
  };

  const togglePoe = async (port: SwitchPort) => {
    setBusyPort(port.name + "-poe"); setMsg(undefined);
    try {
      const res = await api.setSwitchPort({ name: port.name, poe_enabled: !port.poe_enabled });
      if (res.status === "applied") setProbe(res.state);
      else setMsg({ tone: "danger", text: res.error || t("fwd.failed") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally { setBusyPort(undefined); }
  };

  const saveDesc = async () => {
    if (!editDesc) return;
    setBusyPort(editDesc.name); setMsg(undefined);
    try {
      const res = await api.setSwitchPort({ name: editDesc.name, description: editDesc.value });
      if (res.status === "applied") {
        setProbe(res.state);
        setEditDesc(undefined);
        setMsg({ tone: "ok", text: t("ports.noteSaved") });
      } else {
        setMsg({ tone: "danger", text: res.error || t("fwd.failed") });
      }
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally { setBusyPort(undefined); }
  };

  const portLabel = (p: EthPort) =>
    p.wan ? "WAN" : p.name.toUpperCase().replace(/^LAN(\d+)$/, "LAN $1");

  const macLine = (p: EthPort) =>
    `${p.devices[0]?.mac ?? ""}${p.up && p.speed_mbps > 0
      ? ` · ${p.speed_mbps >= 1000 ? `${p.speed_mbps / 1000} Gb/s` : `${p.speed_mbps} Mb/s`}`
      : ""}`;

  // #523: nivel de velocidad de una boca -> tono y número de barras del
  // chasis (8 = 1G+, 5 = 100M, 2 = 10M, 0 = sin enlace). El tono marca el
  // caso accionable: activo a velocidad reducida (ámbar) es el cable de 4
  // hilos del banner #522.
  const speedTier = (p: EthPort): { tone: "ok" | "warn" | "danger" | "off"; bars: number; label: string } => {
    if (!p.up || p.speed_mbps <= 0) return { tone: "off", bars: 0, label: "" };
    if (p.speed_mbps >= 1000) return { tone: "ok", bars: 8, label: `${p.speed_mbps >= 1000 ? `${p.speed_mbps / 1000} G` : p.speed_mbps}` };
    if (p.speed_mbps >= 100) return { tone: "warn", bars: 5, label: `${p.speed_mbps}M` };
    return { tone: "danger", bars: 2, label: `${p.speed_mbps}M` };
  };
  const tierBar = { ok: "bg-ok", warn: "bg-warn", danger: "bg-danger", off: "bg-border" } as const;
  // Leyenda: PoE solo si alguna boca lo soporta (#523).
  const anyPoe = useMemo(() => sorted.some((p) => swByName.get(p.name)?.poe_supported), [sorted, swByName]);

  return (
    <Card index={index} className={className}
      title={oneLine(t("overview.ports"))} icon={Cable} iconTone="teal">
      {!ports ? <SkeletonRows rows={3} /> : sorted.length === 0 ? (
        <EmptyState small illustration={<IlluPlug size={120} />} title={t("overview.portsEmpty")} />
      ) : (
        <>
          <p className="text-caption text-muted mb-3">
            {t("ports.inUse", { used: sorted.filter((p) => p.up).length, total: sorted.length })}
          </p>
          {/* chasis (#523): boca + barras de velocidad + badge PoE +
              etiqueta + dispositivo; bajo el chasis, leyenda de estados. */}
          <div className="flex flex-wrap gap-x-3 gap-y-4 rounded-md border border-border bg-surface-2 px-3 py-3">
            {sorted.map((p) => {
              const swp = swByName.get(p.name);
              const adminDown = swp ? !swp.admin_up : false;
              const label = portLabel(p);
              const device = !p.up ? t("ports.free")
                : p.devices.length === 1 ? p.devices[0].name || p.devices[0].mac
                : p.devices.length > 1 ? t("ports.unmanagedN", { count: p.devices.length })
                : t("ports.busy");
              const tier = speedTier(p);
              const led = adminDown ? "bg-danger"
                : !p.up ? "bg-border-strong"
                : tier.tone === "ok" ? "bg-ok" : tier.tone === "warn" ? "bg-warn" : "bg-danger";
              return (
                <button key={p.name} type="button"
                  onClick={() => setSelected(selected === p.name ? undefined : p.name)}
                  title={p.up ? `${label} · ${p.speed_mbps >= 1000 ? `${p.speed_mbps / 1000} Gb/s` : `${p.speed_mbps} Mb/s`} · ${device}` : label}
                  className={`flex w-[68px] flex-col items-center gap-1 rounded-sm p-1 ring-focus transition-colors hover:bg-surface
                    ${selected === p.name ? "bg-surface shadow-card" : ""}`}>
                  {/* LEDs como en un RJ45 real: uno a cada extremo, alineados
                      con los bordes del icono (mismo w-10), no de la baldosa.
                      El badge PoE va en la esquina del icono. */}
                  <span className="flex w-10 items-center justify-between px-1" aria-hidden="true">
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
                    {swp?.poe_supported && (
                      <Zap size={11} strokeWidth={2.5}
                        aria-label={t("ports.legendPoe")}
                        className={`absolute -top-1.5 -right-1.5 rounded-full bg-surface-2 p-px ${swp.poe_enabled ? "text-warn" : "text-faint"}`} />
                    )}
                  </span>
                  {/* Barras de velocidad: cuántas van encendidas y en qué
                      color dicen el nivel de un vistazo (tipo señal). */}
                  <span className="flex items-end gap-[2px] h-3" aria-hidden="true">
                    {Array.from({ length: 8 }).map((_, i) => (
                      <span key={i} style={{ height: `${4 + i}px` }}
                        className={`w-[3px] rounded-full ${i < tier.bars ? tierBar[tier.tone] : "bg-border/60"}`} />
                    ))}
                  </span>
                  <span className={`text-[10px] font-semibold font-mono tracking-wide ${p.wan ? "text-accent" : "text-muted"}`}>
                    {label}
                  </span>
                  {tier.label && (
                    <span className={`-mt-0.5 text-[10px] font-bold font-mono ${
                      tier.tone === "ok" ? "text-ok" : tier.tone === "warn" ? "text-warn" : "text-danger"
                    }`}>
                      {tier.label}
                    </span>
                  )}
                  <span className="w-full truncate text-center text-[11px] text-text">{device}</span>
                </button>
              );
            })}
          </div>
          {/* Leyenda del chasis (#523) */}
          <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-[10px] text-muted">
            <span className="inline-flex items-center gap-1.5"><span className="h-1.5 w-1.5 rounded-full bg-ok" />{t("ports.legendActive")}</span>
            <span className="inline-flex items-center gap-1.5"><span className="h-1.5 w-1.5 rounded-full bg-warn" />{t("ports.legendReduced")}</span>
            <span className="inline-flex items-center gap-1.5"><span className="h-1.5 w-1.5 rounded-full bg-border-strong" />{t("ports.legendFree")}</span>
            {anyPoe && (
              <span className="inline-flex items-center gap-1.5"><Zap size={11} strokeWidth={2.5} className="text-warn" />{t("ports.legendPoe")}</span>
            )}
          </div>

          {/* Banda de detalle de la boca seleccionada (#484) */}
          {selectedPort && !selectedSw && (
            <div className="mt-3 flex items-center justify-between gap-2 text-small">
              <span className="text-muted font-mono text-caption">{macLine(selectedPort)}</span>
            </div>
          )}
          {selectedPort && selectedSw && (
            <div className="mt-3 rounded-md border border-border bg-surface-2 p-3">
              <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
                <span
                  aria-hidden="true"
                  className={`h-2 w-2 rounded-full shrink-0 ${selectedSw.oper_up ? "bg-ok" : "bg-faint"}`}
                />
                <span className="font-mono text-small font-medium">{portLabel(selectedPort)}</span>
                <Pill tone={selectedSw.admin_up ? "ok" : "muted"} live={selectedSw.oper_up}>
                  {selectedSw.oper_up ? t("ports.up") : t("ports.down")}
                </Pill>
                {!selectedSw.admin_up && <Pill tone="danger">{t("ports.disabled")}</Pill>}
                <span className="font-mono text-caption text-muted">
                  {selectedSw.oper_up && selectedSw.speed_mbps > 0
                    ? `${selectedSw.speed_mbps >= 1000 ? `${selectedSw.speed_mbps / 1000} Gb/s` : `${selectedSw.speed_mbps} Mb/s`}`
                    : macLine(selectedPort)}
                </span>
                <span className="ml-auto flex items-center gap-4">
                  {selectedSw.poe_supported && (
                    <span className="flex items-center gap-1.5">
                      <Zap size={12} className="text-faint" aria-hidden="true" />
                      <span className="text-caption text-muted">PoE</span>
                      <Toggle
                        checked={selectedSw.poe_enabled}
                        busy={busyPort === selectedSw.name + "-poe"}
                        onChange={() => togglePoe(selectedSw)}
                        label={`PoE ${selectedSw.name}`}
                      />
                    </span>
                  )}
                  <span className="flex items-center gap-1.5">
                    <span className="text-caption text-muted">{t("ports.admin")}</span>
                    <Toggle
                      checked={selectedSw.admin_up}
                      busy={busyPort === selectedSw.name || busy}
                      onChange={(v) => (v ? setAdmin(selectedSw, true) : setConfirmOff(selectedSw))}
                      label={selectedSw.name}
                    />
                  </span>
                </span>
              </div>

              {/* Nota editable (lápiz ghost) */}
              <div className="mt-2 min-h-6">
                {editDesc?.name === selectedSw.name ? (
                  <div className="flex gap-1.5">
                    <Input
                      value={editDesc.value}
                      onChange={(e) => setEditDesc({ ...editDesc, value: e.target.value })}
                      autoFocus
                      onKeyDown={(e) => e.key === "Enter" && saveDesc()}
                      aria-label={t("ports.note")}
                      placeholder={t("ports.notePlaceholder")}
                      className="!h-9 text-small"
                    />
                    <Button size="sm" onClick={saveDesc} loading={busyPort === selectedSw.name}>{t("lan.save")}</Button>
                    <Button size="sm" variant="ghost" onClick={() => setEditDesc(undefined)}>{t("common.cancel")}</Button>
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={() => setEditDesc({ name: selectedSw.name, value: selectedSw.description })}
                    title={t("ports.editNote")}
                    className="flex items-center gap-1.5 text-small text-muted hover:text-text transition-colors duration-[var(--dur-fast)] ring-focus rounded-sm max-w-full"
                  >
                    <Pencil size={12} className="shrink-0 text-faint" aria-hidden="true" />
                    <span className="truncate">{selectedSw.description || t("ports.noteAdd")}</span>
                  </button>
                )}
              </div>

              {phase && (
                <div className="mt-3">
                  <ActionBanner phase={phase} detail={detail} onDone={clear} />
                </div>
              )}
              {msg && <p className={`text-caption mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}

              {/* Módulo óptico (jaulas SFP, #485) */}
              {selectedPhys?.sfp && <SfpModuleSection sfp={selectedPhys.sfp} />}

              {/* Configuración física (#500: el gate de modo avanzado se
                  retiró; queda por selección de boca). Sin enlace no hay
                  nada que configurar: el driver rechaza esos writes y la
                  boca solo necesita estado, nota y admin (#535). */}
              {selectedPhys && selectedSw.oper_up && (
                <PhysConfigSection
                  phys={selectedPhys}
                  busy={busyPort === selectedSw.name}
                  onChanged={() => void loadPhys()}
                />
              )}
            </div>
          )}
        </>
      )}

      <ConfirmDialog
        open={!!confirmOff}
        onClose={() => setConfirmOff(undefined)}
        onConfirm={() => { const p = confirmOff; setConfirmOff(undefined); if (p) setAdmin(p, false); }}
        title={t("ports.adminOffTitle", { port: confirmOff?.name ?? "" })}
        consequence={t("ports.adminOffConsequence")}
        confirmLabel={t("ports.adminOffConfirm")}
      />
    </Card>
  );
}
