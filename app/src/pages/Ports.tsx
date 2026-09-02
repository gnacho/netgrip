import type { FwdProbe } from "../types";
import { AdvancedDisclosure, Card } from "../components/ui";
import { PortForwardCard } from "../components/ports/PortForwardCard";
import { PoECard } from "../components/ports/PoECard";
import { SwitchCard } from "../components/ports/SwitchCard";
import { LagCard } from "../components/ports/LagCard";
import { PortStatsCard } from "../components/ports/PortStatsCard";
import { SwitchModesCard } from "../components/ports/SwitchModesCard";
import { PortTemplatesCard } from "../components/ports/PortTemplatesCard";
import { RoleProfilesCard } from "../components/ports/RoleProfilesCard";
import { VLANTable } from "../components/ports/VLANTable";

/**
 * Puertos (ports.md): abrir algo a Internet + plantillas rápidas, PoE y bocas
 * del switch; lo de ingeniería (plantillas de puerto, perfiles, modos, VLANs,
 * estadísticas) bajo "Opciones avanzadas".
 */
export function Ports({ fwd, onFwdChange }: {
  fwd: FwdProbe | undefined;
  onFwdChange: (p: FwdProbe) => void;
}) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-[var(--card-gap)]">
      <div className="md:col-span-2">
        <PortForwardCard probe={fwd} onChange={onFwdChange} />
      </div>

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
