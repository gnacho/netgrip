import { useTranslation } from "react-i18next";
import { LagCard } from "../components/ports/LagCard";
import { PortStatsCard } from "../components/ports/PortStatsCard";
import { PortTemplatesCard } from "../components/ports/PortTemplatesCard";
import { RoleProfilesCard } from "../components/ports/RoleProfilesCard";
import { SfpCard } from "../components/ports/SfpCard";
import { StpBridgeCard, StpPortsCard } from "../components/ports/StpCards";
import { SwitchModesCard } from "../components/ports/SwitchModesCard";
import { MacAclCard, StormControlCard } from "../components/tools/advanced";

/**
 * Switch (avanzadas): STP, control de tormentas, MAC ACL, LAG y los perfiles
 * y modos de puerto. LagCard abre la retícula (index 0).
 */
export function AdvancedSwitchPage() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <p className="text-small text-muted">{t("advanced.pageSwitchIntro")}</p>
      <div className="grid grid-cols-1 gap-[var(--card-gap)] md:grid-cols-2">
        <StpBridgeCard index={0} />
        <SfpCard index={1} />
        <StpPortsCard index={2} />
        <LagCard index={3} />
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
