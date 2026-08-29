import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChartColumn } from "lucide-react";
import { api } from "../../api";
import type { NlbwmonProbe } from "../../types";
import {
  ActionBanner, AdvancedDisclosure, Button, Card, Field, Input, KeyValue, Pill,
  SettingRow, SkeletonRows, Toggle,
} from "../ui";
import { useActionCycle } from "../wifi/action";
import { TechName } from "./shared";

/**
 * Registro de consumo (nlbwmon) — "Saber qué gasta cada equipo"
 * (services.md §8). Los parámetros de retención viven bajo Opciones
 * avanzadas. Si el paquete no está instalado la card no se muestra
 * (comportamiento actual de la página).
 */
export function NlbwmonCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<NlbwmonProbe>();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [doneMsg, setDoneMsg] = useState<string>();
  const [generations, setGenerations] = useState(30);

  useEffect(() => {
    api.nlbwmon().then((p) => {
      setProbe(p);
      if (p.generations > 0) setGenerations(p.generations);
    }).catch(() => {});
  }, []);

  if (probe && !probe.installed) return null;

  const toggle = async (enabled: boolean) => {
    setDoneMsg(undefined);
    const res = await run(() => api.setNlbwmon({ enabled }));
    if (res) {
      setProbe(res.state);
      if (res.status === "applied") setDoneMsg(enabled ? t("nlbwmon.doneOn") : t("nlbwmon.doneOff"));
    }
  };

  const save = async () => {
    setDoneMsg(undefined);
    const res = await run(() => api.setNlbwmon({ generations }));
    if (res) {
      setProbe(res.state);
      if (res.status === "applied") setDoneMsg(t("nlbwmon.saved"));
    }
  };

  return (
    <Card index={index}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : (
        <>
          <SettingRow
            icon={ChartColumn}
            iconTone="muted"
            title={t("nlbwmon.cardTitle")}
            description={t("nlbwmon.desc")}
            checked={probe.running}
            busy={busy}
            onChange={toggle}
            control={
              <span className="flex items-center gap-2">
                <Pill className="max-w-24 sm:max-w-32" tone={probe.running ? "ok" : "muted"}>
                  {probe.running ? t("nlbwmon.running") : t("nlbwmon.stopped")}
                </Pill>
                <Toggle checked={probe.running} busy={busy} onChange={toggle} label={t("nlbwmon.cardTitle")} />
              </span>
            }
          />
          <TechName>nlbwmon</TechName>

          {phase && (
            <div className="mt-2">
              <ActionBanner phase={phase} text={phase === "done" ? doneMsg : undefined} detail={detail} onDone={clear} />
            </div>
          )}

          <AdvancedDisclosure className="mt-2">
            <div className="flex flex-col gap-3">
              <Field label={t("nlbwmon.generations")} hint={t("nlbwmon.generationsHint")}
                help={t("help.nlbwmon.body")} helpTitle={t("help.nlbwmon.title")}>
                <Input
                  type="number" mono value={generations} min={1} max={365}
                  onChange={(e) => setGenerations(Number(e.target.value))}
                  className="w-24"
                />
              </Field>
              <KeyValue items={[
                { label: t("nlbwmon.interval"), value: `${probe.commit_interval} s`, mono: true },
                { label: t("nlbwmon.prealloc"), value: t("nlbwmon.preallocDays", { count: probe.prealloc_days }), mono: true },
              ]} />
              <div className="flex justify-end">
                <Button size="sm" loading={busy} disabled={generations === probe.generations} onClick={save}>
                  {t("nlbwmon.save")}
                </Button>
              </div>
            </div>
          </AdvancedDisclosure>
        </>
      )}
    </Card>
  );
}
