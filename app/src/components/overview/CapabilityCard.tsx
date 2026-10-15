import { useTranslation } from "react-i18next";
import { Check, ListChecks, X } from "lucide-react";
import { Card } from "../ui/Card";
import type { ModeProbe, StorageProbe, WirelessRadio } from "../../types";

/**
 * CapabilityCard (#521): "Detectado en este equipo" - chips con lo que el
 * probe YA sabe del dispositivo (WAN, radios Wi-Fi, switch integrado, USB)
 * y una frase llana de lo que significa su rol. Responde de un vistazo
 * "qué es este cacharro y qué has detectado en él"; antes las capacidades
 * solo se usaban para recortar el menú, sin decirlo nunca.
 */
export function CapabilityCard({ mode, wireless, storage }: {
  mode?: ModeProbe;
  wireless?: WirelessRadio[];
  storage?: StorageProbe;
}) {
  const { t } = useTranslation();
  const role = mode?.role ?? (mode?.mode === "ap" ? "ap" : "router");
  const radios = wireless?.length ?? 0;
  const hasWifi = mode?.has_wifi ?? radios > 0;
  const ports = mode?.port_count ?? 0;

  const chips: { ok: boolean; label: string }[] = [
    mode?.wan_configured
      ? { ok: true, label: t("overview.caps.wan") }
      : { ok: false, label: t("overview.caps.noWan") },
    hasWifi
      ? { ok: true, label: radios > 0 ? t("overview.caps.radios", { count: radios }) : t("overview.caps.wifi") }
      : { ok: false, label: t("overview.caps.noRadios") },
    ports > 0
      ? { ok: true, label: t("overview.caps.switchPorts", { count: ports }) }
      : { ok: false, label: t("overview.caps.noSwitch") },
  ];
  if (storage?.applicable) chips.push({ ok: true, label: t("overview.caps.usb") });

  return (
    <Card index={0} className="md:col-span-12" title={t("overview.detected")} icon={ListChecks} iconTone="muted">
      <div className="flex flex-wrap items-center gap-2">
        {chips.map((c) => (
          <span
            key={c.label}
            className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-caption font-medium border ${
              c.ok
                ? "bg-success-soft text-success border-success/20"
                : "bg-surface-2 text-muted border-border"
            }`}
          >
            {c.ok ? <Check size={13} strokeWidth={2.5} /> : <X size={13} strokeWidth={2.5} />}
            {c.label}
          </span>
        ))}
      </div>
      <p className="mt-2.5 text-caption text-muted">{t(`overview.roleLine.${role}`)}</p>
    </Card>
  );
}
