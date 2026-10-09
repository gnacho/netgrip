import { useTranslation } from "react-i18next";
import { Cable } from "lucide-react";
import type { EthPort } from "../../types";
import { Card, SkeletonRows } from "../ui";
import { oneLine } from "./cards";

/**
 * Mini-chasis del Resumen en modo gateway (#487, mockup de rediseño): las
 * bocas del equipo de un vistazo, solo lectura. La gestión (detalle por
 * boca, PoE, notas) sigue en la página Puertos; aquí solo estado, velocidad
 * y qué hay conectado, con un enlace a esa página.
 */
export function MiniChassis({ ports, onNavigate, index = 3 }: {
  ports?: EthPort[];
  onNavigate: (p: string) => void;
  index?: number;
}) {
  const { t } = useTranslation();

  const label = (p: EthPort) =>
    p.wan ? "WAN" : p.name.toUpperCase().replace(/^LAN(\d+)$/, "LAN $1");
  const speed = (p: EthPort) =>
    p.speed_mbps >= 1000 ? `${p.speed_mbps / 1000} Gb/s` : `${p.speed_mbps} Mb/s`;

  return (
    <Card index={index} className="md:col-span-12 order-8 md:order-none"
      title={oneLine(t("overview.ports"))} icon={Cable} iconTone="teal"
      action={ports && ports.length > 0 && (
        <button type="button" onClick={() => onNavigate("ports")}
          className="text-small text-accent hover:text-accent-hover ring-focus rounded-sm">
          {t("overview.portsManage")} →
        </button>
      )}>
      {!ports ? <SkeletonRows rows={2} /> : ports.length === 0 ? (
        <p className="text-small text-muted">{t("overview.portsEmpty")}</p>
      ) : (
        <>
          <p className="text-caption text-muted mb-3">
            {t("ports.inUse", { used: ports.filter((p) => p.up).length, total: ports.length })}
          </p>
          <div className="flex flex-wrap gap-3">
            {ports.map((p) => {
              const device = !p.up ? t("ports.free")
                : p.devices.length === 1 ? p.devices[0].name || p.devices[0].mac
                : p.devices.length > 1 ? t("ports.unmanagedN", { count: p.devices.length })
                : t("ports.busy");
              const led = !p.up ? "bg-border-strong"
                : p.speed_mbps >= 1000 ? "bg-ok" : "bg-warn";
              return (
                <div key={p.name}
                  title={`${label(p)} · ${p.up ? speed(p) : t("ports.free")} · ${device}`}
                  className={`flex min-w-[88px] flex-1 flex-col items-center gap-1 rounded-md border px-3 py-2.5 text-center
                    ${p.up ? "border-border bg-surface-2" : "border-dashed border-border bg-surface-2/50"}`}>
                  <span className="flex w-9 justify-between" aria-hidden="true">
                    <span className={`h-1.5 w-1.5 rounded-full ${led} ${p.up ? "animate-pulse-dot" : ""}`} />
                    <span className={`h-1.5 w-1.5 rounded-full ${led} ${p.up ? "animate-pulse-dot" : ""}`} />
                  </span>
                  <span className={`text-[11px] font-semibold font-mono tracking-wide ${p.wan ? "text-accent" : "text-muted"}`}>
                    {label(p)}
                  </span>
                  <span className={`w-full truncate text-[11px] ${p.up ? "text-text" : "text-faint"}`}>{device}</span>
                  <span className={`text-[10px] font-mono tabular-nums ${p.up ? "text-muted" : "text-faint"}`}>
                    {p.up && p.speed_mbps > 0 ? speed(p) : "—"}
                  </span>
                </div>
              );
            })}
          </div>
        </>
      )}
    </Card>
  );
}
