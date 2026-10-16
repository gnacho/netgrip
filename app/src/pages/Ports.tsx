import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { EthPort, PhysPortsProbe } from "../types";
import { api } from "../api";
import { degradedPorts, fmtMbps } from "../lib/degradedPorts";
import { Banner } from "../components/ui/Banner";
import { EthPortsCard } from "../components/ports/EthPortsCard";
import { PoECard } from "../components/ports/PoECard";
import { VLANTable } from "../components/ports/VLANTable";
import { LagCard } from "../components/ports/LagCard";
import { StpBridgeCard, StpPortsCard } from "../components/ports/StpCards";
import { SfpCard } from "../components/ports/SfpCard";
import { RoleProfilesCard } from "../components/ports/RoleProfilesCard";
import { PortTemplatesCard } from "../components/ports/PortTemplatesCard";
import { SwitchModesCard } from "../components/ports/SwitchModesCard";
import { IgmpCard, MacAclCard, StormControlCard } from "../components/tools/advanced";
import { MirrorCard, RateLimitCard } from "../components/ports/TrafficCards";
import { PortIsolationCard, PortSecurityCard } from "../components/ports/SecurityCards";
import { SegmentedControl } from "../components/ui/SegmentedControl";

/**
 * Puertos ethernet (#353, antes "Puertos"): el chasis RJ45 es el
 * protagonista (ancho completo) y cada boca abre su detalle al pulsarla
 * (nota, admin y PoE; #484). El port-forwarding (abrir puertos a
 * Internet) vive en la página "Puertos" (Forwards.tsx). El chasis RJ45 de
 * la instalación llega del resumen (#384).
 *
 * Pestañas por capacidad (#487): la página deja de depender del rol y
 * ofrece una pestaña por cada capacidad que el equipo tenga: Puertos
 * (siempre), VLANs (/api/vlans), Agregación (/api/lag), STP y tormentas
 * (modo avanzado + /api/stp, con IGMP al final), Tráfico (modo avanzado +
 * espejo/banda ancha, #443), Ópticas (jaulas SFP en /api/physports) y
 * Perfiles y seguridad (modo avanzado). Las entradas "Redes" y "Switch"
 * avanzadas desaparecen del menú: todo su contenido vive aquí o en
 * Sistema (SNMP). El gating usa tres estados: mientras un probe no
 * responde la pestaña se muestra (estable, sin parpadeo) y solo se oculta
 * tras confirmar que no aplica; cada tarjeta vuelve a cargar su probe al
 * activarse, así que un falso positivo del gate se corrige solo con un
 * contenido vacío.
 */

type PortsTab = "ports" | "vlans" | "lag" | "stp" | "traffic" | "optics" | "profiles";

export function Ports({ ethports }: { ethports?: EthPort[] }) {
  const { t } = useTranslation();
  const [tab, setTab] = useState<PortsTab>("ports");

  // Tri-state de capacidad: undefined = aun no se sabe -> mostrar la
  // pestaña (comportamiento estable, sin apariciones/desapariciones).
  const [vlansOk, setVlansOk] = useState<boolean>();
  const [lagOk, setLagOk] = useState<boolean>();
  const [stpOk, setStpOk] = useState<boolean>();
  const [trafficOk, setTrafficOk] = useState<boolean>();
  const [opticsOk, setOpticsOk] = useState<boolean>();
  // #522: el mismo probe de Ópticas alimenta el banner de puertos que
  // negocian por debajo de su capacidad (un solo fetch para ambas cosas).
  const [physProbe, setPhysProbe] = useState<PhysPortsProbe>();
  useEffect(() => {
    let live = true;
    api.vlans().then((p) => { if (live) setVlansOk(p.applicable); }).catch(() => {});
    api.lag().then((p) => { if (live) setLagOk(p.applicable); }).catch(() => {});
    api.stp().then((p) => { if (live) setStpOk(p.applicable); }).catch(() => {});
    api.physPorts().then((p) => {
      if (!live) return;
      setPhysProbe(p);
      setOpticsOk(p.applicable && p.ports.some((x) => !!x.sfp));
    }).catch(() => {});
    // Tráfico (#443): la pestaña se ofrece si ALGUNA de las dos funciones
    // es aplicable y su capacidad concreta funciona en este kernel.
    Promise.allSettled([api.mirror(), api.rateLimit()]).then(([m, r]) => {
      if (!live) return;
      const mOk = m.status === "fulfilled" && m.value.applicable && m.value.mirror_supported !== false;
      const rOk = r.status === "fulfilled" && r.value.applicable &&
        (r.value.ingress_supported !== false || r.value.egress_supported !== false);
      setTrafficOk(mOk || rOk);
    });
    return () => { live = false; };
  }, []);

  const options = [
    { value: "ports" as PortsTab, label: t("ports.tab.ports") },
    ...(vlansOk !== false ? [{ value: "vlans" as PortsTab, label: t("ports.tab.vlans") }] : []),
    ...(lagOk !== false ? [{ value: "lag" as PortsTab, label: t("ports.tab.lag") }] : []),
    ...(stpOk !== false
      ? [{ value: "stp" as PortsTab, label: t("ports.tab.stp") }]
      : []),
    ...(trafficOk !== false
      ? [{ value: "traffic" as PortsTab, label: t("ports.tab.traffic") }]
      : []),
    ...(opticsOk !== false ? [{ value: "optics" as PortsTab, label: t("ports.tab.optics") }] : []),
    { value: "profiles" as PortsTab, label: t("ports.tab.profiles") },
  ];
  // Si la pestaña activa acaba ocultandose (probe que confirma no-aplica),
  // se vuelve a Puertos en vez de quedarse en un panel vacio.
  const active = options.some((o) => o.value === tab) ? tab : "ports";

  // #522: puertos que negocian por debajo de su capacidad (cable dañado o
  // de 4 hilos). Aviso accionable arriba de la pestaña Puertos.
  const degraded = degradedPorts(physProbe);

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {/* self-start: sin él el contenedor flex-col estira el tablist a
          todo el ancho (align-items: stretch blockifica el inline-flex). */}
      <div className="self-start">
        <SegmentedControl<PortsTab>
          ariaLabel={t("ports.tabsAria")}
          value={active}
          onChange={setTab}
          options={options}
        />
      </div>

      {active === "ports" && degraded.length > 0 && (
        <Banner tone="warn" className="mb-0">
          <span className="font-medium">{t("ports.slowBannerTitle")}</span>{" "}
          {degraded.map((d) => t("ports.slowBannerItem", { port: d.name, speed: fmtMbps(d.speed), max: fmtMbps(d.max) })).join(" · ")}
          {" "}{t("ports.slowBannerHint")}
        </Banner>
      )}

      {active === "ports" && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-[var(--card-gap)]">
          <EthPortsCard ports={ethports} index={0} />
          <PoECard index={1} />
        </div>
      )}

      {active === "vlans" && <VLANTable />}

      {active === "lag" && <LagCard index={0} />}

      {active === "stp" && (
        <div className="flex flex-col gap-[var(--card-gap)]">
          <StpBridgeCard index={0} />
          <StpPortsCard index={1} />
          <StormControlCard />
          <IgmpCard />
        </div>
      )}

      {active === "traffic" && (
        <div className="flex flex-col gap-[var(--card-gap)]">
          <MirrorCard />
          <RateLimitCard />
        </div>
      )}

      {active === "optics" && (
        <div className="flex flex-col gap-[var(--card-gap)]">
          <SfpCard index={0} />
        </div>
      )}

      {active === "profiles" && (
        <div className="flex flex-col gap-[var(--card-gap)]">
          <RoleProfilesCard />
          <PortTemplatesCard />
          <SwitchModesCard />
          <PortIsolationCard />
          <PortSecurityCard />
          <MacAclCard />
        </div>
      )}
    </div>
  );
}
