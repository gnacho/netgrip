import { useState } from "react";
import { useTranslation } from "react-i18next";
import { LagCard } from "../components/ports/LagCard";
import { PortStatsCard } from "../components/ports/PortStatsCard";
import { PortTemplatesCard } from "../components/ports/PortTemplatesCard";
import { RoleProfilesCard } from "../components/ports/RoleProfilesCard";
import { SwitchModesCard } from "../components/ports/SwitchModesCard";
import { VLANTable } from "../components/ports/VLANTable";
import { IgmpCard, MacAclCard, StormControlCard } from "../components/tools/advanced";

type AdvancedKey =
  | "vlan"
  | "igmp"
  | "storm"
  | "macAcl"
  | "lag"
  | "portProfiles"
  | "portTemplates"
  | "switchModes"
  | "portStats";

const MENU: AdvancedKey[] = [
  "vlan",
  "igmp",
  "storm",
  "macAcl",
  "lag",
  "portProfiles",
  "portTemplates",
  "switchModes",
  "portStats",
];

/**
 * Sección Avanzadas (#441), solo visible con `netgrip.main.advanced=1`.
 * Menú desglosado estilo UniFi: una entrada por función con su nombre
 * técnico, explicación avanzada arriba y la tarjeta correspondiente debajo.
 */
export function AdvancedPage() {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<AdvancedKey>("vlan");

  const content: Record<AdvancedKey, React.ReactNode> = {
    vlan: <VLANTable />,
    igmp: <IgmpCard />,
    storm: <StormControlCard />,
    macAcl: <MacAclCard />,
    lag: <LagCard index={0} />,
    portProfiles: <RoleProfilesCard />,
    portTemplates: <PortTemplatesCard />,
    switchModes: <SwitchModesCard />,
    portStats: <PortStatsCard />,
  };

  return (
    <div className="flex flex-col gap-[var(--card-gap)] md:flex-row md:items-start">
      <nav aria-label={t("advanced.menuLabel")} className="md:w-60 md:shrink-0">
        <div className="flex gap-1 overflow-x-auto pb-1 md:flex-col md:overflow-visible md:pb-0">
          {MENU.map((key) => (
            <button
              key={key}
              type="button"
              aria-current={selected === key ? "true" : undefined}
              onClick={() => setSelected(key)}
              className={`shrink-0 whitespace-nowrap rounded-md px-3 py-2 text-left text-body font-medium transition-colors duration-[var(--dur-fast)]
                ${selected === key
                  ? "bg-accent-soft text-accent"
                  : "text-muted hover:bg-surface-2 hover:text-text"}`}
            >
              {t(`advanced.menu.${key}`)}
            </button>
          ))}
        </div>
      </nav>

      <div className="min-w-0 flex-1">
        <p className="text-eyebrow text-faint mb-2">{t(`advanced.menu.${selected}`)}</p>
        <p className="text-small text-muted">{t(`advanced.intro.${selected}`)}</p>
        <div className="mt-[var(--card-gap)]">{content[selected]}</div>
      </div>
    </div>
  );
}
