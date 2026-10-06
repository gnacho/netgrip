import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Radar } from "lucide-react";
import { api } from "../../api";
import type { WifiChanRadio } from "../../types";
import { Button, Card, ConfirmDialog, Pill } from "../ui";

/** Card "Canal recomendado" (#446): encuesta RF por radio (airtime del canal
 *  actual + BSS vecinos de un scan en background) y una sugerencia
 *  determinista de canal/ancho. Aplicar nunca es automático: solo cuando el
 *  usuario confirma, reutilizando el mismo path que el editor de radio. */
export function WifiChanCard({ index = 4 }: { index?: number }) {
  const { t } = useTranslation();
  const [radios, setRadios] = useState<WifiChanRadio[]>();
  const [pending, setPending] = useState<WifiChanRadio>();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  const load = useCallback(() => {
    api.wifiChan().then((r) => setRadios(r.radios ?? [])).catch(() => setRadios([]));
  }, []);

  useEffect(() => { load(); }, [load]);

  if (radios === undefined) return null;
  if (radios.length === 0) return null;

  const apply = async (rec: WifiChanRadio) => {
    const sug = rec.suggestion;
    if (!sug) return;
    setBusy(true);
    setMsg(undefined);
    try {
      await api.setWifiChan({ radio: rec.radio, channel: sug.channel, width: sug.width });
      setMsg({ tone: "ok", text: t("wifi.chanApplied") });
      setPending(undefined);
      load();
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
      setPending(undefined);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card index={index} title={t("wifi.chanCardTitle")} help="wifiChan" icon={Radar} iconTone="accent">
      <div className="divide-y divide-border/60">
        {radios.map((rec) => {
          const sug = rec.suggestion;
          const bandLabel = t(rec.band === "5g" ? "wifi.band5" : "wifi.band24");
          const sameChannel = !sug || sug.channel === rec.channel;
          return (
            <div key={rec.radio} className="flex flex-wrap items-center gap-x-4 gap-y-2 py-3">
              <span className="text-small font-medium w-20 shrink-0">{bandLabel}</span>
              <span className="text-small text-muted">
                {t("wifi.chanCurrent", { channel: rec.channel, width: rec.width })}
                {" · "}
                {t("wifi.chanUsage", { pct: Math.round(rec.utilization) })}
                {rec.neighbors.length > 0 && <>{" · "}{t("wifi.chanNeighbors", { count: rec.neighbors.length })}</>}
              </span>
              {sug && (
                <span className="text-small min-w-0 flex-1">
                  {sameChannel ? (
                    <span className="text-ok">{t("wifi.chanReason.current_ok")}</span>
                  ) : (
                    <>
                      <span className="text-text font-medium">
                        {t("wifi.chanSuggest", { channel: sug.channel, width: sug.width })}
                      </span>
                      <span className="text-muted">{" - "}{t(`wifi.chanReason.${sug.reason}`)}</span>
                    </>
                  )}
                </span>
              )}
              <span className="flex items-center gap-2 shrink-0 ml-auto">
                {sug && sug.confidence !== "high" && (
                  <Pill tone={sug.confidence === "low" ? "warn" : "muted"}>
                    {t(`wifi.chanConfidence.${sug.confidence}`)}
                  </Pill>
                )}
                {!rec.scan_complete && (
                  <Pill tone="warn">{t("wifi.chanPartial")}</Pill>
                )}
                {sug && !sameChannel && (
                  <Button variant="secondary" size="sm" onClick={() => setPending(rec)}>
                    {t("wifi.chanApply")}
                  </Button>
                )}
              </span>
            </div>
          );
        })}
      </div>
      {msg && <p className={`text-caption mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
      <ConfirmDialog
        open={!!pending}
        onClose={() => setPending(undefined)}
        onConfirm={() => pending && apply(pending)}
        busy={busy}
        title={pending?.suggestion ? t("wifi.chanConfirmTitle", { channel: pending.suggestion.channel }) : ""}
        consequence={t("wifi.chanConfirmConsequence")}
        confirmLabel={t("wifi.chanApply")}
      />
    </Card>
  );
}
