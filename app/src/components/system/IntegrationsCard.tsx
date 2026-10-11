import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Bell, Radio, Webhook } from "lucide-react";
import { Card, Pill } from "../ui";
import { CollapsibleSection } from "./CollapsibleSection";
import { TelegramSettings } from "./TelegramSettings";
import { NtfySettings } from "./NtfySettings";
import { MQTTSettings } from "./MQTTSettings";

type Channel = boolean | undefined;

/**
 * Telegram + ntfy + MQTT en una sola tarjeta (#499): cada canal es una
 * línea (nombre + estado + chevron) que se despliega al pulsar. Los
 * formularios permanecen montados (ocultos) para no perder el estado ni
 * repetir peticiones; MQTT además solo sondea el broker con su sección
 * abierta.
 */
export function IntegrationsCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const [telegram, setTelegram] = useState<Channel>();
  const [ntfy, setNtfy] = useState<Channel>();
  const [mqtt, setMqtt] = useState<{ enabled: Channel; connected: Channel }>({ enabled: undefined, connected: undefined });
  const [mqttOpen, setMqttOpen] = useState(false);

  const pill = (on: Channel, extra?: { tone: "ok" | "warn" | "muted"; label: string }) =>
    on === undefined ? undefined : extra && on
      ? <Pill tone={extra.tone}>{extra.label}</Pill>
      : <Pill tone={on ? "ok" : "muted"}>{t(on ? "integrations.on" : "integrations.off")}</Pill>;

  const mqttPill = mqtt.enabled === undefined
    ? undefined
    : mqtt.enabled && mqtt.connected === false
      ? <Pill tone="warn">{t("integrations.noConn")}</Pill>
      : <Pill tone={mqtt.enabled ? "ok" : "muted"}>{t(mqtt.enabled ? "integrations.on" : "integrations.off")}</Pill>;

  return (
    <Card index={index} title={t("integrations.title")} icon={Webhook}>
      <div className="flex flex-col gap-2">
        <CollapsibleSection icon={Bell} title={t("telegram.title")} status={pill(telegram)}>
          <TelegramSettings onStatus={setTelegram} />
        </CollapsibleSection>
        <CollapsibleSection icon={Bell} title={t("ntfy.title")} status={pill(ntfy)}>
          <NtfySettings onStatus={setNtfy} />
        </CollapsibleSection>
        <CollapsibleSection icon={Radio} title={t("mqtt.title")} status={mqttPill}
          onOpenChange={setMqttOpen}>
          <MQTTSettings active={mqttOpen} onStatus={(enabled, connected) => setMqtt({ enabled, connected })} />
        </CollapsibleSection>
      </div>
    </Card>
  );
}
