import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { BookText, CloudOff, Lock, Server, ShieldCheck, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { DNSConfig } from "../../types";
import {
  ActionBanner, Banner, Button, Card, EmptyState, Field, SettingRow, SkeletonRows, useToast,
} from "../ui";
import { useActionCycle } from "../wifi/action";
import { isValidIp } from "./LanConfigCard";

type DnsKey = "rebind_protection" | "override_dns" | "dns_vpn";

/** Card "DNS: la agenda de nombres" (lan.md §3). */
export function DnsCard({ index = 1 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [cfg, setCfg] = useState<DNSConfig>();
  const [lan, setLan] = useState<import("../../types").LANConfig>();
  const [dns1, setDns1] = useState("");
  const [dns2, setDns2] = useState("");
  const [error, setError] = useState(false);
  const [vpnActive, setVpnActive] = useState<boolean>();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [hHost, setHHost] = useState("");
  const [hIp, setHIp] = useState("");

  const load = useCallback(async () => {
    setError(false);
    try {
      setCfg(await api.dns());
      api.lan().then((l) => {
        setLan(l);
        setDns1(l.dhcp.dns1 ?? "");
        setDns2(l.dhcp.dns2 ?? "");
      }).catch(() => {});
    } catch {
      setError(true);
    }
  }, []);

  useEffect(() => {
    load();
    api.wireguard().then((w) => setVpnActive(w.active)).catch(() => {});
  }, [load]);

  const toggle = (key: DnsKey, value: boolean) => {
    run(() => api.setDns({ [key]: value })).then((res) => {
      if (res?.status === "applied") setCfg(res.state);
    });
  };

  const saveDns = async () => {
    if (!lan) return;
    const res = await run(() => api.setDhcp({ ...lan.dhcp, dns1: dns1.trim(), dns2: dns2.trim() }));
    if (res?.status === "applied") {
      setLan(res.state);
      push({ tone: "ok", text: t("dns.dnsSaved") });
    }
  };

  const host = (remove: boolean) => {
    run(() => api.setDnsHost(hIp.trim(), hHost.trim(), remove)).then((res) => {
      if (res?.status === "applied") {
        setCfg(res.state);
        if (!remove) { setHHost(""); setHIp(""); }
      }
    });
  };

  const removeHost = (ip: string, hostname: string) => {
    run(() => api.setDnsHost(ip, hostname, true)).then((res) => {
      if (res?.status === "applied") setCfg(res.state);
    });
  };

  const rows: { key: DnsKey; icon: typeof Server; title: string; desc: string; disabled?: boolean; reason?: string }[] = [
    { key: "rebind_protection", icon: ShieldCheck, title: t("dns.rebindShort"), desc: t("dns.rebindDesc") },
    { key: "override_dns", icon: Server, title: t("dns.overrideShort"), desc: t("dns.overrideDesc") },
    {
      key: "dns_vpn", icon: Lock, title: t("dns.dnsVpnShort"), desc: t("dns.dnsVpnDesc"),
      disabled: vpnActive === false, reason: t("dns.vpnInactive"),
    },
  ];

  return (
    <Card index={index} icon={BookText} title={t("dns.agenda")} help="dns">
      {error ? (
        <EmptyState
          small
          illustration={<CloudOff size={24} />}
          title={t("common.loadError")}
          action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retry")}</Button>}
        />
      ) : !cfg ? (
        <SkeletonRows rows={4} />
      ) : (
        <>
          {cfg.adguard_active && (
            <Banner tone="info" className="mb-1">{t("dns.adguardNote")}</Banner>
          )}

          <div className="divide-y divide-border/60">
            {rows.map((r) => (
              <SettingRow
                key={r.key}
                icon={r.icon}
                title={r.title}
                description={r.desc}
                checked={cfg[r.key]}
                busy={busy}
                disabled={r.disabled}
                disabledReason={r.reason}
                onChange={(v) => toggle(r.key, v)}
              />
            ))}
          </div>

          {cfg.override_dns && (
            <div className="mt-3 grid gap-2 sm:grid-cols-[1fr_1fr_auto] items-end">
              <Field label={t("lan.dns1")} mono
                inputProps={{ value: dns1, onChange: (e) => setDns1(e.target.value), placeholder: t("lan.optional") }} />
              <Field label={t("lan.dns2")} mono
                inputProps={{ value: dns2, onChange: (e) => setDns2(e.target.value), placeholder: t("lan.optional") }} />
              <Button size="sm" onClick={saveDns} loading={busy}>{t("lan.save")}</Button>
            </div>
          )}

          <div className="mt-2 border-t border-border/60 pt-4">
            <p className="text-body font-medium">{t("dns.hostsTitle")}</p>
            <p className="text-caption text-muted mt-0.5">{t("dns.hostsHint")}</p>

            {cfg.hosts.length > 0 && (
              <div className="mt-3 divide-y divide-border/50">
                {cfg.hosts.map((h) => (
                  <div key={h.ip + h.hostname} className="flex items-center gap-2 py-1.5 animate-fade-up">
                    <span title={h.hostname} className="text-body font-medium flex-1 min-w-0 truncate">{h.hostname}</span>
                    <span className="font-mono text-small text-muted">{h.ip}</span>
                    <button
                      type="button"
                      onClick={() => removeHost(h.ip, h.hostname)}
                      disabled={busy}
                      aria-label={`${t("dns.remove")} ${h.hostname}`}
                      className="text-faint hover:text-danger transition-colors duration-[var(--dur-fast)] ring-focus rounded-sm p-1"
                    >
                      <Trash2 size={16} />
                    </button>
                  </div>
                ))}
              </div>
            )}

            <div className="mt-3 grid gap-2 grid-cols-[1fr_1fr_auto] items-end">
              <Field label={t("dns.hostName")}
                inputProps={{ value: hHost, onChange: (e) => setHHost(e.target.value), placeholder: "nas" }} />
              <Field label={t("dns.hostIp")} mono error={hIp && !isValidIp(hIp) ? t("lan.invalidIp") : undefined}
                inputProps={{ value: hIp, onChange: (e) => setHIp(e.target.value), placeholder: "192.168.8.10" }} />
              <Button size="sm" onClick={() => host(false)}
                disabled={busy || !hHost || !isValidIp(hIp)} loading={busy}>
                {t("guest.add")}
              </Button>
            </div>
          </div>

          {phase && (
            <div className="mt-3">
              <ActionBanner phase={phase} detail={detail} onDone={clear} />
            </div>
          )}
        </>
      )}
    </Card>
  );
}
