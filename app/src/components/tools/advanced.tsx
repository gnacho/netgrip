import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, Lock, ShieldAlert, ShieldCheck, X } from "lucide-react";
import { api } from "../../api";
import type { IGMPProbe, MACACLProbe, StormPort, StormProbe } from "../../types";
import { Banner, Button, Card, Pill, SegmentedControl, SettingRow, Toggle } from "../ui";
import { CardLoadError } from "./diagnostics";
import { PortAddPicker, PortOverrides } from "../ports/PortOverrides";

/**
 * Cajon avanzado (tools.md §4): IGMP snooping, control de tormentas y
 * listas MAC por boca. Todo con HelpTip, lejos del primer vistazo.
 * Las tres cards de por-boca siguen el patron "ajuste global +
 * excepciones" (#487): solo se listan las bocas con ajuste propio.
 */

/** IGMP snooping: SettingRow con toggle (`/api/igmp`). */
export function IgmpCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<IGMPProbe>();
  const [error, setError] = useState(false);
  const [busy, setBusy] = useState(false);
  const [failMsg, setFailMsg] = useState<string>();

  const load = useCallback(async () => {
    setError(false);
    try { setProbe(await api.igmp()); }
    catch { setError(true); }
  }, []);
  useEffect(() => { load(); }, [load]);

  if (error) {
    return (
      <Card variant="subtle" animate={false} title={t("tools.igmp")} icon={ShieldCheck}>
        <CardLoadError onRetry={load} />
      </Card>
    );
  }
  if (!probe || !probe.applicable) return null;

  const toggle = async (v: boolean) => {
    setBusy(true);
    setFailMsg(undefined);
    try { setProbe(await api.setIgmp(v)); }
    catch (e) { setFailMsg(e instanceof Error ? e.message : String(e)); }
    finally { setBusy(false); }
  };

  return (
    <Card variant="subtle" animate={false}>
      <SettingRow
        icon={ShieldCheck}
        title={t("tools.igmp")}
        description={t("tools.igmpDesc")}
        helpTitle={t("help.igmp.title")}
        help={t("help.igmp.body")}
        checked={probe.enabled}
        busy={busy}
        onChange={toggle}
      />
      {failMsg && <Banner tone="danger" onDismiss={() => setFailMsg(undefined)}>{failMsg}</Banner>}
    </Card>
  );
}

/** Percent comun de una lista (el modal; empates al menor, estable). */
function modalPercent(vals: number[]): number {
  const freq = new Map<number, number>();
  for (const v of vals) freq.set(v, (freq.get(v) ?? 0) + 1);
  return [...freq.entries()].sort((a, b) => b[1] - a[1] || a[0] - b[0])[0][0];
}

const LAN_FIRST = (a: { port: string }, b: { port: string }) =>
  a.port.localeCompare(b.port, undefined, { numeric: true });

/**
 * Control de tormentas (`/api/storm`, payload en %). El backend aplica el
 * mismo porcentaje a broadcast y multicast, asi que hay UNA sola fila
 * global con toggle + slider (estilo mockup) y una lista colapsada de
 * bocas con ajuste propio (#487). "Aplicar a todas" es una sola llamada
 * (`all: true`), no 52 POSTs.
 */
export function StormControlCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<StormProbe>();
  const [error, setError] = useState(false);
  const [busyAll, setBusyAll] = useState(false);
  const [busyPort, setBusyPort] = useState<string>();
  const [globalPct, setGlobalPct] = useState(10);
  const [appliedPct, setAppliedPct] = useState(0); // 0 = global apagado
  const [drafts, setDrafts] = useState<Record<string, number>>({});
  const [extras, setExtras] = useState<string[]>([]);
  const [failMsg, setFailMsg] = useState<string>();

  const load = useCallback(async () => {
    setError(false);
    try {
      const p = await api.stormControl();
      setProbe(p);
      // Global derivado del estado real: el percent comun de las bocas
      // configuradas (el modal si hay mezcla); sin ninguna, apagado.
      const configured = p.ports.filter((x) => x.percent > 0).map((x) => x.percent);
      const derived = configured.length ? modalPercent(configured) : 0;
      setAppliedPct(derived);
      setGlobalPct(derived || 10);
      setDrafts({});
      setExtras((prev) => prev.filter((port) => p.ports.some((x) => x.port === port)));
    } catch { setError(true); }
  }, []);
  useEffect(() => { load(); }, [load]);

  if (error) {
    return (
      <Card variant="subtle" animate={false} title={t("tools.stormControl")} icon={ShieldAlert}>
        <CardLoadError onRetry={load} />
      </Card>
    );
  }
  if (!probe?.applicable) return null;

  const wired = probe.ports.filter((p) => p.port.startsWith("lan")).sort(LAN_FIRST);
  // Excepcion: boca con limite propio distinto del global. El 0 nunca es
  // excepcion (storm.conf no distingue "sin tocar" de "fijado a 0": al
  // guardar un 0 la entrada desaparece y la boca vuelve al global).
  const isException = (sp: StormPort) => sp.percent > 0 && sp.percent !== appliedPct;
  const shown = wired.filter((sp) => isException(sp) || extras.includes(sp.port));
  const shownSet = new Set(shown.map((sp) => sp.port));
  const candidates = wired.filter((sp) => !shownSet.has(sp.port)).map((sp) => sp.port);

  const applyAll = async (pct: number) => {
    setBusyAll(true);
    setFailMsg(undefined);
    try {
      await api.setStormControlAll(pct);
      await load();
    } catch (e) {
      setFailMsg(e instanceof Error ? e.message : String(e));
    } finally { setBusyAll(false); }
  };

  const applyPort = async (port: string, pct: number) => {
    setBusyPort(port);
    setFailMsg(undefined);
    try {
      await api.setStormControl(port, pct);
      await load();
    } catch (e) {
      setFailMsg(`${port}: ${e instanceof Error ? e.message : String(e)}`);
    } finally { setBusyPort(undefined); }
  };

  const commitGlobal = () => {
    if (appliedPct > 0 && globalPct !== appliedPct) void applyAll(globalPct);
  };

  return (
    <Card variant="subtle" animate={false} title={t("tools.stormControl")} icon={ShieldAlert}>
      <p className="text-small text-muted mb-3">{t("tools.stormNote")}</p>
      {probe.tc_installed === false && (
        <p className="text-small text-muted mb-3">{t("tools.stormTcMissing")}</p>
      )}
      {/* Fila global estilo mockup: titulo+descripcion | slider | % | toggle. */}
      <div className="flex items-center gap-3 sm:gap-4 py-1">
        <div className="flex-1 min-w-0">
          <div className="text-small font-medium">{t("tools.stormGlobalTitle")}</div>
          <div className="text-caption text-muted mt-0.5">{t("tools.stormGlobalDesc")}</div>
        </div>
        <input
          type="range"
          min={1}
          max={100}
          value={globalPct}
          disabled={appliedPct === 0 || busyAll}
          aria-label={t("tools.stormGlobalTitle")}
          onChange={(e) => setGlobalPct(+e.target.value)}
          onPointerUp={commitGlobal}
          onTouchEnd={commitGlobal}
          onKeyUp={(e) => {
            if (e.key.startsWith("Arrow") || e.key === "Enter") commitGlobal();
          }}
          className="w-28 sm:w-44 shrink-0 accent-accent disabled:opacity-40"
        />
        <span className="w-20 sm:w-24 shrink-0 text-right text-caption tabular-nums text-muted">
          {appliedPct > 0 ? t("tools.stormPctOfLink", { pct: globalPct }) : t("tools.stormOff")}
        </span>
        <Toggle
          checked={appliedPct > 0}
          busy={busyAll}
          onChange={(v) => void applyAll(v ? globalPct : 0)}
          label={t("tools.stormGlobalTitle")}
        />
      </div>
      {/* Excepciones: solo bocas cuyo percent difiere del global. */}
      <PortOverrides count={shown.length}>
        {shown.map((sp) => {
          const pct = drafts[sp.port] ?? sp.percent;
          const commit = () => {
            if (pct !== sp.percent) void applyPort(sp.port, pct);
          };
          return (
            <div key={sp.port} className="flex items-center gap-2 sm:gap-3">
              <span className="w-14 shrink-0 font-mono text-small">{sp.port}</span>
              <input
                type="range"
                min={0}
                max={100}
                value={pct}
                disabled={busyPort === sp.port}
                aria-label={t("tools.stormAria", { port: sp.port })}
                onChange={(e) => setDrafts((prev) => ({ ...prev, [sp.port]: +e.target.value }))}
                onPointerUp={commit}
                onTouchEnd={commit}
                onKeyUp={(e) => {
                  if (e.key.startsWith("Arrow") || e.key === "Enter") commit();
                }}
                className="min-w-0 flex-1 accent-accent"
              />
              <span className="w-12 shrink-0 text-right text-caption tabular-nums text-muted">
                {pct > 0 ? `${pct} %` : t("tools.stormOff")}
              </span>
              <span className="hidden lg:block w-44 shrink-0 text-caption text-faint">
                {t("tools.stormNow", { broadcast: sp.broadcast_kbps, multicast: sp.multicast_kbps })}
              </span>
              <button
                type="button"
                onClick={() => void applyPort(sp.port, appliedPct)}
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

type MacAclMode = "off" | "allow" | "deny";

const MAC_PILL_TONE: Record<MacAclMode, "muted" | "ok" | "danger"> = {
  off: "muted",
  allow: "ok",
  deny: "danger",
};

/**
 * Listas MAC por boca (`/api/mac-acl`, #487): solo bocas configuradas
 * (modo != off) en filas densas con edicion inline desplegable; se anaden
 * bocas una a una con el picker. Nada de rejilla de 52 celdas.
 */
export function MacAclCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<MACACLProbe>();
  const [error, setError] = useState(false);
  const [busyPort, setBusyPort] = useState<string>();
  const [modes, setModes] = useState<Record<string, MacAclMode>>({});
  const [macText, setMacText] = useState<Record<string, string>>({});
  const [expanded, setExpanded] = useState<string[]>([]);
  const [extras, setExtras] = useState<string[]>([]);
  const [failMsg, setFailMsg] = useState<string>();

  const load = useCallback(async () => {
    setError(false);
    try {
      const p = await api.macAcl();
      setProbe(p);
      setExtras((prev) => prev.filter((port) => p.ports.some((x) => x.port === port)));
    } catch { setError(true); }
  }, []);
  useEffect(() => { load(); }, [load]);

  if (error) {
    return (
      <Card variant="subtle" animate={false} title={t("tools.macAclTitle")} icon={Lock} help="macacl">
        <CardLoadError onRetry={load} />
      </Card>
    );
  }
  if (!probe?.applicable) return null;

  const wired = probe.ports.filter((p) => p.port.startsWith("lan")).sort(LAN_FIRST);
  const shown = wired.filter((p) => p.mode !== "off" || extras.includes(p.port));
  const shownSet = new Set(shown.map((p) => p.port));
  const candidates = wired.filter((p) => !shownSet.has(p.port)).map((p) => p.port);

  /** Modo efectivo: el borrador local si existe, si no el del probe. */
  const modeOf = (port: string): MacAclMode => {
    const m = modes[port] ?? probe?.ports.find((x) => x.port === port)?.mode;
    return m === "allow" || m === "deny" ? m : "off";
  };

  const apply = async (port: string) => {
    setBusyPort(port);
    setFailMsg(undefined);
    try {
      const mode = modeOf(port);
      const macs = (macText[port] ?? "")
        .split("\n").map((m) => m.trim()).filter(Boolean);
      await api.setMacAcl(port, mode, macs);
      await load();
    } catch (e) {
      setFailMsg(`${port}: ${e instanceof Error ? e.message : String(e)}`);
    } finally { setBusyPort(undefined); }
  };

  const remove = async (port: string, mode: string) => {
    if (mode === "off") {
      setExtras((prev) => prev.filter((x) => x !== port));
      return;
    }
    setBusyPort(port);
    setFailMsg(undefined);
    try {
      await api.setMacAcl(port, "off", []);
      await load();
    } catch (e) {
      setFailMsg(`${port}: ${e instanceof Error ? e.message : String(e)}`);
    } finally { setBusyPort(undefined); }
  };

  const modeLabel: Record<MacAclMode, string> = {
    off: t("tools.macAclOff"),
    allow: t("tools.macAclAllow"),
    deny: t("tools.macAclDeny"),
  };

  return (
    <Card variant="subtle" animate={false} title={t("tools.macAclTitle")} icon={Lock} help="macacl">
      <p className="text-small text-muted mb-3">{t("tools.macAclIntro")}</p>
      <PortOverrides count={shown.length}>
        {shown.map((p) => {
          const mode = modeOf(p.port);
          const macs = macText[p.port] ?? p.macs.join("\n");
          const isOpen = expanded.includes(p.port);
          const dirty =
            mode !== (p.mode === "allow" || p.mode === "deny" ? p.mode : "off") ||
            macs !== p.macs.join("\n");
          return (
            <div key={p.port} className="rounded-md border border-border/60">
              <div className="flex items-center gap-2 px-2.5 py-2">
                <span className="w-14 shrink-0 font-mono text-small">{p.port}</span>
                <Pill tone={MAC_PILL_TONE[mode]}>{modeLabel[mode]}</Pill>
                <span className="text-caption text-muted">{t("tools.macAclCount", { count: p.macs.length })}</span>
                <button
                  type="button"
                  onClick={() =>
                    setExpanded((prev) =>
                      prev.includes(p.port) ? prev.filter((x) => x !== p.port) : [...prev, p.port],
                    )}
                  aria-label={`${t("tools.macAclEdit")} ${p.port}`}
                  className="ml-auto inline-flex h-8 w-8 items-center justify-center rounded-md text-muted hover:text-text transition-colors duration-[var(--dur-fast)] ring-focus"
                >
                  <ChevronDown
                    size={16}
                    aria-hidden="true"
                    className={`transition-transform duration-200 ease-[var(--ease-soft)] ${isOpen ? "rotate-180" : ""}`}
                  />
                </button>
                <button
                  type="button"
                  onClick={() => void remove(p.port, p.mode)}
                  disabled={busyPort === p.port}
                  title={`${t("ports.removeOverride")} (${p.port})`}
                  aria-label={`${t("ports.removeOverride")} (${p.port})`}
                  className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-faint hover:text-danger transition-colors duration-[var(--dur-fast)] ring-focus disabled:opacity-50"
                >
                  <X size={16} aria-hidden="true" />
                </button>
              </div>
              {isOpen && (
                <div className="border-t border-border/60 px-2.5 py-2.5">
                  <div className="flex flex-wrap items-center gap-2">
                    <SegmentedControl<MacAclMode>
                      size="sm"
                      ariaLabel={t("tools.macAclTitle")}
                      value={mode}
                      onChange={(v) => setModes((prev) => ({ ...prev, [p.port]: v }))}
                      options={[
                        { value: "off", label: t("tools.macAclOff") },
                        { value: "allow", label: t("tools.macAclAllow") },
                        { value: "deny", label: t("tools.macAclDeny") },
                      ]}
                    />
                    <Button
                      variant="secondary"
                      size="sm"
                      className="ml-auto"
                      loading={busyPort === p.port}
                      disabled={!dirty}
                      onClick={() => void apply(p.port)}
                    >
                      {t("tools.macAclApply")}
                    </Button>
                  </div>
                  {mode !== "off" && (
                    <textarea
                      value={macs}
                      rows={3}
                      onChange={(e) => setMacText((prev) => ({ ...prev, [p.port]: e.target.value }))}
                      placeholder={t("tools.macAclPlaceholder")}
                      className="mt-2 w-full rounded-[10px] border border-transparent bg-fill px-3 py-2 font-mono text-small
                        placeholder:text-faint outline-none transition-[background-color,border-color,box-shadow] duration-[var(--dur-fast)]
                        hover:border-border-strong focus:bg-surface focus:shadow-[0_0_0_2px_var(--color-accent)]"
                    />
                  )}
                </div>
              )}
            </div>
          );
        })}
        <PortAddPicker candidates={candidates} onAdd={(port) => setExtras((prev) => [...prev, port])} />
      </PortOverrides>
      {failMsg && <Banner tone="danger" className="mt-2" onDismiss={() => setFailMsg(undefined)}>{failMsg}</Banner>}
    </Card>
  );
}
