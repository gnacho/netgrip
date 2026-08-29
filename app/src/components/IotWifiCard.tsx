import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Eye, EyeOff, KeyRound, Lightbulb, Shuffle, Wifi } from "lucide-react";
import { api } from "../api";
import type { IoTProbe } from "../types";
import { ActionBanner, Button, Card, HelpTip, Input, KeyValue, Pill, SettingRow, SkeletonRows } from "./ui";
import { QrBox, useWifiQr } from "./wifi/qr";
import { useActionCycle } from "./wifi/action";

function generateKey(): string {
  const chars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789";
  const buf = new Uint32Array(14);
  crypto.getRandomValues(buf);
  return Array.from(buf, (n) => chars[n % chars.length]).join("");
}

/**
 * WiFi para aparatos del hogar (wifi.md §4). Solo 2,4 GHz, aislada del resto
 * de la red. La clave es write-only: el ojo solo revela una clave escrita en
 * esta sesión.
 */
export function IotWifiCard({ probe, mainSsid, onChange }: {
  probe: IoTProbe | undefined;
  mainSsid?: string;
  onChange: (p: IoTProbe) => void;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [sessionKey, setSessionKey] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [setup, setSetup] = useState(false);
  const [ssid, setSsid] = useState("");
  const [key, setKey] = useState("");
  const [doneMsg, setDoneMsg] = useState<string>();

  const qr = useWifiQr(probe?.ssid ?? "", sessionKey, "sae-mixed", 80);

  const apply = async (enabled: boolean, cfg?: { ssid?: string; key?: string }) => {
    const res = await run(() => api.setIotwifi({ enabled, band: "2g", ...cfg }));
    if (res) {
      onChange(res.state);
      if (res.status === "applied") {
        setDoneMsg(enabled ? t("iot.doneOn") : t("iot.doneOff"));
        setSetup(false);
        if (cfg?.key) setSessionKey(cfg.key);
        setKey("");
      }
    }
  };

  const flip = (on: boolean) => {
    if (!probe) return;
    if (on && !probe.ssid) {
      setSsid(mainSsid ? `${mainSsid}-IoT` : "");
      setSetup(true);
      return;
    }
    apply(on, on ? { ssid: probe.ssid, ...(sessionKey ? { key: sessionKey } : {}) } : {});
  };

  return (
    <Card index={4} className="md:col-span-6">
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : (
        <>
          <SettingRow
            icon={Lightbulb}
            iconTone="teal"
            title={t("iot.cardTitle")}
            description={t("iot.desc")}
            help={t("help.iot.body")}
            helpTitle={t("help.iot.title")}
            checked={probe.active}
            busy={busy}
            onChange={flip}
          />

          {phase && (
            <div className="mt-2">
              <ActionBanner
                phase={phase}
                text={phase === "done" ? doneMsg : undefined}
                detail={detail}
                onDone={clear}
              />
            </div>
          )}

          {probe.active && !setup && (
            <div className="mt-3 flex items-start gap-3">
              <div className="flex-1 min-w-0">
                <KeyValue items={[
                  { label: "SSID", value: probe.ssid },
                  {
                    label: t("iot.keyLabel"),
                    value: (
                      <span className="inline-flex items-center gap-1.5">
                        <span className="font-mono text-small">
                          {showKey && sessionKey ? sessionKey : "••••••••••"}
                        </span>
                        {sessionKey && (
                          <button type="button" onClick={() => setShowKey((s) => !s)}
                            className="text-faint hover:text-text ring-focus rounded-sm" aria-label={t("iot.keyLabel")}>
                            {showKey ? <EyeOff size={14} /> : <Eye size={14} />}
                          </button>
                        )}
                      </span>
                    ),
                  },
                  { label: t("iot.clients"), value: t("iot.devices", { count: probe.clients }) },
                ]} />
                {probe.isolated && (
                  <div className="mt-2.5 flex items-center gap-1">
                    <Pill tone="ok">{t("iot.isolatedPill")}</Pill>
                    <HelpTip title={t("help.isolation.title")} body={t("help.isolation.body")} />
                  </div>
                )}
              </div>
              {qr && (
                <div className="shrink-0 flex flex-col items-center gap-1">
                  <QrBox data={qr} size={80} />
                  <p className="text-[10px] text-muted">{t("wifi.scanQr")}</p>
                </div>
              )}
            </div>
          )}

          {!probe.active && !setup && (
            <p className="text-small text-muted text-center py-4 px-2">{t("iot.empty")}</p>
          )}

          {setup && (
            <div className="mt-3 flex flex-col gap-3">
              <Input icon={Wifi} value={ssid} onChange={(e) => setSsid(e.target.value)} placeholder="SSID" maxLength={32} />
              <div className="flex gap-2">
                <div className="flex-1">
                  <Input type="password" mono icon={KeyRound} value={key} onChange={(e) => setKey(e.target.value)}
                    placeholder={t("wifi.keyMin")} autoComplete="new-password"
                    error={key.length > 0 && key.length < 8} />
                </div>
                <Button variant="secondary" size="sm" icon={Shuffle} className="h-[var(--input-h)]" onClick={() => setKey(generateKey())}>
                  {t("wifi.generateKey")}
                </Button>
              </div>
              <div className="flex gap-2">
                <Button size="sm" loading={busy} disabled={!ssid.trim() || key.length < 8}
                  onClick={() => apply(true, { ssid, key })}>
                  {t("iot.activate")}
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setSetup(false)}>
                  {t("common.cancel")}
                </Button>
              </div>
            </div>
          )}
        </>
      )}
    </Card>
  );
}
