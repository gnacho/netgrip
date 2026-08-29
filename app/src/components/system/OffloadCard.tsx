import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Zap } from "lucide-react";
import { api } from "../../api";
import type { OffloadProbe } from "../../types";
import { Card, Pill, SettingRow, useToast } from "../ui";

/**
 * Aceleración de red (offload). Es ajuste de ingeniero: la página la
 * muestra bajo "Opciones avanzadas" (§6.20), por eso variant="subtle".
 */
export function OffloadCard() {
  const { t } = useTranslation();
  const { push } = useToast();
  const [probe, setProbe] = useState<OffloadProbe>();
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.offload().then(setProbe).catch(() => {});
  }, []);

  const toggle = async (enabled: boolean) => {
    setBusy(true);
    try {
      const res = await api.setOffload(enabled);
      if (res.status !== "applied") {
        push({ tone: "danger", text: res.error || t("action.failed") });
      }
      setProbe(await api.offload());
    } catch (err) {
      push({ tone: "danger", text: err instanceof Error ? err.message : String(err) });
    } finally {
      setBusy(false);
    }
  };

  if (!probe || !probe.applicable) return null;

  return (
    <Card variant="subtle" animate={false} title={t("offload.title")} icon={Zap}
      action={<Pill tone={probe.software ? "ok" : "muted"}>{probe.software ? t("offload.on") : t("offload.off")}</Pill>}>
      <SettingRow
        title={t("offload.software")}
        description={t("offload.softwareHint")}
        checked={probe.software}
        busy={busy}
        onChange={toggle}
      />
      <p className="text-caption text-muted border-t border-border/60 pt-2">
        <span className="font-medium">{t("offload.hardwareNote")}</span> {t("offload.hardwareHint")}
      </p>
    </Card>
  );
}
