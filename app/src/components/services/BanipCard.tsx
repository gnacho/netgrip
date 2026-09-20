import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ShieldBan } from "lucide-react";
import { api } from "../../api";
import type { BanipStatus } from "../../types";
import { Button, Card, Pill, SkeletonRows } from "../ui";
import { TechName } from "./shared";

/** banIP en Servicios: estado compacto y atajo a su página (#351). */
export function BanipCard({ index = 0, onNavigate }: { index?: number; onNavigate?: (p: string) => void }) {
  const { t } = useTranslation();
  const [st, setSt] = useState<BanipStatus>();

  useEffect(() => {
    api.banipStatus().then(setSt).catch(() => {});
  }, []);

  if (!st) {
    return <Card index={index} icon={ShieldBan} title={t("banip.cardTitle")}><SkeletonRows rows={2} /></Card>;
  }
  // Solo aplica en el gateway (mismo criterio que la entrada del menú).
  if (!st.applicable) return null;

  const active = st.enabled && st.running;
  const stateKey = !st.installed ? "banip.cardStateNotInstalled" : active ? "banip.cardStateActive" : "banip.cardStateInactive";

  return (
    <Card
      index={index}
      icon={ShieldBan}
      iconTone={active ? "success" : "muted"}
      title={t("banip.cardTitle")}
      action={onNavigate && (
        <Button variant="secondary" size="sm" onClick={() => onNavigate("banip")}>
          {t("banip.cardManage")}
        </Button>
      )}
    >
      <TechName>banIP</TechName>
      <div className="mt-1 flex items-center gap-2">
        <Pill tone={active ? "ok" : "muted"} live={active}>{t(stateKey)}</Pill>
      </div>
      <p className="mt-2 text-caption text-muted">{t("banip.cardDesc")}</p>
    </Card>
  );
}
