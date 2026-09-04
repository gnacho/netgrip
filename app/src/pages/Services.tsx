import type { CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import type { DDNSProbe, IPv6Probe, MDNSProbe, OVPNProbe, SQMProbe, TSProbe, WGProbe } from "../types";
import { WireguardCard } from "../components/services/WireguardCard";
import { OpenvpnCard } from "../components/services/OpenvpnCard";
import { TailscaleCard } from "../components/services/TailscaleCard";
import { DdnsCard } from "../components/services/DdnsCard";
import { MdnsCard } from "../components/services/MdnsCard";
import { Ipv6Card } from "../components/services/Ipv6Card";
import { SqmCard } from "../components/services/SqmCard";
import { NlbwmonCard } from "../components/services/NlbwmonCard";
import { FirewallCard } from "../components/services/FirewallCard";
import { AdguardCard } from "../components/services/AdguardCard";

/** Cabecera de grupo: eyebrow + una frase llana small muted (services.md §1). */
function GroupHeader({ title, desc, index }: { title: string; desc: string; index: number }) {
  return (
    <div style={{ "--i": index } as CSSProperties} className="animate-fade-up">
      <p className="text-eyebrow text-faint">{title}</p>
      <p className="text-small text-muted mt-0.5">{desc}</p>
    </div>
  );
}

/**
 * Servicios (services.md): toggles con superpoderes agrupados por tarea.
 * Entrada escalonada por grupo (VPN inmediato, siguientes +80ms).
 */
export function Services({ wg, onWgChange, ipv6, onIpv6Change, ddns, onDdnsChange, mdns, onMdnsChange, sqm, onSqmChange, ovpn, onOvpnChange, ts, onTsChange }: {
  wg: WGProbe | undefined;
  onWgChange: (p: WGProbe) => void;
  ipv6: IPv6Probe | undefined;
  onIpv6Change: (p: IPv6Probe) => void;
  ddns: DDNSProbe | undefined;
  onDdnsChange: (p: DDNSProbe) => void;
  mdns: MDNSProbe | undefined;
  onMdnsChange: (p: MDNSProbe) => void;
  sqm: SQMProbe | undefined;
  onSqmChange: (p: SQMProbe) => void;
  ovpn: OVPNProbe | undefined;
  onOvpnChange: (p: OVPNProbe) => void;
  ts: TSProbe | undefined;
  onTsChange: (p: TSProbe) => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-6 md:gap-8">
      {/* Bloqueo y reglas — el bloqueador de anuncios arriba */}
      <section className="flex flex-col gap-[var(--card-gap)]">
        <GroupHeader index={0} title={t("services.groupRules")} desc={t("services.groupRulesDesc")} />
        <AdguardCard index={0} />
        <NlbwmonCard index={0} />
        <FirewallCard index={0} />
      </section>

      {/* Tu conexión — cómo sale tu casa a Internet */}
      <section className="flex flex-col gap-[var(--card-gap)]">
        <GroupHeader index={2} title={t("services.groupConn")} desc={t("services.groupConnDesc")} />
        <DdnsCard probe={ddns} onChange={onDdnsChange} index={2} />
        <MdnsCard probe={mdns} onChange={onMdnsChange} index={2} />
        <Ipv6Card probe={ipv6} onChange={onIpv6Change} index={2} />
        <SqmCard probe={sqm} onChange={onSqmChange} index={2} />
      </section>

      {/* VPN — para entrar a tu casa desde fuera (al final) */}
      <section className="flex flex-col gap-[var(--card-gap)]">
        <GroupHeader index={4} title={t("services.groupVpn")} desc={t("services.groupVpnDesc")} />
        <WireguardCard probe={wg} onChange={onWgChange} index={4} />
        <OpenvpnCard probe={ovpn} onChange={onOvpnChange} index={4} />
        <TailscaleCard probe={ts} onChange={onTsChange} index={4} />
      </section>
    </div>
  );
}
