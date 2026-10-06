import { useTranslation } from "react-i18next";
import { LagCard } from "../components/ports/LagCard";
import { PortStatsCard } from "../components/ports/PortStatsCard";
import { PortTemplatesCard } from "../components/ports/PortTemplatesCard";
import { RoleProfilesCard } from "../components/ports/RoleProfilesCard";
import { SwitchModesCard } from "../components/ports/SwitchModesCard";
import { MacAclCard, StormControlCard } from "../components/tools/advanced";

/**
 * Switch (avanzadas): control de tormentas, MAC ACL, LAG y los perfiles y
 * modos de puerto. LagCard abre la retícula (index 0).
 */
export function AdvancedSwitchPage() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <p className="text-small text-muted">{t("advanced.pageSwitchIntro")}</p>
      <div className="grid grid-cols-1 gap-[var(--card-gap)] md:grid-cols-2">
        <LagCard index={0} />
        <StormControlCard />
        <MacAclCard />
        <RoleProfilesCard />
        <PortTemplatesCard />
        <SwitchModesCard />
        <PortStatsCard />
      </div>
    </div>
  );
}
