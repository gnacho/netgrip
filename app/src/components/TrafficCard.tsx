import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Activity } from "lucide-react";
import { api } from "../api";
import type { IfaceCounters } from "../types";
import { Card } from "./Card";

type Rates = Record<string, { rx: number; tx: number }>;

function fmtRate(bytesPerSec: number): string {
  if (bytesPerSec > 1048576) return `${(bytesPerSec / 1048576).toFixed(1)} MB/s`;
  if (bytesPerSec > 1024) return `${(bytesPerSec / 1024).toFixed(0)} KB/s`;
  return `${Math.round(bytesPerSec)} B/s`;
}

export function TrafficCard() {
  const { t } = useTranslation();
  const [rates, setRates] = useState<Rates>({});
  const prev = useRef<{ counters: IfaceCounters[]; ts: number } | undefined>(undefined);

  useEffect(() => {
    let cancelled = false;
    const poll = async () => {
      try {
        const next = await api.netdev();
        if (cancelled) return;
        const before = prev.current;
        if (before) {
          const dt = (next.ts - before.ts) / 1000;
          const nextRates: Rates = {};
          for (const c of next.counters) {
            const old = before.counters.find((o) => o.name === c.name);
            if (old && dt > 0) {
              nextRates[c.name] = {
                rx: Math.max(0, (c.rx_bytes - old.rx_bytes) / dt),
                tx: Math.max(0, (c.tx_bytes - old.tx_bytes) / dt),
              };
            }
          }
          setRates(nextRates);
        }
        prev.current = next;
      } catch { /* keep previous */ }
    };
    poll();
    const id = setInterval(poll, 3000);
    return () => { cancelled = true; clearInterval(id); };
  }, []);

  const names = Object.keys(rates);

  return (
    <Card title={t("traffic.title")} icon={Activity}>
      {names.length === 0 ? (
        <p className="text-sm text-muted">{t("traffic.measuring")}</p>
      ) : (
        names.map((name) => (
          <div key={name} className="flex justify-between gap-4 py-1 border-b border-border/50 last:border-0 text-sm">
            <span className="font-mono text-xs self-center">{name}</span>
            <span className="text-right text-xs">
              <span className="text-ok">↓ {fmtRate(rates[name].rx)}</span>
              {" · "}
              <span className="text-accent">↑ {fmtRate(rates[name].tx)}</span>
            </span>
          </div>
        ))
      )}
    </Card>
  );
}
