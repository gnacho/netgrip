import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeftRight, Plus, Trash2 } from "lucide-react";
import { api } from "../api";
import type { FwdProbe } from "../types";
import { Card } from "./Card";

export function PortForwardCard({ probe, onChange }: {
  probe: FwdProbe | undefined;
  onChange: (p: FwdProbe) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [srcDport, setSrcDport] = useState("");
  const [destIP, setDestIP] = useState("");
  const [destPort, setDestPort] = useState("");
  const [proto, setProto] = useState("tcp");

  const run = async (fn: () => Promise<{ state: FwdProbe; status: string; error?: string }>) => {
    setBusy(true);
    setMsg(undefined);
    try {
      const result = await fn();
      onChange(result.state);
      setMsg(result.status === "applied"
        ? { tone: "ok", text: t("fwd.applied") }
        : { tone: "danger", text: result.error || t("fwd.rolledBack") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("fwd.failed") });
      onChange(await api.portforward());
    } finally {
      setBusy(false);
    }
  };

  const addRule = async (e: React.FormEvent) => {
    e.preventDefault();
    await run(() => api.addFwdRule(srcDport, destIP, destPort, proto));
    setSrcDport(""); setDestIP(""); setDestPort("");
  };

  const applicable = probe?.has_wan && probe?.firewall;

  if (!probe || !applicable) return null;
  return (
    <Card title={t("fwd.title")} icon={ArrowLeftRight}>
      <>
          {probe.rules.length === 0 && <p className="text-xs text-muted mb-2">{t("fwd.empty")}</p>}
          {probe.rules.map((r) => (
            <div key={r.section} className="flex items-center gap-2 py-1 border-b border-border/50 last:border-0 text-sm">
              <span className="flex-1">
                :{r.src_dport} → {r.dest_ip}:{r.dest_port}
                <span className="text-xs text-muted ml-2">{r.proto}</span>
              </span>
              <button onClick={() => run(() => api.deleteFwdRule(r.section))}
                className="text-muted hover:text-danger p-1" title={t("fwd.delete")}>
                <Trash2 size={14} />
              </button>
            </div>
          ))}
          <form onSubmit={addRule} className="mt-2 flex flex-col gap-2">
            <div className="flex gap-2">
              <input value={srcDport} onChange={(e) => setSrcDport(e.target.value)}
                placeholder={t("fwd.extPort")} inputMode="numeric" required
                className="w-24 bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
              <span className="self-center text-muted">→</span>
              <input value={destIP} onChange={(e) => setDestIP(e.target.value)}
                placeholder="192.168.1.x" required
                className="flex-1 bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
              <input value={destPort} onChange={(e) => setDestPort(e.target.value)}
                placeholder={t("fwd.intPort")} inputMode="numeric" required
                className="w-24 bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
            </div>
            <div className="flex gap-2">
              <select value={proto} onChange={(e) => setProto(e.target.value)}
                className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent">
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
                <option value="tcpudp">TCP+UDP</option>
              </select>
              <button type="submit" disabled={busy}
                className="text-sm bg-border hover:bg-border/70 disabled:opacity-40 rounded-lg px-3 py-1.5 flex items-center gap-1">
                <Plus size={14} /> {t("fwd.add")}
              </button>
            </div>
          </form>
      </>
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
