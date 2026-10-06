import { useTranslation } from "react-i18next";
import { VLANTable } from "../components/ports/VLANTable";
import { IgmpCard, MacAclCard, StormControlCard } from "../components/tools/advanced";

/**
 * Sección Avanzadas (#441), solo visible con `netgrip.main.advanced=1`.
 * Agrupa lo que antes estaba disperso (VLAN en Puertos, IGMP/storm/MAC ACL
 * en el cajón de Herramientas) y crece con SNMP (#442) y switch avanzado
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
      </section>
    </div>
  );
}
