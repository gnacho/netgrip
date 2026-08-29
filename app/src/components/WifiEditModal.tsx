import { useState } from "react";
import { useTranslation } from "react-i18next";
import { EyeOff, KeyRound, RadioTower, Shuffle, Wifi } from "lucide-react";
import { api } from "../api";
import type { WifiUI } from "../types";
import {
  ActionBanner, Banner, Button, Field, Input, Modal, Pill, SegmentedControl, SettingRow,
} from "./ui";
import { QrBox, useWifiQr } from "./wifi/qr";
import { useActionCycle } from "./wifi/action";

type Sec = "psk2" | "sae" | "sae-mixed" | "none";

function toSec(encryption: string): Sec {
  if (encryption === "sae" || encryption === "sae-mixed" || encryption === "none") return encryption;
  return "psk2"; // psk2, psk-mixed y variantes antiguas → WPA2
}

function generateKey(): string {
  const chars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789";
  const buf = new Uint32Array(14);
  crypto.getRandomValues(buf);
  return Array.from(buf, (n) => chars[n % chars.length]).join("");
}

/**
 * Modal de edición WiFi (wifi.md §2): nombre, seguridad segmentada, clave con
 * generador, red oculta, emisión de la banda, QR en vivo y aviso de reinicio.
 * Guardar → ActionBanner applying/verifying; rollback → "sigue como estaba".
 */
export function WifiEditModal({ iface, onClose, onSaved }: {
  iface: WifiUI;
  onClose: () => void;
  onSaved: (updated: WifiUI, sessionKey?: string) => void;
}) {
  const { t } = useTranslation();
  const [ssid, setSsid] = useState(iface.ssid);
  const [key, setKey] = useState("");
  const [sec, setSec] = useState<Sec>(toSec(iface.encryption));
  const [hidden, setHidden] = useState(iface.hidden);
  const [emitting, setEmitting] = useState(!iface.disabled);
  const { phase, detail, busy, run } = useActionCycle();

  const bandLabel = iface.band === "5g" ? t("wifi.band5") : t("wifi.band24");
  const keyError = key.length > 0 && key.length < 8 ? t("wifi.keyMin") : undefined;
  const qr = useWifiQr(ssid, key, sec, 160);

  const segments: { value: Sec; label: string }[] = [
    { value: "psk2", label: "WPA2" },
    { value: "sae", label: "WPA3" },
    { value: "sae-mixed", label: "WPA2/WPA3" },
  ];
  if (iface.encryption === "none") segments.push({ value: "none", label: t("wifi.open") });

  const save = () => {
    run(async () => {
      const edit: { section: string; ssid: string; encryption: string; hidden: boolean; disabled: boolean; key?: string } = {
        section: iface.section,
        ssid,
        encryption: sec,
        hidden,
        disabled: !emitting,
      };
      if (key) edit.key = key;
      return api.setWifi(edit);
    }).then((res) => {
      if (res?.status === "applied") {
        onSaved(res.state, key || undefined);
        setTimeout(onClose, 900);
      }
    });
  };

  return (
    <Modal
      open
      onClose={onClose}
      title={
        <span className="flex items-center gap-2 min-w-0">
          <span className="truncate" title={t("wifi.editTitle")}>{t("wifi.editTitle")}</span>
          <span className="shrink-0"><Pill tone="accent">{bandLabel}</Pill></span>
        </span>
      }
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>{t("common.cancel")}</Button>
          <Button onClick={save} loading={busy} disabled={!ssid.trim() || !!keyError}>
            {t("access.save")}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <Field label={t("wifi.ssid")}>
          <Input icon={Wifi} value={ssid} onChange={(e) => setSsid(e.target.value)} maxLength={32} />
        </Field>

        <Field label={t("wifi.encryption")} hint={t("wifi.securityCaption")}>
          <SegmentedControl
            ariaLabel={t("wifi.encryption")}
            options={segments}
            value={sec}
            onChange={setSec}
            size="sm"
          />
        </Field>

        <Field
          label={t("wifi.key")}
          hint={!key && iface.has_key ? t("wifi.keepKey") : undefined}
          error={keyError}
        >
          <div className="flex gap-2">
            <div className="flex-1">
              <Input
                type="password"
                mono
                icon={KeyRound}
                value={key}
                onChange={(e) => setKey(e.target.value)}
                placeholder={iface.has_key ? "••••••••" : ""}
                autoComplete="new-password"
                error={!!keyError}
              />
            </div>
            <Button variant="secondary" size="sm" icon={Shuffle} className="h-[var(--input-h)]" onClick={() => setKey(generateKey())}>
              {t("wifi.generateKey")}
            </Button>
          </div>
        </Field>

        <div className="rounded-md border border-border/60 px-3 divide-y divide-border/60">
          <SettingRow
            icon={EyeOff}
            iconTone="teal"
            title={t("wifi.hiddenNet")}
            description={t("wifi.hiddenDesc")}
            help={t("help.hidden.body")}
            helpTitle={t("help.hidden.title")}
            checked={hidden}
            onChange={setHidden}
          />
          <SettingRow
            icon={RadioTower}
            iconTone="teal"
            title={t("wifi.emitBand")}
            description={t("wifi.emitBandDesc")}
            checked={emitting}
            onChange={setEmitting}
          />
        </div>

        {qr ? (
          <div className="flex flex-col items-center gap-1.5 py-1">
            <QrBox data={qr} size={160} />
            <p className="text-caption text-muted">{t("wifi.scanQr")}</p>
          </div>
        ) : (
          <p className="text-small text-muted text-center">{t("wifi.qrNote")}</p>
        )}

        <Banner tone="warn">{t("wifi.saveRestartWarn")}</Banner>

        {phase && (
          <ActionBanner
            phase={phase}
            text={phase === "done" ? t("wifi.savedOk") : phase === "failed" ? t("wifi.rollbackNote") : undefined}
            detail={detail}
          />
        )}
      </div>
    </Modal>
  );
}
