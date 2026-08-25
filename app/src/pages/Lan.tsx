import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Info, Trash2 } from "lucide-react";
import { api } from "../api";
import type { LANConfig } from "../types";
import { Card } from "../components/Card";
import { DnsCard } from "../components/DnsCard";

export function LanPage() {
  const { t } = useTranslation();
  const [cfg, setCfg] = useState<LANConfig>();
  const [ipaddr, setIpaddr] = useState("");
  const [netmask, setNetmask] = useState("");
  const [apIsolation, setApIsolation] = useState(false);
  const [dhcpOn, setDhcpOn] = useState(true);
  const [start, setStart] = useState(100);
  const [limit, setLimit] = useState(150);
  const [lease, setLease] = useState(720);
  const [gateway, setGateway] = useState("");
  const [dns1, setDns1] = useState("");
  const [dns2, setDns2] = useState("");
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string } | undefined>();
  const [rMac, setRMac] = useState("");
  const [rIp, setRIp] = useState("");

  const load = (c: LANConfig) => {
    setCfg(c);
    setIpaddr(c.ipaddr); setNetmask(c.netmask); setApIsolation(c.ap_isolation);
    setDhcpOn(c.dhcp.enabled); setStart(c.dhcp.start); setLimit(c.dhcp.limit);
    setLease(c.dhcp.lease_time); setGateway(c.dhcp.gateway || "");
    setDns1(c.dhcp.dns1 || ""); setDns2(c.dhcp.dns2 || "");
  };

  useEffect(() => {
    api.lan().then(load).catch(() => {});
  }, []);

  const run = async (fn: () => Promise<{ status: string; error?: string }>, ok: string) => {
    setBusy(true); setMsg(undefined);
    try {
      const r = await fn();
      api.lan().then(load).catch(() => {});
      setMsg({ tone: "ok", text: ok });
      void r;
    } catch (err) {
      setMsg({ tone: "danger", text: err instanceof Error ? err.message : String(err) });
    } finally {
      setBusy(false);
    }
  };

  if (cfg && !cfg.applicable) {
    return (
      <Card title={t("lan.title")} icon={Info}>
        <p className="text-sm text-warn">{t("lan.notApplicable")}</p>
      </Card>
    );
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      {/* LAN */}
      <Card title={t("lan.title")} icon={Info}>
        <div className="flex flex-col gap-3 text-sm">
          <p className="text-xs text-muted">{t("lan.privateNote")}</p>
          <Field label={t("lan.ip")} value={
            <input value={ipaddr} onChange={(e) => setIpaddr(e.target.value)}
              className="bg-bg border border-border rounded-lg px-2 py-1 text-sm text-right outline-none focus:border-accent w-40" />
          } />
          <Field label={t("lan.netmask")} value={
            <input value={netmask} onChange={(e) => setNetmask(e.target.value)}
              className="bg-bg border border-border rounded-lg px-2 py-1 text-sm text-right outline-none focus:border-accent w-40" />
          } />
          <div className="flex items-center justify-between border-b border-border/40 py-1">
            <span className="text-xs text-muted">{t("lan.apIsolation")}</span>
            <input type="checkbox" checked={apIsolation} onChange={(e) => setApIsolation(e.target.checked)} className="accent-accent" />
          </div>
          <button onClick={() => run(() => api.setLan({ ipaddr, netmask, ap_isolation: apIsolation }), t("access.saved"))}
            disabled={busy || !ipaddr} className="text-sm bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium self-start">
            <Apply />
          </button>
        </div>
      </Card>

      {/* DHCP */}
      <Card title={t("lan.dhcpTitle")} icon={Info}>
        <div className="flex flex-col gap-3 text-sm">
          <div className="flex items-center justify-between border-b border-border/40 py-1">
            <span className="text-xs text-muted">{t("lan.dhcpEnable")}</span>
            <input type="checkbox" checked={dhcpOn} onChange={(e) => setDhcpOn(e.target.checked)} className="accent-accent" />
          </div>
          <Field label={t("lan.dhcpStart")} value={<Num val={start} set={setStart} />} />
          <Field label={t("lan.dhcpEnd")} value={<Num val={limit} set={setLimit} />} />
          <Field label={t("lan.leaseTime")} value={
            <div className="flex items-center gap-1">
              <Num val={lease} set={setLease} />
              <span className="text-xs text-muted">{t("lan.minutes")}</span>
            </div>
          } />
          <Field label={t("lan.gateway")} value={<Text val={gateway} set={setGateway} pl={t("lan.optional")} />} />
          <Field label={t("lan.dns1")} value={<Text val={dns1} set={setDns1} pl={t("lan.optional")} />} />
          <Field label={t("lan.dns2")} value={<Text val={dns2} set={setDns2} pl={t("lan.optional")} />} />
          <button onClick={() => run(() => api.setDhcp({ enabled: dhcpOn, start, limit, lease_time: lease, gateway, dns1, dns2 }), t("access.saved"))}
            disabled={busy} className="text-sm bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium self-start">
            <Apply />
          </button>
        </div>
      </Card>

      {/* Reservations */}
      <div className="sm:col-span-2">
        <Card title={t("lan.resTitle")} icon={Info} action={        cfg && cfg.reservations.length > 0 ? (
          <button onClick={() => run(() => api.clearReservations(), t("lan.cleared"))}
            className="text-xs text-muted hover:text-danger flex items-center gap-1">
            <Trash2 size={12} /> {t("lan.clearAll")}
          </button>
        ) : undefined
      }>
        <div className="flex flex-col gap-2 text-sm">
          <div className="flex gap-2">
            <input value={rMac} onChange={(e) => setRMac(e.target.value)} placeholder={t("lan.mac")}
              className="bg-bg border border-border rounded-lg px-2 py-1 text-xs outline-none focus:border-accent flex-1" />
            <input value={rIp} onChange={(e) => setRIp(e.target.value)} placeholder={t("lan.ip")}
              className="bg-bg border border-border rounded-lg px-2 py-1 text-xs outline-none focus:border-accent w-28" />
            <button onClick={() => run(() => api.setReservation(rMac, rIp, "", true), t("lan.added"))}
              disabled={busy || !rMac || !rIp}
              className="text-xs bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1 font-medium">
              {t("guest.add")}
            </button>
          </div>

          <table className="w-full text-xs">
            <thead>
              <tr className="text-muted border-b border-border/50">
                <th className="text-left py-1">MAC</th>
                <th className="text-left py-1">IP</th>
                <th className="text-right py-1">{t("lan.action")}</th>
              </tr>
            </thead>
            <tbody>
              {(cfg?.reservations ?? []).map((r) => (
                <tr key={r.mac} className="border-b border-border/30">
                  <td className="py-1 font-mono">{r.mac}</td>
                  <td className="py-1">{r.ip}</td>
                  <td className="py-1 text-right">
                    <button onClick={() => run(() => api.setReservation(r.mac, r.ip, "", false), t("lan.removed"))}
                      className="text-muted hover:text-danger">···</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        </Card>
      </div>

      <div className="sm:col-span-2">
        <DnsCard />
      </div>

      {msg && <p className={`text-xs sm:col-span-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </div>
  );
}

function Apply() {
  const { t } = useTranslation();
  return t("access.save");
}

function Field({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2 border-b border-border/40 py-1">
      <span className="text-xs text-muted shrink-0">{label}</span>
      <div>{value}</div>
    </div>
  );
}

function Num({ val, set }: { val: number; set: (n: number) => void }) {
  return (
    <input type="number" value={val || ""} onChange={(e) => set(Number(e.target.value))}
      className="bg-bg border border-border rounded-lg px-2 py-1 text-sm text-right outline-none focus:border-accent w-24" />
  );
}

function Text({ val, set, pl }: { val: string; set: (s: string) => void; pl?: string }) {
  return (
    <input value={val} onChange={(e) => set(e.target.value)} placeholder={pl}
      className="bg-bg border border-border rounded-lg px-2 py-1 text-sm text-right outline-none focus:border-accent w-40" />
  );
}
