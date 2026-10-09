import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { GitBranch, ShieldAlert } from "lucide-react";
import { api } from "../../api";
import type { STPBridgeEdit, STPPort, STPProbe } from "../../types";
import { Banner, Button, Card, ConfirmDialog, Input, SkeletonRows, Toggle } from "../ui";

/**
 * STP 802.1d del kernel (#485), referencia visual: tabla "Configuración de
 * puertos" de RTLPlayground. Las columnas Borde (portfast) y Punto a punto
 * no existen en el STP del kernel; RSTP/portfast requeriria mstpd (#449).
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

export function StpPortsCard({ index = 1 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<STPProbe>();
  const [error, setError] = useState(false);
  const [busyPort, setBusyPort] = useState<string>();
  const [drafts, setDrafts] = useState<Record<string, { cost: string; prio: string }>>({});
  const [failMsg, setFailMsg] = useState<string>();

  const load = useCallback(async () => {
    setError(false);
    try { setProbe(await api.stp()); }
    catch { setError(true); }
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

  const STATE_CLASS: Record<string, string> = {
    forwarding: "text-ok",
    disabled: "text-muted",
    blocking: "text-danger",
    listening: "text-warn",
    learning: "text-warn",
    unknown: "text-muted",
  };

  return (
    <Card index={index} className="md:col-span-2" title={t("stp.portsTitle")} icon={ShieldAlert}>
      <p className="text-small text-muted mb-3">
        {t("stp.portsIntro")}
        {" "}
        <span className="text-faint">{t("stp.noRstpNote")}</span>
      </p>
      <div className="overflow-x-auto">
        <table className="w-full text-small">
          <thead>
            <tr className="text-left text-caption text-muted border-b border-border">
              <th className="py-1.5 pr-2 font-medium">{t("stp.colPort")}</th>
              <th className="py-1.5 pr-2 font-medium">{t("stp.colState")}</th>
              <th className="py-1.5 pr-2 font-medium">{t("stp.colCost")}</th>
              <th className="py-1.5 pr-2 font-medium">{t("stp.colPriority")}</th>
              <th className="py-1.5 pr-2 text-center font-medium">{t("stp.colGuard")}</th>
              <th className="py-1.5 pr-2 text-center font-medium">{t("stp.colFilter")}</th>
              <th className="py-1.5 text-center font-medium">{t("stp.colRootBlock")}</th>
            </tr>
          </thead>
          <tbody>
            {probe.ports.map((p) => {
              const d = drafts[p.port];
              return (
                <tr key={p.port} className="border-b border-border/40 last:border-0">
                  <td className="py-1.5 pr-2 font-mono">{p.port}</td>
                  <td className="py-1.5 pr-2">
                    <span className={`text-caption font-medium ${STATE_CLASS[p.state_name] ?? "text-muted"}`}>
                      {t(`stp.state.${p.state_name}`, p.state_name)}
                    </span>
                  </td>
                  <td className="py-1.5 pr-2">
                    <Input
                      type="number"
                      value={d?.cost ?? String(p.path_cost)}
                      onChange={(e) => setDrafts((prev) => ({ ...prev, [p.port]: { cost: e.target.value, prio: d?.prio ?? String(p.priority) } }))}
                      onBlur={() => saveDraft(p)}
                      onKeyDown={(e) => e.key === "Enter" && saveDraft(p)}
                      aria-label={`${t("stp.colCost")} ${p.port}`}
                      className="!h-7 w-20 text-caption"
                    />
                  </td>
                  <td className="py-1.5 pr-2">
                    <Input
                      type="number"
                      step={16}
                      value={d?.prio ?? String(p.priority)}
                      onChange={(e) => setDrafts((prev) => ({ ...prev, [p.port]: { cost: d?.cost ?? String(p.path_cost), prio: e.target.value } }))}
                      onBlur={() => saveDraft(p)}
                      onKeyDown={(e) => e.key === "Enter" && saveDraft(p)}
                      aria-label={`${t("stp.colPriority")} ${p.port}`}
                      className="!h-7 w-16 text-caption"
                    />
                  </td>
                  <td className="py-1.5 pr-2 text-center">
                    <Toggle checked={p.bpdu_guard} busy={busyPort === p.port} onChange={() => toggle(p, "bpdu_guard")} label={`${t("stp.colGuard")} ${p.port}`} />
                  </td>
                  <td className="py-1.5 pr-2 text-center">
                    <Toggle checked={p.bpdu_filter} busy={busyPort === p.port} onChange={() => toggle(p, "bpdu_filter")} label={`${t("stp.colFilter")} ${p.port}`} />
                  </td>
                  <td className="py-1.5 text-center">
                    <Toggle checked={p.root_block} busy={busyPort === p.port} onChange={() => toggle(p, "root_block")} label={`${t("stp.colRootBlock")} ${p.port}`} />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      {failMsg && <Banner tone="danger" onDismiss={() => setFailMsg(undefined)}>{failMsg}</Banner>}
    </Card>
  );
}
