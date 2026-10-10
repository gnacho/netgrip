import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, GitBranch, ShieldAlert, X } from "lucide-react";
import { api } from "../../api";
import type { STPBridgeEdit, STPPort, STPProbe } from "../../types";
import { Banner, Button, Card, ConfirmDialog, Input, SkeletonRows, Toggle } from "../ui";
import { PortAddPicker, PortOverrides } from "./PortOverrides";

/**
 * STP 802.1d del kernel (#485, rediseño de por-boca en #487). Las columnas
 * Borde (portfast) y Punto a punto no existen en el STP del kernel;
 * RSTP/portfast requeriria mstpd (#449).
 */

export function StpBridgeCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<STPProbe>();
  const [error, setError] = useState(false);
  const [form, setForm] = useState<STPBridgeEdit>();
  const [busy, setBusy] = useState(false);
  const [confirmOn, setConfirmOn] = useState(false);
  const [failMsg, setFailMsg] = useState<string>();

  const load = useCallback(async () => {
    setError(false);
    try {
      const p = await api.stp();
      setProbe(p);
      if (p.applicable && !form) {
        setForm({
          enabled: p.bridge_info.enabled,
          priority: p.bridge_info.priority,
          hello_time: p.bridge_info.hello_time,
          max_age: p.bridge_info.max_age,
          forward_delay: p.bridge_info.forward_delay,
        });
      }
    } catch {
      setError(true);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  useEffect(() => { load(); }, [load]);

  if (error || (probe && !probe.applicable)) return null;
  if (!probe || !form) {
    return (
      <Card index={index} title={t("stp.bridgeTitle")} icon={GitBranch}>
        <SkeletonRows rows={3} />
      </Card>
    );
  }

  const dirty =
    form.priority !== probe.bridge_info.priority ||
    form.hello_time !== probe.bridge_info.hello_time ||
    form.max_age !== probe.bridge_info.max_age ||
    form.forward_delay !== probe.bridge_info.forward_delay;

  const save = async (edit: STPBridgeEdit) => {
    setBusy(true);
    setFailMsg(undefined);
    try {
      await api.setSTPBridge(edit);
      setForm(edit);
      await load();
    } catch (e) {
      setFailMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const toggleStp = (v: boolean) => {
    if (v) {
      setConfirmOn(true);
    } else {
      void save({ ...form, enabled: false });
    }
  };

  const numField = (
    key: "priority" | "hello_time" | "max_age" | "forward_delay",
    label: string,
    hint: string,
  ) => (
    <label className="flex flex-col gap-1">
      <span className="text-caption text-muted">{label}</span>
      <Input
        type="number"
        value={String(form[key])}
        onChange={(e) => setForm({ ...form, [key]: parseInt(e.target.value, 10) || 0 })}
        aria-label={label}
        className="!h-9 text-small"
      />
      <span className="text-caption text-faint">{hint}</span>
    </label>
  );

  return (
    <Card index={index} title={t("stp.bridgeTitle")} icon={GitBranch}>
      <p className="text-small text-muted mb-3">{t("stp.bridgeIntro")}</p>
      <div className="flex items-center gap-2 mb-4">
        <span className="text-small">STP</span>
        <Toggle
          checked={probe.bridge_info.enabled}
          busy={busy}
          onChange={toggleStp}
          label={t("stp.bridgeTitle")}
        />
        {probe.bridge_info.topology_change && (
          <span className="text-caption text-warn">{t("stp.topologyChange")}</span>
        )}
      </div>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {numField("priority", t("stp.priority"), t("stp.priorityHint"))}
        {numField("hello_time", t("stp.hello"), t("stp.helloHint"))}
        {numField("max_age", t("stp.maxAge"), t("stp.maxAgeHint"))}
        {numField("forward_delay", t("stp.forwardDelay"), t("stp.forwardDelayHint"))}
      </div>
      <div className="mt-3 flex items-center gap-2">
        <Button size="sm" loading={busy} disabled={!dirty} onClick={() => void save(form)}>
          {t("stp.apply")}
        </Button>
      </div>
      {probe.bridge_info.bridge_id && (
        <p className="text-caption text-faint mt-3 font-mono break-all">
          {t("stp.bridgeId", { id: probe.bridge_info.bridge_id })}
          {probe.bridge_info.designated_root &&
            ` · ${t("stp.designatedRoot", { id: probe.bridge_info.designated_root })}`}
        </p>
      )}
      {failMsg && <Banner tone="danger" onDismiss={() => setFailMsg(undefined)}>{failMsg}</Banner>}
      <ConfirmDialog
        open={confirmOn}
        onClose={() => setConfirmOn(false)}
        onConfirm={() => { setConfirmOn(false); void save({ ...form, enabled: true }); }}
        title={t("stp.enableTitle")}
        consequence={t("stp.enableConsequence")}
        confirmLabel={t("stp.enableConfirm")}
      />
    </Card>
  );
}

/**
 * Ajustes STP por boca (#487): solo se listan las bocas con algun valor
 * distinto del por defecto del kernel (`custom` en el probe: coste segun
 * velocidad, prioridad 32, sin guard/filtro/root block) mas las que se
 * anaden a mano; cada fila se despliega para editar coste/prioridad y los
 * tres toggles compactos. Nada de tabla de 52 filas.
 */
export function StpPortsCard({ index = 1 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<STPProbe>();
  const [error, setError] = useState(false);
  const [busyPort, setBusyPort] = useState<string>();
  const [drafts, setDrafts] = useState<Record<string, { cost: string; prio: string }>>({});
  const [expanded, setExpanded] = useState<string[]>([]);
  const [extras, setExtras] = useState<string[]>([]);
  const [failMsg, setFailMsg] = useState<string>();

  const load = useCallback(async () => {
    setError(false);
    try {
      const p = await api.stp();
      setProbe(p);
      setExtras((prev) => prev.filter((port) => p.ports.some((x) => x.port === port)));
    } catch { setError(true); }
  }, []);
  useEffect(() => { load(); }, [load]);

  if (error || (probe && !probe.applicable)) return null;
  if (!probe) {
    return (
      <Card index={index} className="md:col-span-2" title={t("stp.portsTitle")} icon={ShieldAlert}>
        <SkeletonRows rows={5} />
      </Card>
    );
  }

  const wired = probe.ports.filter((p) => p.port.startsWith("lan"))
    .sort((a, b) => a.port.localeCompare(b.port, undefined, { numeric: true }));
  const shown = wired.filter((p) => p.custom || extras.includes(p.port));
  const shownSet = new Set(shown.map((p) => p.port));
  const candidates = wired.filter((p) => !shownSet.has(p.port)).map((p) => p.port);

  const apply = async (port: STPPort, edit: Parameters<typeof api.setSTPPort>[0]) => {
    setBusyPort(port.port);
    setFailMsg(undefined);
    try {
      await api.setSTPPort(edit);
      await load();
    } catch (e) {
      setFailMsg(`${port.port}: ${e instanceof Error ? e.message : String(e)}`);
    } finally { setBusyPort(undefined); }
  };

  const toggle = (port: STPPort, key: "bpdu_guard" | "bpdu_filter" | "root_block") =>
    void apply(port, { name: port.port, [key]: !port[key] });

  const saveDraft = (port: STPPort) => {
    const d = drafts[port.port];
    if (!d) return;
    const cost = parseInt(d.cost, 10);
    const prio = parseInt(d.prio, 10);
    const edit: Parameters<typeof api.setSTPPort>[0] = { name: port.port };
    if (!Number.isNaN(cost) && cost !== port.path_cost) edit.path_cost = cost;
    if (!Number.isNaN(prio) && prio !== port.priority) edit.priority = prio;
    setDrafts((prev) => { const next = { ...prev }; delete next[port.port]; return next; });
    if (edit.path_cost !== undefined || edit.priority !== undefined) void apply(port, edit);
  };

  /** Devuelve la boca a los valores por defecto del kernel y la quita. */
  const reset = (port: STPPort) =>
    void apply(port, {
      name: port.port,
      path_cost: port.default_path_cost,
      priority: 32,
      bpdu_guard: false,
      bpdu_filter: false,
      root_block: false,
    });

  const STATE_CLASS: Record<string, string> = {
    forwarding: "text-ok",
    disabled: "text-muted",
    blocking: "text-danger",
    listening: "text-warn",
    learning: "text-warn",
    unknown: "text-muted",
  };

  /** Resumen compacto de en que se sale la boca del defecto. */
  const summary = (p: STPPort): string => {
    const parts: string[] = [];
    if (p.path_cost !== p.default_path_cost) parts.push(`${t("stp.colCost")} ${p.path_cost}`);
    if (p.priority !== 32) parts.push(`${t("stp.colPriority")} ${p.priority}`);
    if (p.bpdu_guard) parts.push(t("stp.colGuard"));
    if (p.bpdu_filter) parts.push(t("stp.colFilter"));
    if (p.root_block) parts.push(t("stp.colRootBlock"));
    return parts.join(" · ");
  };

  return (
    <Card index={index} className="md:col-span-2" title={t("stp.portsTitle")} icon={ShieldAlert}>
      <p className="text-small text-muted mb-3">
        {t("stp.portsIntro")}
        {" "}
        <span className="text-faint">{t("stp.noRstpNote")}</span>
      </p>
      <PortOverrides count={shown.length}>
        {shown.map((p) => {
          const d = drafts[p.port];
          const isOpen = expanded.includes(p.port);
          const busy = busyPort === p.port;
          return (
            <div key={p.port} className="rounded-md border border-border/60">
              <div className="flex items-center gap-2 px-2.5 py-2">
                <button
                  type="button"
                  onClick={() =>
                    setExpanded((prev) =>
                      prev.includes(p.port) ? prev.filter((x) => x !== p.port) : [...prev, p.port],
                    )}
                  aria-label={`${t("stp.expandPort")} ${p.port}`}
                  className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-muted hover:text-text transition-colors duration-[var(--dur-fast)] ring-focus"
                >
                  <ChevronDown
                    size={16}
                    aria-hidden="true"
                    className={`transition-transform duration-200 ease-[var(--ease-soft)] ${isOpen ? "rotate-180" : ""}`}
                  />
                </button>
                <span className="w-14 shrink-0 font-mono text-small">{p.port}</span>
                <span className={`text-caption font-medium ${STATE_CLASS[p.state_name] ?? "text-muted"}`}>
                  {t(`stp.state.${p.state_name}`, p.state_name)}
                </span>
                <span className="ml-auto min-w-0 truncate text-right text-caption text-muted">
                  {summary(p)}
                </span>
              </div>
              {isOpen && (
                <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-t border-border/60 px-2.5 py-2.5">
                  <label className="flex items-center gap-1.5">
                    <span className="text-caption text-muted">{t("stp.colCost")}</span>
                    <Input
                      type="number"
                      value={d?.cost ?? String(p.path_cost)}
                      disabled={busy}
                      onChange={(e) => setDrafts((prev) => ({ ...prev, [p.port]: { cost: e.target.value, prio: d?.prio ?? String(p.priority) } }))}
                      onBlur={() => saveDraft(p)}
                      onKeyDown={(e) => e.key === "Enter" && saveDraft(p)}
                      aria-label={`${t("stp.colCost")} ${p.port}`}
                      className="!h-7 w-20 text-caption"
                    />
                  </label>
                  <label className="flex items-center gap-1.5">
                    <span className="text-caption text-muted">{t("stp.colPriority")}</span>
                    <Input
                      type="number"
                      step={16}
                      value={d?.prio ?? String(p.priority)}
                      disabled={busy}
                      onChange={(e) => setDrafts((prev) => ({ ...prev, [p.port]: { cost: d?.cost ?? String(p.path_cost), prio: e.target.value } }))}
                      onBlur={() => saveDraft(p)}
                      onKeyDown={(e) => e.key === "Enter" && saveDraft(p)}
                      aria-label={`${t("stp.colPriority")} ${p.port}`}
                      className="!h-7 w-16 text-caption"
                    />
                  </label>
                  <label className="flex items-center gap-1.5">
                    <span className="text-caption text-muted">{t("stp.colGuard")}</span>
                    <Toggle checked={p.bpdu_guard} busy={busy} onChange={() => toggle(p, "bpdu_guard")} label={`${t("stp.colGuard")} ${p.port}`} />
                  </label>
                  <label className="flex items-center gap-1.5">
                    <span className="text-caption text-muted">{t("stp.colFilter")}</span>
                    <Toggle checked={p.bpdu_filter} busy={busy} onChange={() => toggle(p, "bpdu_filter")} label={`${t("stp.colFilter")} ${p.port}`} />
                  </label>
                  <label className="flex items-center gap-1.5">
                    <span className="text-caption text-muted">{t("stp.colRootBlock")}</span>
                    <Toggle checked={p.root_block} busy={busy} onChange={() => toggle(p, "root_block")} label={`${t("stp.colRootBlock")} ${p.port}`} />
                  </label>
                  <button
                    type="button"
                    onClick={() => reset(p)}
                    disabled={busy}
                    title={`${t("stp.resetPort")} (${p.port})`}
                    aria-label={`${t("stp.resetPort")} (${p.port})`}
                    className="ml-auto inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-faint hover:text-danger transition-colors duration-[var(--dur-fast)] ring-focus disabled:opacity-50"
                  >
                    <X size={16} aria-hidden="true" />
                  </button>
                </div>
              )}
            </div>
          );
        })}
        <PortAddPicker candidates={candidates} onAdd={(port) => setExtras((prev) => [...prev, port])} />
      </PortOverrides>
      {failMsg && <Banner tone="danger" onDismiss={() => setFailMsg(undefined)}>{failMsg}</Banner>}
    </Card>
  );
}
