import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { EthPort } from "../types";
import { useMode } from "../hooks/useMode";
import { EthPortsCard } from "../components/ports/EthPortsCard";
import { PoECard } from "../components/ports/PoECard";
import { VLANTable } from "../components/ports/VLANTable";
import { LagCard } from "../components/ports/LagCard";
import { StpBridgeCard, StpPortsCard } from "../components/ports/StpCards";
import { StormControlCard } from "../components/tools/advanced";
import { SegmentedControl } from "../components/ui/SegmentedControl";
import { Badge } from "../components/ui";

/**
 * Puertos ethernet (#353, antes "Puertos"): el chasis RJ45 es el
 * protagonista (ancho completo) y cada boca abre su detalle al pulsarla
 * (nota, admin y PoE; #484). La tarjeta "Las bocas del switch" se
 * eliminó: su funcionalidad vive en esa banda de detalle. Lo de
 * ingeniería (LAG, plantillas de puerto, perfiles, modos, VLANs,
 * estadísticas, IGMP, storm control, MAC ACL) vive en la página
 * "Avanzadas" (#441). El port-forwarding (abrir puertos a Internet) vive
 * en la página "Puertos" (Forwards.tsx). El chasis RJ45 de la
 * instalación llega del resumen (#384).
 *
 * Rol=switch (#487): la página se convierte en "Puertos y VLANs" con
 * pestañas - Puertos (chasis + detalle del clic + PoE, igual que antes),
 * VLANs (editor 802.1Q), Agregación (LAG) y STP y tormentas (marcada
 * como avanzada). Para cualquier otro rol el layout no cambia: misma
 * retícula de dos columnas de siempre. La decisión espera a useMode
 * (hidrata desde caché, #484) para no parpadear entre layouts.
 */

type PortsTab = "ports" | "vlans" | "lag" | "stp";

export function Ports({ ethports }: { ethports?: EthPort[] }) {
  const { t } = useTranslation();
  const { mode, modeReady } = useMode();
  const [tab, setTab] = useState<PortsTab>("ports");

  const isSwitch = modeReady && mode?.role === "switch";

  if (!isSwitch) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-2 gap-[var(--card-gap)]">
        <EthPortsCard ports={ethports} index={0} />
        <PoECard index={1} />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {/* self-start: sin él el contenedor flex-col estira el tablist a
          todo el ancho (align-items: stretch blockifica el inline-flex). */}
      <div className="self-start">
        <SegmentedControl<PortsTab>
          ariaLabel={t("ports.tabsAria")}
          value={tab}
          onChange={setTab}
          options={[
            { value: "ports", label: t("ports.tab.ports") },
            { value: "vlans", label: t("ports.tab.vlans") },
            { value: "lag", label: t("ports.tab.lag") },
            {
              value: "stp",
              label: (
                <span className="inline-flex items-center gap-1.5">
                  {t("ports.tab.stp")}
                  <Badge tone="accent">{t("ports.tab.advanced")}</Badge>
                </span>
              ),
            },
          ]}
        />
      </div>

      {tab === "ports" && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-[var(--card-gap)]">
          <EthPortsCard ports={ethports} index={0} />
          <PoECard index={1} />
        </div>
      )}

      {tab === "vlans" && <VLANTable />}

      {tab === "lag" && <LagCard index={0} />}

      {tab === "stp" && (
        <div className="flex flex-col gap-[var(--card-gap)]">
          <StpBridgeCard index={0} />
          <StpPortsCard index={1} />
          <StormControlCard />
        </div>
      )}
    </div>
  );
}
