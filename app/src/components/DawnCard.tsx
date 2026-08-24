import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Radio } from "lucide-react";
import type { DawnAP } from "../types";
import { Card, Pill } from "./Card";

// Deterministic radial mesh: this router at the center, peers on a
// circle, radio chips on a small orbit around each node. No graph lib.
const VB_W = 900;
const VB_H = 430;
const CX = VB_W / 2;
const CY = VB_H / 2;
const PEER_R = 155;
const RADIO_ORBIT = 52;

type MeshNode = {
  hostname: string;
  local: boolean;
  aps: DawnAP[];
};

export function DawnCard({ aps, error }: { aps: DawnAP[] | undefined; error: boolean }) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<string>(); // bssid

  const nodes = useMemo<MeshNode[]>(() => {
    const byHost = new Map<string, MeshNode>();
    for (const ap of aps || []) {
      const key = ap.hostname || ap.bssid;
      const node = byHost.get(key) || { hostname: key, local: false, aps: [] };
      node.local = node.local || ap.local;
      node.aps.push(ap);
      byHost.set(key, node);
    }
    return [...byHost.values()].sort((a, b) => (a.local === b.local ? 0 : a.local ? -1 : 1));
  }, [aps]);

  const localNode = nodes.find((n) => n.local);
  const peers = nodes.filter((n) => !n.local);

  const posOf = (i: number) => {
    const angle = -Math.PI / 2 + (i * 2 * Math.PI) / Math.max(peers.length, 1);
    return { x: CX + PEER_R * Math.cos(angle), y: CY + PEER_R * Math.sin(angle) };
  };

  const selectedAp = (aps || []).find((a) => a.bssid === selected)
    || (localNode?.aps ?? []).slice().sort((a, b) => b.num_sta - a.num_sta)[0];

  const radioChip = (ap: DawnAP, nx: number, ny: number, idx: number, total: number) => {
    const angle = Math.PI / 2 + ((idx - (total - 1) / 2) * Math.PI) / 3;
    const x = nx + RADIO_ORBIT * Math.cos(angle);
    const y = ny + RADIO_ORBIT * Math.sin(angle);
    const is5g = ap.freq > 5000;
    return (
      <g key={ap.bssid} transform={`translate(${x}, ${y})`}
        onClick={(e) => { e.stopPropagation(); setSelected(ap.bssid); }}
        className="cursor-pointer">
        <circle r="16"
          className={`${is5g ? "fill-accent/20 stroke-accent" : "fill-warn/20 stroke-warn"} ${selected === ap.bssid ? "stroke-2" : "stroke-1"}`} />
        <text y="4" textAnchor="middle" fontSize="11" className="fill-text">
          {ap.num_sta}
        </text>
        <text y="30" textAnchor="middle" fontSize="9.5" className="fill-muted">
          {is5g ? "5G" : "2.4G"} ch{ap.channel}
        </text>
      </g>
    );
  };

  if (error) {
    return (
      <Card title={t("dawn.title")} icon={Radio}>
        <p className="text-sm text-muted">{t("dawn.absent")}</p>
      </Card>
    );
  }

  return (
    <Card title={t("dawn.title")} icon={Radio}>
      {!aps || aps.length === 0 ? (
        <p className="text-sm text-muted">{t("dawn.empty")}</p>
      ) : (
        <>
          <svg viewBox={`0 0 ${VB_W} ${VB_H}`} className="w-full">
            {peers.map((peer, i) => {
              const p = posOf(i);
              return (
                <line key={`link-${peer.hostname}`} x1={CX} y1={CY} x2={p.x} y2={p.y}
                  className="stroke-border" strokeWidth="1.5" />
              );
            })}
            {peers.map((_, i) => {
              const a = posOf(i);
              return peers.slice(i + 1).map((other, j) => {
                const b = posOf(peers.indexOf(other));
                return (
                  <line key={`mesh-${i}-${j}`} x1={a.x} y1={a.y} x2={b.x} y2={b.y}
                    className="stroke-border/40" strokeWidth="1" strokeDasharray="3 4" />
                );
              });
            })}
            {localNode && (
              <g transform={`translate(${CX}, ${CY})`}>
                <circle r="32" className="fill-accent/25 stroke-accent" strokeWidth="2" />
                <text y="5" textAnchor="middle" fontSize="13" className="fill-text font-medium">
                  {localNode.hostname}
                </text>
                <text y="50" textAnchor="middle" fontSize="10.5" className="fill-accent">
                  {t("dawn.thisRouter")}
                </text>
                {localNode.aps.map((ap, i) => radioChip(ap, 0, 0, i, localNode.aps.length))}
              </g>
            )}
            {peers.map((peer, i) => {
              const p = posOf(i);
              return (
                <g key={peer.hostname} transform={`translate(${p.x}, ${p.y})`}>
                  <circle r="25" className="fill-card stroke-muted" strokeWidth="1.5" />
                  <text y="5" textAnchor="middle" fontSize="12" className="fill-text">
                    {peer.hostname}
                  </text>
                  {peer.aps.map((ap, j) => radioChip(ap, 0, 0, j, peer.aps.length))}
                </g>
              );
            })}
          </svg>

          {selectedAp && (
            <div className="mt-1">
              <div className="flex items-center gap-2 text-xs text-muted mb-1">
                <Pill tone="muted">{selectedAp.hostname}</Pill>
                <span>{selectedAp.ssid} · {selectedAp.freq > 5000 ? t("wifi.band5") : t("wifi.band24")} ch{selectedAp.channel} · {t("dawn.util", { count: selectedAp.util })}</span>
                <span className="ml-auto">{t("wifi.clients", { count: selectedAp.num_sta })}</span>
              </div>
              {selectedAp.clients.length > 0 && (
                <div className="grid grid-cols-2 gap-x-4">
                  {[...selectedAp.clients]
                    .sort((a, b) => b.signal - a.signal)
                    .slice(0, 6)
                    .map((c) => (
                      <div key={c.mac} className="flex justify-between text-xs text-muted py-0.5">
                        <span className="font-mono">{c.mac}</span>
                        <span>{c.signal} dBm</span>
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
