import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Network } from "lucide-react";
import { api } from "../api";
import type { SwitchProbe, SwitchPort } from "../types";
import { Card, Pill } from "./Card";
import { Toggle } from "./Toggle";

export function SwitchCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<SwitchProbe>();
  const [busy, setBusy] = useState<string>();
  const [editDesc, setEditDesc] = useState<{ name: string; value: string }>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  useEffect(() => { api.switchPorts().then(setProbe).catch(() => {}); }, []);

  if (!probe?.applicable) return null;

  const toggleAdmin = async (port: SwitchPort) => {
    setBusy(port.name); setMsg(undefined);
    try {
      const res = await api.setSwitchPort({ name: port.name, admin_up: !port.admin_up });
      setProbe(res.state);
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    } finally { setBusy(undefined); }
  };

  const togglePoe = async (port: SwitchPort) => {
    setBusy(port.name + "-poe"); setMsg(undefined);
    try {
      const res = await api.setSwitchPort({ name: port.name, poe_enabled: !port.poe_enabled });
      setProbe(res.state);
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    } finally { setBusy(undefined); }
  };

  const saveDesc = async () => {
    if (!editDesc) return;
    setBusy(editDesc.name); setMsg(undefined);
    try {
      const res = await api.setSwitchPort({ name: editDesc.name, description: editDesc.value });
      setProbe(res.state);
      setEditDesc(undefined);
      setMsg({ tone: "ok", text: t("switch.descSaved") });
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    } finally { setBusy(undefined); }
  };

  const fmtSpeed = (mbps: number) => {
    if (mbps >= 1000) return `${(mbps / 1000).toFixed(0)}G`;
    if (mbps > 0) return `${mbps}M`;
    return "—";
  };

  return (
    <Card title={t("switch.title")} icon={Network}>
      <p className="text-xs text-muted mb-3">{t("switch.intro")}</p>

      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="text-xs text-muted border-b border-border">
              <th className="text-left font-medium py-1.5 px-1">{t("switch.port")}</th>
              <th className="text-center font-medium py-1.5 px-1">{t("switch.admin")}</th>
              <th className="text-center font-medium py-1.5 px-1">{t("switch.link")}</th>
              <th className="text-center font-medium py-1.5 px-1">{t("switch.speed")}</th>
              <th className="text-center font-medium py-1.5 px-1">{t("switch.poe")}</th>
              <th className="text-left font-medium py-1.5 px-1">{t("switch.description")}</th>
            </tr>
          </thead>
          <tbody>
            {probe.ports.map((p) => (
              <tr key={p.name} className="border-b border-border/40 last:border-0">
                <td className="py-1.5 px-1 font-mono text-xs">{p.name}</td>
                <td className="py-1.5 px-1 text-center">
                  <Toggle
                    checked={p.admin_up}
                    busy={busy === p.name}
                    onChange={() => toggleAdmin(p)}
                  />
                </td>
                <td className="py-1.5 px-1 text-center">
                  <Pill tone={p.oper_up ? "ok" : "muted"}>
                    {p.oper_up ? t("switch.up") : t("switch.down")}
                  </Pill>
                </td>
                <td className="py-1.5 px-1 text-center text-xs font-mono">
                  {p.oper_up ? fmtSpeed(p.speed_mbps) : "—"}
                </td>
                <td className="py-1.5 px-1 text-center">
                  {p.poe_supported ? (
                    <Toggle
                      checked={p.poe_enabled}
                      busy={busy === p.name + "-poe"}
                      onChange={() => togglePoe(p)}
                    />
                  ) : (
                    <span className="text-muted text-xs">—</span>
                  )}
                </td>
                <td className="py-1.5 px-1">
                  {editDesc?.name === p.name ? (
                    <div className="flex gap-1">
                      <input
                        value={editDesc.value}
                        onChange={(e) => setEditDesc({ ...editDesc, value: e.target.value })}
                        className="bg-bg border border-border rounded px-2 py-0.5 text-xs w-32"
                        autoFocus
                        onKeyDown={(e) => e.key === "Enter" && saveDesc()}
                      />
                      <button onClick={saveDesc} disabled={busy === p.name}
                        className="text-xs bg-accent/15 text-accent px-1.5 py-0.5 rounded">
                        {busy === p.name ? "…" : "✓"}
                      </button>
                      <button onClick={() => setEditDesc(undefined)} className="text-xs text-muted">x</button>
                    </div>
                  ) : (
                    <button
                      onClick={() => setEditDesc({ name: p.name, value: p.description })}
                      className="text-xs text-muted hover:text-text truncate max-w-32"
                    >
                      {p.description || <span className="italic">{t("switch.addDesc")}</span>}
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
