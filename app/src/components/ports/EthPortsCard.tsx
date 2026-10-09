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
export function EthPortsCard({ ports, index = 0 }: { ports?: EthPort[]; index?: number }) {
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
  const [advanced, setAdvanced] = useState(false);
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
    api.advanced().then((p) => setAdvanced(p.advanced)).catch(() => {});
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
    if (phys?.applicable) for (const p of phys.ports) m.set(p.name, p);
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

  return (
    <Card index={index} className="md:col-span-2"
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
              const swp = swByName.get(p.name);
              const adminDown = swp ? !swp.admin_up : false;
              const label = portLabel(p);
              const device = !p.up ? t("ports.free")
                : p.devices.length === 1 ? p.devices[0].name || p.devices[0].mac
                : p.devices.length > 1 ? t("ports.unmanagedN", { count: p.devices.length })
                : t("ports.busy");
              const led = adminDown ? "bg-danger"
                : !p.up ? "bg-border-strong"
                : p.speed_mbps >= 1000 ? "bg-ok" : "bg-warn";
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

              {/* Configuración física: solo con modo avanzado */}
              {advanced && selectedPhys && (
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
