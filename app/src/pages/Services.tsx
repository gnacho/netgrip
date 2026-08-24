import type { DDNSProbe, FwdProbe, IoTProbe, IPv6Probe, OVPNProbe, SQMProbe, TSProbe, WGProbe } from "../types";
import { WireguardCard } from "../components/WireguardCard";
import { Ipv6Card } from "../components/Ipv6Card";
import { DdnsCard } from "../components/DdnsCard";
import { SqmCard } from "../components/SqmCard";
import { OpenvpnCard } from "../components/OpenvpnCard";
import { IotWifiCard } from "../components/IotWifiCard";
import { PortForwardCard } from "../components/PortForwardCard";
import { TailscaleCard } from "../components/TailscaleCard";

export function Services({ wg, onWgChange, ipv6, onIpv6Change, ddns, onDdnsChange, sqm, onSqmChange, ovpn, onOvpnChange, iot, onIotChange, fwd, onFwdChange, ts, onTsChange }: {
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
  iot: IoTProbe | undefined;
  onIotChange: (p: IoTProbe) => void;
  fwd: FwdProbe | undefined;
  onFwdChange: (p: FwdProbe) => void;
  ts: TSProbe | undefined;
  onTsChange: (p: TSProbe) => void;
}) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <WireguardCard probe={wg} onChange={onWgChange} />
      <DdnsCard probe={ddns} onChange={onDdnsChange} />
      <Ipv6Card probe={ipv6} onChange={onIpv6Change} />
      <SqmCard probe={sqm} onChange={onSqmChange} />
      <OpenvpnCard probe={ovpn} onChange={onOvpnChange} />
      <IotWifiCard probe={iot} onChange={onIotChange} />
      <PortForwardCard probe={fwd} onChange={onFwdChange} />
      <TailscaleCard probe={ts} onChange={onTsChange} />
    </div>
  );
}
