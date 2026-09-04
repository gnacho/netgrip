import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { CloudOff, Pencil, PlugZap, Zap } from "lucide-react";
import { api } from "../../api";
import type { SwitchPort, SwitchProbe } from "../../types";
import {
  ActionBanner, Button, Card, ConfirmDialog, EmptyState, Input, Pill, Toggle,
} from "../ui";
import { useActionCycle } from "../wifi/action";

function fmtSpeed(mbps: number): string {
  if (mbps >= 1000) return `${(mbps / 1000).toFixed(0)}G`;
  if (mbps > 0) return `${mbps}M`;
  return "—";
}

/** Card "Las bocas del switch" (ports.md §5): grid de bocas con control. */
export function SwitchCard({ index = 2 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<SwitchProbe>();
  const [error, setError] = useState(false);
  const [busyPort, setBusyPort] = useState<string>();
  const [editDesc, setEditDesc] = useState<{ name: string; value: string }>();
  const [confirmOff, setConfirmOff] = useState<SwitchPort>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const { phase, detail, busy, run, clear } = useActionCycle();

  const load = useCallback(async () => {
    setError(false);
    try {
      setProbe(await api.switchPorts());
    } catch {
      setError(true);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  // Hardware sin switch: la card no existe. Solo se muestra si aplica o hay error.
  if (!error && (!probe || !probe.applicable)) return null;

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
        setMsg({ tone: "ok", text: t("switch.descSaved") });
      } else {
        setMsg({ tone: "danger", text: res.error || t("fwd.failed") });
      }
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally { setBusyPort(undefined); }
  };

  return (
    <Card index={index} icon={PlugZap} title={t("switch.portsTitle")}>
      {error ? (
        <EmptyState
          small
          illustration={<CloudOff size={24} />}
          title={t("common.loadError")}
          action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retry")}</Button>}
        />
      ) : !probe ? null : (
        <>
          <div className="grid gap-3 sm:grid-cols-2">
            {(probe.ports ?? []).map((p) => (
              <div key={p.name} className="rounded-md bg-surface-2 border border-border p-3">
                <div className="flex items-center gap-2">
                  <span
                    aria-hidden="true"
                    className={`h-2 w-2 rounded-full shrink-0 transition-colors duration-200 ${p.oper_up ? "bg-ok" : "bg-faint"}`}
                  />
                  <span title={p.name} className="font-mono text-small font-medium flex-1 min-w-0 truncate">{p.name}</span>
                  <span className="font-mono text-caption text-muted">
                    {p.oper_up ? fmtSpeed(p.speed_mbps) : t("switch.noLink")}
                  </span>
                  <Pill tone={p.admin_up ? "ok" : "muted"}>
                    {p.admin_up ? t("switch.onState") : t("switch.offState")}
                  </Pill>
                </div>

                {/* Descripción editable (lápiz ghost) */}
                <div className="mt-2 min-h-6">
                  {editDesc?.name === p.name ? (
                    <div className="flex gap-1.5">
                      <Input
                        value={editDesc.value}
                        onChange={(e) => setEditDesc({ ...editDesc, value: e.target.value })}
                        autoFocus
                        onKeyDown={(e) => e.key === "Enter" && saveDesc()}
                        aria-label={t("switch.description")}
                        placeholder={t("switch.addPlaceholder")}
                        className="!h-9 text-small"
                      />
                      <Button size="sm" onClick={saveDesc} loading={busyPort === p.name}>{t("lan.save")}</Button>
                      <Button size="sm" variant="ghost" onClick={() => setEditDesc(undefined)}>{t("common.cancel")}</Button>
                    </div>
                  ) : (
                    <button
                      type="button"
                      onClick={() => setEditDesc({ name: p.name, value: p.description })}
                      title={t("switch.editDesc")}
                      className="flex items-center gap-1.5 text-small text-muted hover:text-text transition-colors duration-[var(--dur-fast)] ring-focus rounded-sm max-w-full"
                    >
                      <Pencil size={12} className="shrink-0 text-faint" aria-hidden="true" />
                      <span className="truncate">{p.description || t("switch.addDesc")}</span>
                    </button>
                  )}
                </div>

                <div className="mt-2 flex items-center justify-between gap-2 border-t border-border/60 pt-2">
                  <span className="text-caption text-muted">{t("switch.admin")}</span>
                  <span className="flex items-center gap-3">
                    {p.poe_supported && (
                      <span className="flex items-center gap-1.5">
                        <Zap size={12} className="text-faint" aria-hidden="true" />
                        <Toggle
                          checked={p.poe_enabled}
                          busy={busyPort === p.name + "-poe"}
                          onChange={() => togglePoe(p)}
                          label={`PoE ${p.name}`}
                        />
                      </span>
                    )}
                    <Toggle
                      checked={p.admin_up}
                      busy={busyPort === p.name || busy}
                      onChange={(v) => (v ? setAdmin(p, true) : setConfirmOff(p))}
                      label={p.name}
                    />
                  </span>
                </div>
              </div>
            ))}
          </div>

          {phase && (
            <div className="mt-3">
              <ActionBanner phase={phase} detail={detail} onDone={clear} />
            </div>
          )}
          {msg && <p className={`text-caption mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
        </>
      )}

      <ConfirmDialog
        open={!!confirmOff}
        onClose={() => setConfirmOff(undefined)}
        onConfirm={() => { const p = confirmOff; setConfirmOff(undefined); if (p) setAdmin(p, false); }}
        title={t("switch.offTitle", { port: confirmOff?.name ?? "" })}
        consequence={t("switch.offConsequence")}
        confirmLabel={t("switch.offConfirm")}
      />
    </Card>
  );
}
