import type { DDNSProbe, IPv6Probe, OVPNProbe, SQMProbe, TSProbe, WGProbe } from "../types";
import { WireguardCard } from "../components/WireguardCard";
import { Ipv6Card } from "../components/Ipv6Card";
import { DdnsCard } from "../components/DdnsCard";
import { SqmCard } from "../components/SqmCard";
import { OpenvpnCard } from "../components/OpenvpnCard";
import { TailscaleCard } from "../components/TailscaleCard";
import { NlbwmonCard } from "../components/NlbwmonCard";

export function Services({ wg, onWgChange, ipv6, onIpv6Change, ddns, onDdnsChange, sqm, onSqmChange, ovpn, onOvpnChange, ts, onTsChange }: {
  wg: WGProbe | undefined;
  onWgChange: (p: WGProbe) => void;
  ipv6: IPv6Probe | undefined;
  onIpv6Change: (p: IPv6Probe) => void;
  ddns: DDNSProbe | undefined;
  onDdnsChange: (p: DDNSProbe) => void;
  sqm: SQMProbe | undefined;
  onSqmChange: (p: SQMProbe) => void;
  ovpn: OVPNProbe | undefined;
  onOvpnChange: (p: OVPNProbe) => void;
  ts: TSProbe | undefined;
  onTsChange: (p: TSProbe) => void;
}) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <div className="sm:col-span-2 xl:col-span-3">
        <WireguardCard probe={wg} onChange={onWgChange} />
      </div>
      <DdnsCard probe={ddns} onChange={onDdnsChange} />
      <Ipv6Card probe={ipv6} onChange={onIpv6Change} />
      <SqmCard probe={sqm} onChange={onSqmChange} />
      <div className="sm:col-span-2 xl:col-span-3">
        <OpenvpnCard probe={ovpn} onChange={onOvpnChange} />
      </div>
      <TailscaleCard probe={ts} onChange={onTsChange} />
      <NlbwmonCard />
    </div>
  );
}
