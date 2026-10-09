import { useTranslation } from "react-i18next";
import { VLANTable } from "../components/ports/VLANTable";
import { IgmpCard } from "../components/tools/advanced";
import { SnmpCard } from "../components/advanced/SnmpCard";
import { useMode } from "../hooks/useMode";

/**
 * Redes (avanzadas): VLAN 802.1Q y snooping IGMP.
 *
 * Rol=switch (#487): la tabla de VLANs vive en la pestaña "VLANs" de
 * "Puertos y VLANs", así que aquí se oculta y la página se reduce a
 * IGMP + SNMP. Para cualquier otro rol se pinta todo como hasta ahora.
 */
export function AdvancedNetworkPage() {
  const { t } = useTranslation();
  const { mode, modeReady } = useMode();
  const isSwitch = modeReady && mode?.role === "switch";
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <p className="text-small text-muted">{t("advanced.pageNetworkIntro")}</p>
      {!isSwitch && <VLANTable />}
      <IgmpCard />
      <section aria-label={t("advanced.sectionMonitoring")}>
        <p className="text-eyebrow text-faint mb-2">{t("advanced.sectionMonitoring")}</p>
        <SnmpCard />
      </section>
    </div>
  );
}
