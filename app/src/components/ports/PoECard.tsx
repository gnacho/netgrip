import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, Zap } from "lucide-react";
import { api } from "../../api";
import type { PoEPort, PoEProbe, PoEWatchdogState, SwitchProbe } from "../../types";
import { ActionBanner, Button, Card, ConfirmDialog, HelpTip, Input, Pill, SettingRow } from "../ui";
import { useActionCycle } from "../wifi/action";

/** Card "Alimentación por cable (PoE)" (ports.md §4). Solo si el hardware aplica. */
export function PoECard({ index = 1 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<PoEProbe>();
  const [sw, setSw] = useState<SwitchProbe>();
  const [busyPort, setBusyPort] = useState<string>();
  const [confirmOff, setConfirmOff] = useState<PoEPort>();
  const [schedOpen, setSchedOpen] = useState<string>();
  const [sched, setSched] = useState<{ on: string; off: string }>({ on: "", off: "" });
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const { phase, detail, busy, run, clear } = useActionCycle();

  const [watchdogs, setWatchdogs] = useState<PoEWatchdogState[]>([]);
  const [wdOpen, setWdOpen] = useState<string>();
  const [wd, setWd] = useState({ target: "", threshold: 5, interval: 30, cooldown: 120 });

  const load = useCallback(() => {
    api.poe().then(setProbe).catch(() => {});
    api.switchPorts().then((r) => { if (r.applicable) setSw(r); }).catch(() => {});
    api.poeWatchdogs().then((r) => setWatchdogs(r.watchdogs ?? [])).catch(() => {});
  }, []);

  useEffect(() => { load(); }, [load]);

  // Sin PoE en este modelo: la card no se muestra (ports.md §4).
  if (!probe?.applicable) return null;

  const budgetPct = probe.total_budget_w > 0 ? Math.round((probe.used_w / probe.total_budget_w) * 100) : 0;

  const descFor = (name: string) => sw?.ports.find((p) => p.name === name)?.description;

  const wdFor = (port: string) => watchdogs.find((w) => w.config.port === port);

  const setPower = (port: PoEPort, on: boolean) => {
    setBusyPort(port.name);
    run(() => api.setSwitchPort({ name: port.name, poe_enabled: on })).then((res) => {
      setBusyPort(undefined);
      if (res?.status === "applied") api.poe().then(setProbe).catch(() => {});
    });
  };

  const saveSchedule = async (port: string) => {
    setBusyPort(port); setMsg(undefined);
    try {
      const res = await api.setPoESchedule({ port, on_time: sched.on, off_time: sched.off });
      setProbe(res.state);
      setSchedOpen(undefined);
      setMsg({ tone: "ok", text: t("poe.scheduleSaved") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally { setBusyPort(undefined); }
  };

  const saveWatchdog = async (port: string, enabled: boolean) => {
    setBusyPort(port); setMsg(undefined);
    try {
      const res = await api.setPoEWatchdog({
        port, enabled,
        target: wd.target.trim(),
        threshold: wd.threshold,
        interval_s: wd.interval,
        cooldown_s: wd.cooldown,
      });
      setWatchdogs(res.watchdogs ?? []);
      setMsg({ tone: "ok", text: enabled ? t("poe.wdSavedOn") : t("poe.wdSavedOff") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally { setBusyPort(undefined); }
  };

  return (
    <Card
      index={index}
      icon={Zap}
      iconTone="warn"
      title={t("poe.powerTitle")}
      help="poe"
      action={probe.total_budget_w > 0
        ? <Pill tone={budgetPct > 80 ? "warn" : "accent"}>{t("poe.budgetPill", { used: probe.used_w.toFixed(0), total: probe.total_budget_w.toFixed(0) })}</Pill>
        : undefined}
    >
      {probe.total_budget_w > 0 && (
        <div className="h-1.5 rounded-full bg-border overflow-hidden mb-1" role="img"
          aria-label={t("poe.budgetPill", { used: probe.used_w.toFixed(0), total: probe.total_budget_w.toFixed(0) })}>
          <div
            className={`h-full rounded-full transition-all duration-200 ${budgetPct > 80 ? "bg-warn" : "bg-accent"}`}
            style={{ width: `${Math.min(budgetPct, 100)}%` }}
          />
        </div>
      )}

      <div className="divide-y divide-border/60">
        {probe.ports.map((p) => {
          const label = descFor(p.name) || t("poe.portFallback", { name: p.name });
          const consumption = p.power_w > 0
            ? t("poe.consumption", { w: p.power_w.toFixed(1), cls: p.class || "?" })
            : (p.status || p.name);
          return (
            <div key={p.name}>
              <SettingRow
                title={label}
                description={consumption}
                checked={p.enabled}
                busy={busyPort === p.name && busy}
                onChange={(v) => (v ? setPower(p, true) : setConfirmOff(p))}
              />
              {/* Horario bajo chevron */}
              <div className="pb-2 -mt-1 flex items-center gap-1">
                <button
                  type="button"
                  onClick={() => {
                    setSchedOpen(schedOpen === p.name ? undefined : p.name);
                    setSched({ on: p.schedule_on || "", off: p.schedule_off || "" });
                  }}
                  aria-expanded={schedOpen === p.name}
                  className="flex items-center gap-1 text-caption text-muted hover:text-text transition-colors ring-focus rounded-sm"
                >
                  <ChevronDown size={12} className={`transition-transform duration-200 ${schedOpen === p.name ? "rotate-180" : ""}`} aria-hidden="true" />
                  {t("poe.schedule")}
                  {(p.schedule_on || p.schedule_off) && schedOpen !== p.name && (
                    <span className="font-mono">{`${p.schedule_on || "—"}–${p.schedule_off || "—"}`}</span>
                  )}
                </button>
                <HelpTip title={t("help.poeSchedule.title")} body={t("help.poeSchedule.body")} />
              </div>
              {schedOpen === p.name && (
                <div className="pb-2 mt-1 flex items-end gap-2 flex-wrap animate-fade-up">
                  <label className="text-caption text-muted">
                    {t("poe.scheduleOn")}
                    <Input type="time" value={sched.on} onChange={(e) => setSched({ ...sched, on: e.target.value })}
                      className="!h-9 mt-1" aria-label={t("poe.scheduleOn")} />
                  </label>
                  <label className="text-caption text-muted">
                    {t("poe.scheduleOff")}
                    <Input type="time" value={sched.off} onChange={(e) => setSched({ ...sched, off: e.target.value })}
                      className="!h-9 mt-1" aria-label={t("poe.scheduleOff")} />
                  </label>
                  <Button size="sm" onClick={() => saveSchedule(p.name)} loading={busyPort === p.name && !busy}>
                    {t("lan.save")}
                  </Button>
                </div>
              )}

              {/* Watchdog: recicla la alimentación si el aparato deja de responder */}
              <div className="pb-2 -mt-1 flex items-center gap-1">
                <button
                  type="button"
                  onClick={() => {
                    setWdOpen(wdOpen === p.name ? undefined : p.name);
                    const cur = wdFor(p.name);
                    setWd({
                      target: cur?.config.target ?? "",
                      threshold: cur?.config.threshold ?? 5,
                      interval: cur?.config.interval_s ?? 30,
                      cooldown: cur?.config.cooldown_s ?? 120,
                    });
                  }}
                  aria-expanded={wdOpen === p.name}
                  className="flex items-center gap-1 text-caption text-muted hover:text-text transition-colors ring-focus rounded-sm"
                >
                  <ChevronDown size={12} className={`transition-transform duration-200 ${wdOpen === p.name ? "rotate-180" : ""}`} aria-hidden="true" />
                  {t("poe.watchdog")}
                  {wdFor(p.name)?.config.enabled && wdOpen !== p.name && (
                    <span className="font-mono">{wdFor(p.name)?.config.target}</span>
                  )}
                </button>
                <HelpTip title={t("help.poeWatchdog.title")} body={t("help.poeWatchdog.body")} />
              </div>
              {wdOpen === p.name && (
                <div className="pb-3 mt-1 flex flex-col gap-2 animate-fade-up">
                  <div className="flex items-end gap-2 flex-wrap">
                    <label className="text-caption text-muted">
                      {t("poe.wdTarget")}
                      <Input value={wd.target} onChange={(e) => setWd({ ...wd, target: e.target.value })}
                        mono placeholder="192.168.1.50" className="!h-9 mt-1" aria-label={t("poe.wdTarget")} />
                    </label>
                    <label className="text-caption text-muted">
                      {t("poe.wdThreshold")}
                      <Input type="number" min={1} value={wd.threshold}
                        onChange={(e) => setWd({ ...wd, threshold: Number(e.target.value) })}
                        className="!h-9 mt-1 w-20" aria-label={t("poe.wdThreshold")} />
                    </label>
                    <label className="text-caption text-muted">
                      {t("poe.wdInterval")}
                      <Input type="number" min={5} value={wd.interval}
                        onChange={(e) => setWd({ ...wd, interval: Number(e.target.value) })}
                        className="!h-9 mt-1 w-20" aria-label={t("poe.wdInterval")} />
                    </label>
                    <label className="text-caption text-muted">
                      {t("poe.wdCooldown")}
                      <Input type="number" min={0} value={wd.cooldown}
                        onChange={(e) => setWd({ ...wd, cooldown: Number(e.target.value) })}
                        className="!h-9 mt-1 w-20" aria-label={t("poe.wdCooldown")} />
                    </label>
                  </div>
                  <div className="flex items-center gap-2 flex-wrap">
                    <Button size="sm" variant="secondary"
                      disabled={wdFor(p.name)?.config.enabled}
                      onClick={() => saveWatchdog(p.name, true)}>
                      {t("poe.wdEnable")}
                    </Button>
                    <Button size="sm" variant="ghost"
                      disabled={!wdFor(p.name)?.config.enabled}
                      onClick={() => saveWatchdog(p.name, false)}>
                      {t("poe.wdDisable")}
                    </Button>
                    {(() => {
                      const st = wdFor(p.name);
                      if (!st?.config.enabled) return null;
                      return (
                        <span className="text-caption text-muted">
                            {t("poe.wdStatus", {
                              failures: st.failures,
                              threshold: st.config.threshold,
                            })}
                            {st.last_cycle && ` · ${t("poe.wdLastCycle", { ts: new Date(st.last_cycle).toLocaleTimeString() })}`}
                        </span>
                      );
                    })()}
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>

      {phase && (
        <div className="mt-2">
          <ActionBanner phase={phase} detail={detail} onDone={clear} />
        </div>
      )}
      {msg && <p className={`text-caption mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}

      <ConfirmDialog
        open={!!confirmOff}
        onClose={() => setConfirmOff(undefined)}
        onConfirm={() => { const p = confirmOff; setConfirmOff(undefined); if (p) setPower(p, false); }}
        title={t("poe.offTitle", { port: confirmOff ? descFor(confirmOff.name) || confirmOff.name : "" })}
        consequence={t("poe.offConsequence")}
        confirmLabel={t("poe.offConfirm")}
      />
    </Card>
  );
}
