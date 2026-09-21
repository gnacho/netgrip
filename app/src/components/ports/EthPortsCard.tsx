import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Cable } from "lucide-react";
import type { EthPort } from "../../types";
import { Card, EmptyState, SkeletonRows } from "../ui";
import { IlluPlug } from "../ui/illustrations";

/** Título de tarjeta a una línea (design-rev2 §3): ellipsis + tooltip nativo. */
function oneLine(text: string) {
  return <span className="block truncate" title={text}>{text}</span>;
}

/**
 * Chasis RJ45 de la página Puertos ethernet (#384): sale del resumen,
 * donde ocupaba una fila entera para un vistazo que encaja mejor junto al
 * resto del detalle de puertos.
 */
export function EthPortsCard({ ports, index = 0 }: { ports?: EthPort[]; index?: number }) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<string>();

  const sorted = useMemo(() => !ports ? [] : [...ports].sort((a, b) => {
    if (a.wan !== b.wan) return a.wan ? -1 : 1;
    return a.name.localeCompare(b.name, undefined, { numeric: true });
  }), [ports]);

  const selectedPort = sorted.find((p) => p.name === selected);

  return (
    <Card index={index} className="md:col-span-2"
      title={oneLine(t("overview.ports"))} icon={Cable} iconTone="teal">
      {!ports ? <SkeletonRows rows={3} /> : sorted.length === 0 ? (
        <EmptyState small illustration={<IlluPlug size={120} />} title={t("overview.portsEmpty")} />
      ) : (
        <>
          <p className="text-caption text-muted mb-3">
            {t("ports.inUse", { used: sorted.filter((p) => p.up).length, total: sorted.length })}
          </p>
          {/* chasis RJ45 redibujado: boca + LED + etiqueta + dispositivo */}
          <div className="flex flex-wrap gap-x-3 gap-y-4 rounded-md border border-border bg-surface-2 px-3 py-3">
            {sorted.map((p) => {
              const label = p.wan ? "WAN" : p.name.toUpperCase().replace(/^LAN(\d+)$/, "LAN $1");
              const device = !p.up ? t("ports.free")
                : p.devices.length === 1 ? p.devices[0].name || p.devices[0].mac
                : p.devices.length > 1 ? t("ports.unmanagedN", { count: p.devices.length })
                : t("ports.busy");
              const led = !p.up ? "bg-border-strong" : p.speed_mbps >= 1000 ? "bg-ok" : "bg-warn";
              return (
                <button key={p.name} type="button"
                  onClick={() => setSelected(selected === p.name ? undefined : p.name)}
                  title={p.up ? `${label} · ${p.speed_mbps >= 1000 ? `${p.speed_mbps / 1000} Gb/s` : `${p.speed_mbps} Mb/s`} · ${device}` : label}
                  className={`flex w-[64px] flex-col items-center gap-1 rounded-sm p-1 ring-focus transition-colors hover:bg-surface
                    ${selected === p.name ? "bg-surface shadow-card" : ""}`}>
                  <span className="flex w-8 justify-between" aria-hidden="true">
                    <span className={`h-1.5 w-1.5 rounded-full ${led}`} />
                    <span className={`h-1.5 w-1.5 rounded-full ${led} ${p.up ? "animate-pulse-dot" : ""}`} />
                  </span>
                  <span className={`relative h-10 w-10 rounded-sm border-2 ${p.up ? "border-border-strong bg-surface" : "border-border bg-surface-2"}`} aria-hidden="true">
                    <span className="absolute inset-x-[6px] top-[4px] flex justify-between">
                      {Array.from({ length: 6 }).map((_, i) => (
                        <span key={i} className={`h-2 w-[2px] rounded-full ${p.up ? "bg-warn/70" : "bg-border"}`} />
                      ))}
                    </span>
                    <span className="absolute inset-x-[5px] bottom-[4px] h-[14px] rounded-[3px] border border-border bg-bg" />
                  </span>
                  <span className={`text-[10px] font-semibold font-mono tracking-wide ${p.wan ? "text-accent" : "text-muted"}`}>{label}</span>
                  <span className="w-full truncate text-center text-[11px] text-text">{device}</span>
                </button>
              );
            })}
          </div>
          {selectedPort && (
            <div className="mt-3 flex items-center justify-between gap-2 text-small">
              <span className="text-muted font-mono text-caption">
                {selectedPort.devices[0]?.mac ?? ""}
                {selectedPort.up && selectedPort.speed_mbps > 0 && ` · ${selectedPort.speed_mbps >= 1000 ? `${selectedPort.speed_mbps / 1000} Gb/s` : `${selectedPort.speed_mbps} Mb/s`}`}
              </span>
            </div>
          )}
        </>
      )}
    </Card>
  );
}
