import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Clock, Pause, Play } from "lucide-react";
import { api } from "../../api";
import type { WifiSchedule, WifiScheduleState, WifiUI } from "../../types";
import { Button, Card, Field, Input, Pill, Toggle } from "../ui";

const DAYS = [0, 1, 2, 3, 4, 5, 6];

/** Card "Horarios" (#445): apaga la emisión de cada SSID fuera de su franja
 *  semanal. Un "pausar ahora" manual gana sobre el horario, como en el
 *  control parental (#304). */
export function WifiScheduleCard({ ifaces, index = 3 }: { ifaces: WifiUI[]; index?: number }) {
  const { t } = useTranslation();
  const [schedules, setSchedules] = useState<WifiScheduleState[]>();
  const [open, setOpen] = useState<string>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [busySec, setBusySec] = useState<string>();

  const load = useCallback(() => {
    api.wifiSchedule().then((r) => setSchedules(r.schedules ?? [])).catch(() => {});
  }, []);

  useEffect(() => { load(); }, [load]);

  // Solo las redes "principales" (las del propio panel, sin invitados/IoT
  // secundarias): coinciden con las tarjetas de arriba.
  const rows = (ifaces ?? [])
    .map((i) => schedules?.find((s) => s.section === i.section))
    .filter((s): s is WifiScheduleState => !!s);

  if (rows.length === 0 && schedules === undefined) return null;

  const persist = async (sched: WifiSchedule) => {
    setBusySec(sched.section);
    setMsg(undefined);
    try {
      await api.setWifiSchedule(sched);
      load();
      setMsg({ tone: "ok", text: t("wifi.scheduleSaved") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusySec(undefined);
    }
  };

  return (
    <Card index={index} title={t("wifi.scheduleCardTitle")} help="wifiSchedule" icon={Clock} iconTone="accent">
      <div className="divide-y divide-border/60">
        {rows.map((st) => {
          const sched = st.schedule;
          const enabled = sched?.enabled ?? false;
          return (
            <ScheduleRow
              key={st.section}
              state={st}
              open={open === st.section}
              busy={busySec === st.section}
              onToggleOpen={() => setOpen(open === st.section ? undefined : st.section)}
              onToggleEnabled={(v) =>
                persist({ section: st.section, enabled: v, days: sched?.days ?? DAYS, start: sched?.start ?? "23:00", end: sched?.end ?? "07:00", paused: sched?.paused ?? false })
              }
              onSave={(days, start, end) =>
                persist({ section: st.section, enabled: true, days, start, end, paused: sched?.paused ?? false })
              }
              onTogglePause={() =>
                persist({ section: st.section, enabled, days: sched?.days ?? DAYS, start: sched?.start ?? "23:00", end: sched?.end ?? "07:00", paused: !sched?.paused })
              }
            />
          );
        })}
      </div>
      {msg && <p className={`text-caption mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}

function ScheduleRow({ state, open, busy, onToggleOpen, onToggleEnabled, onSave, onTogglePause }: {
  state: WifiScheduleState;
  open: boolean;
  busy: boolean;
  onToggleOpen: () => void;
  onToggleEnabled: (v: boolean) => void;
  onSave: (days: number[], start: string, end: string) => void;
  onTogglePause: () => void;
}) {
  const { t } = useTranslation();
  const sched = state.schedule;
  const [days, setDays] = useState<number[]>(sched?.days ?? [...DAYS]);
  const [start, setStart] = useState(sched?.start ?? "23:00");
  const [end, setEnd] = useState(sched?.end ?? "07:00");

  useEffect(() => {
    setDays(sched?.days?.length ? [...sched.days].sort() : [...DAYS]);
    setStart(sched?.start || "23:00");
    setEnd(sched?.end || "07:00");
  }, [sched]);

  const toggleDay = (d: number) => {
    setDays((cur) => (cur.includes(d) ? cur.filter((x) => x !== d) : [...cur, d].sort()));
  };

  return (
    <div className="py-3 first:pt-0 last:pb-0">
      <div className="flex items-center gap-3">
        <div className="min-w-0 flex-1">
          <p className="truncate text-small font-medium">{state.ssid}</p>
          <p className="text-caption text-muted truncate">
            {sched
              ? `${sched.start} - ${sched.end} · ${DAYS.filter((d) => sched.days.includes(d)).map((d) => t(`clients.day${d}`)).join(" ")}`
              : t("wifi.scheduleEmpty")}
          </p>
        </div>
        {state.off_by_schedule && <Pill tone="warn">{t("wifi.scheduleOffNow")}</Pill>}
        {!state.disabled && !state.off_by_schedule && <Pill tone="ok">{t("wifi.broadcasting")}</Pill>}
        {state.disabled && !state.off_by_schedule && <Pill tone="muted">{t("wifi.bandOff")}</Pill>}
        <Toggle checked={sched?.enabled ?? false} onChange={onToggleEnabled} label={t("wifi.scheduleToggle")} busy={busy} />
        <button type="button" onClick={onToggleOpen} aria-expanded={open}
          className="text-caption text-muted hover:text-text ring-focus rounded-sm">
          {open ? t("common.close") : t("wifi.scheduleEdit")}
        </button>
      </div>

      {open && (
        <div className="mt-3 rounded-md border border-border/60 p-3">
          <div className="flex flex-wrap gap-1.5 mb-3">
            {DAYS.map((d) => (
              <button key={d} type="button" onClick={() => toggleDay(d)} aria-pressed={days.includes(d)}
                className={`h-8 w-8 rounded-md text-caption font-medium ring-focus transition-colors
                  ${days.includes(d) ? "bg-accent text-white" : "bg-surface-2 text-muted hover:text-text"}`}>
                {t(`clients.day${d}`)}
              </button>
            ))}
          </div>
          <div className="grid grid-cols-2 gap-3 mb-3">
            <Field label={t("clients.parentalStart")}>
              <Input type="time" value={start} onChange={(e) => setStart(e.target.value)} />
            </Field>
            <Field label={t("clients.parentalEnd")}>
              <Input type="time" value={end} onChange={(e) => setEnd(e.target.value)} />
            </Field>
          </div>
          <p className="text-caption text-muted mb-3">{t("wifi.scheduleHint")}</p>
          <div className="flex flex-wrap items-center gap-2">
            <Button size="sm" loading={busy} onClick={() => onSave(days, start, end)}>{t("common.save")}</Button>
            <Button size="sm" variant={sched?.paused ? "secondary" : "ghost"} loading={busy}
              icon={sched?.paused ? Play : Pause} onClick={onTogglePause}>
              {sched?.paused ? t("clients.parentalResume") : t("clients.parentalPause")}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
