import type { LucideIcon } from "lucide-react";
import {
  AirVent, Camera, Cpu, Gamepad2, HardDrive, Headphones, Laptop, Lightbulb,
  Monitor, Printer, Refrigerator, Router, Server, Smartphone, Speaker, Tablet,
  Tv, Watch,
} from "lucide-react";
import type { DeviceType } from "../../types";

/** Catálogo de tipos de dispositivo (design §clientes): etiqueta i18n + icono. */
export const DEVICE_TYPES: Record<DeviceType, { icon: LucideIcon; labelKey: string }> = {
  pc: { icon: Monitor, labelKey: "clients.type.pc" },
  laptop: { icon: Laptop, labelKey: "clients.type.laptop" },
  phone: { icon: Smartphone, labelKey: "clients.type.phone" },
  tablet: { icon: Tablet, labelKey: "clients.type.tablet" },
  camera: { icon: Camera, labelKey: "clients.type.camera" },
  wearable: { icon: Watch, labelKey: "clients.type.wearable" },
  printer: { icon: Printer, labelKey: "clients.type.printer" },
  audio: { icon: Headphones, labelKey: "clients.type.audio" },
  speaker: { icon: Speaker, labelKey: "clients.type.speaker" },
  tv: { icon: Tv, labelKey: "clients.type.tv" },
  gaming: { icon: Gamepad2, labelKey: "clients.type.gaming" },
  iot: { icon: Lightbulb, labelKey: "clients.type.iot" },
  gateway: { icon: Router, labelKey: "clients.type.gateway" },
  nas: { icon: HardDrive, labelKey: "clients.type.nas" },
  server: { icon: Server, labelKey: "clients.type.server" },
  appliance: { icon: Refrigerator, labelKey: "clients.type.appliance" },
  ac: { icon: AirVent, labelKey: "clients.type.ac" },
  other: { icon: Cpu, labelKey: "clients.type.other" },
};

export const DEVICE_TYPE_KEYS = Object.keys(DEVICE_TYPES) as DeviceType[];

/** Icono por tipo de dispositivo; fallback a "other". */
export function deviceTypeIcon(type?: string, name = ""): LucideIcon {
  if (type && type in DEVICE_TYPES) return DEVICE_TYPES[type as DeviceType].icon;
  // Inferencia ligera por nombre cuando no hay tipo asignado.
  const n = name.toLowerCase();
  if (/(phone|m[oó]vil|iphone|android|pixel|galaxy)/.test(n)) return Smartphone;
  if (/(tablet|ipad|tab)/.test(n)) return Tablet;
  if (/(laptop|macbook|thinkpad|notebook|port[aá]til)/.test(n)) return Laptop;
  if (/(desktop|imac|pc|escritorio)/.test(n)) return Monitor;
  if (/(\btv\b|televisi[oó]n|roku|bravia|chromecast)/.test(n)) return Tv;
  if (/(impresora|printer|laserjet|deskjet)/.test(n)) return Printer;
  if (/(nas|synology|qnap|servidor|server)/.test(n)) return Server;
  if (/(c[aá]mara|camera|cam-|webcam)/.test(n)) return Camera;
  if (/(roborock|roomba|aspiradora|robot)/.test(n)) return Lightbulb;
  return Cpu;
}
