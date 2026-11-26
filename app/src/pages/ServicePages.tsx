import type { DDNSProbe, OVPNProbe, SQMProbe, TSProbe, WGProbe } from "../types";
import { WireguardCard } from "../components/services/WireguardCard";
import { OpenvpnCard } from "../components/services/OpenvpnCard";
import { TailscaleCard } from "../components/services/TailscaleCard";
import { DdnsCard } from "../components/services/DdnsCard";
import { SqmCard } from "../components/services/SqmCard";
import { NlbwmonCard } from "../components/services/NlbwmonCard";
import { LanServicesCard } from "../components/services/LanServicesCard";

/**
 * Secciones de Servicios desglosadas como entradas propias del menú (#691).
 * Eran grupos dentro de una única página; el menú ahora las nombra una a una
 * y cada una abre solo sus tarjetas.
 */

export function VpnPage({ wg, onWgChange, ovpn, onOvpnChange, ts, onTsChange }: {
  wg: WGProbe | undefined;
  onWgChange: (p: WGProbe) => void;
  ovpn: OVPNProbe | undefined;
  onOvpnChange: (p: OVPNProbe) => void;
  ts: TSProbe | undefined;
  onTsChange: (p: TSProbe) => void;
}) {
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <WireguardCard probe={wg} onChange={onWgChange} index={0} />
      <OpenvpnCard probe={ovpn} onChange={onOvpnChange} index={1} />
      <TailscaleCard probe={ts} onChange={onTsChange} index={2} />
    </div>
  );
}

export function QualityPage({ sqm, onSqmChange }: {
  sqm: SQMProbe | undefined;
  onSqmChange: (p: SQMProbe) => void;
}) {
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <SqmCard probe={sqm} onChange={onSqmChange} index={0} />
      <NlbwmonCard index={1} />
    </div>
  );
}

export function DdnsPage({ ddns, onDdnsChange }: {
  ddns: DDNSProbe | undefined;
  onDdnsChange: (p: DDNSProbe) => void;
}) {
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <DdnsCard probe={ddns} onChange={onDdnsChange} index={0} />
    </div>
  );
}

export function LanServicesPage() {
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <LanServicesCard />
    </div>
  );
}
