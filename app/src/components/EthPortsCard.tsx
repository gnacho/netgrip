import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Cable, Lightbulb } from "lucide-react";
import type { EthPort } from "../types";
import { Card } from "./Card";

// NetPulse-style chassis, cloned from the reference: tall RJ45 jacks with
// two LEDs on top, a row of golden contacts and the dark connector
// cavity. Teal accent for WAN, green for linked LAN ports.
const JACK_W = 48;
const JACK_H = 62;
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
  const height = 148;
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
      <svg viewBox={`0 0 ${width} ${height}`} width={width} height={height}>
        <rect x="1" y="1" width={width - 2} height={height - 2} rx="10"
          className="fill-bg stroke-border" />
        {sorted.map((p, i) => {
          const x = PAD + i * (JACK_W + JACK_GAP);
          const y = 16;
          const dev = deviceLine(p);
          const isWan = p.name === "wan";
          const jackStroke = p.up ? (isWan ? "#2dd4bf" : "#34d399") : "#2c3e52";
          return (
            <g key={p.name} transform={`translate(${x}, ${y})`}
              onClick={() => setSelected(selected === p.name ? undefined : p.name)}
              className="cursor-pointer">
              {/* status LEDs above the jack */}
              <circle cx={JACK_W / 2 - 8} cy={0} r="3.5" fill={p.up ? "#34d399" : "#2c3e52"} />
              <circle cx={JACK_W / 2 + 8} cy={0} r="3.5" fill={p.up ? "#34d399" : "#2c3e52"} />
              {/* RJ45 jack */}
              <rect y={8} width={JACK_W} height={JACK_H} rx="6"
                fill="#0f1826" stroke={jackStroke}
                strokeWidth={selected === p.name ? 2.5 : 1.5} />
              {/* golden contacts row */}
              {Array.from({ length: 8 }).map((_, pin) => (
                <rect key={pin} x={6.5 + pin * 4.6} y={13} width="2.6" height="10" rx="0.5"
                  fill={p.up ? "#eab308" : "#4b5b6e"} />
              ))}
              {/* connector cavity with clip notch */}
              <rect x="7" y={27} width={JACK_W - 14} height={JACK_H - 27 - 6} rx="2.5"
                fill="#0b1118" stroke="#22303f" strokeWidth="1" />
              <rect x={JACK_W / 2 - 5} y={JACK_H - 11} width="10" height="5" rx="1"
                fill="#0f1826" stroke="#22303f" strokeWidth="1" />
              {/* labels */}
              <text x={JACK_W / 2} y={JACK_H + 22} textAnchor="middle"
                fill={isWan ? "#2dd4bf" : "#8ba3bb"}
                fontSize="11" fontWeight={isWan ? "bold" : "normal"}>
                {portLabel(p.name)}
              </text>
              <text x={JACK_W / 2} y={JACK_H + 38} textAnchor="middle"
                className={dev.cls} fontSize="10.5">{dev.text}</text>
              {p.up && p.speed_mbps > 0 && (
                <text x={JACK_W / 2} y={JACK_H + 53} textAnchor="middle"
                  className="fill-muted" fontSize="9.5">{fmtSpeed(p.speed_mbps)}</text>
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
