import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Shield, Plus, Trash2 } from "lucide-react";
import { api } from "../api";
import type { FirewallProbe, FWRule } from "../types";
import { Card, Pill } from "./Card";

export function FirewallCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<FirewallProbe>();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [showAdd, setShowAdd] = useState(false);
  const [confirmDel, setConfirmDel] = useState<string>();
  const [form, setForm] = useState({ name: "", src: "wan", dest: "", proto: "tcp", dest_port: "", target: "ACCEPT" });

  useEffect(() => { api.firewall().then(setProbe).catch(() => {}); }, []);

  if (!probe?.applicable) return null;

  const addRule = async () => {
    setBusy(true); setMsg(undefined);
    try {
      const res = await api.addFirewallRule(form);
      setProbe(res.state);
      setShowAdd(false);
      setForm({ name: "", src: "wan", dest: "", proto: "tcp", dest_port: "", target: "ACCEPT" });
      setMsg({ tone: "ok", text: t("firewall.ruleAdded") });
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    } finally { setBusy(false); }
  };

  const delRule = async (section: string) => {
    setBusy(true); setMsg(undefined);
    try {
      const res = await api.deleteFirewallRule(section);
      setProbe(res.state);
      setMsg({ tone: "ok", text: t("firewall.ruleDeleted") });
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    } finally { setBusy(false); setConfirmDel(undefined); }
  };

  const toneFor = (v: string): "ok" | "warn" | "danger" | "muted" => {
    if (v === "ACCEPT") return "ok";
    if (v === "REJECT" || v === "DROP") return "danger";
    return "muted";
  };

  return (
    <Card title={t("firewall.title")} icon={Shield}>
      <p className="text-xs text-muted mb-3">{t("firewall.intro")}</p>

      {/* Zones */}
      <div className="mb-3">
        <p className="text-xs font-medium mb-1">{t("firewall.zones")}</p>
        <div className="flex flex-wrap gap-2">
          {probe.zones.map((z) => (
            <div key={z.name} className="bg-bg/50 border border-border/50 rounded-lg px-2 py-1 text-xs">
              <span className="font-medium">{z.name}</span>
              <span className="text-muted ml-1">
                ({z.network.join(", ") || "—"})
              </span>
              <span className="ml-1">
                <Pill tone={toneFor(z.input)}>{z.input}</Pill>
              </span>
              {z.masq && <Pill tone="warn">NAT</Pill>}
            </div>
          ))}
        </div>
      </div>

      {/* Rules */}
      <div>
        <div className="flex items-center justify-between mb-1">
          <p className="text-xs font-medium">{t("firewall.rules")}</p>
          <button onClick={() => setShowAdd(!showAdd)}
            className="text-xs bg-accent/15 text-accent px-2 py-0.5 rounded-lg hover:bg-accent/25 flex items-center gap-1">
            <Plus size={10} /> {t("firewall.addRule")}
          </button>
        </div>

        {showAdd && (
          <div className="bg-bg/50 border border-border/50 rounded-lg p-2 mb-2 grid grid-cols-2 gap-2 text-xs">
            <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder={t("firewall.ruleName")} className="bg-bg border border-border rounded px-2 py-1 col-span-2" />
            <select value={form.src} onChange={(e) => setForm({ ...form, src: e.target.value })}
              className="bg-bg border border-border rounded px-2 py-1">
              <option value="wan">wan</option>
              <option value="lan">lan</option>
              {probe.zones.filter((z) => z.name !== "wan" && z.name !== "lan").map((z) => (
                <option key={z.name} value={z.name}>{z.name}</option>
              ))}
            </select>
            <select value={form.proto} onChange={(e) => setForm({ ...form, proto: e.target.value })}
              className="bg-bg border border-border rounded px-2 py-1">
              <option value="tcp">TCP</option>
              <option value="udp">UDP</option>
              <option value="tcp udp">TCP+UDP</option>
            </select>
            <input value={form.dest_port} onChange={(e) => setForm({ ...form, dest_port: e.target.value })}
              placeholder={t("firewall.destPort")} className="bg-bg border border-border rounded px-2 py-1" />
            <select value={form.target} onChange={(e) => setForm({ ...form, target: e.target.value })}
              className="bg-bg border border-border rounded px-2 py-1">
              <option value="ACCEPT">ACCEPT</option>
              <option value="REJECT">REJECT</option>
              <option value="DROP">DROP</option>
            </select>
            <div className="col-span-2 flex justify-end gap-2">
              <button onClick={() => setShowAdd(false)} className="text-muted px-2 py-1">{t("firewall.cancel")}</button>
              <button onClick={addRule} disabled={busy || !form.name || !form.dest_port}
                className="bg-accent text-white px-3 py-1 rounded-lg disabled:opacity-50">
                {busy ? "…" : t("firewall.add")}
              </button>
            </div>
          </div>
        )}

        {probe.rules.length === 0 ? (
          <p className="text-sm text-muted">{t("firewall.noRules")}</p>
        ) : (
          <div className="flex flex-col gap-1">
            {probe.rules.map((r: FWRule) => (
              <div key={r.section} className="flex items-center gap-2 py-1 border-b border-border/30 last:border-0 text-xs">
                <span className="font-medium flex-1 truncate">{r.name || r.section}</span>
                <Pill tone="muted">{r.src || "*"}</Pill>
                <span className="text-muted">→</span>
                <Pill tone="muted">{r.dest || "*"}</Pill>
                <span className="font-mono text-muted">{r.proto} {r.dest_port}</span>
                <Pill tone={toneFor(r.target)}>{r.target}</Pill>
                {confirmDel === r.section ? (
                  <div className="flex gap-0.5">
                    <button onClick={() => delRule(r.section)} disabled={busy}
                      className="bg-danger/20 hover:bg-danger/30 rounded px-1.5 py-0.5">{t("firewall.confirmDel")}</button>
                    <button onClick={() => setConfirmDel(undefined)} className="text-muted">x</button>
                  </div>
                ) : (
                  <button onClick={() => setConfirmDel(r.section)} disabled={busy}
                    className="text-muted hover:text-danger p-0.5">
                    <Trash2 size={10} />
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
