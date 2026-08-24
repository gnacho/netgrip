import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Cable, Lightbulb } from "lucide-react";
import type { EthPort } from "../types";
import { Card } from "./Card";

/**
 * Clon 1:1 del panel de bocas de NetPulse (PortPanel.tsx): misma
 * geometría (columna de 84px, LEDs 6px con pulso, cuerpo 48px border-2,
 * pines 2x10 inset 7/5, cavidad h-18 inset 6/5), separador vertical tras
 * WAN, chasis flex-wrap gap-x-4, leyenda. Tokens adaptados a owpanel.
 */

function Jack({ port, isWan, onClick, selected }: {
  port: EthPort;
  isWan: boolean;
  onClick: () => void;
  selected: boolean;
}) {
  const { t } = useTranslation();
  const up = port.up;

  const deviceText = !up
    ? t("ports.free")
    : port.devices.length === 1
      ? port.devices[0].name || port.devices[0].mac
      : port.devices.length > 1
        ? t("ports.unmanagedN", { count: port.devices.length })
        : t("ports.busy");

  return (
    <div className="group flex w-[84px] flex-col items-center cursor-pointer" onClick={onClick}>
      {/* LEDs link / act */}
      <div className="mb-1 flex w-9 items-center justify-between">
        <span className={`h-1.5 w-1.5 rounded-full ${up ? "bg-ok" : "bg-border/60"}`} />
        <span className={`h-1.5 w-1.5 rounded-full ${up ? "bg-ok" : "bg-border/60"} ${up ? "animate-pulse" : ""}`} />
      </div>

      {/* Cuerpo del conector */}
      <div
        className={`relative h-12 w-12 rounded-md border-2 transition-all duration-150 group-hover:-translate-y-0.5 ${
          up && isWan ? "border-accent/60 bg-bg shadow-[0_0_12px_0_rgba(37,99,235,0.35)]"
          : up ? "border-ok/50 bg-bg"
          : "border-border bg-bg/50"
        } ${selected ? "ring-2 ring-accent" : ""}`}
      >
        {/* Pines dorados */}
        <div className="absolute inset-x-[7px] top-[5px] flex justify-between">
          {Array.from({ length: 8 }).map((_, i) => (
            <span key={i} className={`h-2.5 w-[2px] rounded-full ${up ? "bg-warn/80" : "bg-border/60"}`} />
          ))}
        </div>
        {/* Apertura */}
        <div
          className={`absolute inset-x-[6px] bottom-[5px] h-[18px] rounded-[3px] border ${
            up ? "border-border/80 bg-black/50" : "border-border/50 bg-black/25"
          }`}
        />
      </div>

      {/* Etiquetas */}
      <div className="mt-1.5 w-full text-center">
        <div className={`font-mono text-[10px] font-semibold tracking-wide ${isWan ? "text-accent" : "text-muted"}`}>
          {isWan ? "WAN" : port.name.toUpperCase().replace(/^LAN(\d+)$/, "LAN $1")}
        </div>
        {up ? (
          <>
            <div className={`mt-0.5 w-full truncate text-xs font-medium ${port.devices.length > 1 ? "text-warn" : "text-text"}`}>
              {deviceText}
            </div>
            {port.speed_mbps > 0 && (
              <div className="font-mono text-[10px] text-muted">
                {port.speed_mbps >= 1000 ? `${(port.speed_mbps / 1000).toFixed(port.speed_mbps % 1000 === 0 ? 0 : 1)} Gbps` : `${port.speed_mbps} Mbps`}
              </div>
            )}
          </>
        ) : (
          <div className="mt-0.5 text-xs text-muted">{t("ports.free")}</div>
        )}
      </div>
    </div>
  );
}

export function EthPortsCard({ ports }: { ports: EthPort[] | undefined }) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<string>();

  if (!ports || ports.length === 0) {
    return (
      <Card title={t("ports.title")} icon={Cable}>
        <p className="text-sm text-muted">{t("ports.empty")}</p>
      </Card>
    );
  }

  const sorted = [...ports].sort((a, b) => {
    if (a.name === "wan") return -1;
    if (b.name === "wan") return 1;
    return a.name.localeCompare(b.name, undefined, { numeric: true });
  });
  const wanPorts = sorted.filter((p) => p.wan);
  const lanPorts = sorted.filter((p) => !p.wan);
  const inUse = sorted.filter((p) => p.up).length;
  const selectedPort = sorted.find((p) => p.name === selected);

  return (
    <Card title={t("ports.title")} icon={Cable}>
      <p className="text-xs text-muted mt-0.5">{t("ports.inUse", { used: inUse, total: sorted.length })}</p>

      {/* Chasis */}
      <div className="mt-3 flex flex-wrap items-start gap-x-4 gap-y-5 rounded-xl border border-border/70 bg-card/40 px-4 py-4">
        {wanPorts.map((p) => (
          <Jack key={p.name} port={p} isWan
            selected={selected === p.name}
            onClick={() => setSelected(selected === p.name ? undefined : p.name)} />
        ))}
        {wanPorts.length > 0 && lanPorts.length > 0 && (
          <div className="mx-1 hidden h-[104px] w-px self-center bg-border/70 sm:block" aria-hidden="true" />
        )}
        {lanPorts.map((p) => (
          <Jack key={p.name} port={p} isWan={false}
            selected={selected === p.name}
            onClick={() => setSelected(selected === p.name ? undefined : p.name)} />
        ))}
      </div>

      {/* Leyenda */}
      <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted">
        <span className="inline-flex items-center gap-1.5">
          <span className="h-1.5 w-1.5 rounded-full bg-ok" /> {t("ports.busy")}
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span className="h-1.5 w-1.5 rounded-full bg-border/60" /> {t("ports.free")}
        </span>
        {wanPorts.some((p) => p.up) && (
          <span className="inline-flex items-center gap-1.5">
            <span className="font-mono text-[10px] font-semibold text-accent">WAN</span> {t("ports.wanInternet")}
          </span>
        )}
      </div>

      {selectedPort && selectedPort.devices.length === 1 && (
        <p className="text-xs text-muted font-mono mt-2">{selectedPort.devices[0].mac}</p>
      )}
      {selectedPort && selectedPort.devices.length > 1 && (
        <div className="mt-2 text-xs">
          <p className="text-warn mb-1">{t("ports.switchDetected", { count: selectedPort.devices.length })}</p>
          <div className="flex gap-1.5 text-muted">
            <Lightbulb size={14} className="shrink-0 mt-0.5" />
            <ul className="list-disc list-inside space-y-0.5">
              <li>{t("ports.hintDirect")}</li>
              <li>{t("ports.hintManaged")}</li>
            </ul>
          </div>
        </div>
      )}
    </Card>
  );
}
