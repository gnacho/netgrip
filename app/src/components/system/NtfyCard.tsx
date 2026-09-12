import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Bell, KeyRound, Server, Tag } from "lucide-react";
import { api } from "../../api";
import { Button, Card, Field, SettingRow, useToast } from "../ui";

export function NtfyCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [server, setServer] = useState("");
  const [topic, setTopic] = useState("");
  const [token, setToken] = useState("");
  const [tokenSet, setTokenSet] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  useEffect(() => {
    api.ntfyGet().then((d) => {
      setServer(d.server || "");
      setTopic(d.topic || "");
      setTokenSet(d.tokenSet);
      setEnabled(d.enabled);
    }).catch(() => {});
  }, []);

  const save = async () => {
    setSaving(true);
    try {
      await api.ntfySet(server, topic, token, enabled);
      if (token) setTokenSet(true);
      push({ tone: "ok", text: t("ntfy.saved") });
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    }
    setSaving(false);
  };

  const test = async () => {
    setTesting(true);
    try {
      await api.ntfyTest();
      push({ tone: "ok", text: t("ntfy.testSent") });
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    }
    setTesting(false);
  };

  return (
    <Card index={index} title={t("ntfy.title")} icon={Bell}>
      <SettingRow
        title={t("ntfy.enabled")}
        description={t("ntfy.description")}
        checked={enabled}
        onChange={setEnabled}
      />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-1">
        <Field
          label={t("ntfy.server")}
          icon={Server}
          helpTitle={t("help.ntfy.title")}
          help={t("help.ntfy.body")}
          inputProps={{
            mono: true, value: server,
            onChange: (e) => setServer(e.target.value),
            placeholder: "https://ntfy.sh",
          }}
        />
        <Field
          label={t("ntfy.topic")}
          icon={Tag}
          inputProps={{
            mono: true, value: topic,
            onChange: (e) => setTopic(e.target.value),
            placeholder: "netgrip-casa",
          }}
        />
        <Field
          label={t("ntfy.token")}
          icon={KeyRound}
          hint={tokenSet ? t("ntfy.tokenSet") : undefined}
          inputProps={{
            type: "password", mono: true, value: token,
            onChange: (e) => setToken(e.target.value),
            placeholder: tokenSet ? "••••••••" : undefined,
          }}
        />
      </div>
      <div className="flex gap-2 mt-4">
        <Button onClick={save} loading={saving} disabled={!topic}>{t("ntfy.save")}</Button>
        <Button variant="secondary" icon={Bell} onClick={test} loading={testing} disabled={!enabled}>
          {t("ntfy.sendTest")}
        </Button>
      </div>
    </Card>
  );
}
