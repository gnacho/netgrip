import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Cable, Lightbulb } from "lucide-react";
import type { EthPort } from "../types";
import { Card } from "./Card";

// Deterministic router-chassis render (NetPulse style): jacks in a row,
// wan first. Per jack: the single connected device (name + MAC), or an
// unmanaged-switch hint when several MACs are learned on the port.
const JACK_W = 40;
const JACK_H = 22;
const JACK_GAP = 14;
const PAD = 14;

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

  const width = PAD * 2 + sorted.length * JACK_W + (sorted.length - 1) * JACK_GAP;
  const height = 96;
  const selectedPort = sorted.find((p) => p.name === selected);

  const portState = (p: EthPort): "down" | "empty" | "single" | "switch" => {
    if (!p.up) return "down";
    if (p.devices.length === 0) return "empty";
    if (p.devices.length === 1) return "single";
    return "switch";
  };

  return (
    <Card title={t("ports.title")} icon={Cable}>
      <svg viewBox={`0 0 ${width} ${height}`} className="w-full max-w-lg">
        <rect x="1" y="1" width={width - 2} height={height - 2} rx="10"
          className="fill-bg stroke-border" />
        {sorted.map((p, i) => {
          const x = PAD + i * (JACK_W + JACK_GAP);
          const y = 20;
          const state = portState(p);
          const up = p.up;
          return (
            <g key={p.name} transform={`translate(${x}, ${y})`}
              onClick={() => setSelected(selected === p.name ? undefined : p.name)}
              className="cursor-pointer">
              <rect width={JACK_W} height={JACK_H} rx="3"
                className={
                  state === "down" ? "fill-border/40 stroke-border"
                  : state === "switch" ? "fill-warn/25 stroke-warn"
                  : "fill-ok/25 stroke-ok"
                }
                strokeWidth={selected === p.name ? 2.5 : 1.5} />
              {Array.from({ length: 4 }).map((_, pin) => (
                <line key={pin} x1={7 + pin * 8} y1={4} x2={7 + pin * 8} y2={JACK_H - 4}
                  className={up ? "stroke-ok/70" : "stroke-muted/40"} strokeWidth="1.5" />
              ))}
              {state === "switch" && (
                <>
                  <circle cx={JACK_W - 3} cy={3} r="8" className="fill-warn" />
                  <text x={JACK_W - 3} y={6.5} textAnchor="middle" fill="#0b1118" fontSize="8.5" fontWeight="bold">
                    {p.devices.length}
                  </text>
                </>
              )}
              <text x={JACK_W / 2} y={JACK_H + 14} textAnchor="middle"
                className="fill-text" fontSize="10" fontFamily="monospace">{p.name}</text>
              <text x={JACK_W / 2} y={JACK_H + 27} textAnchor="middle"
                className="fill-muted" fontSize="8">
                {state === "down" ? t("ports.down")
                  : state === "switch" ? t("ports.unmanaged")
                  : up && p.speed_mbps > 0 ? `${p.speed_mbps}M`
                  : t("ports.up")}
              </text>
            </g>
          );
        })}
      </svg>

      {selectedPort && portState(selectedPort) === "single" && (
        <div className="mt-1 text-sm">
          <span className="text-text">{selectedPort.devices[0].name || t("ports.unknownDevice")}</span>
          <span className="text-xs text-muted font-mono ml-2">{selectedPort.devices[0].mac}</span>
        </div>
      )}
      {selectedPort && portState(selectedPort) === "switch" && (
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
      {selectedPort && portState(selectedPort) === "empty" && (
        <p className="text-xs text-muted mt-1">{t("ports.noMacs")}</p>
      )}
    </Card>
  );
}
