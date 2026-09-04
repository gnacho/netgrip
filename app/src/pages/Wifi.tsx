import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Ban, CloudOff, Copy, Eye, EyeOff, Pencil, QrCode as QrCodeIcon, Wifi as WifiIcon } from "lucide-react";
import { api } from "../api";
import type { BlockedClient, GuestProbe, IoTProbe, WifiUI, WirelessRadio } from "../types";
import {
  Button, Card, EmptyState,
  Modal, Pill, Skeleton,
} from "../components/ui";
import { IlluWifiWaves } from "../components/ui/illustrations";
import { QrBox, useWifiQr } from "../components/wifi/qr";
import { WifiEditModal } from "../components/WifiEditModal";
import { GuestWifiCard } from "../components/GuestWifiCard";
import { IotWifiCard } from "../components/IotWifiCard";

function fmtWidth(htmode: string): string {
  const m = htmode.match(/(\d+)/);
  return m ? `${m[1]} MHz` : htmode;
}

function oneLine(text: string) {
  return <span className="block truncate" title={text}>{text}</span>;
}

export function WifiPage({ iot, onIotChange, guest, onGuestChange }: {
  iot: IoTProbe | undefined;
  onIotChange: (p: IoTProbe) => void;
  guest: GuestProbe | undefined;
  onGuestChange: (p: GuestProbe) => void;
}) {
  const { t } = useTranslation();
  const [ifaces, setIfaces] = useState<WifiUI[]>();
  const [radios, setRadios] = useState<WirelessRadio[]>();
  const [error, setError] = useState(false);
  const [editing, setEditing] = useState<WifiUI>();
  const [keys, setKeys] = useState<Record<string, string>>({});
  const [blocked, setBlocked] = useState<BlockedClient[]>([]);
  const [meta, setMeta] = useState<Record<string, { name: string; device_type: string }>>({});
  const [qrOpen, setQrOpen] = useState<WifiUI | undefined>();
  const [blockedOpen, setBlockedOpen] = useState<"2g" | "5g" | undefined>();

  const refreshBlocked = useCallback(async () => {
    try {
      const r = await api.blockedClients();
      setBlocked(r.blocked ?? []);
    } catch { /* no crítico */ }
  }, []);

  const load = useCallback(async () => {
    setError(false);
    try {
      const [w, r, m] = await Promise.all([api.wifi(), api.wireless(), api.clientMeta()]);
      setIfaces(w.interfaces);
      setRadios(r);
      setMeta(m.meta ?? {});
      await refreshBlocked();
    } catch {
      setError(true);
    }
  }, [refreshBlocked]);

  useEffect(() => { load(); }, [load]);

  // Cargar claves de todas las radios al montar (autenticado).
  useEffect(() => {
    if (!ifaces) return;
    const main = mainIfaces(ifaces, guest, iot);
    Promise.all(main.map((i) => api.wifiKey(i.section).catch(() => ({ key: "" }))))
      .then((results) => {
        const next: Record<string, string> = {};
        main.forEach((i, idx) => { next[i.section] = results[idx]?.key ?? ""; });
        setKeys(next);
      });
  }, [ifaces, guest, iot]);

  const secondary = [...(guest?.ifaces ?? []), ...(iot?.ifaces ?? [])];
  const secondarySsids = [guest?.ssid, iot?.ssid].filter(Boolean);
  const main = (ifaces ?? []).filter((i) => !secondary.includes(i.ifname) && !secondarySsids.includes(i.ssid));

  const saved = (updated: WifiUI, sessionKey?: string) => {
    setIfaces((prev) => prev?.map((p) => (p.section === updated.section ? updated : p)));
    if (sessionKey) setKeys((k) => ({ ...k, [updated.section]: sessionKey }));
  };

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <Card index={0}>
        {error ? (
          <EmptyState
            small
            title={t("wifi.loadError")}
            illustration={<CloudOff size={24} />}
            action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retry")}</Button>}
          />
        ) : !ifaces ? (
          <div className="space-y-3">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-40" />
            <Skeleton className="h-40" />
          </div>
        ) : main.length === 0 ? (
          <EmptyState title={t("wifi.noRadios")} illustration={<IlluWifiWaves size={120} />} />
        ) : (
          <div className="flex flex-col gap-[var(--card-gap)]">
            {main.map((iface, i) => (
              <RadioCard
                key={iface.section}
                iface={iface}
                radio={radios?.find((r) => r.name === iface.radio)}
                index={i + 1}
                passkey={keys[iface.section]}
                blocked={blocked.filter((b) => b.type === "wifi" && (b.bands ?? []).includes(iface.band))}
                onEdit={() => setEditing(iface)}
                onEnlargeQr={() => setQrOpen(iface)}
                onManageBlocked={() => setBlockedOpen(iface.band as "2g" | "5g")}
              />
            ))}
          </div>
        )}
      </Card>

      <GuestWifiCard probe={guest} mainSsid={main[0]?.ssid} onChange={onGuestChange} />
      <IotWifiCard probe={iot} mainSsid={main[0]?.ssid} onChange={onIotChange} />

      {editing && (
        <WifiEditModal iface={editing} onClose={() => setEditing(undefined)} onSaved={saved} />
      )}

      <QrModal
        iface={qrOpen}
        passkey={qrOpen ? keys[qrOpen.section] : undefined}
        onClose={() => setQrOpen(undefined)}
      />

      <WifiBlockedModal
        band={blockedOpen}
        blocked={blocked.filter((b) => b.type === "wifi" && (b.bands ?? []).includes(blockedOpen ?? ""))}
        meta={meta}
        onUnblock={async (mac, band) => {
          await api.blockClient(mac, "wifi", false, band);
          await refreshBlocked();
        }}
        onClose={() => setBlockedOpen(undefined)}
      />
    </div>
  );
}

function mainIfaces(ifaces: WifiUI[], guest: GuestProbe | undefined, iot: IoTProbe | undefined): WifiUI[] {
  const secondary = [...(guest?.ifaces ?? []), ...(iot?.ifaces ?? [])];
  const secondarySsids = [guest?.ssid, iot?.ssid].filter(Boolean);
  return ifaces.filter((i) => !secondary.includes(i.ifname) && !secondarySsids.includes(i.ssid));
}

/* ══════════════ Tarjeta full-width por radio (#168) ══════════════ */

function RadioCard({ iface, radio, index, passkey, blocked, onEdit, onEnlargeQr, onManageBlocked }: {
  iface: WifiUI;
  radio: WirelessRadio | undefined;
  index: number;
  passkey?: string;
  blocked: BlockedClient[];
  onEdit: () => void;
  onEnlargeQr: () => void;
  onManageBlocked: () => void;
}) {
  const { t } = useTranslation();
  const band = iface.band === "5g" ? "band5" : "band24";
  const on = !iface.disabled;
  const [showKey, setShowKey] = useState(false);
  const [copied, setCopied] = useState(false);
  const qr = useWifiQr(iface.ssid, passkey ?? "", iface.encryption, 96);

  const copyKey = async () => {
    if (!passkey) return;
    try {
      await navigator.clipboard.writeText(passkey);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch { /* clipboard bloqueado */ }
  };

  return (
    <Card
      variant="subtle"
      index={index}
      title={oneLine(iface.ssid)}
      icon={WifiIcon}
      iconTone="teal"
      help={band}
      action={
        <span className="flex items-center gap-2">
          <Pill tone={on ? "ok" : "muted"}>{on ? t("wifi.broadcasting") : t("wifi.bandOff")}</Pill>
          <span className="text-caption text-muted">{t(`wifi.${band}`)}</span>
        </span>
      }
    >
      <div className="flex flex-col md:flex-row gap-5">
        {/* Izquierda: info técnica + clave */}
        <div className="flex-1 min-w-0 space-y-3">
          {on ? (
            <>
              <div className="flex flex-wrap gap-x-4 gap-y-1 text-small">
                <span className="text-muted">{t("wifi.channel")}: <span className="font-medium text-text">{radio?.channel ?? "—"}</span></span>
                <span className="text-muted">{t("wifi.width")}: <span className="font-medium text-text">{radio ? fmtWidth(radio.htmode) : "—"}</span></span>
                <span className="text-muted">{t("wifi.power")}: <span className="font-medium text-text">{radio ? `${radio.txpower} dBm` : "—"}</span></span>
              </div>
              <p className="text-small text-muted">{t("wifi.devices", { count: iface.clients.length })}</p>

              <div className="flex items-center gap-2">
                <span className="text-muted text-small shrink-0">{t("wifi.keyLabel")}:</span>
                {passkey ? (
                  <>
                    <span className="font-mono text-small text-text truncate">
                      {showKey ? passkey : "••••••••"}
                    </span>
                    <button type="button" onClick={() => setShowKey((s) => !s)}
                      title={showKey ? t("wifi.hideKey") : t("wifi.showKey")}
                      aria-label={showKey ? t("wifi.hideKey") : t("wifi.showKey")}
                      className="inline-flex h-7 w-7 items-center justify-center rounded-sm text-muted hover:text-text hover:bg-surface-2 ring-focus transition-colors">
                      {showKey ? <EyeOff size={14} /> : <Eye size={14} />}
                    </button>
                    <button type="button" onClick={copyKey}
                      title={t("wifi.copyKey")}
                      aria-label={t("wifi.copyKey")}
                      className="inline-flex h-7 w-7 items-center justify-center rounded-sm text-muted hover:text-text hover:bg-surface-2 ring-focus transition-colors">
                      <Copy size={14} />
                    </button>
                    {copied && <span className="text-caption text-ok">{t("wifi.copied")}</span>}
                  </>
                ) : (
                  <span className="text-caption text-muted">{t("wifi.keyHidden")}</span>
                )}
              </div>
            </>
          ) : (
            <p className="text-small text-muted">{t("wifi.bandOffHint")}</p>
          )}

          <div className="pt-1 flex flex-wrap items-center gap-2">
            <Button variant="secondary" size="sm" icon={Pencil} onClick={onEdit}>{t("wifi.bandSettings")}</Button>
            {blocked.length > 0 && (
              <button type="button" onClick={onManageBlocked}
                className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-surface px-2.5 py-1 text-small text-muted hover:text-text hover:bg-surface-2 ring-focus transition-colors">
                <Ban size={13} className="text-danger" aria-hidden="true" />
                <span>{t("wifi.manageBlocked", { count: blocked.length })}</span>
              </button>
            )}
          </div>
        </div>

        {/* Derecha: QR */}
        <div className="shrink-0 flex flex-col items-center gap-1.5 self-start">
          {qr ? (
            <>
              <button type="button" onClick={onEnlargeQr}
                title={t("wifi.enlargeQr")}
                aria-label={t("wifi.enlargeQr")}
                className="rounded-md ring-focus transition-transform hover:scale-[1.03]">
                <QrBox data={qr} size={96} />
              </button>
              <p className="text-[10px] text-muted">{t("wifi.scanQr")}</p>
            </>
          ) : (
            <div className="w-[96px] h-[96px] rounded-md border border-dashed border-border-strong flex items-center justify-center text-faint">
              <QrCodeIcon size={24} aria-hidden="true" />
            </div>
          )}
        </div>
      </div>
    </Card>
  );
}

/* ══════════════ Modal de bloqueados por banda (#168) ══════════════ */

function WifiBlockedModal({ band, blocked, meta, onUnblock, onClose }: {
  band: "2g" | "5g" | undefined;
  blocked: BlockedClient[];
  meta: Record<string, { name: string; device_type: string }>;
  onUnblock: (mac: string, band?: string) => Promise<void> | void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState<string>();
  const bandLabel = band ? t(band === "5g" ? "wifi.band5" : "wifi.band24") : "";
  const nameFor = (mac: string) => meta[mac]?.name;

  const doUnblock = async (mac: string, bandScope?: string) => {
    setBusy(mac + ":" + (bandScope ?? ""));
    try {
      await onUnblock(mac, bandScope);
    } finally {
      setBusy(undefined);
    }
  };

  return (
    <Modal open={!!band} onClose={onClose}
      title={t("wifi.blockedTitle", { band: bandLabel, count: blocked.length })}
      footer={<Button variant="ghost" onClick={onClose}>{t("common.close")}</Button>}>
      {blocked.length === 0 ? (
        <EmptyState small title={t("clients.blockedEmpty")} />
      ) : (
        <ul className="flex flex-col divide-y divide-border/60">
          {blocked.map((b) => {
            const name = nameFor(b.mac);
            const thisBusy = busy?.startsWith(b.mac + ":");
            return (
              <li key={b.mac} className="flex items-center gap-3 py-2.5">
                <Ban size={15} className="text-danger shrink-0" aria-hidden="true" />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-small font-medium">{name ?? <span className="font-mono">{b.mac}</span>}</p>
                  <div className="flex flex-wrap gap-1 mt-0.5">
                    {(b.bands ?? []).map((x) => (
                      <Pill key={x} tone={x === band ? "warn" : "muted"}>{t(x === "5g" ? "wifi.band5" : "wifi.band24")}</Pill>
                    ))}
                  </div>
                </div>
                <div className="flex flex-col items-end gap-1 shrink-0">
                  <button type="button" disabled={thisBusy}
                    onClick={() => doUnblock(b.mac, band)}
                    className="text-small text-muted hover:text-text ring-focus disabled:opacity-40">
                    {t("wifi.unblockBand")}
                  </button>
                  {(b.bands?.length ?? 0) > 1 && (
                    <button type="button" disabled={thisBusy}
                      onClick={() => doUnblock(b.mac)}
                      className="text-small text-danger hover:underline ring-focus disabled:opacity-40">
                      {t("clients.unblockAll")}
                    </button>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </Modal>
  );
}


/* ══════════════ Modal QR ampliado con clave copiable (#168) ══════════════ */

function QrModal({ iface, passkey, onClose }: {
  iface: WifiUI | undefined;
  passkey: string | undefined;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [showKey, setShowKey] = useState(false);
  const [copied, setCopied] = useState(false);
  const qr = useWifiQr(iface?.ssid ?? "", passkey ?? "", iface?.encryption ?? "psk2", 280);
  const open = !!iface;

  const copyKey = async () => {
    if (!passkey) return;
    try {
      await navigator.clipboard.writeText(passkey);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch { /* clipboard bloqueado */ }
  };

  useEffect(() => { setShowKey(false); setCopied(false); }, [iface?.section]);

  return (
    <Modal open={open} onClose={onClose} title={iface ? oneLine(iface.ssid) : ""}>
      <div className="flex flex-col items-center gap-3 py-2">
        {qr && <QrBox data={qr} size={280} />}
        <p className="text-small text-muted">{t("wifi.scanQr")}</p>
        {passkey && (
          <div className="flex items-center gap-2 rounded-md border border-border/60 px-3 py-2 w-full max-w-sm">
            <span className="text-muted text-small shrink-0">{t("wifi.keyLabel")}:</span>
            <span className="font-mono text-small text-text flex-1 min-w-0 truncate">
              {showKey ? passkey : "••••••••"}
            </span>
            <button type="button" onClick={() => setShowKey((s) => !s)}
              aria-label={showKey ? t("wifi.hideKey") : t("wifi.showKey")}
              className="inline-flex h-7 w-7 items-center justify-center rounded-sm text-muted hover:text-text hover:bg-surface-2 ring-focus transition-colors">
              {showKey ? <EyeOff size={14} /> : <Eye size={14} />}
            </button>
            <button type="button" onClick={copyKey}
              aria-label={t("wifi.copyKey")}
              className="inline-flex h-7 w-7 items-center justify-center rounded-sm text-muted hover:text-text hover:bg-surface-2 ring-focus transition-colors">
              <Copy size={14} />
            </button>
            {copied && <span className="text-caption text-ok">{t("wifi.copied")}</span>}
          </div>
        )}
      </div>
    </Modal>
  );
}
