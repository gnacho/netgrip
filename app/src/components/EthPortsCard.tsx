import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Cable } from "lucide-react";
import type { EthPort } from "../types";
import { Card } from "./Card";

// Deterministic router-chassis render: jacks in a row, wan first.
// No graph library: positions are computed by index.
const JACK_W = 34;
const JACK_H = 22;
const JACK_GAP = 12;
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
  const height = 92;
  const selectedPort = sorted.find((p) => p.name === selected);

  return (
    <Card title={t("ports.title")} icon={Cable}>
      <svg viewBox={`0 0 ${width} ${height}`} className="w-full max-w-md">
        <rect x="1" y="1" width={width - 2} height={height - 2} rx="10"
          className="fill-bg stroke-border" />
        {sorted.map((p, i) => {
          const x = PAD + i * (JACK_W + JACK_GAP);
          const y = 22;
          const up = p.up;
          return (
            <g key={p.name} transform={`translate(${x}, ${y})`}
              onClick={() => setSelected(selected === p.name ? undefined : p.name)}
              className="cursor-pointer">
              <rect width={JACK_W} height={JACK_H} rx="3"
                className={up ? "fill-ok/25 stroke-ok" : "fill-border/40 stroke-border"}
                strokeWidth={selected === p.name ? 2.5 : 1.5} />
              {Array.from({ length: 4 }).map((_, pin) => (
                <line key={pin} x1={5 + pin * 7} y1={4} x2={5 + pin * 7} y2={JACK_H - 4}
                  className={up ? "stroke-ok/70" : "stroke-muted/40"} strokeWidth="1.5" />
              ))}
              <text x={JACK_W / 2} y={JACK_H + 14} textAnchor="middle"
                className="fill-text" fontSize="10" fontFamily="monospace">{p.name}</text>
              <text x={JACK_W / 2} y={JACK_H + 27} textAnchor="middle"
                className="fill-muted" fontSize="8.5">
                {up ? (p.speed_mbps > 0 ? `${p.speed_mbps}M` : t("ports.up")) : t("ports.down")}
              </text>
              {p.macs.length > 0 && (
                <>
                  <circle cx={JACK_W - 3} cy={3} r="7" className="fill-accent" />
                  <text x={JACK_W - 3} y={6.5} textAnchor="middle" fill="#fff" fontSize="8">
                    {p.macs.length}
                  </text>
                </>
              )}
            </g>
          );
        })}
      </svg>
      {selectedPort && selectedPort.macs.length > 0 && (
        <div className="mt-2">
          <p className="text-xs text-muted mb-1">{t("ports.learnedOn", { port: selectedPort.name })}</p>
          <div className="text-xs font-mono text-muted leading-5 break-words">
            {selectedPort.macs.join("  ")}
          </div>
        </div>
      )}
      {selectedPort && selectedPort.macs.length === 0 && (
        <p className="text-xs text-muted mt-2">{t("ports.noMacs")}</p>
      )}
    </Card>
  );
}
