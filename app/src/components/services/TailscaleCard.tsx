import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ExternalLink, Waypoints } from "lucide-react";
import { api } from "../../api";
import type { TSProbe } from "../../types";
import {
  ActionBanner, Banner, Button, Card, EmptyState, KeyValue, Pill,
  SettingRow, SkeletonRows, Toggle,
} from "../ui";
import { useActionCycle } from "../wifi/action";
import { CopyButton, Reveal, TechName } from "./shared";

/**
 * Tailscale — "Tu red privada entre casas y móviles" (services.md §4).
 * Estado traducido; si hay auth_url, banner con CTA para vincular el router.
 */
export function TailscaleCard({ probe, onChange, index = 0 }: {
  probe: TSProbe | undefined;
  onChange: (p: TSProbe) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [doneMsg, setDoneMsg] = useState<string>();

  const toggle = async (v: boolean) => {
    setDoneMsg(undefined);
    const res = await run(() => api.setTailscale(v));
    if (res) {
      onChange(res.state);
      if (res.status === "applied") setDoneMsg(v ? t("ts.doneOn") : t("ts.doneOff"));
    } else {
      onChange(await api.tailscale());
    }
  };

  const state = (probe?.state ?? "").toLowerCase();
  const connected = !!probe?.running && (state === "running" || state === "connected");
  const needsLogin = state === "needslogin";

  return (
    <Card index={index}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : !probe.installed ? (
        <EmptyState
          small
          illustration={<Waypoints size={24} />}
          title={t("ts.notInstalled")}
          body={t("services.installFromTools")}
        />
      ) : (
        <>
          <SettingRow
            icon={Waypoints}
            iconTone="violet"
            title={t("ts.cardTitle")}
            description={t("ts.desc")}
            help={t("help.tailscale.body")}
            helpTitle={t("help.tailscale.title")}
            checked={probe.running}
            busy={busy}
            onChange={toggle}
            control={
              <span className="flex items-center gap-2">
                <Pill className="max-w-24 sm:max-w-32" tone={connected ? "ok" : needsLogin ? "warn" : "muted"}>
                  {connected ? t("ts.connected") : needsLogin ? t("ts.waitingLogin") : t("ts.off")}
                </Pill>
                <Toggle checked={probe.running} busy={busy} onChange={toggle} label={t("ts.cardTitle")} />
              </span>
            }
          />
          <TechName>Tailscale</TechName>

          {phase && (
            <div className="mt-2">
              <ActionBanner phase={phase} text={phase === "done" ? doneMsg : undefined} detail={detail} onDone={clear} />
            </div>
          )}

          {needsLogin && probe.auth_url && (
            <div className="mt-2">
              <Banner tone="info">
                <span className="block">{t("ts.linkHint")}</span>
                <span className="mt-2 inline-block">
                  <Button size="sm" icon={ExternalLink}
                    onClick={() => window.open(probe.auth_url, "_blank", "noopener,noreferrer")}>
                    {t("ts.linkCta")}
                  </Button>
                </span>
              </Banner>
            </div>
          )}

          <Reveal open={connected && probe.ips.length > 0}>
            <div className="pt-2">
              <KeyValue items={probe.ips.map((ip, i) => ({
                label: i === 0 ? t("ts.ip") : "",
                value: (
                  <span className="inline-flex items-center gap-1">
                    <span className="font-mono text-small">{ip}</span>
                    <CopyButton text={ip} />
                  </span>
                ),
                mono: true,
              }))} />
            </div>
          </Reveal>
        </>
      )}
    </Card>
  );
}
