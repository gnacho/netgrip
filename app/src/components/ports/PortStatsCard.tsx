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

  if (!probe || (probe.ports ?? []).length === 0) return null;
  const ports = probe.ports ?? [];

  const hasErrors = ports.some((p) => p.rx_errors > 0 || p.tx_errors > 0 || p.rx_drops > 0 || p.tx_drops > 0);
  // Máximos por dirección para las mini-barras de proporción (design-rev2 §5).
  const maxRx = Math.max(0, ...ports.map((p) => rates[p.name]?.rx ?? 0));
  const maxTx = Math.max(0, ...ports.map((p) => rates[p.name]?.tx ?? 0));

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
      {/* Leyenda de la rejilla densa (issue #485). */}
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-caption text-muted mb-2">
        <span><span className="text-chart-rx">↓</span> RX</span>
        <span><span className="text-chart-tx">↑</span> TX</span>
        <span className="inline-flex items-center gap-1">
          <TriangleAlert size={12} className="text-danger" />
          {t("portStats.errors")} / {t("portStats.drops")}
        </span>
      </div>

      {/* Rejilla densa: 2 columnas en movil, 3 en md, 4 en xl. */}
      <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-x-4 gap-y-2.5">
        {ports.map((p) => {
          const r = rates[p.name];
          const errs = p.rx_errors + p.tx_errors;
          const drops = p.rx_drops + p.tx_drops;
          const errSpan = (count: number, label: string) => (
            <span
              title={label}
              className={`inline-flex items-center gap-0.5 ${count > 0 ? "text-danger font-semibold" : "text-faint"}`}
            >
              {count > 0 && <TriangleAlert size={11} />}
              {count}
            </span>
          );
          return (
            <div key={p.name} className="min-w-0">
              <div className="flex items-baseline justify-between gap-2">
                <span className="font-mono text-small truncate">{p.name}</span>
                <span className="flex items-center gap-2 text-caption font-mono tabular-nums shrink-0">
                  {errSpan(errs, t("portStats.errors"))}
                  {errSpan(drops, t("portStats.drops"))}
                </span>
              </div>
              <div className="flex items-center gap-3 text-caption font-mono tabular-nums text-muted">
                <span className="inline-flex items-center gap-1 whitespace-nowrap">
                  <span className="text-chart-rx">↓</span> {r ? fmtRate(r.rx) : "-"}
                </span>
                <span className="inline-flex items-center gap-1 whitespace-nowrap">
                  <span className="text-chart-tx">↑</span> {r ? fmtRate(r.tx) : "-"}
                </span>
              </div>
              {r && maxRx > 0 && ratioBar(r.rx, maxRx, "var(--color-accent)")}
              {r && maxTx > 0 && ratioBar(r.tx, maxTx, "var(--color-teal)")}
            </div>
          );
        })}
      </div>

      {hasErrors && <p className="text-caption text-warn mt-2">{t("portStats.hasErrors")}</p>}
    </Card>
  );
}
