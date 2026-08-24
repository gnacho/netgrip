import { useTranslation } from "react-i18next";
import { Cable } from "lucide-react";
import type { EthPort } from "../types";
import { Card, Pill } from "./Card";

export function EthPortsCard({ ports }: { ports: EthPort[] | undefined }) {
  const { t } = useTranslation();
  return (
    <Card title={t("ports.title")} icon={Cable}>
      {!ports || ports.length === 0 ? (
        <p className="text-sm text-muted">{t("ports.empty")}</p>
      ) : (
        ports.map((p) => (
          <div key={p.name} className="py-1.5 border-b border-border/50 last:border-0">
            <div className="flex items-center gap-2 text-sm">
              <span className="font-mono">{p.name}</span>
              <Pill tone={p.up ? "ok" : "muted"}>{p.up ? t("ports.up") : t("ports.down")}</Pill>
              {p.up && p.speed_mbps > 0 && <span className="text-xs text-muted">{p.speed_mbps} Mbps</span>}
              {p.macs.length > 0 && (
                <span className="text-xs text-muted ml-auto">{t("ports.macs", { count: p.macs.length })}</span>
              )}
            </div>
            {p.macs.length > 0 && (
              <div className="mt-1 ml-2 text-xs text-muted font-mono">{p.macs.join(" · ")}</div>
            )}
          </div>
        ))
      )}
    </Card>
  );
}
