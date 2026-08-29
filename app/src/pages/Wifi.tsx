import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { CloudOff, Pencil, QrCode as QrCodeIcon, Wifi as WifiIcon } from "lucide-react";
import { api } from "../api";
import type { GuestProbe, IoTProbe, WifiUI, WirelessRadio } from "../types";
import {
  ActionBanner, Button, Card, ConfirmDialog, EmptyState, HelpTip,
  Modal, Pill, SettingRow, Skeleton,
} from "../components/ui";
import { IlluWifiWaves } from "../components/ui/illustrations";
import { QrBox, useWifiQr } from "../components/wifi/qr";
import { useActionCycle } from "../components/wifi/action";
import { WifiEditModal } from "../components/WifiEditModal";
import { GuestWifiCard } from "../components/GuestWifiCard";
import { IotWifiCard } from "../components/IotWifiCard";

/** htmode OpenWrt ("HE80", "HT20") → "80 MHz" */
function fmtWidth(htmode: string): string {
  const m = htmode.match(/(\d+)/);
  return m ? `${m[1]} MHz` : htmode;
}

/** Título a una línea (design-rev2 §3): ellipsis + tooltip nativo. */
function oneLine(text: string) {
  return <span className="block truncate" title={text}>{text}</span>;
}

export function WifiPage({ iot, onIotChange, guest, onGuestChange }: {
  iot: IoTProbe | undefined;
  onIotChange: (p: IoTProbe) => void;
  guest: GuestProbe | undefined;
  onGuestChange: (p: GuestProbe) => void;
}) {
  const [ifaces, setIfaces] = useState<WifiUI[]>();
  const [radios, setRadios] = useState<WirelessRadio[]>();
  const [error, setError] = useState(false);
  const [editing, setEditing] = useState<WifiUI>();
  const [mainKey, setMainKey] = useState<string>();
  const [qrBig, setQrBig] = useState(false);
  const [confirmOff, setConfirmOff] = useState(false);

  const load = useCallback(async () => {
    setError(false);
    try {
      const [w, r] = await Promise.all([api.wifi(), api.wireless()]);
      setIfaces(w.interfaces);
      setRadios(r);
    } catch {
      setError(true);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  // Red principal: las interfaces que no son de visitas ni de aparatos
  // (por ifname; por SSID como refuerzo mientras llegan los probes).
  const secondary = [...(guest?.ifaces ?? []), ...(iot?.ifaces ?? [])];
  const secondarySsids = [guest?.ssid, iot?.ssid].filter(Boolean);
  const main = (ifaces ?? []).filter((i) => !secondary.includes(i.ifname) && !secondarySsids.includes(i.ssid));

  const saved = (updated: WifiUI, sessionKey?: string) => {
    setIfaces((prev) => prev?.map((p) => (p.section === updated.section ? updated : p)));
    if (sessionKey) setMainKey(sessionKey);
  };

  return (
    <div className="grid grid-cols-1 md:grid-cols-12 gap-[var(--card-gap)]">
      <HeroCard
        ifaces={ifaces}
        radios={radios}
        error={error}
        main={main}
        mainKey={mainKey}
        onRetry={load}
        onEdit={setEditing}
        onEnlargeQr={() => setQrBig(true)}
        onSavedAll={setIfaces}
        askOff={() => setConfirmOff(true)}
        confirmOffOpen={confirmOff}
        closeConfirmOff={() => setConfirmOff(false)}
      />

      <GuestWifiCard probe={guest} mainSsid={main[0]?.ssid} onChange={onGuestChange} />
      <IotWifiCard probe={iot} mainSsid={main[0]?.ssid} onChange={onIotChange} />

      {editing && (
        <WifiEditModal iface={editing} onClose={() => setEditing(undefined)} onSaved={saved} />
      )}

      {main[0] && mainKey && (
        <QrModal ssid={main[0].ssid} enc={main[0].encryption} passkey={mainKey} open={qrBig} onClose={() => setQrBig(false)} />
      )}
    </div>
  );
}

/* ══════════════ Hero — Tu WiFi (wifi.md §2) ══════════════ */

function HeroCard({ ifaces, radios, error, main, mainKey, onRetry, onEdit, onEnlargeQr, onSavedAll, askOff, confirmOffOpen, closeConfirmOff }: {
  ifaces: WifiUI[] | undefined;
  radios: WirelessRadio[] | undefined;
  error: boolean;
  main: WifiUI[];
  mainKey: string | undefined;
  onRetry: () => void;
  onEdit: (i: WifiUI) => void;
  onEnlargeQr: () => void;
  onSavedAll: (i: WifiUI[]) => void;
  askOff: () => void;
  confirmOffOpen: boolean;
  closeConfirmOff: () => void;
}) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [killMsg, setKillMsg] = useState<string>();

  const anyOn = main.some((i) => !i.disabled);
  const clientCount = main.reduce((n, i) => n + (i.disabled ? 0 : i.clients.length), 0);
  const first = main[0];
  const qr = useWifiQr(first?.ssid ?? "", mainKey ?? "", first?.encryption ?? "psk2", 120);

  const setAll = (on: boolean) => {
    setKillMsg(on ? t("wifi.allOnOk") : t("wifi.allOffOk"));
    run(async () => {
      let last: Awaited<ReturnType<typeof api.setWifi>> | undefined;
      for (const i of main) {
        if (i.disabled === on) continue; // ya está como queremos
        const r = await api.setWifi({ section: i.section, disabled: !on });
        if (r.status !== "applied") return r;
        last = r;
      }
      if (!last) throw new Error("nothing to change");
      return last;
    }).then(async (res) => {
      if (res?.status === "applied") {
        try {
          const w = await api.wifi();
          onSavedAll(w.interfaces);
        } catch { /* la lista se refrescará en el próximo load */ }
      }
    });
  };

  return (
    <Card index={0} className="md:col-span-12">
      {error ? (
        <EmptyState
          small
          title={t("wifi.loadError")}
          illustration={<CloudOff size={24} />}
          action={<Button variant="secondary" size="sm" onClick={onRetry}>{t("common.retry")}</Button>}
        />
      ) : !ifaces ? (
        <div>
          <Skeleton className="h-3 w-16 mb-2" />
          <Skeleton className="h-7 w-48 mb-4" />
          <div className="grid gap-[var(--card-gap)] sm:grid-cols-2">
            <Skeleton className="h-32" />
            <Skeleton className="h-32" />
          </div>
        </div>
      ) : main.length === 0 ? (
        <EmptyState
          title={t("wifi.noRadios")}
          illustration={<IlluWifiWaves size={120} />}
        />
      ) : (
        <>
          <div className="flex flex-col md:flex-row md:items-start gap-5">
            <div className="flex-1 min-w-0">
              <p className="text-eyebrow text-faint mb-1">{t("wifi.yours")}</p>
              <div className="flex items-center gap-1.5">
                <h1 className="text-h1 font-bold truncate" title={first.ssid}>{first.ssid}</h1>
                <HelpTip title={t("help.wifiMain.title")} body={t("help.wifiMain.body")} />
              </div>
              <div className="mt-2">
                {anyOn ? (
                  <Pill tone="ok">{t("wifi.activeCount", { count: clientCount })}</Pill>
                ) : (
                  <Pill tone="muted">{t("wifi.off")}</Pill>
                )}
              </div>
              <div className="mt-4">
                <Button icon={Pencil} onClick={() => onEdit(first)}>{t("wifi.editMain")}</Button>
              </div>
            </div>

            <div className="shrink-0 flex flex-col items-center gap-2 self-center">
              {qr ? (
                <>
                  <QrBox data={qr} size={120} />
                  <p className="text-caption text-muted">{t("wifi.scanQr")}</p>
                  <Button variant="ghost" size="sm" onClick={onEnlargeQr}>{t("wifi.enlargeQr")}</Button>
                </>
              ) : (
                <div className="w-[120px] h-[120px] rounded-md border border-dashed border-border-strong flex items-center justify-center text-faint">
                  <QrCodeIcon size={24} aria-hidden="true" />
                </div>
              )}
            </div>
          </div>

          <div className="grid gap-[var(--card-gap)] sm:grid-cols-2 mt-5">
            {main.map((iface, i) => (
              <BandCard
                key={iface.section}
                iface={iface}
                radio={radios?.find((r) => r.name === iface.radio)}
                index={i + 1}
                onEdit={() => onEdit(iface)}
                passkey={mainKey}
              />
            ))}
          </div>

          <div className="mt-2 border-t border-border/60">
            <SettingRow
              title={t("wifi.killAll")}
              description={t("wifi.killAllDesc")}
              checked={anyOn}
              busy={busy}
              onChange={(v) => (v ? setAll(true) : askOff())}
            />
            {phase && (
              <ActionBanner phase={phase} text={phase === "done" ? killMsg : undefined} detail={detail} onDone={clear} />
            )}
          </div>

          <ConfirmDialog
            open={confirmOffOpen}
            onClose={closeConfirmOff}
            onConfirm={() => { closeConfirmOff(); setAll(false); }}
            title={t("wifi.killAllConfirmTitle")}
            consequence={t("wifi.killAllConsequence")}
            confirmLabel={t("wifi.killAllConfirm")}
          />
        </>
      )}
    </Card>
  );
}

/* ══════════════ Mini-card de banda ══════════════ */

function BandCard({ iface, radio, index, onEdit, passkey }: {
  iface: WifiUI;
  radio: WirelessRadio | undefined;
  index: number;
  onEdit: () => void;
  passkey?: string;
}) {
  const { t } = useTranslation();
  const band = iface.band === "5g" ? "band5" : "band24";
  const on = !iface.disabled;
  const qr = useWifiQr(iface.ssid, passkey ?? "", iface.encryption, 72);

  return (
    <Card
      variant="subtle"
      index={index}
      title={oneLine(t(`wifi.${band}`))}
      icon={WifiIcon}
      iconTone="teal"
      help={band}
      action={<Pill tone={on ? "ok" : "muted"}>{on ? t("wifi.broadcasting") : t("wifi.bandOff")}</Pill>}
    >
      {on ? (
        <div className="flex items-start gap-3">
          <div className="flex-1 min-w-0">
            <div className="flex flex-wrap gap-x-4 gap-y-1 text-small">
              <span className="text-muted">{t("wifi.channel")}: <span className="font-medium text-text">{radio?.channel ?? "—"}</span></span>
              <span className="text-muted">{t("wifi.width")}: <span className="font-medium text-text">{radio ? fmtWidth(radio.htmode) : "—"}</span></span>
              <span className="text-muted">{t("wifi.power")}: <span className="font-medium text-text">{radio ? `${radio.txpower} dBm` : "—"}</span></span>
            </div>
            <p className="text-small text-muted mt-1">{t("wifi.devices", { count: iface.clients.length })}</p>
          </div>
          <div className="shrink-0 flex flex-col items-center gap-1">
            {qr ? (
              <>
                <QrBox data={qr} size={72} />
                <p className="text-[10px] text-muted">{t("wifi.scanQr")}</p>
              </>
            ) : (
              <div className="w-[72px] h-[72px] rounded-sm border border-dashed border-border-strong flex items-center justify-center text-faint">
                <QrCodeIcon size={18} aria-hidden="true" />
              </div>
            )}
          </div>
        </div>
      ) : (
        <p className="text-small text-muted">{t("wifi.bandOffHint")}</p>
      )}
      <div className="mt-3">
        <Button variant="secondary" size="sm" onClick={onEdit}>{t("wifi.bandSettings")}</Button>
      </div>
    </Card>
  );
}

/* ══════════════ Modal QR ampliado ══════════════ */

function QrModal({ ssid, enc, passkey, open, onClose }: {
  ssid: string;
  enc: string;
  passkey: string;
  open: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const qr = useWifiQr(ssid, passkey, enc, 260);
  return (
    <Modal open={open} onClose={onClose} title={oneLine(t("wifi.qrModalTitle", { ssid }))}>
      <div className="flex flex-col items-center gap-2 py-2">
        {qr && <QrBox data={qr} size={260} />}
        <p className="text-small text-muted">{t("wifi.scanQr")}</p>
      </div>
    </Modal>
  );
}
