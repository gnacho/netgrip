import { useTranslation } from "react-i18next";
import { GitBranch, ShieldCheck, Waves } from "lucide-react";
import { Card } from "../components/ui";
import { LagCard } from "../components/ports/LagCard";
import { PortStatsCard } from "../components/ports/PortStatsCard";
import { PortTemplatesCard } from "../components/ports/PortTemplatesCard";
import { RoleProfilesCard } from "../components/ports/RoleProfilesCard";
import { SfpCard } from "../components/ports/SfpCard";
import { StpBridgeCard, StpPortsCard } from "../components/ports/StpCards";
import { SwitchModesCard } from "../components/ports/SwitchModesCard";
import { MacAclCard, StormControlCard } from "../components/tools/advanced";

/**
 * Switch (avanzadas, #485): pocas tarjetas densas de ancho total en vez de
 * una retícula a medias. Cada grupo envuelve las tarjetas existentes con
 * columnas internas: STP (ajustes de puente + tabla de puertos), LAG a todo
 * el ancho, protección y perfiles (tormentas a todo el ancho, luego MAC ACL,
 * perfiles, plantillas y modos por parejas) y ópticos con estadísticas.
 */
export function AdvancedSwitchPage() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <p className="text-small text-muted">{t("advanced.pageSwitchIntro")}</p>

      <Card variant="subtle" animate={false} icon={GitBranch} title={t("advanced.groupStp")}>
        <div className="flex flex-col gap-[var(--card-gap)]">
          <StpBridgeCard index={0} />
          <StpPortsCard index={1} />
        </div>
      </Card>

      <LagCard index={2} />

      <Card variant="subtle" animate={false} icon={ShieldCheck} title={t("advanced.groupProtection")}>
        {/* Tarjetas apiladas a ancho total: nada al 50% dentro de un grupo. */}
        <div className="flex flex-col gap-[var(--card-gap)]">
          <StormControlCard />
          <MacAclCard />
          <RoleProfilesCard />
          <PortTemplatesCard />
          <SwitchModesCard />
        </div>
      </Card>

      <Card variant="subtle" animate={false} icon={Waves} title={t("advanced.groupOptics")}>
        <div className="flex flex-col gap-[var(--card-gap)]">
          <SfpCard index={0} />
          <PortStatsCard />
        </div>
      </Card>
    </div>
  );
}
