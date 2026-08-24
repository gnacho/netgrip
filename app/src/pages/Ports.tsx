import type { FwdProbe } from "../types";
import { PortForwardCard } from "../components/PortForwardCard";

export function Ports({ fwd, onFwdChange }: {
  fwd: FwdProbe | undefined;
  onFwdChange: (p: FwdProbe) => void;
}) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <PortForwardCard probe={fwd} onChange={onFwdChange} />
    </div>
  );
}
