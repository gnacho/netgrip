import type { CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import type { DDNSProbe, OVPNProbe, SQMProbe, TSProbe, WGProbe } from "../types";
import { WireguardCard } from "../components/services/WireguardCard";
import { OpenvpnCard } from "../components/services/OpenvpnCard";
import { TailscaleCard } from "../components/services/TailscaleCard";
import { DdnsCard } from "../components/services/DdnsCard";
import { SqmCard } from "../components/services/SqmCard";
import { NlbwmonCard } from "../components/services/NlbwmonCard";
import { AdguardCard } from "../components/services/AdguardCard";
import { LanServicesCard } from "../components/services/LanServicesCard";
import { BanipCard } from "../components/services/BanipCard";

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
export function Services({ wg, onWgChange, ddns, onDdnsChange, sqm, onSqmChange, ovpn, onOvpnChange, ts, onTsChange, apMode = false, onNavigate }: {
  wg: WGProbe | undefined;
  onWgChange: (p: WGProbe) => void;
  ddns: DDNSProbe | undefined;
  onDdnsChange: (p: DDNSProbe) => void;
  sqm: SQMProbe | undefined;
  onSqmChange: (p: SQMProbe) => void;
  ovpn: OVPNProbe | undefined;
  onOvpnChange: (p: OVPNProbe) => void;
  ts: TSProbe | undefined;
  onTsChange: (p: TSProbe) => void;
  apMode?: boolean;
  onNavigate?: (p: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-6 md:gap-8">
      {/* Protección: lo que se bloquea. Contabilidad y bloqueo por IP solo en
          el gateway (#500): un AP no enruta, así que nlbwmon no puede contar
          tráfico por equipo. Los encabezados también solo en gateway: describen
          funciones que en un AP no se pintan. */}
      {!apMode && (
        <section className="flex flex-col gap-[var(--card-gap)]">
          <GroupHeader index={0} title={t("services.groupProtection")} desc={t("services.groupProtectionDesc")} />
          <AdguardCard index={0} />
          <BanipCard index={0} onNavigate={onNavigate} />
        </section>
      )}

      {/* Calidad: qué usa la línea y cómo se reparte. nlbwmon responde "qué
          está gastando la línea", que es la misma pregunta que resuelve SQM. */}
      {!apMode && (
        <section className="flex flex-col gap-[var(--card-gap)]">
          <GroupHeader index={2} title={t("services.groupQuality")} desc={t("services.groupQualityDesc")} />
          <SqmCard probe={sqm} onChange={onSqmChange} index={2} />
          <NlbwmonCard index={2} />
        </section>
      )}

      {/* Nombre de casa: una tarjeta y un trabajo, mantener el dominio
          apuntando a la IP que cambia. */}
      {!apMode && (
        <section className="flex flex-col gap-[var(--card-gap)]">
          <GroupHeader index={4} title={t("services.groupDdns")} desc={t("services.groupDdnsDesc")} />
          <DdnsCard probe={ddns} onChange={onDdnsChange} index={4} />
        </section>
      )}

      {/* VPN — para entrar a tu casa desde fuera (al final). Solo gateway:
          un AP no enruta, así que los túneles no aplican (#500). */}
      {!apMode && (
      <section className="flex flex-col gap-[var(--card-gap)]">
        <GroupHeader index={6} title={t("services.groupVpn")} desc={t("services.groupVpnDesc")} />
        <WireguardCard probe={wg} onChange={onWgChange} index={6} />
        <OpenvpnCard probe={ovpn} onChange={onOvpnChange} index={6} />
        <TailscaleCard probe={ts} onChange={onTsChange} index={6} />
      </section>
      )}

      {/* Servicios locales: tus servicios self-hosted en otras máquinas.
          Solo gateway (#500): en un AP es información ajena a su función. */}
      {!apMode && (
      <section className="flex flex-col gap-[var(--card-gap)]">
        <GroupHeader index={8} title={t("services.groupLocal")} desc={t("services.groupLocalDesc")} />
        <LanServicesCard />
      </section>
      )}
    </div>
  );
}
