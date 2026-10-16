import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Eye, Gauge, TriangleAlert, X } from "lucide-react";
import { api } from "../../api";
import type { MirrorProbe, RateProbe } from "../../types";
import { Banner, Button, Card, ConfirmDialog, EmptyState, Pill, Toggle } from "../ui";
import { CardLoadError } from "../tools/diagnostics";
import { PortAddPicker, PortOverrides } from "./PortOverrides";

/**
 * Pestaña "Tráfico" de Puertos (#443): espejo de puertos (tc mirred) y
 * banda ancha por boca (police de entrada + tbf de salida). Ambas cards se
 * auto-ocultan si su probe confirma que no aplican, y reportan "no
 * soportado por este dispositivo" cuando el probe de capacidad falla.
 */

const LAN_FIRST = (a: { port: string }, b: { port: string }) =>
  a.port.localeCompare(b.port, undefined, { numeric: true });

/** Percent comun de una lista (empates al menor, igual que storm). */
function modalPercent(vals: number[]): number {
  const freq = new Map<number, number>();
  for (const v of vals) freq.set(v, (freq.get(v) ?? 0) + 1);
  return [...freq.entries()].sort((a, b) => b[1] - a[1] || a[0] - b[0])[0][0];
}

/**
 * Espejo de puertos (`/api/mirror`): una boca monitor recibe copia de
 * todo el tráfico (en ambos sentidos) de las bocas origen. Configuración
 * con borrador + botón Aplicar; si el borrador toca la boca de gestión
 * (la del uplink, heurística del backend) se pide confirmación explícita.
 */
export function MirrorCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<MirrorProbe>();
  const [error, setError] = useState(false);
  const [busy, setBusy] = useState(false);
  const [failMsg, setFailMsg] = useState<string>();
  const [confirmApply, setConfirmApply] = useState(false);

  // Borrador: parte de la config real y se edita hasta "Aplicar".
  const [draftMonitor, setDraftMonitor] = useState("");
  const [draftSources, setDraftSources] = useState<string[]>([]);
  const [syncKey, setSyncKey] = useState("");

  const load = useCallback(async () => {
    setError(false);
    try {
      const p = await api.mirror();
      // Defensa ante un sources null del backend (espejo nunca
      // configurado): la card hace join() sobre el y crashearia.
      setProbe({ ...p, sources: p.sources ?? [] });    } catch { setError(true); }
  }, []);
  useEffect(() => { load(); }, [load]);

  // Sincroniza el borrador cuando cambia la config real (carga inicial y
  // tras aplicar); las ediciones del usuario no tocan el probe, asi que no
  // lo pisan.
  useEffect(() => {
    if (!probe) return;
    const key = `${probe.monitor}|${probe.sources.join(",")}`;
    if (key !== syncKey) {
      setSyncKey(key);
      setDraftMonitor(probe.monitor);
      setDraftSources(probe.sources);
    }
  }, [probe, syncKey]);

  if (error) {
    return (
      <Card variant="subtle" animate={false} title={t("ports.mirrorTitle")} icon={Eye}>
        <CardLoadError onRetry={load} />
      </Card>
    );
  }
  if (!probe?.applicable) return null;

  const supported = probe.mirror_supported !== false;
  const wired = [...probe.ports].sort(LAN_FIRST);
  const activeByPort = new Map(probe.ports.map((p) => [p.port, p.active]));

  const dirty = draftMonitor !== probe.monitor ||
    draftSources.join(",") !== probe.sources.join(",");

  const valid = draftMonitor !== "" && draftSources.length > 0 && dirty;

  // Avisos de seguridad (#443): la heurística del backend marca la boca
  // por la que el equipo sale al gateway (típicamente la de gestión).
  const mgmt = probe.management_port ?? "";
  const warnMonitor = mgmt !== "" && draftMonitor === mgmt;
  const warnSource = mgmt !== "" && draftSources.includes(mgmt);

  const apply = async () => {
    setBusy(true);
    setFailMsg(undefined);
    try {
      await api.setMirror(draftMonitor, draftSources);
      await load();
    } catch (e) {
      setFailMsg(e instanceof Error ? e.message : String(e));
    } finally { setBusy(false); }
  };

  const disable = async () => {
    setBusy(true);
    setFailMsg(undefined);
    try {
      await api.setMirror("", []);
      await load();
    } catch (e) {
      setFailMsg(e instanceof Error ? e.message : String(e));
    } finally { setBusy(false); }
  };

  const requestApply = () => {
    if (!valid) return;
    if (warnMonitor || warnSource) setConfirmApply(true);
    else void apply();
  };

  const removeSource = (port: string) =>
    setDraftSources((prev) => prev.filter((x) => x !== port));

  const candidates = wired
    .filter((p) => p.port !== draftMonitor && !draftSources.includes(p.port))
    .map((p) => p.port);

  return (
    <Card variant="subtle" animate={false} title={t("ports.mirrorTitle")} icon={Eye} help="mirror">
      <p className="text-small text-muted mb-3">{t("ports.mirrorDesc")}</p>
      {!supported ? (
        <EmptyState small title={t("ports.mirrorUnsupported")} />
      ) : (
        <>
          {probe.tc_installed === false && (
            <p className="text-small text-muted mb-3">{t("ports.tcMissing")}</p>
          )}
          {(warnMonitor || warnSource) && (
            <Banner tone="warn" className="mb-3" icon={TriangleAlert}>
              {warnMonitor ? t("ports.warnMgmtMonitor", { port: mgmt }) : t("ports.warnMgmtSource", { port: mgmt })}
            </Banner>
          )}
          {/* Boca monitor */}
          <div className="flex flex-wrap items-center gap-2 py-1">
            <span className="w-28 shrink-0 text-small font-medium">{t("ports.monitorPort")}</span>
            <select
              value={draftMonitor}
              disabled={busy}
              onChange={(e) => setDraftMonitor(e.target.value)}
              aria-label={t("ports.monitorPort")}
              className="h-9 w-40 max-w-full rounded-sm border border-border bg-surface-2 px-3 text-small outline-none focus:border-accent ring-focus disabled:opacity-40"
            >
              <option value="">{t("ports.monitorNone")}</option>
              {wired.map((p) => <option key={p.port} value={p.port}>{p.port}</option>)}
            </select>
            <span className="text-caption text-muted">{t("ports.monitorHint")}</span>
          </div>
          {/* Bocas origen */}
          <div className="mt-2">
            <div className="text-small font-medium mb-1">{t("ports.sources")}</div>
            {draftSources.length === 0 ? (
              <p className="text-caption text-muted mb-2">{t("ports.noSources")}</p>
            ) : (
              <div className="flex flex-col gap-1.5 mb-2">
                {draftSources.map((port) => (
                  <div key={port} className="flex items-center gap-2 sm:gap-3">
                    <span className="w-14 shrink-0 font-mono text-small">{port}</span>
                    {activeByPort.get(port) && <Pill tone="ok">{t("ports.mirrorActive")}</Pill>}
                    {mgmt === port && <Pill tone="muted">{t("ports.mgmtPill")}</Pill>}
                    <button
                      type="button"
                      onClick={() => removeSource(port)}
                      disabled={busy}
                      title={`${t("ports.removeOverride")} (${port})`}
                      aria-label={`${t("ports.removeOverride")} (${port})`}
                      className="ml-auto inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-faint hover:text-danger transition-colors duration-[var(--dur-fast)] ring-focus disabled:opacity-50"
                    >
                      <X size={16} aria-hidden="true" />
                    </button>
                  </div>
                ))}
              </div>
            )}
            <PortAddPicker candidates={candidates} onAdd={(port) => setDraftSources((prev) => [...prev, port])} />
          </div>
          <div className="mt-3 flex items-center gap-2">
            <Button variant="primary" size="sm" loading={busy} disabled={!valid} onClick={requestApply}>
              {t("ports.apply")}
            </Button>
            {probe.monitor !== "" && (
              <Button
                variant="secondary"
                size="sm"
                loading={busy}
                disabled={busy}
                onClick={() => void disable()}
              >
                {t("ports.mirrorOff")}
              </Button>
            )}
          </div>
        </>
      )}
      {failMsg && <Banner tone="danger" className="mt-2" onDismiss={() => setFailMsg(undefined)}>{failMsg}</Banner>}
      <ConfirmDialog
        open={confirmApply}
        onClose={() => setConfirmApply(false)}
        onConfirm={() => { setConfirmApply(false); void apply(); }}
        title={t("ports.warnConfirmTitle")}
        consequence={warnMonitor
          ? t("ports.warnMgmtMonitor", { port: mgmt })
          : t("ports.warnMgmtSource", { port: mgmt })}
        confirmLabel={t("ports.apply")}
        busy={busy}
      />
    </Card>
  );
}

/**
 * Banda ancha por boca (`/api/ratelimit`): límites de entrada y salida en
 * % del ancho de enlace, con el patrón "global + excepciones" de storm
 * (#487): una fila global y una lista colapsada de bocas con ajuste propio.
 */
export function RateLimitCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<RateProbe>();
  const [error, setError] = useState(false);
  const [busyAll, setBusyAll] = useState(false);
  const [busyPort, setBusyPort] = useState<string>();
  const [globalPct, setGlobalPct] = useState(10);
  const [appliedPct, setAppliedPct] = useState(0); // 0 = global apagado
  const [drafts, setDrafts] = useState<Record<string, { in: number; out: number }>>({});
  const [extras, setExtras] = useState<string[]>([]);
  const [failMsg, setFailMsg] = useState<string>();

  const load = useCallback(async () => {
    setError(false);
    try {
      const p = await api.rateLimit();
      setProbe(p);
      const configured = p.ports.filter((x) => x.ingress_percent > 0 || x.egress_percent > 0);
      const derived = configured.length ? modalPercent(configured.map((x) => x.ingress_percent || x.egress_percent)) : 0;
      setAppliedPct(derived);
      setGlobalPct(derived || 10);
      setDrafts({});
      setExtras((prev) => prev.filter((port) => p.ports.some((x) => x.port === port)));
    } catch { setError(true); }
  }, []);
  useEffect(() => { load(); }, [load]);

  if (error) {
    return (
      <Card variant="subtle" animate={false} title={t("ports.rateTitle")} icon={Gauge}>
        <CardLoadError onRetry={load} />
      </Card>
    );
  }
  if (!probe?.applicable) return null;

  const ingressOK = probe.ingress_supported !== false;
  const egressOK = probe.egress_supported !== false;
  if (!ingressOK && !egressOK) {
    return (
      <Card variant="subtle" animate={false} title={t("ports.rateTitle")} icon={Gauge} help="ratelimit">
        <p className="text-small text-muted mb-3">{t("ports.rateDesc")}</p>
        <EmptyState small title={t("ports.rateUnsupported")} />
      </Card>
    );
  }

  const wired = probe.ports.filter((p) => p.port.startsWith("lan")).sort(LAN_FIRST);
  const pair = (p: { ingress_percent: number; egress_percent: number }) => ({
    in: p.ingress_percent,
    out: egressOK ? p.egress_percent : 0,
  });
  // Excepción: boca con límite propio distinto del global (o del 0 global).
  const isException = (p: (typeof wired)[number]) => {
    const c = pair(p);
    return (c.in > 0 || c.out > 0) && (c.in !== appliedPct || c.out !== appliedPct);
  };
  const shown = wired.filter((p) => isException(p) || extras.includes(p.port));
  const shownSet = new Set(shown.map((p) => p.port));
  const candidates = wired.filter((p) => !shownSet.has(p.port)).map((p) => p.port);

  const applyAll = async (pct: number) => {
    setBusyAll(true);
    setFailMsg(undefined);
    try {
      await api.setRateLimitAll(pct, egressOK ? pct : 0);
      await load();
    } catch (e) {
      setFailMsg(e instanceof Error ? e.message : String(e));
    } finally { setBusyAll(false); }
  };

  const applyPort = async (port: string, inPct: number, outPct: number) => {
    setBusyPort(port);
    setFailMsg(undefined);
    try {
      await api.setRateLimit(port, inPct, egressOK ? outPct : 0);
      await load();
    } catch (e) {
      setFailMsg(`${port}: ${e instanceof Error ? e.message : String(e)}`);
    } finally { setBusyPort(undefined); }
  };

  const commitGlobal = () => {
    if (appliedPct > 0 && globalPct !== appliedPct) void applyAll(globalPct);
  };

  return (
    <Card variant="subtle" animate={false} title={t("ports.rateTitle")} icon={Gauge} help="ratelimit">
      <p className="text-small text-muted mb-3">{t("ports.rateDesc")}</p>
      {probe.tc_installed === false && (
        <p className="text-small text-muted mb-3">{t("ports.tcMissing")}</p>
      )}
      {!egressOK && (
        <p className="text-small text-muted mb-3">{t("ports.egressUnsupported")}</p>
      )}
      {egressOK && (
        <p className="text-small text-muted mb-3">{t("ports.egressNote")}</p>
      )}
      {/* Fila global: titulo+descripcion | slider | % | toggle. */}
      <div className="flex items-center gap-3 sm:gap-4 py-1">
        <div className="flex-1 min-w-0">
          <div className="text-small font-medium">{t("ports.globalTitle")}</div>
          <div className="text-caption text-muted mt-0.5">{t("ports.globalDesc")}</div>
        </div>
        <input
          type="range"
          min={1}
          max={100}
          value={globalPct}
          disabled={appliedPct === 0 || busyAll || !ingressOK}
          aria-label={t("ports.globalTitle")}
          onChange={(e) => setGlobalPct(+e.target.value)}
          onPointerUp={commitGlobal}
          onTouchEnd={commitGlobal}
          onKeyUp={(e) => {
            if (e.key.startsWith("Arrow") || e.key === "Enter") commitGlobal();
          }}
          className="w-28 sm:w-44 shrink-0 accent-accent disabled:opacity-40"
        />
        <span className="w-20 sm:w-24 shrink-0 text-right text-caption tabular-nums text-muted">
          {appliedPct > 0 ? t("ports.pctOfLink", { pct: globalPct }) : t("ports.rateOff")}
        </span>
        <Toggle
          checked={appliedPct > 0}
          busy={busyAll}
          onChange={(v) => void applyAll(v ? globalPct : 0)}
          label={t("ports.globalTitle")}
        />
      </div>
      {/* Excepciones: solo bocas cuyo límite difiere del global. */}
      <PortOverrides count={shown.length}>
        {shown.map((sp) => {
          const cur = pair(sp);
          const d = drafts[sp.port] ?? cur;
          const commit = () => {
            if (d.in !== cur.in || d.out !== cur.out) void applyPort(sp.port, d.in, d.out);
          };
          return (
            <div key={sp.port} className="flex items-center gap-2 sm:gap-3">
              <span className="w-14 shrink-0 font-mono text-small">{sp.port}</span>
              <input
                type="range"
                min={0}
                max={100}
                value={d.in}
                disabled={busyPort === sp.port || !ingressOK}
                aria-label={`${t("ports.ingress")} ${sp.port}`}
                onChange={(e) => setDrafts((prev) => ({ ...prev, [sp.port]: { ...d, in: +e.target.value } }))}
                onPointerUp={commit}
                onTouchEnd={commit}
                onKeyUp={(e) => {
                  if (e.key.startsWith("Arrow") || e.key === "Enter") commit();
                }}
                className="min-w-0 flex-1 accent-accent disabled:opacity-40"
              />
              <span className="w-12 shrink-0 text-right text-caption tabular-nums text-muted">
                {d.in > 0 ? `${d.in} %` : t("ports.rateOff")}
              </span>
              {egressOK && (
                <>
                  <input
                    type="range"
                    min={0}
                    max={100}
                    value={d.out}
                    disabled={busyPort === sp.port}
                    aria-label={`${t("ports.egress")} ${sp.port}`}
                    onChange={(e) => setDrafts((prev) => ({ ...prev, [sp.port]: { ...d, out: +e.target.value } }))}
                    onPointerUp={commit}
                    onTouchEnd={commit}
                    onKeyUp={(e) => {
                      if (e.key.startsWith("Arrow") || e.key === "Enter") commit();
                    }}
                    className="min-w-0 flex-1 accent-accent disabled:opacity-40"
                  />
                  <span className="w-12 shrink-0 text-right text-caption tabular-nums text-muted">
                    {d.out > 0 ? `${d.out} %` : t("ports.rateOff")}
                  </span>
                </>
              )}
              <button
                type="button"
                onClick={() => void applyPort(sp.port, appliedPct, egressOK ? appliedPct : 0)}
                disabled={busyPort === sp.port}
                title={`${t("ports.removeOverride")} (${sp.port})`}
                aria-label={`${t("ports.removeOverride")} (${sp.port})`}
                className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-faint hover:text-danger transition-colors duration-[var(--dur-fast)] ring-focus disabled:opacity-50"
              >
                <X size={16} aria-hidden="true" />
              </button>
            </div>
          );
        })}
        <PortAddPicker candidates={candidates} onAdd={(port) => setExtras((prev) => [...prev, port])} />
      </PortOverrides>
      {failMsg && <Banner tone="danger" className="mt-2" onDismiss={() => setFailMsg(undefined)}>{failMsg}</Banner>}
    </Card>
  );
}
