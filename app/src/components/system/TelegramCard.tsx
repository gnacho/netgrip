import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Send, Tag } from "lucide-react";
import { api } from "../../api";
import { Button, Card, Field, SettingRow, useToast } from "../ui";

export function TelegramCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [botToken, setBotToken] = useState("");
  const [chatId, setChatId] = useState("");
  const [enabled, setEnabled] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [botName, setBotName] = useState("");
  const [chatName, setChatName] = useState("");

  useEffect(() => {
    api.telegramGet().then((d) => {
      setBotToken(d.botToken || "");
      setChatId(d.chatId || "");
      setEnabled(d.enabled);
    }).catch(() => {});
  }, []);

  const save = async () => {
    setSaving(true);
    try {
      const res = await api.telegramSet(botToken, chatId, enabled);
      push({ tone: "ok", text: t("telegram.saved") });
      if (res.botName) setBotName(res.botName);
      if (res.chatName) setChatName(res.chatName);
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    }
    setSaving(false);
  };

  const test = async () => {
    setTesting(true);
    try {
      await api.telegramTest();
      push({ tone: "ok", text: t("telegram.testSent") });
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    }
    setTesting(false);
  };

  return (
    <Card index={index} title={t("telegram.title")} icon={Send}>
      <SettingRow
        title={t("telegram.enabled")}
        description={t("telegram.description")}
        checked={enabled}
        onChange={setEnabled}
      />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-1">
        <Field
          label={t("telegram.botToken")}
          icon={Send}
          helpTitle={t("help.telegram.title")}
          help={t("help.telegram.body")}
          hint={botName ? `@${botName}` : undefined}
          inputProps={{
            type: "password", mono: true, value: botToken,
            onChange: (e) => setBotToken(e.target.value),
            placeholder: "123456:ABC-DEF…",
          }}
        />
        <Field
          label={t("telegram.chatId")}
          icon={Tag}
          hint={chatName || undefined}
          inputProps={{
            mono: true, value: chatId,
            onChange: (e) => setChatId(e.target.value),
            placeholder: "-1001234567890",
          }}
        />
      </div>
      <div className="flex gap-2 mt-4">
        <Button onClick={save} loading={saving} disabled={!botToken || !chatId}>{t("telegram.save")}</Button>
        <Button variant="secondary" icon={Send} onClick={test} loading={testing} disabled={!enabled}>
          {t("telegram.sendTest")}
        </Button>
      </div>
    </Card>
  );
}
