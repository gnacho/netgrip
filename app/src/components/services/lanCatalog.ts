import type { LucideIcon } from "lucide-react";
import {
  Boxes, Container, Database, Film, Filter, Globe, HardDrive, Home,
  Image, LineChart, Play, Server, ShieldCheck,
} from "lucide-react";

/**
 * Catálogo de servicios locales (#303): icono + clave i18n por preset.
 * Los puertos/esquemas por defecto viven en el backend (`lanservices.go`),
 * que los expone en `GET /api/lanservices`; aquí solo la presentación.
 */
export const LAN_SERVICE_KINDS: Record<string, { icon: LucideIcon; labelKey: string }> = {
  homeassistant: { icon: Home, labelKey: "lanservices.kind.homeassistant" },
  pihole: { icon: Filter, labelKey: "lanservices.kind.pihole" },
  adguardhome: { icon: ShieldCheck, labelKey: "lanservices.kind.adguardhome" },
  proxmox: { icon: Server, labelKey: "lanservices.kind.proxmox" },
  immich: { icon: Image, labelKey: "lanservices.kind.immich" },
  jellyfin: { icon: Film, labelKey: "lanservices.kind.jellyfin" },
  plex: { icon: Play, labelKey: "lanservices.kind.plex" },
  truenas: { icon: Database, labelKey: "lanservices.kind.truenas" },
  synology: { icon: HardDrive, labelKey: "lanservices.kind.synology" },
  openmediavault: { icon: Container, labelKey: "lanservices.kind.openmediavault" },
  portainer: { icon: Boxes, labelKey: "lanservices.kind.portainer" },
  grafana: { icon: LineChart, labelKey: "lanservices.kind.grafana" },
};

export const LAN_SERVICE_KIND_KEYS = Object.keys(LAN_SERVICE_KINDS);

/** Icono por preset; fallback a un globo (servicios personalizados). */
export function lanServiceIcon(kind: string): LucideIcon {
  return LAN_SERVICE_KINDS[kind]?.icon ?? Globe;
}
