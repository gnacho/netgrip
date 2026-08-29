import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Network } from "lucide-react";
import { api } from "../../api";
import type { IPv6Probe } from "../../types";
import {
  ActionBanner, Card, KeyValue, Pill, SettingRow, SkeletonRows, Toggle,
} from "../ui";
import { useActionCycle } from "../wifi/action";
import { CopyButton, Reveal, shortKey } from "./shared";

/**
 * IPv6 (services.md §6). Pill por estado (Activa / Parcial / Apagada);
 * detalle con la IP local v6 y los modos RA/DHCPv6 en caption técnico.
 */
export function Ipv6Card({ probe, onChange, index = 0 }: {
  probe: IPv6Probe | undefined;
  onChange: (p: IPv6Probe) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [doneMsg, setDoneMsg] = useState<string>();

  const toggle = async (enabled: boolean) => {
    setDoneMsg(undefined);
    const res = await run(() => api.setIpv6(enabled));
    if (res) {
      onChange(res.state);
      if (res.status === "applied") setDoneMsg(enabled ? t("ipv6.doneOn") : t("ipv6.doneOff"));
    } else {
      onChange(await api.ipv6());
    }
  };

  const enabled = probe?.state === "enabled";

  return (
    <Card index={index}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : (
        <>
          <SettingRow
            icon={Network}
            iconTone="accent"
            title="IPv6"
            description={t("ipv6.desc")}
            help={t("help.ipv6.body")}
            helpTitle={t("help.ipv6.title")}
            checked={enabled}
            busy={busy}
            onChange={toggle}
            control={
              <span className="flex items-center gap-2">
                <Pill className="max-w-24 sm:max-w-32" tone={enabled ? "ok" : probe.state === "partial" ? "warn" : "muted"}>
                  {t(`ipv6.${probe.state}`)}
                </Pill>
                <Toggle checked={enabled} busy={busy} onChange={toggle} label="IPv6" />
              </span>
            }
          />

          {phase && (
            <div className="mt-2">
              <ActionBanner phase={phase} text={phase === "done" ? doneMsg : undefined} detail={detail} onDone={clear} />
            </div>
          )}

          <Reveal open={probe.state !== "disabled"}>
            <div className="pt-2">
              {probe.lan_ipv6 && (
                <KeyValue items={[{
                  label: t("ipv6.localIp"),
                  value: (
                    <span className="inline-flex items-center gap-1">
                      <span className="font-mono text-small">{shortKey(probe.lan_ipv6)}</span>
                      <CopyButton text={probe.lan_ipv6} />
                    </span>
                  ),
                }]} />
              )}
              <p className="text-caption text-faint mt-2 font-mono">
                {t("ipv6.modes", {
                  odhcpd: probe.odhcpd_enabled ? t("ipv6.on") : t("ipv6.off"),
                  ra: probe.ra_mode || "-",
                  dhcpv6: probe.dhcpv6_mode || "-",
                })}
              </p>
            </div>
          </Reveal>
        </>
      )}
    </Card>
  );
}
