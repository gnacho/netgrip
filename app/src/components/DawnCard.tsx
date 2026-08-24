import { useTranslation } from "react-i18next";
import { Radio } from "lucide-react";
import type { DawnAP } from "../types";
import { Card, Pill } from "./Card";

export function DawnCard({ aps, error }: { aps: DawnAP[] | undefined; error: boolean }) {
  const { t } = useTranslation();

  const sorted = [...(aps || [])].sort((a, b) => {
    if (a.local !== b.local) return a.local ? -1 : 1;
    return a.hostname.localeCompare(b.hostname) || a.freq - b.freq;
  });

  return (
    <Card title={t("dawn.title")} icon={Radio}>
      {error ? (
        <p className="text-sm text-muted">{t("dawn.absent")}</p>
      ) : !aps || aps.length === 0 ? (
        <p className="text-sm text-muted">{t("dawn.empty")}</p>
      ) : (
        sorted.map((ap) => (
          <div key={ap.bssid} className="py-1.5 border-b border-border/50 last:border-0">
            <div className="flex items-center gap-2 text-sm flex-wrap">
              <span className="font-medium">{ap.hostname || ap.bssid}</span>
              {ap.local && <Pill tone="ok">{t("dawn.thisRouter")}</Pill>}
              <span className="text-xs text-muted">
                {ap.ssid} · {ap.freq > 5000 ? t("wifi.band5") : t("wifi.band24")} ch{ap.channel} · {t("dawn.util", { count: ap.util })}
              </span>
              <span className="text-xs text-muted ml-auto">{t("wifi.clients", { count: ap.num_sta })}</span>
            </div>
            {ap.clients.length > 0 && (
              <div className="mt-1 ml-2">
                {[...ap.clients]
                  .sort((a, b) => b.signal - a.signal)
                  .slice(0, 5)
                  .map((c) => (
                    <div key={c.mac} className="flex justify-between text-xs text-muted py-0.5">
                      <span className="font-mono">{c.mac}</span>
                      <span>{c.signal} dBm</span>
                    </div>
                  ))}
              </div>
            )}
          </div>
        ))
      )}
    </Card>
  );
}
