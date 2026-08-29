import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Gauge } from "lucide-react";
import { api } from "../../api";
import type { SQMProbe } from "../../types";
import {
  ActionBanner, Card, Field, Pill, SettingRow, SkeletonRows, Toggle,
} from "../ui";
import { useActionCycle } from "../wifi/action";
import { TechName } from "./shared";

/**
 * Prioridad de tráfico (SQM/CAKE) — "Que Internet no se atasque"
 * (services.md §7). La jerga vive solo en el caption y en el HelpTip.
 * Sin WAN la card queda atenuada ("Solo en modo router").
 */
export function SqmCard({ probe, onChange, index = 0 }: {
  probe: SQMProbe | undefined;
  onChange: (p: SQMProbe) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [doneMsg, setDoneMsg] = useState<string>();
  const [download, setDownload] = useState("");
  const [upload, setUpload] = useState("");

  useEffect(() => {
    if (probe) {
      setDownload(probe.download || "");
      setUpload(probe.upload || "");
    }
  }, [probe]);

  const active = probe?.active ?? false;
  const hasWan = probe?.has_wan ?? false;
  const ratesOk = Number(download) > 0 && Number(upload) > 0;

  const apply = async (enabled: boolean) => {
    setDoneMsg(undefined);
    const res = await run(() => api.setSqm(
      enabled ? { enabled, download, upload } : { enabled },
    ));
    if (res) {
      onChange(res.state);
      if (res.status === "applied") setDoneMsg(enabled ? t("sqm.doneOn") : t("sqm.doneOff"));
    } else {
      onChange(await api.sqm());
    }
  };

  return (
    <Card index={index} className={probe && !hasWan ? "opacity-70" : ""}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : (
        <>
          <SettingRow
            icon={Gauge}
            iconTone="accent"
            title={t("sqm.cardTitle")}
            description={t("sqm.desc")}
            help={t("help.sqm.body")}
            helpTitle={t("help.sqm.title")}
            checked={active}
            busy={busy}
            disabled={!hasWan || (!active && !ratesOk)}
            disabledReason={!hasWan ? t("sqm.onlyRouter") : undefined}
            onChange={apply}
            control={
              <span className="flex items-center gap-2">
                <Pill className="max-w-24 sm:max-w-32" tone={probe.running ? "ok" : active ? "warn" : "muted"}>
                  {probe.running ? t("sqm.running") : active ? t("sqm.configured") : t("sqm.off")}
                </Pill>
                <Toggle checked={active} busy={busy} disabled={!hasWan || (!active && !ratesOk)}
                  onChange={apply} label={t("sqm.cardTitle")} />
              </span>
            }
          />
          <TechName>{t("sqm.tech")}</TechName>

          {phase && (
            <div className="mt-2">
              <ActionBanner phase={phase} text={phase === "done" ? doneMsg : undefined} detail={detail} onDone={clear} />
            </div>
          )}

          {hasWan && (
            // Los campos de tasas también se muestran apagado: hacen falta
            // para poder activar (el toggle exige tasas válidas).
            <div className="pt-2 flex flex-col gap-3">
              <div className="grid grid-cols-2 gap-3">
                <Field label={t("sqm.download")} mono
                  inputProps={{ value: download, onChange: (e) => setDownload(e.target.value), inputMode: "numeric", disabled: active }} />
                <Field label={t("sqm.upload")} mono
                  inputProps={{ value: upload, onChange: (e) => setUpload(e.target.value), inputMode: "numeric", disabled: active }} />
              </div>
              <div>
                <p className="text-caption text-muted">{t("sqm.rateHelp")}</p>
                <p className="text-caption text-faint mt-0.5 font-mono">{t("sqm.rateExample")}</p>
              </div>
            </div>
          )}
        </>
      )}
    </Card>
  );
}
