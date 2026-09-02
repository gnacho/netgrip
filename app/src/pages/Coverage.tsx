import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Radio } from "lucide-react";
import type { UsteerAP } from "../types";
import { Card, Pill, SkeletonRows } from "../components/ui";
import { signalColor } from "../lib/format";

// Malla radial determinista: este router al centro, peers en un círculo,
// chips de radio en una órbita pequeña alrededor de cada nodo. Sin librería.
const VB_W = 900;
const VB_H = 430;
const CX = VB_W / 2;
const CY = VB_H / 2;
const PEER_R = 155;
const RADIO_ORBIT = 52;

interface MeshNode {
  hostname: string;
  local: boolean;
  aps: UsteerAP[];
}

function nodesOf(aps?: UsteerAP[]): MeshNode[] {
  const byHost = new Map<string, MeshNode>();
  for (const ap of aps ?? []) {
    const key = ap.hostname || ap.bssid;
    const node = byHost.get(key) ?? { hostname: key, local: false, aps: [] };
    node.local = node.local || ap.local;
    node.aps.push(ap);
    byHost.set(key, node);
  }
  return [...byHost.values()].sort((a, b) => (a.local === b.local ? 0 : a.local ? -1 : 1));
}

/** Cobertura inalámbrica (coverage.md): malla usteer con varios routers activos. */
export function CoveragePage({ aps, error }: { aps?: UsteerAP[]; error: boolean }) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<string>(); // bssid

  const nodes = useMemo<MeshNode[]>(() => nodesOf(aps), [aps]);
  const localNode = nodes.find((n) => n.local);
  const peers = nodes.filter((n) => !n.local);

  const posOf = (i: number) => {
    const angle = -Math.PI / 2 + (i * 2 * Math.PI) / Math.max(peers.length, 1);
    return { x: CX + PEER_R * Math.cos(angle), y: CY + PEER_R * Math.sin(angle) };
  };

  const selectedAp = (aps ?? []).find((a) => a.bssid === selected)
    || (localNode?.aps ?? []).slice().sort((a, b) => b.num_sta - a.num_sta)[0];

  const radioChip = (ap: UsteerAP, nx: number, ny: number, idx: number, total: number) => {
    const angle = Math.PI / 2 + ((idx - (total - 1) / 2) * Math.PI) / 3;
    const x = nx + RADIO_ORBIT * Math.cos(angle);
    const y = ny + RADIO_ORBIT * Math.sin(angle);
    const is5g = ap.freq > 5000;
    return (
      <g key={ap.bssid} transform={`translate(${x}, ${y})`}
        onClick={(e) => { e.stopPropagation(); setSelected(ap.bssid); }}
        className="cursor-pointer">
        <circle r="16"
          fill={is5g ? "var(--color-accent)" : "var(--color-warn)"} fillOpacity={0.2}
          stroke={is5g ? "var(--color-accent)" : "var(--color-warn)"}
          strokeWidth={selected === ap.bssid ? 2 : 1} />
        <text y="4" textAnchor="middle" fontSize="11" fill="var(--color-text)">{ap.num_sta}</text>
        <text y="30" textAnchor="middle" fontSize="9.5" fill="var(--color-muted)">
          {is5g ? "5G" : "2.4G"} ch{ap.channel}
        </text>
      </g>
    );
  };

  if (error) {
    return (
      <Card title={t("coverage.title")} icon={Radio} iconTone="teal">
        <p className="text-small text-muted">{t("coverage.absent")}</p>
      </Card>
    );
  }

  if (!aps) return <Card title={t("coverage.title")} icon={Radio} iconTone="teal"><SkeletonRows rows={4} /></Card>;

  return (
    <Card index={0} title={t("coverage.title")} icon={Radio} iconTone="teal">
      {aps.length === 0 || nodes.length <= 1 ? (
        <p className="text-small text-muted">{t("coverage.empty")}</p>
      ) : (
        <>
          <svg viewBox={`0 0 ${VB_W} ${VB_H}`} className="w-full" role="img" aria-label={t("coverage.title")}>
            {/* enlaces al router local */}
            {peers.map((peer, i) => {
              const p = posOf(i);
              return (
                <line key={`link-${peer.hostname}`} x1={CX} y1={CY} x2={p.x} y2={p.y}
                  stroke="var(--color-border-strong)" strokeWidth="1.5" />
              );
            })}
            {/* enlaces mesh entre peers */}
            {peers.map((_, i) => {
              const a = posOf(i);
              return peers.slice(i + 1).flatMap((_, j) => {
                const other = peers[i + 1 + j];
                const b = posOf(peers.indexOf(other));
                return (
                  <line key={`mesh-${i}-${j}`} x1={a.x} y1={a.y} x2={b.x} y2={b.y}
                    stroke="var(--color-border)" strokeWidth="1" strokeDasharray="3 4" />
                );
              });
            })}
            {localNode && (
              <g transform={`translate(${CX}, ${CY})`}>
                <circle r="32" fill="var(--color-accent)" fillOpacity={0.25} stroke="var(--color-accent)" strokeWidth="2" />
                <text y="5" textAnchor="middle" fontSize="13" fill="var(--color-text)" fontWeight={500}>{localNode.hostname}</text>
                <text y="50" textAnchor="middle" fontSize="10.5" fill="var(--color-accent)">{t("coverage.this")}</text>
                {localNode.aps.map((ap, i) => radioChip(ap, 0, 0, i, localNode.aps.length))}
              </g>
            )}
            {peers.map((peer, i) => {
              const p = posOf(i);
              return (
                <g key={peer.hostname} transform={`translate(${p.x}, ${p.y})`}>
                  <circle r="25" fill="var(--color-surface-2)" stroke="var(--color-muted)" strokeWidth="1.5" />
                  <text y="5" textAnchor="middle" fontSize="12" fill="var(--color-text)">{peer.hostname}</text>
                  {peer.aps.map((ap, j) => radioChip(ap, 0, 0, j, peer.aps.length))}
                </g>
              );
            })}
          </svg>

          {selectedAp && (
            <div className="mt-1">
              <div className="flex items-center gap-2 text-small text-muted mb-1">
                <Pill tone="muted">{selectedAp.hostname}</Pill>
                <span>{selectedAp.ssid} · {selectedAp.freq > 5000 ? t("wifi.band5") : t("wifi.band24")} ch{selectedAp.channel} · {t("coverage.util", { count: selectedAp.util })}</span>
                <span className="ml-auto">{t("coverage.stations", { count: selectedAp.num_sta })}</span>
              </div>
              {selectedAp.clients.length > 0 && (
                <div className="grid grid-cols-2 gap-x-4">
                  {[...selectedAp.clients]
                    .sort((a, b) => b.signal - a.signal)
                    .slice(0, 6)
                    .map((c) => (
                      <div key={c.mac} className="flex justify-between text-small text-muted py-0.5">
                        <span className="font-mono">{c.mac}</span>
                        <span className="font-medium" style={{ color: signalColor(c.signal) }}>{c.signal} dBm</span>
                      </div>
                    ))}
                </div>
              )}
            </div>
          )}
        </>
      )}
    </Card>
  );
}
