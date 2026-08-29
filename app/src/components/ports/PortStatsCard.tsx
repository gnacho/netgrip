import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Activity, TriangleAlert } from "lucide-react";
import { api } from "../../api";
import type { PortStatsProbe } from "../../types";
import { Card } from "../ui";
import { fmtRate } from "../../lib/format";

type Rates = Record<string, { rx: number; tx: number }>;

/** Estadísticas por boca (ports.md §6): rx/tx en vivo; errores y drops en rojo si > 0. */
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
        // se reintenta en el próximo poll
      }
    };
    load();
    const interval = setInterval(load, 3000);
    return () => clearInterval(interval);
  }, []);

  if (!probe || probe.ports.length === 0) return null;

  const hasErrors = probe.ports.some((p) => p.rx_errors > 0 || p.tx_errors > 0 || p.rx_drops > 0 || p.tx_drops > 0);
  // Máximos por dirección para las mini-barras de proporción (design-rev2 §5).
  const maxRx = Math.max(0, ...probe.ports.map((p) => rates[p.name]?.rx ?? 0));
  const maxTx = Math.max(0, ...probe.ports.map((p) => rates[p.name]?.tx ?? 0));

  /** Mini-barra de proporción (relleno accent RX / teal TX sobre pista accent-soft). */
  const ratioBar = (v: number, max: number, fill: string) => (
    <span aria-hidden="true" className="mt-1 block h-[5px] rounded-full" style={{ background: "var(--color-accent-soft)" }}>
      <span
        className="block h-full rounded-full transition-[width] duration-[var(--dur-fast)]"
        style={{ width: v > 0 ? `${Math.max(3, (v / max) * 100)}%` : "0%", background: fill }}
      />
    </span>
  );

  return (
    <Card variant="subtle" animate={false} icon={Activity} title={t("portStats.title")} help="portstats">
      <div className="overflow-x-auto">
        <table className="w-full border-collapse">
          <thead>
            <tr className="text-caption text-muted border-b border-border">
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
                  <td className="py-1.5 px-1 font-mono text-small">{p.name}</td>
                  <td className="py-1.5 px-1 text-right text-small font-mono whitespace-nowrap tabular-nums text-muted min-w-24">
                    <span className="text-chart-rx">↓</span> {r ? fmtRate(r.rx) : "—"}
                    {r && maxRx > 0 && ratioBar(r.rx, maxRx, "var(--color-accent)")}
                  </td>
                  <td className="py-1.5 px-1 text-right text-small font-mono whitespace-nowrap tabular-nums text-muted min-w-24">
                    <span className="text-chart-tx">↑</span> {r ? fmtRate(r.tx) : "—"}
                    {r && maxTx > 0 && ratioBar(r.tx, maxTx, "var(--color-teal)")}
                  </td>
                  <td className="py-1.5 px-1 text-right text-small font-mono tabular-nums">
                    {errs > 0 ? (
                      <span className="text-danger inline-flex items-center justify-end gap-1 font-semibold">
                        <TriangleAlert size={12} /> {errs}
                      </span>
                    ) : (
                      <span className="text-faint">0</span>
                    )}
                  </td>
                  <td className="py-1.5 px-1 text-right text-small font-mono tabular-nums">
                    {drops > 0 ? (
                      <span className="text-danger inline-flex items-center justify-end gap-1 font-semibold">
                        <TriangleAlert size={12} /> {drops}
                      </span>
                    ) : (
                      <span className="text-faint">0</span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {hasErrors && <p className="text-caption text-warn mt-2">{t("portStats.hasErrors")}</p>}
    </Card>
  );
}
