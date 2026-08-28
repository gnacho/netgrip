import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Activity, AlertTriangle } from "lucide-react";
import { api } from "../api";
import type { PortStatsProbe } from "../types";
import { Card } from "./Card";

type Rates = Record<string, { rx: number; tx: number }>;

function fmtRate(bps: number): string {
  if (bps > 1048576) return `${(bps / 1048576).toFixed(1)} MB/s`;
  if (bps > 1024) return `${(bps / 1024).toFixed(1)} KB/s`;
  if (bps > 0) return `${Math.round(bps)} B/s`;
  return "—";
}

export function PortStatsCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<PortStatsProbe>();
  const [rates, setRates] = useState<Rates>({});
  const prev = useRef<PortStatsProbe | undefined>(undefined);

  useEffect(() => {
    const load = async () => {
      try {
        const next = await api.portStats();
        const before = prev.current;
        if (before) {
          const dt = (next.ts - before.ts) / 1000;
          const nextRates: Rates = {};
          for (const p of next.ports) {
            const old = before.ports.find((o) => o.name === p.name);
            if (old && dt > 0) {
              nextRates[p.name] = {
                rx: Math.max(0, (p.rx_bytes - old.rx_bytes) / dt),
                tx: Math.max(0, (p.tx_bytes - old.tx_bytes) / dt),
              };
            }
          }
          setRates(nextRates);
        }
        prev.current = next;
        setProbe(next);
      } catch {
        // ignore
      }
    };
    load();
    const interval = setInterval(load, 3000);
    return () => clearInterval(interval);
  }, []);

  if (!probe || probe.ports.length === 0) return null;

  const hasErrors = probe.ports.some((p) => p.rx_errors > 0 || p.tx_errors > 0 || p.rx_drops > 0 || p.tx_drops > 0);

  return (
    <Card title={t("portStats.title")} icon={Activity}>
      <p className="text-xs text-muted mb-3">{t("portStats.intro")}</p>

      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr className="text-xs text-muted border-b border-border">
              <th className="text-left font-medium py-1.5 px-1">{t("portStats.port")}</th>
              <th className="text-right font-medium py-1.5 px-1">RX</th>
              <th className="text-right font-medium py-1.5 px-1">TX</th>
              <th className="text-right font-medium py-1.5 px-1">{t("portStats.errors")}</th>
              <th className="text-right font-medium py-1.5 px-1">{t("portStats.drops")}</th>
            </tr>
          </thead>
          <tbody>
            {probe.ports.map((p) => {
              const r = rates[p.name];
              const errs = p.rx_errors + p.tx_errors;
              const drops = p.rx_drops + p.tx_drops;
              return (
                <tr key={p.name} className="border-b border-border/40 last:border-0">
                  <td className="py-1.5 px-1 font-mono text-xs">{p.name}</td>
                  <td className="py-1.5 px-1 text-right text-xs whitespace-nowrap">
                    <span className="text-ok">↓</span> {r ? fmtRate(r.rx) : "—"}
                  </td>
                  <td className="py-1.5 px-1 text-right text-xs whitespace-nowrap">
                    <span className="text-accent">↑</span> {r ? fmtRate(r.tx) : "—"}
                  </td>
                  <td className="py-1.5 px-1 text-right text-xs">
                    {errs > 0 ? (
                      <span className="text-danger flex items-center justify-end gap-1">
                        <AlertTriangle size={10} /> {errs}
                      </span>
                    ) : (
                      <span className="text-muted">0</span>
                    )}
                  </td>
                  <td className="py-1.5 px-1 text-right text-xs">
                    {drops > 0 ? (
                      <span className="text-warn flex items-center justify-end gap-1">
                        <AlertTriangle size={10} /> {drops}
                      </span>
                    ) : (
                      <span className="text-muted">0</span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {hasErrors && <p className="text-xs text-warn mt-2">{t("portStats.hasErrors")}</p>}
    </Card>
  );
}
