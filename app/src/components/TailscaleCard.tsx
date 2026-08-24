import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ExternalLink, Waypoints } from "lucide-react";
import { api } from "../api";
import type { TSProbe } from "../types";
import { Card, Pill, Row } from "./Card";
import { Toggle } from "./Toggle";

export function TailscaleCard({ probe, onChange }: {
  probe: TSProbe | undefined;
  onChange: (p: TSProbe) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  const run = async (enabled: boolean) => {
    setBusy(true);
    setMsg(undefined);
    try {
      const result = await api.setTailscale(enabled);
      onChange(result.state);
      setMsg(result.status === "applied"
        ? { tone: "ok", text: t("ts.applied") }
        : { tone: "danger", text: result.error || t("ts.rolledBack") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("ts.failed") });
      onChange(await api.tailscale());
    } finally {
      setBusy(false);
    }
  };

  const connected = probe?.state === "Running";
  const needsLogin = probe?.state === "NeedsLogin";

  return (
    <Card title="Tailscale" icon={Waypoints}>
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <span className="text-sm">Tailscale</span>
          {probe && (
            <Pill tone={connected ? "ok" : needsLogin ? "warn" : "muted"}>
              {connected ? t("ts.connected") : needsLogin ? t("ts.needsLogin") : t("ts.off")}
            </Pill>
          )}
        </div>
        <Toggle checked={probe?.running ?? false} busy={busy} disabled={!probe} onChange={run} />
      </div>

      {connected && probe.ips.length > 0 && (
        <Row label={t("ts.ip")} value={probe.ips.join(", ")} />
      )}
      {needsLogin && probe.auth_url && (
        <a href={probe.auth_url} target="_blank" rel="noreferrer"
          className="mt-1 flex items-center gap-1 text-sm text-accent hover:underline break-all">
          <ExternalLink size={14} className="shrink-0" /> {t("ts.loginLink")}
        </a>
      )}
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
