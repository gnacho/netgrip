import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Download, Plus, Shield, Trash2 } from "lucide-react";
import { api } from "../api";
import type { OVPNProbe } from "../types";
import { Card, Pill, Row } from "./Card";
import { Toggle } from "./Toggle";

export function OpenvpnCard({ probe, onChange }: {
  probe: OVPNProbe | undefined;
  onChange: (p: OVPNProbe) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [clientName, setClientName] = useState("");

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setMsg(undefined);
    try {
      await fn();
      setMsg({ tone: "ok", text: t("ovpn.applied") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("ovpn.failed") });
      onChange(await api.openvpn());
    } finally {
      setBusy(false);
    }
  };

  const toggle = (v: boolean) => run(async () => {
    const result = await api.setOpenvpn(v ? "enable" : "disable");
    onChange(result.state);
    if (result.status !== "applied") throw new Error(result.error || t("ovpn.rolledBack"));
  });

  const addClient = (e: React.FormEvent) => {
    e.preventDefault();
    return run(async () => {
      const result = await api.addOvpnClient(clientName);
      onChange(result.state);
      // Download the .ovpn with everything embedded
      const blob = new Blob([result.config], { type: "application/x-openvpn-profile" });
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = `${clientName}.ovpn`;
      a.click();
      URL.revokeObjectURL(a.href);
      setClientName("");
    });
  };

  return (
    <Card title={t("ovpn.title")} icon={Shield}>
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <span className="text-sm">OpenVPN</span>
          {probe && (
            <Pill tone={probe.running ? "ok" : probe.active ? "warn" : "muted"}>
              {probe.running ? t("ovpn.running") : probe.active ? t("ovpn.configured") : t("ovpn.off")}
            </Pill>
          )}
        </div>
        <Toggle checked={probe?.active ?? false} busy={busy} disabled={!probe} onChange={toggle} />
      </div>

      {probe?.active && (
        <>
          <Row label={t("ovpn.port")} value={probe.port} />
          <Row label={t("ovpn.subnet")} value={probe.subnet} />

          <div className="mt-3">
            <p className="text-xs text-muted uppercase tracking-wider mb-1">{t("ovpn.clients")}</p>
            {probe.clients.length === 0 && <p className="text-xs text-muted">{t("ovpn.noClients")}</p>}
            {probe.clients.map((c) => (
              <div key={c.name} className="flex items-center gap-2 py-1 border-b border-border/50 last:border-0 text-sm">
                <span className="flex-1">{c.name}</span>
                <button onClick={() => run(async () => {
                  const result = await api.deleteOvpnClient(c.name);
                  onChange(result.state);
                })} className="text-muted hover:text-danger p-1" title={t("ovpn.deleteClient")}>
                  <Trash2 size={14} />
                </button>
              </div>
            ))}
            <form onSubmit={addClient} className="mt-2 flex gap-2">
              <input value={clientName} onChange={(e) => setClientName(e.target.value)}
                placeholder={t("ovpn.clientName")} required
                className="flex-1 bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
              <button type="submit" disabled={busy || !clientName}
                className="text-sm bg-border hover:bg-border/70 disabled:opacity-40 rounded-lg px-3 py-1.5 flex items-center gap-1">
                <Plus size={14} /> {t("ovpn.addClient")}
              </button>
            </form>
            <p className="text-xs text-muted mt-1 flex items-center gap-1">
              <Download size={12} /> {t("ovpn.downloadHint")}
            </p>
          </div>
        </>
      )}
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
