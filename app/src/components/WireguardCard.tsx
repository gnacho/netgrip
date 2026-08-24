import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Lock, Plus, Trash2 } from "lucide-react";
import { api } from "../api";
import type { WGProbe } from "../types";
import { Card, Pill, Row } from "./Card";
import { Toggle } from "./Toggle";

export function WireguardCard({ probe, onChange }: {
  probe: WGProbe | undefined;
  onChange: (p: WGProbe) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [peerName, setPeerName] = useState("");
  const [peerKey, setPeerKey] = useState("");
  const [peerAdmin, setPeerAdmin] = useState(false);

  const run = async (fn: () => Promise<{ state: WGProbe; status: string; error?: string }>) => {
    setBusy(true);
    setMsg(undefined);
    try {
      const result = await fn();
      onChange(result.state);
      setMsg(result.status === "applied"
        ? { tone: "ok", text: t("wg.applied") }
        : { tone: "danger", text: result.error || t("wg.rolledBack") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("wg.failed") });
      onChange(await api.wireguard());
    } finally {
      setBusy(false);
    }
  };

  const addPeer = async (e: React.FormEvent) => {
    e.preventDefault();
    await run(() => api.addWgPeer(peerName, peerKey, peerAdmin));
    setPeerName(""); setPeerKey(""); setPeerAdmin(false);
  };

  const short = (key: string) => key.length > 16 ? key.slice(0, 12) + "…" : key;

  return (
    <Card title={t("wg.title")} icon={Lock}>
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <span className="text-sm">WireGuard</span>
          {probe && (
            <Pill tone={probe.running ? "ok" : probe.active ? "warn" : "muted"}>
              {probe.running ? t("wg.running") : probe.active ? t("wg.configured") : t("wg.off")}
            </Pill>
          )}
        </div>
        <Toggle checked={probe?.active ?? false} busy={busy} disabled={!probe}
          onChange={(v) => run(() => api.setWireguard(v ? "enable" : "disable"))} />
      </div>

      {probe?.active && (
        <>
          <Row label={t("wg.serverKey")} value={<span className="font-mono text-xs">{short(probe.public_key)}</span>} />
          <Row label={t("wg.port")} value={probe.port} />
          <Row label={t("wg.address")} value={probe.address} />

          <div className="mt-3">
            <p className="text-xs text-muted uppercase tracking-wider mb-1">{t("wg.peers")}</p>
            {probe.peers.length === 0 && <p className="text-xs text-muted">{t("wg.noPeers")}</p>}
            {probe.peers.map((p) => (
              <div key={p.public_key} className="flex items-center gap-2 py-1 border-b border-border/50 last:border-0 text-sm">
                <div className="flex-1 min-w-0">
                  <span className="block truncate">{p.name || short(p.public_key)}</span>
                  <span className="block text-xs text-muted font-mono">{p.allowed_ips.join(", ")}</span>
                </div>
                {p.admin
                  ? <Pill tone="muted">admin</Pill>
                  : (
                    <button onClick={() => run(() => api.deleteWgPeer(p.public_key))}
                      className="text-muted hover:text-danger p-1" title={t("wg.deletePeer")}>
                      <Trash2 size={14} />
                    </button>
                  )}
              </div>
            ))}

            <form onSubmit={addPeer} className="mt-2 flex flex-col gap-2">
              <input value={peerName} onChange={(e) => setPeerName(e.target.value)}
                placeholder={t("wg.peerName")}
                className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
              <input value={peerKey} onChange={(e) => setPeerKey(e.target.value)}
                placeholder={t("wg.peerKey")} required
                className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm font-mono outline-none focus:border-accent" />
              <label className="flex items-center gap-2 text-xs text-muted">
                <input type="checkbox" checked={peerAdmin} onChange={(e) => setPeerAdmin(e.target.checked)} />
                {t("wg.peerAdmin")}
              </label>
              <button type="submit" disabled={busy || !peerKey}
                className="text-sm bg-border hover:bg-border/70 disabled:opacity-40 rounded-lg px-3 py-1.5 self-start flex items-center gap-1">
                <Plus size={14} /> {t("wg.addPeer")}
              </button>
            </form>
          </div>
        </>
      )}
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
