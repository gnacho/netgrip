import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { EthPort } from "../types";
import { api } from "../api";
import { EthPortsCard } from "../components/ports/EthPortsCard";
import { PoECard } from "../components/ports/PoECard";
import { VLANTable } from "../components/ports/VLANTable";
import { LagCard } from "../components/ports/LagCard";
import { StpBridgeCard, StpPortsCard } from "../components/ports/StpCards";
import { SfpCard } from "../components/ports/SfpCard";
import { PortStatsCard } from "../components/ports/PortStatsCard";
import { RoleProfilesCard } from "../components/ports/RoleProfilesCard";
import { PortTemplatesCard } from "../components/ports/PortTemplatesCard";
import { SwitchModesCard } from "../components/ports/SwitchModesCard";
import { IgmpCard, MacAclCard, StormControlCard } from "../components/tools/advanced";
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
 * (modo avanzado + /api/stp, con IGMP al final), Ópticas (jaulas SFP en
 * /api/physports) y Perfiles y seguridad (modo avanzado). Las entradas
 * "Redes" y "Switch" avanzadas desaparecen del menú: todo su contenido
 * vive aquí o en Sistema (SNMP). El gating usa tres estados: mientras un
 * probe no responde la pestaña se muestra (estable, sin parpadeo) y solo
 * se oculta tras confirmar que no aplica; cada tarjeta vuelve a cargar su
 * probe al activarse, así que un falso positivo del gate se corrige solo
 * con un contenido vacío.
 */

type PortsTab = "ports" | "vlans" | "lag" | "stp" | "optics" | "profiles";

export function Ports({ ethports }: { ethports?: EthPort[] }) {
  const { t } = useTranslation();
  const [tab, setTab] = useState<PortsTab>("ports");

  // Tri-state de capacidad: undefined = aun no se sabe -> mostrar la
  // pestaña (comportamiento estable, sin apariciones/desapariciones).
  const [vlansOk, setVlansOk] = useState<boolean>();
  const [lagOk, setLagOk] = useState<boolean>();
  const [stpOk, setStpOk] = useState<boolean>();
  const [opticsOk, setOpticsOk] = useState<boolean>();
  const [advanced, setAdvanced] = useState<boolean>();

  useEffect(() => {
    let live = true;
    api.vlans().then((p) => { if (live) setVlansOk(p.applicable); }).catch(() => {});
    api.lag().then((p) => { if (live) setLagOk(p.applicable); }).catch(() => {});
    api.stp().then((p) => { if (live) setStpOk(p.applicable); }).catch(() => {});
    api.physPorts().then((p) => {
      if (live) setOpticsOk(p.applicable && p.ports.some((x) => !!x.sfp));
    }).catch(() => {});
    api.advanced().then((p) => { if (live) setAdvanced(p.advanced); }).catch(() => {});
    return () => { live = false; };
  }, []);

  const options = [
    { value: "ports" as PortsTab, label: t("ports.tab.ports") },
    ...(vlansOk !== false ? [{ value: "vlans" as PortsTab, label: t("ports.tab.vlans") }] : []),
    ...(lagOk !== false ? [{ value: "lag" as PortsTab, label: t("ports.tab.lag") }] : []),
    ...((advanced ?? true) && stpOk !== false
      ? [{ value: "stp" as PortsTab, label: t("ports.tab.stp") }]
      : []),
    ...(opticsOk !== false ? [{ value: "optics" as PortsTab, label: t("ports.tab.optics") }] : []),
    ...(advanced ?? true ? [{ value: "profiles" as PortsTab, label: t("ports.tab.profiles") }] : []),
  ];
  // Si la pestaña activa acaba ocultandose (probe que confirma no-aplica),
  // se vuelve a Puertos en vez de quedarse en un panel vacio.
  const active = options.some((o) => o.value === tab) ? tab : "ports";

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

      {active === "optics" && (
        <div className="flex flex-col gap-[var(--card-gap)]">
          <SfpCard index={0} />
          <PortStatsCard />
        </div>
      )}

      {active === "profiles" && (
        <div className="flex flex-col gap-[var(--card-gap)]">
          <RoleProfilesCard />
          <PortTemplatesCard />
          <SwitchModesCard />
          <MacAclCard />
        </div>
      )}
    </div>
  );
}
