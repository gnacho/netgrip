import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Eye, EyeOff, KeyRound, Shuffle, Users, Wifi } from "lucide-react";
import { api } from "../api";
import type { GuestProbe } from "../types";
import { ActionBanner, Banner, Button, Card, Input, KeyValue, Pill, SettingRow, SkeletonRows, Toggle } from "./ui";
import { QrBox, useWifiQr } from "./wifi/qr";
import { useActionCycle } from "./wifi/action";

/**
 * WiFi para visitas (wifi.md §3). SettingRow héroe + datos cuando está activa.
 * Solo gateway: atenuada con pill explicativa. La clave es write-only: el ojo
 * y el QR solo funcionan con una clave escrita en esta sesión.
 */
export function GuestWifiCard({ probe, mainSsid, onChange }: {
  probe: GuestProbe | undefined;
  mainSsid?: string;
  onChange: (p: GuestProbe) => void;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [sessionKey, setSessionKey] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [setup, setSetup] = useState(false);
  const [ssid, setSsid] = useState("");
  const [key, setKey] = useState("");
  const [changingKey, setChangingKey] = useState(false);
  const [newKey, setNewKey] = useState("");
  const [doneMsg, setDoneMsg] = useState<string>();

  const qr = useWifiQr(probe?.ssid ?? "", sessionKey, "sae-mixed", 96);

  const apply = async (enabled: boolean, cfg?: { ssid?: string; key?: string }, doneText?: string) => {
    const res = await run(() => api.setGuestwifi({ enabled, ...cfg }));
    if (res) {
      onChange(res.state);
      if (res.status === "applied") {
        setDoneMsg(doneText ?? (enabled ? t("guest.doneOn") : t("guest.doneOff")));
        setSetup(false);
        setChangingKey(false);
        if (cfg?.key) setSessionKey(cfg.key);
        setKey("");
        setNewKey("");
      }
    }
  };

  const flip = (on: boolean) => {
    if (!probe) return;
    if (on && !probe.ssid) {
      // Primera activación: hace falta nombre y clave
      setSsid(mainSsid ? `${mainSsid}-Invitados` : "");
      setSetup(true);
      return;
    }
    apply(on, on ? { ssid: probe.ssid, ...(sessionKey ? { key: sessionKey } : {}) } : {});
  };

  const generate = () => {
    const chars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789";
    const buf = new Uint32Array(14);
    crypto.getRandomValues(buf);
    return Array.from(buf, (n) => chars[n % chars.length]).join("");
  };

  const readOnly = !!probe?.gl_conflict;

  return (
    <Card index={3} className={`md:col-span-6 ${probe && !probe.gateway ? "opacity-70" : ""}`}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : (
        <>
          <SettingRow
            icon={Users}
            iconTone="teal"
            title={t("guest.cardTitle")}
            description={t("guest.desc")}
            help={t("help.guest.body")}
            helpTitle={t("help.guest.title")}
            checked={probe.active}
            busy={busy}
            disabled={!probe.gateway || readOnly}
            onChange={flip}
            control={!probe.gateway ? (
              <span className="flex items-center gap-2">
                <Pill tone="muted">{t("wifi.gatewayOnly")}</Pill>
                <Toggle checked={false} disabled onChange={() => {}} label={t("guest.cardTitle")} />
              </span>
            ) : undefined}
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

          {probe.gl_conflict && (
            <div className="mt-2">
              <Banner tone="warn">{t("guest.conflict")}</Banner>
            </div>
          )}

          {probe.gateway && probe.active && !setup && (
            <div className="mt-3 flex flex-col gap-3">
              <KeyValue items={[
                { label: "SSID", value: probe.ssid },
                {
                  label: t("guest.keyLabel"),
                  value: (
                    <span className="inline-flex items-center gap-1.5">
                      <span className="font-mono text-small">
                        {showKey && sessionKey ? sessionKey : "••••••••••"}
                      </span>
                      {sessionKey && (
                        <button type="button" onClick={() => setShowKey((s) => !s)}
                          className="text-faint hover:text-text ring-focus rounded-sm" aria-label={t("guest.keyLabel")}>
                          {showKey ? <EyeOff size={14} /> : <Eye size={14} />}
                        </button>
                      )}
                    </span>
                  ),
                },
                { label: t("iot.clients"), value: t("guest.clientsNow", { count: probe.clients }) },
              ]} />

              {!readOnly && (
                changingKey ? (
                  <div className="flex flex-col gap-2">
                    <div className="flex gap-2">
                      <div className="flex-1">
                        <Input type="password" mono icon={KeyRound} value={newKey} onChange={(e) => setNewKey(e.target.value)}
                          placeholder={t("wifi.keyMin")} autoComplete="new-password"
                          error={newKey.length > 0 && newKey.length < 8} />
                      </div>
                      <Button variant="secondary" size="sm" icon={Shuffle} className="h-[var(--input-h)]" onClick={() => setNewKey(generate())}>
                        {t("wifi.generateKey")}
                      </Button>
                    </div>
                    <div className="flex gap-2">
                      <Button size="sm" loading={busy} disabled={newKey.length < 8}
                        onClick={() => apply(true, { ssid: probe.ssid, key: newKey }, t("guest.keySaved"))}>
                        {t("access.save")}
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => { setChangingKey(false); setNewKey(""); }}>
                        {t("common.cancel")}
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div>
                    <Button variant="ghost" size="sm" onClick={() => setChangingKey(true)}>
                      {t("guest.changeKey")}
                    </Button>
                  </div>
                )
              )}

              {qr && (
                <div className="flex flex-col items-center gap-1.5 pt-1">
                  <QrBox data={qr} size={96} />
                  <p className="text-caption text-muted">{t("wifi.scanQr")}</p>
                </div>
              )}
            </div>
          )}

          {probe.gateway && !probe.active && !setup && (
            <p className="text-small text-muted text-center py-4 px-2">{t("guest.empty")}</p>
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
                <Button variant="secondary" size="sm" icon={Shuffle} className="h-[var(--input-h)]" onClick={() => setKey(generate())}>
                  {t("wifi.generateKey")}
                </Button>
              </div>
              <div className="flex gap-2">
                <Button size="sm" loading={busy} disabled={!ssid.trim() || key.length < 8}
                  onClick={() => apply(true, { ssid, key })}>
                  {t("guest.activate")}
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
