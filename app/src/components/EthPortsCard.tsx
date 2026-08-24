import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Cable, Lightbulb } from "lucide-react";
import type { EthPort } from "../types";
import { Card } from "./Card";

// NetPulse-style chassis: RJ45 jacks with two status LEDs on top, golden
// pins when linked, and the connected device name below each one.
const JACK_W = 44;
const JACK_H = 34;
const JACK_GAP = 22;
const PAD = 16;

function portLabel(name: string): string {
  if (name === "wan") return "WAN";
  const m = name.match(/^(?:lan|eth|swp)?(\d+)$/);
  return m ? `LAN ${m[1]}` : name.toUpperCase();
}

function fmtSpeed(mbps: number): string {
  if (mbps >= 1000) return `${(mbps / 1000).toFixed(mbps % 1000 === 0 ? 0 : 1)} Gbps`;
  return `${mbps} Mbps`;
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

  const inUse = sorted.filter((p) => p.up).length;
  const width = PAD * 2 + sorted.length * JACK_W + (sorted.length - 1) * JACK_GAP;
  const height = 118;
  const selectedPort = sorted.find((p) => p.name === selected);

  const deviceLine = (p: EthPort): { text: string; cls: string } => {
    if (!p.up) return { text: t("ports.free"), cls: "fill-muted" };
    if (p.devices.length === 1) {
      return { text: p.devices[0].name || p.devices[0].mac, cls: "fill-text" };
    }
    if (p.devices.length > 1) {
      return { text: t("ports.unmanagedN", { count: p.devices.length }), cls: "fill-warn" };
    }
    return { text: t("ports.busy"), cls: "fill-text" };
  };

  return (
    <Card title={t("ports.title")} icon={Cable}>
      <p className="text-xs text-muted mb-3">{t("ports.inUse", { used: inUse, total: sorted.length })}</p>
      <svg viewBox={`0 0 ${width} ${height}`} className="w-full max-w-xl">
        <rect x="1" y="1" width={width - 2} height={height - 2} rx="10"
          className="fill-bg stroke-border" />
        {sorted.map((p, i) => {
          const x = PAD + i * (JACK_W + JACK_GAP);
          const y = 16;
          const dev = deviceLine(p);
          const isWan = p.name === "wan";
          return (
            <g key={p.name} transform={`translate(${x}, ${y})`}
              onClick={() => setSelected(selected === p.name ? undefined : p.name)}
              className="cursor-pointer">
              {/* status LEDs */}
              <circle cx={JACK_W / 2 - 7} cy={0} r="3" className={p.up ? "fill-ok" : "fill-border"} />
              <circle cx={JACK_W / 2 + 7} cy={0} r="3" className={p.up ? "fill-ok" : "fill-border"} />
              {/* jack */}
              <rect y={8} width={JACK_W} height={JACK_H} rx="5"
                className={`fill-card ${p.up ? (isWan ? "stroke-[#2dd4bf]" : "stroke-ok") : "stroke-border"}`}
                strokeWidth={selected === p.name ? 2.5 : 1.5} />
              {Array.from({ length: 8 }).map((_, pin) => (
                <line key={pin} x1={6 + pin * 4.6} y1={13} x2={6 + pin * 4.6} y2={25}
                  stroke={p.up ? "#eab308" : "#4b5b6e"} strokeWidth="2" />
              ))}
              <rect x="6" y={27} width={JACK_W - 12} height={JACK_H - 27 - 5} rx="2"
                className="fill-bg" />
              {/* labels */}
              <text x={JACK_W / 2} y={JACK_H + 22} textAnchor="middle"
                className={`${isWan ? "fill-[#2dd4bf]" : "fill-muted"}`}
                fontSize="10" fontWeight={isWan ? "bold" : "normal"}>
                {portLabel(p.name)}
              </text>
              <text x={JACK_W / 2} y={JACK_H + 36} textAnchor="middle"
                className={dev.cls} fontSize="9.5">{dev.text}</text>
              {p.up && p.speed_mbps > 0 && (
                <text x={JACK_W / 2} y={JACK_H + 49} textAnchor="middle"
                  className="fill-muted" fontSize="8.5">{fmtSpeed(p.speed_mbps)}</text>
              )}
            </g>
          );
        })}
      </svg>

      {/* legend */}
      <div className="flex items-center gap-4 mt-1 text-xs text-muted">
        <span className="flex items-center gap-1.5">
          <span className="w-2 h-2 rounded-full bg-ok inline-block" /> {t("ports.busy")}
        </span>
        <span className="flex items-center gap-1.5">
          <span className="w-2 h-2 rounded-full bg-border inline-block" /> {t("ports.free")}
        </span>
        <span className="flex items-center gap-1.5">
          <span className="w-2 h-2 rounded-full bg-[#2dd4bf] inline-block" /> {t("ports.wanInternet")}
        </span>
      </div>

      {selectedPort && selectedPort.devices.length === 1 && (
        <p className="text-xs text-muted font-mono mt-1">{selectedPort.devices[0].mac}</p>
      )}
      {selectedPort && selectedPort.devices.length > 1 && (
        <div className="mt-1 text-xs">
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
