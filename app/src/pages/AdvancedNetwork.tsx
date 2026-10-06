import { useTranslation } from "react-i18next";
import { VLANTable } from "../components/ports/VLANTable";
import { IgmpCard } from "../components/tools/advanced";
import { SnmpCard } from "../components/advanced/SnmpCard";

/**
 * Redes (avanzadas): VLAN 802.1Q y snooping IGMP.
 */
export function AdvancedNetworkPage() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <p className="text-small text-muted">{t("advanced.pageNetworkIntro")}</p>
      <VLANTable />
      <IgmpCard />
      <section aria-label={t("advanced.sectionMonitoring")}>
        <p className="text-eyebrow text-faint mb-2">{t("advanced.sectionMonitoring")}</p>
        <SnmpCard />
      </section>
    </div>
  );
}
