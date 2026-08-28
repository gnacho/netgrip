import type { FwdProbe } from "../types";
import { PortForwardCard } from "../components/PortForwardCard";
import { VLANTable } from "../components/VLANTable";
import { SwitchCard } from "../components/SwitchCard";
import { PortStatsCard } from "../components/PortStatsCard";
import { SwitchModesCard } from "../components/SwitchModesCard";
import { PoECard } from "../components/PoECard";
import { PortTemplatesCard } from "../components/PortTemplatesCard";
import { RoleProfilesCard } from "../components/RoleProfilesCard";

export function Ports({ fwd, onFwdChange }: {
  fwd: FwdProbe | undefined;
  onFwdChange: (p: FwdProbe) => void;
}) {
  return (
    <div className="grid gap-4">
      <PortForwardCard probe={fwd} onChange={onFwdChange} />
      <SwitchCard />
      <PortStatsCard />
      <PoECard />
      <PortTemplatesCard />
      <RoleProfilesCard />
      <SwitchModesCard />
      <VLANTable />
    </div>
  );
}
