import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Cast } from "lucide-react";
import { api } from "../../api";
import type { MDNSProbe } from "../../types";
import { Card, Pill, SettingRow, useToast } from "../ui";

/**
 * mDNS reflector (umdns). Anuncia el router como hostname.local en la red
 * local para que dispositivos puedan resolverlo por nombre en lugar de IP.
 */
export function MdnsCard({ probe, onChange, index = 0 }: {
  probe: MDNSProbe | undefined;
  onChange: (p: MDNSProbe) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  const [local, setLocal] = useState<MDNSProbe | undefined>(probe);

  useEffect(() => {
    setLocal(probe);
  }, [probe]);

  const toggle = async (enabled: boolean) => {
    setBusy(true);
    try {
      const res = await api.setMdns(enabled);
      if (res.status !== "applied") {
        push({ tone: "danger", text: res.error || t("action.failed") });
      }
      const updated = await api.mdns();
      setLocal(updated);
      onChange(updated);
    } catch (err) {
      push({ tone: "danger", text: err instanceof Error ? err.message : String(err) });
      try {
        const updated = await api.mdns();
        setLocal(updated);
        onChange(updated);
      } catch { /* ignore */ }
    } finally {
      setBusy(false);
    }
  };

  if (!local) return <Card index={index}><div className="skeleton h-16" /></Card>;

  return (
    <Card index={index} title={t("mdns.title")} icon={Cast}
      action={<Pill tone={local.enabled ? "ok" : "muted"}>{local.enabled ? t("mdns.on") : t("mdns.off")}</Pill>}>
      <SettingRow
        title={t("mdns.toggle")}
        description={t("mdns.hint")}
        checked={local.enabled}
        busy={busy}
        onChange={toggle}
      />
      {local.enabled && local.domain && (
        <p className="text-caption text-muted border-t border-border/60 pt-2">
          <span className="font-medium">{t("mdns.domainLabel")}</span>{" "}
          <code className="text-foreground">{local.domain}</code>
        </p>
      )}
      {local.enabled && (
        <p className="text-caption text-muted mt-1"> {t("mdns.domainHint")}</p>
      )}
    </Card>
  );
}
