import type { IPv6Probe, WGProbe } from "../types";
import { WireguardCard } from "../components/WireguardCard";
import { Ipv6Card } from "../components/Ipv6Card";

export function Services({ wg, onWgChange, ipv6, onIpv6Change }: {
  wg: WGProbe | undefined;
  onWgChange: (p: WGProbe) => void;
  ipv6: IPv6Probe | undefined;
  onIpv6Change: (p: IPv6Probe) => void;
}) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <WireguardCard probe={wg} onChange={onWgChange} />
      <Ipv6Card probe={ipv6} onChange={onIpv6Change} />
    </div>
  );
}
