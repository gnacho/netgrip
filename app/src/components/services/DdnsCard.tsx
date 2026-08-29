import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { AtSign } from "lucide-react";
import { api } from "../../api";
import type { DDNSProbe } from "../../types";
import {
  ActionBanner, AdvancedDisclosure, Card, Field, KeyValue, Pill,
  SettingRow, SkeletonRows, Toggle,
} from "../ui";
import { useActionCycle } from "../wifi/action";
import { Reveal, TechName } from "./shared";

/** "actualizado hace 12 min"; si la fecha no parsea → undefined (mostrar "—"). */
function useRelativeUpdate(iso: string | undefined): { text: string; valid: boolean } {
  const { t } = useTranslation();
  if (!iso) return { text: "—", valid: false };
  const ms = Date.parse(iso);
  if (Number.isNaN(ms)) return { text: "—", valid: false };
  const mins = Math.max(0, Math.floor((Date.now() - ms) / 60000));
  if (mins < 60) return { text: t("ddns.agoMinutes", { count: mins }), valid: true };
  const hours = Math.floor(mins / 60);
  if (hours < 48) return { text: t("ddns.agoHours", { count: hours }), valid: true };
  return { text: t("ddns.agoDays", { count: Math.floor(hours / 24) }), valid: true };
}

/**
 * DDNS — "Un nombre fijo para tu casa" (services.md §5). Los campos solo se
 * editan con el servicio apagado (comportamiento real de la API actual: el
 * alta envía la config completa). La contraseña es write-only.
 */
export function DdnsCard({ probe, onChange, index = 0 }: {
  probe: DDNSProbe | undefined;
  onChange: (p: DDNSProbe) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [doneMsg, setDoneMsg] = useState<string>();
  const [serviceName, setServiceName] = useState("");
  const [domain, setDomain] = useState("");
  const [lookupHost, setLookupHost] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  useEffect(() => {
    if (probe) {
      setServiceName(probe.service_name || "");
      setDomain(probe.domain || "");
      setLookupHost(probe.lookup_host || "");
      setUsername(probe.username || "");
    }
  }, [probe]);

  const active = probe?.active ?? false;
  const lastUpdate = useRelativeUpdate(probe?.last_update);

  const apply = async (enabled: boolean) => {
    setDoneMsg(undefined);
    const res = await run(() => api.setDdns(
      enabled
        ? { enabled, service_name: serviceName, domain, lookup_host: lookupHost, username, password }
        : { enabled },
    ));
    if (res) {
      onChange(res.state);
      setPassword("");
      if (res.status === "applied") setDoneMsg(enabled ? t("ddns.doneOn") : t("ddns.doneOff"));
    } else {
      onChange(await api.ddns());
    }
  };

  const canEnable = serviceName.trim() !== "" && domain.trim() !== "";

  return (
    <Card index={index}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : (
        <>
          <SettingRow
            icon={AtSign}
            iconTone="accent"
            title={t("ddns.cardTitle")}
            description={t("ddns.desc")}
            help={t("help.ddns.body")}
            helpTitle={t("help.ddns.title")}
            checked={active}
            busy={busy}
            disabled={!active && !canEnable}
            onChange={apply}
            control={
              <span className="flex items-center gap-2">
                <Pill className="max-w-24 sm:max-w-32" tone={probe.running ? "ok" : active ? "warn" : "muted"}>
                  {probe.running ? t("ddns.running") : active ? t("ddns.configured") : t("ddns.off")}
                </Pill>
                <Toggle checked={active} busy={busy} disabled={!active && !canEnable}
                  onChange={apply} label={t("ddns.cardTitle")} />
              </span>
            }
          />
          <TechName>DDNS</TechName>

          {phase && (
            <div className="mt-2">
              <ActionBanner phase={phase} text={phase === "done" ? doneMsg : undefined} detail={detail} onDone={clear} />
            </div>
          )}

          <Reveal open={active}>
            <div className="pt-2">
              <KeyValue items={[
                { label: t("ddns.domainLabel"), value: probe.domain || "—", mono: true },
                ...(probe.registered_ip ? [{ label: t("ddns.registeredIp"), value: probe.registered_ip, mono: true }] : []),
                {
                  label: t("ddns.lastUpdate"),
                  value: lastUpdate.valid
                    ? lastUpdate.text
                    : <span title={probe.last_update}>—</span>,
                },
              ]} />
            </div>
          </Reveal>

          <AdvancedDisclosure label={t("ddns.edit")} className="mt-2">
            <div className="flex flex-col gap-3">
              <Field label={t("ddns.providerLabel")} mono
                inputProps={{ value: serviceName, onChange: (e) => setServiceName(e.target.value), placeholder: "duckdns.org", disabled: active }} />
              <Field label={t("ddns.domainLabel")} mono
                inputProps={{ value: domain, onChange: (e) => setDomain(e.target.value), placeholder: "casa.duckdns.org", disabled: active }} />
              <Field label={t("ddns.lookupHostLabel")} mono
                inputProps={{ value: lookupHost, onChange: (e) => setLookupHost(e.target.value), disabled: active }} />
              <Field label={t("ddns.username")}
                inputProps={{ value: username, onChange: (e) => setUsername(e.target.value), autoComplete: "off", disabled: active }} />
              <Field label={t("ddns.password")} hint={probe.username ? t("ddns.passwordKeep") : undefined}
                inputProps={{ type: "password", value: password, onChange: (e) => setPassword(e.target.value), autoComplete: "new-password", disabled: active }} />
              {active && <p className="text-caption text-muted">{t("ddns.editLocked")}</p>}
            </div>
          </AdvancedDisclosure>
        </>
      )}
    </Card>
  );
}
