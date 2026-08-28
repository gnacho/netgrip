import type { FwdProbe } from "../types";
import { PortForwardCard } from "../components/PortForwardCard";
import { VLANTable } from "../components/VLANTable";

export function Ports({ fwd, onFwdChange }: {
  fwd: FwdProbe | undefined;
  onFwdChange: (p: FwdProbe) => void;
}) {
  return (
    <div className="grid gap-4">
      <PortForwardCard probe={fwd} onChange={onFwdChange} />
      <VLANTable />
    </div>
  );
}
