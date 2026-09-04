import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Gauge, RadioTower, Waves } from "lucide-react";
import { api } from "../api";
import type { WirelessRadio } from "../types";
import { ActionBanner, Banner, Button, Field, Input, Modal, Pill } from "./ui";
import { useActionCycle } from "./wifi/action";

// Canal y ancho admisibles por banda (permissivos; el firmware valida).
const CH_2G = ["auto", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13"];
const CH_5G = ["auto", "36", "40", "44", "48", "52", "56", "60", "64", "100", "104",
  "108", "112", "116", "120", "124", "128", "132", "136", "140", "144", "149", "153", "157", "161", "165"];
const HT_2G = ["HT20", "HT40", "HE20", "HE40"];
const HT_5G = ["VHT20", "VHT40", "VHT80", "VHT160", "HE20", "HE40", "HE80", "HE160"];

/**
 * Ajustes de la radio (#246): canal, modo/ancho (htmode) y potencia (dBm) de un
 * radio wifi-device. El canal y el modo se eligen de una lista por banda; la
 * potencia es un número en dBm (vacío = dejar la actual). Guardar aplica vía
 * /api/wifi/radio con snapshot/rollback del backend.
 */
export function WifiRadioModal({ radio, onClose, onSaved }: {
  radio: WirelessRadio;
  onClose: () => void;
  onSaved: (updated: WirelessRadio) => void;
}) {
  const { t } = useTranslation();
  const is5g = radio.band === "5g";
  const channels = is5g ? CH_5G : CH_2G;
  const htmodes = is5g ? HT_5G : HT_2G;
  const [channel, setChannel] = useState(radio.channel || "auto");
  const [htmode, setHtmode] = useState(radio.htmode || (is5g ? "HE80" : "HE20"));
  const [txpower, setTxpower] = useState(radio.txpower > 0 ? String(radio.txpower) : "");
  const { phase, detail, busy, run } = useActionCycle();

  const bandLabel = is5g ? t("wifi.band5") : t("wifi.band24");

  const save = () => {
    run(() => api.setWifiRadio({
      radio: radio.name,
      channel,
      htmode,
      txpower: txpower ? Number(txpower) : 0,
    })).then((res) => {
      if (res?.status === "applied") {
        onSaved(res.state);
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
          <span className="truncate">{t("wifi.radioTitle")}</span>
          <span className="shrink-0"><Pill tone="accent">{bandLabel}</Pill></span>
        </span>
      }
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>{t("common.cancel")}</Button>
          <Button onClick={save} loading={busy}>{t("access.save")}</Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <Field label={t("wifi.channel")} hint={t("wifi.radioHint")}>
          <Input icon={RadioTower} mono list="wg-channel" value={channel} onChange={(e) => setChannel(e.target.value)} />
          <datalist id="wg-channel">{channels.map((c) => <option key={c} value={c} />)}</datalist>
        </Field>

        <Field label={t("wifi.width")} hint={t("wifi.widthHint")}>
          <Input icon={Waves} mono list="wg-htmode" value={htmode} onChange={(e) => setHtmode(e.target.value)} />
          <datalist id="wg-htmode">{htmodes.map((h) => <option key={h} value={h} />)}</datalist>
        </Field>

        <Field label={t("wifi.power")} hint={t("wifi.powerHint")}>
          <Input icon={Gauge} mono type="number" value={txpower} onChange={(e) => setTxpower(e.target.value)} placeholder={t("wifi.auto")} />
        </Field>

        <Banner tone="warn">{t("wifi.radioWarn")}</Banner>

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
