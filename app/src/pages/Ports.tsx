import { AdvancedDisclosure, Card } from "../components/ui";
import { PoECard } from "../components/ports/PoECard";
import { SwitchCard } from "../components/ports/SwitchCard";
import { LagCard } from "../components/ports/LagCard";
import { PortStatsCard } from "../components/ports/PortStatsCard";
import { SwitchModesCard } from "../components/ports/SwitchModesCard";
import { PortTemplatesCard } from "../components/ports/PortTemplatesCard";
import { RoleProfilesCard } from "../components/ports/RoleProfilesCard";
import { VLANTable } from "../components/ports/VLANTable";

/**
 * Puertos (ports.md): plantillas rápidas, PoE y bocas del switch; lo de
 * ingeniería (plantillas de puerto, perfiles, modos, VLANs, estadísticas)
 * bajo "Opciones avanzadas". El port-forwarding (abrir puertos a Internet)
 * vive ahora en la página WAN.
 */
export function Ports() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-[var(--card-gap)]">
      <PoECard index={1} />
      <SwitchCard index={2} />
      <LagCard index={2} />

      <Card index={3} className="md:col-span-2">
        <AdvancedDisclosure>
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <RoleProfilesCard />
            <SwitchModesCard />
            <PortTemplatesCard />
            <PortStatsCard />
            <div className="lg:col-span-2">
              <VLANTable />
            </div>
          </div>
        </AdvancedDisclosure>
      </Card>
    </div>
  );
}
