import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Layers } from "lucide-react";
import { api } from "../../api";
import type { VLANProbe } from "../../types";
import { Card, SkeletonRows } from "../ui";
import { oneLine } from "./cards";

/**
 * VLANs activas del Resumen en modo switch (#487, mockup de rediseño):
 * lista de solo lectura con las redes separadas que pasan por el chasis.
 * La edición (802.1Q, PVID por boca) vive en la pestaña VLANs de la
 * página Puertos; aquí el probe /api/vlans ya existente, sin endpoints nuevos.
 */
export function VlansCard({ index = 4 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<VLANProbe>();

  useEffect(() => {
    api.vlans().then(setProbe).catch(() => {});
  }, []);

  const vlans = (probe?.vlans ?? []).filter((v) => v.vid > 0);

  if (probe && (!probe.applicable || vlans.length === 0)) return null;
  return (
    <Card index={index} className="md:col-span-6 order-7 md:order-none"
      title={oneLine(t("overview.vlansActive"))} icon={Layers} iconTone="violet">
      {!probe ? <SkeletonRows rows={1} /> : (
        <div className="flex flex-wrap gap-2">
          {vlans.map((v) => (
            <span key={v.vid}
              className="inline-flex items-center gap-1.5 rounded-full bg-accent-soft px-3 py-1 text-small font-medium text-accent">
              <span className="font-mono tabular-nums">{v.vid}</span>
              <span className="text-muted">·</span>
              {v.name || t("overview.vlanUntagged")}
            </span>
          ))}
        </div>
      )}
    </Card>
  );
}
