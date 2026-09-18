import { useState } from "react";
import { useTranslation } from "react-i18next";
import { EyeOff, KeyRound, Link2, RadioTower, Shuffle, Wifi } from "lucide-react";
import { api } from "../api";
import type { WifiUI } from "../types";
import {
  ActionBanner, Banner, Button, Field, Input, Modal, Pill, SegmentedControl, SettingRow,
} from "./ui";
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
 * generador, red oculta, emisión de la banda y aviso de reinicio.
 * Cuando varias radios comparten SSID ofrece "Unificar bandas" (#328): activado
 * (default) un solo nombre/clave para todas; desactivado, un campo de nombre
 * por banda para separarlas en una sola guardada (#328).
 * Guardar → ActionBanner applying/verifying; rollback → "sigue como estaba".
 */
export function WifiEditModal({ iface, group, main, onClose, onSaved }: {
  iface: WifiUI;
  /** Interfaces que comparten el SSID de `iface` (la red ya unificada). */
  group: WifiUI[];
  /** Todas las interfaces principales del router (radios 2.4/5/6). */
  main: WifiUI[];
  onClose: () => void;
  onSaved: (updated: WifiUI, sessionKey?: string) => void;
}) {
  const { t } = useTranslation();
  const [ssid, setSsid] = useState(iface.ssid);
  const [bandSsids, setBandSsids] = useState<Record<string, string>>(() =>
    Object.fromEntries(group.map((g) => [g.section, g.ssid])));
  const [key, setKey] = useState("");
  const [sec, setSec] = useState<Sec>(toSec(iface.encryption));
  const [hidden, setHidden] = useState(iface.hidden);
  const [emitting, setEmitting] = useState(!iface.disabled);
  const [unify, setUnify] = useState(group.length > 1);
  const { phase, detail, busy, run } = useActionCycle();

  // Objetivo del cambio: al unificar, todas las radios principales si la red
  // estaba suelta; si ya estaba unificada, su propio grupo (sin arrastrar otras
  // redes). Sin unificar, solo esta interfaz (con nombre por banda si el grupo
  // tiene varias radios).
  const target = unify ? (group.length > 1 ? group : main) : [iface];
  const perBand = !unify && group.length > 1;
  const bandName = (b: string) => (b === "5g" ? t("wifi.band5") : t("wifi.band24"));
  const bands = [...new Set(target.map((i) => bandName(i.band)))];
  const bandLabel = bands.join(" + ");
  const keyError = key.length > 0 && key.length < 8 ? t("wifi.keyMin") : undefined;
  // Aviso: sin unificar pero con nombres iguales, las bandas siguen agrupadas.
  const sameBandNames = perBand
    && new Set(Object.values(bandSsids).map((s) => s.trim())).size <= 1;
  const saveDisabled = perBand
    ? group.some((g) => !bandSsids[g.section]?.trim()) || !!keyError
    : !ssid.trim() || !!keyError;

  const segments: { value: Sec; label: string }[] = [
    { value: "psk2", label: "WPA2" },
    { value: "sae", label: "WPA3" },
    { value: "sae-mixed", label: "WPA2/WPA3" },
  ];
  if (iface.encryption === "none") segments.push({ value: "none", label: t("wifi.open") });

  const save = () => {
    run(async () => {
      if (perBand) {
        // Renombra primero las demás bandas del grupo (solo el nombre, para no
        // tocar su seguridad/emisión); la banda propia lleva el resto de campos.
        const renamed = group.filter(
          (g) => g.section !== iface.section && bandSsids[g.section].trim() !== g.ssid,
        );
        for (const g of renamed) {
          const r = await api.setWifi({ section: g.section, sections: [g.section], ssid: bandSsids[g.section].trim() });
          if (r.status !== "applied") return r;
        }
        return api.setWifi({
          section: iface.section,
          sections: [iface.section],
          ssid: bandSsids[iface.section].trim(),
          encryption: sec,
          hidden,
          disabled: !emitting,
          ...(key ? { key } : {}),
        });
      }
      const edit: { section: string; sections: string[]; ssid: string; encryption: string; hidden: boolean; disabled: boolean; key?: string } = {
        section: iface.section,
        sections: target.map((i) => i.section),
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
          <Button onClick={save} loading={busy} disabled={saveDisabled}>
            {t("access.save")}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        {main.length > 1 && (
          <SettingRow
            icon={Link2}
            iconTone="teal"
            title={t("wifi.unifyBands")}
            description={t("wifi.unifyBandsDesc")}
            checked={unify}
            onChange={setUnify}
          />
        )}

        {perBand ? (
          group.map((g) => (
            <Field key={g.section} label={t("wifi.ssidForBand", { band: bandName(g.band) })}>
              <Input
                icon={Wifi}
                value={bandSsids[g.section]}
                onChange={(e) => setBandSsids((s) => ({ ...s, [g.section]: e.target.value }))}
                maxLength={32}
              />
            </Field>
          ))
        ) : (
          <Field
            label={t("wifi.ssid")}
            hint={target.length > 1 ? t("wifi.nameAppliesTo", { bands: bandLabel }) : undefined}
          >
            <Input icon={Wifi} value={ssid} onChange={(e) => setSsid(e.target.value)} maxLength={32} />
          </Field>
        )}

        {sameBandNames && <Banner tone="info">{t("wifi.sameNamesWarn")}</Banner>}

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
