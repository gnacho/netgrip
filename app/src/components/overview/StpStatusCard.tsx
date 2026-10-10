import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ShieldCheck } from "lucide-react";
import { api } from "../../api";
import type { STPProbe } from "../../types";
import { Card, Pill, SkeletonRows } from "../ui";
import { oneLine } from "./cards";

/**
 * Estado STP del Resumen en modo switch (#487, mockup de rediseño): una
 * banda compacta que confirma que el protocolo anti-bucles vigila el
 * chasis. La configuración (prioridad, portfast por boca) vive en la
 * pestaña STP de la página Puertos; aquí el probe /api/stp ya existente.
 */
export function StpStatusCard({ index = 4 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<STPProbe>();

  useEffect(() => {
    api.stp().then(setProbe).catch(() => {});
  }, []);

  if (probe && !probe.applicable) return null;
  const enabled = probe?.bridge_info.enabled ?? false;

  return (
    <Card index={index} className="md:col-span-6 order-8 md:order-none"
      title={oneLine(t("overview.stpTitle"))} icon={ShieldCheck}
      iconTone={enabled ? "ok" : "muted"}
      action={probe && (
        <Pill tone={enabled ? "ok" : "muted"} live={enabled}>
          {enabled ? t("overview.stpActive") : t("overview.stpOff")}
        </Pill>
      )}>
      {!probe ? <SkeletonRows rows={1} /> : (
        <p className="text-small text-muted">{t("overview.stpBody")}</p>
      )}
    </Card>
  );
}
