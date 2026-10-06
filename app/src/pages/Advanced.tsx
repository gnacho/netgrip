import { useTranslation } from "react-i18next";
import { LagCard } from "../components/ports/LagCard";
import { PortStatsCard } from "../components/ports/PortStatsCard";
import { PortTemplatesCard } from "../components/ports/PortTemplatesCard";
import { RoleProfilesCard } from "../components/ports/RoleProfilesCard";
import { SwitchModesCard } from "../components/ports/SwitchModesCard";
import { VLANTable } from "../components/ports/VLANTable";
import { IgmpCard, MacAclCard, StormControlCard } from "../components/tools/advanced";

/**
 * Sección Avanzadas (#441), solo visible con `netgrip.main.advanced=1`.
 * Agrupa lo que antes estaba disperso (VLAN en Puertos, IGMP/storm/MAC ACL
 * en el cajón de Herramientas, LAG y opciones de ingeniería de puertos bajo
 * "Opciones avanzadas" en Puertos) y crece con SNMP (#442) y switch avanzado
 * (#443). Aire UniFi: secciones por dominio, tarjetas limpias, riesgo
 * documentado en cada una.
 */
export function AdvancedPage() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <section aria-label={t("advanced.sectionNetworks")}>
        <p className="text-eyebrow text-faint mb-2">{t("advanced.sectionNetworks")}</p>
        <VLANTable />
      </section>

      <section aria-label={t("advanced.sectionSwitch")}>
        <p className="text-eyebrow text-faint mb-2">{t("advanced.sectionSwitch")}</p>
        <div className="grid grid-cols-1 gap-[var(--card-gap)] md:grid-cols-2">
          <IgmpCard />
          <StormControlCard />
          <MacAclCard />
        </div>

        <p className="text-eyebrow text-faint mt-[var(--card-gap)] mb-2">
          {t("advanced.sectionPortEngineering")}
        </p>
        <div className="grid grid-cols-1 gap-[var(--card-gap)] md:grid-cols-2">
          <LagCard index={0} />
          <RoleProfilesCard />
          <SwitchModesCard />
          <PortTemplatesCard />
          <PortStatsCard />
        </div>
      </section>
    </div>
  );
}
