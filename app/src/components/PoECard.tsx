import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Zap } from "lucide-react";
import { api } from "../api";
import type { PoEProbe, PoEPort } from "../types";
import { Card, Pill } from "./Card";

export function PoECard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<PoEProbe>();
  const [busy, setBusy] = useState<string>();
  const [editSched, setEditSched] = useState<{ port: string; on: string; off: string }>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  useEffect(() => { api.poe().then(setProbe).catch(() => {}); }, []);

  if (!probe?.applicable) return null;

  const budgetPct = probe.total_budget_w > 0 ? Math.round((probe.used_w / probe.total_budget_w) * 100) : 0;

  const saveSchedule = async () => {
    if (!editSched) return;
    setBusy(editSched.port); setMsg(undefined);
    try {
      const res = await api.setPoESchedule({ port: editSched.port, on_time: editSched.on, off_time: editSched.off });
      setProbe(res.state);
      setEditSched(undefined);
      setMsg({ tone: "ok", text: t("poe.scheduleSaved") });
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    } finally { setBusy(undefined); }
  };

  return (
    <Card title={t("poe.title")} icon={Zap}>
      {probe.total_budget_w > 0 && (
        <div className="mb-3">
          <div className="flex items-center justify-between text-xs mb-1">
            <span className="text-muted">{t("poe.budget")}</span>
            <span>{probe.used_w.toFixed(1)}W / {probe.total_budget_w.toFixed(0)}W ({budgetPct}%)</span>
          </div>
          <div className="h-2 bg-bg rounded-full overflow-hidden">
            <div
              className={`h-full rounded-full transition-all ${budgetPct > 80 ? "bg-danger" : budgetPct > 60 ? "bg-warn" : "bg-ok"}`}
              style={{ width: `${budgetPct}%` }}
            />
          </div>
        </div>
      )}

      {probe.ports.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="text-xs text-muted border-b border-border">
                <th className="text-left font-medium py-1.5 px-1">{t("poe.port")}</th>
                <th className="text-center font-medium py-1.5 px-1">{t("poe.status")}</th>
                <th className="text-right font-medium py-1.5 px-1">{t("poe.power")}</th>
                <th className="text-center font-medium py-1.5 px-1">{t("poe.schedule")}</th>
              </tr>
            </thead>
            <tbody>
              {probe.ports.map((p: PoEPort) => (
                <tr key={p.name} className="border-b border-border/40 last:border-0">
                  <td className="py-1.5 px-1 font-mono text-xs">{p.name}</td>
                  <td className="py-1.5 px-1 text-center">
                    <Pill tone={p.enabled ? "ok" : "muted"}>{p.status || (p.enabled ? "on" : "off")}</Pill>
                  </td>
                  <td className="py-1.5 px-1 text-right text-xs font-mono">
                    {p.power_w > 0 ? `${p.power_w.toFixed(1)}W` : "—"}
                  </td>
                  <td className="py-1.5 px-1 text-center">
                    {editSched?.port === p.name ? (
                      <div className="flex items-center gap-1">
                        <input type="time" value={editSched.on}
                          onChange={(e) => setEditSched({ ...editSched, on: e.target.value })}
                          className="bg-bg border border-border rounded px-1 py-0.5 text-[10px] w-16" />
                        <span className="text-muted text-[10px]">-</span>
                        <input type="time" value={editSched.off}
                          onChange={(e) => setEditSched({ ...editSched, off: e.target.value })}
                          className="bg-bg border border-border rounded px-1 py-0.5 text-[10px] w-16" />
                        <button onClick={saveSchedule} disabled={busy === p.name}
                          className="text-[10px] bg-accent/15 text-accent px-1.5 py-0.5 rounded">
                          {busy === p.name ? "…" : "✓"}
                        </button>
                        <button onClick={() => setEditSched(undefined)} className="text-[10px] text-muted">x</button>
                      </div>
                    ) : (
                      <button
                        onClick={() => setEditSched({ port: p.name, on: p.schedule_on || "", off: p.schedule_off || "" })}
                        className="text-[10px] text-muted hover:text-text"
                      >
                        {p.schedule_on || p.schedule_off
                          ? `${p.schedule_on || "—"} - ${p.schedule_off || "—"}`
                          : t("poe.addSchedule")}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
