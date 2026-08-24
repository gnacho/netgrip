import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { CloudCog } from "lucide-react";
import { api } from "../api";
import type { DDNSProbe } from "../types";
import { Card, Pill, Row } from "./Card";
import { Toggle } from "./Toggle";

export function DdnsCard({ probe, onChange }: {
  probe: DDNSProbe | undefined;
  onChange: (p: DDNSProbe) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [serviceName, setServiceName] = useState("");
  const [domain, setDomain] = useState("");
  const [lookupHost, setLookupHost] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  useEffect(() => {
    if (probe) {
      setServiceName(probe.service_name || "");
      setDomain(probe.domain || "");
      setLookupHost(probe.lookup_host || "");
      setUsername(probe.username || "");
    }
  }, [probe]);

  const run = async (enabled: boolean) => {
    setBusy(true);
    setMsg(undefined);
    try {
      const result = await api.setDdns(
        enabled
          ? { enabled, service_name: serviceName, domain, lookup_host: lookupHost, username, password }
          : { enabled },
      );
      onChange(result.state);
      setPassword("");
      setMsg(result.status === "applied"
        ? { tone: "ok", text: t("ddns.applied") }
        : { tone: "danger", text: result.error || t("ddns.rolledBack") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("ddns.failed") });
      onChange(await api.ddns());
    } finally {
      setBusy(false);
    }
  };

  const canEnable = serviceName.trim() !== "" && domain.trim() !== "";

  return (
    <Card title={t("ddns.title")} icon={CloudCog}>
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <span className="text-sm">DDNS</span>
          {probe && (
            <Pill tone={probe.running ? "ok" : probe.active ? "warn" : "muted"}>
              {probe.running ? t("ddns.running") : probe.active ? t("ddns.configured") : t("ddns.off")}
            </Pill>
          )}
        </div>
        <Toggle checked={probe?.active ?? false} busy={busy}
          disabled={!probe || (!probe.active && !canEnable)}
          onChange={run} />
      </div>

      {probe?.active && (
        <>
          {probe.registered_ip && <Row label={t("ddns.registeredIp")} value={probe.registered_ip} />}
          {probe.last_update && <Row label={t("ddns.lastUpdate")} value={new Date(probe.last_update).toLocaleString()} />}
        </>
      )}

      <div className="mt-2 flex flex-col gap-2">
        <input value={serviceName} onChange={(e) => setServiceName(e.target.value)}
          placeholder={t("ddns.provider")} disabled={probe?.active}
          className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent disabled:opacity-50" />
        <input value={domain} onChange={(e) => setDomain(e.target.value)}
          placeholder={t("ddns.domain")} disabled={probe?.active}
          className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent disabled:opacity-50" />
        <input value={lookupHost} onChange={(e) => setLookupHost(e.target.value)}
          placeholder={t("ddns.lookupHost")} disabled={probe?.active}
          className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent disabled:opacity-50" />
        <input value={username} onChange={(e) => setUsername(e.target.value)}
          placeholder={t("ddns.username")} autoComplete="off" disabled={probe?.active}
          className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent disabled:opacity-50" />
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)}
          placeholder={probe?.username ? t("ddns.passwordKeep") : t("ddns.password")}
          autoComplete="new-password" disabled={probe?.active}
          className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent disabled:opacity-50" />
      </div>
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
