import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Server } from "lucide-react";
import { api } from "../api";
import type { DNSConfig } from "../types";
import { Card } from "./Card";

export function DnsCard() {
  const { t } = useTranslation();
  const [cfg, setCfg] = useState<DNSConfig>();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string } | undefined>();
  const [hIp, setHIp] = useState("");
  const [hHost, setHHost] = useState("");

  useEffect(() => { api.dns().then(setCfg).catch(() => {}); }, []);

  const run = async (fn: () => Promise<unknown>, ok: string) => {
    setBusy(true); setMsg(undefined);
    try {
      await fn();
      setCfg(await api.dns());
      setMsg({ tone: "ok", text: ok });
    } catch (err) {
      setMsg({ tone: "danger", text: err instanceof Error ? err.message : String(err) });
    } finally {
      setBusy(false);
    }
  };

  const toggle = (key: "rebind_protection" | "override_dns" | "dns_vpn", value: boolean) =>
    run(() => api.setDns({ [key]: value }), t("access.saved"));

  if (cfg && !cfg.applicable) {
    return (
      <Card title={t("dns.title")} icon={Server}>
        <p className="text-sm text-warn">{t("lan.notApplicable")}</p>
      </Card>
    );
  }

  const items: { key: "rebind_protection" | "override_dns" | "dns_vpn"; label: string }[] = [
    { key: "rebind_protection", label: t("dns.rebind") },
    { key: "override_dns", label: t("dns.override") },
    { key: "dns_vpn", label: t("dns.dnsVpn") },
  ];

  return (
    <Card title={t("dns.title")} icon={Server}>
      <div className="flex flex-col gap-3 text-sm">
        {cfg?.adguard_active && (
          <p className="text-xs text-warn bg-warn/10 border border-warn/30 rounded-lg p-2">{t("dns.adguardNotice")}</p>
        )}
        {items.map((item) => (
          <div key={item.key} className="flex items-center justify-between gap-2">
            <span className="text-xs text-muted flex-1">{item.label}</span>
            <input type="checkbox" checked={cfg?.[item.key] ?? false} disabled={busy}
              onChange={(e) => toggle(item.key, e.target.checked)} className="accent-accent" />
          </div>
        ))}

        <div className="border-t border-border/40 pt-2">
          <div className="text-xs font-medium uppercase tracking-wider text-muted mb-2">{t("dns.hosts")}</div>
          <div className="flex gap-2 mb-2">
            <input value={hIp} onChange={(e) => setHIp(e.target.value)} placeholder={t("dns.hostIp")}
              className="bg-bg border border-border rounded-lg px-2 py-1 text-xs outline-none focus:border-accent flex-1" />
            <input value={hHost} onChange={(e) => setHHost(e.target.value)} placeholder={t("dns.hostName")}
              className="bg-bg border border-border rounded-lg px-2 py-1 text-xs outline-none focus:border-accent flex-1" />
            <button onClick={() => run(() => api.setDnsHost(hIp, hHost, false), t("lan.added"))}
              disabled={busy || !hIp || !hHost}
              className="text-xs bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1 font-medium">
              {t("guest.add")}
            </button>
          </div>
          {cfg?.hosts.map((h) => (
            <div key={h.ip + h.hostname} className="flex items-center gap-2 text-xs py-0.5">
              <span className="font-mono text-muted">{h.ip}</span>
              <span className="flex-1">{h.hostname}</span>
              <button onClick={() => run(() => api.setDnsHost(h.ip, h.hostname, true), t("lan.removed"))}
                className="text-muted hover:text-danger">✕</button>
            </div>
          ))}
        </div>

        {msg && <p className={`text-xs ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
      </div>
    </Card>
  );
}
