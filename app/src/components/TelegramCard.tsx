import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Send, Eye, EyeOff } from "lucide-react";
import { api } from "../api";
import { Card } from "./Card";

export function TelegramCard() {
  const { t } = useTranslation();
  const [botToken, setBotToken] = useState("");
  const [chatId, setChatId] = useState("");
  const [enabled, setEnabled] = useState(false);
  const [showToken, setShowToken] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [msg, setMsg] = useState<{ type: "ok" | "err"; text: string } | null>(null);
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
    setMsg(null);
    try {
      const res = await api.telegramSet(botToken, chatId, enabled);
      setMsg({ type: "ok", text: t("telegram.saved") });
      if (res.botName) setBotName(res.botName);
      if (res.chatName) setChatName(res.chatName);
    } catch (e: any) {
      setMsg({ type: "err", text: e.message });
    }
    setSaving(false);
  };

  const test = async () => {
    setTesting(true);
    setMsg(null);
    try {
      await api.telegramTest();
      setMsg({ type: "ok", text: "✓" });
    } catch (e: any) {
      setMsg({ type: "err", text: e.message });
    }
    setTesting(false);
  };

  return (
    <Card title={t("telegram.title")} icon={Send}>
      <p className="text-sm text-gray-400 mb-4">{t("telegram.description")}</p>

      <label className="flex items-center gap-2 mb-4 text-sm">
        <input
          type="checkbox"
          checked={enabled}
          onChange={(e) => setEnabled(e.target.checked)}
          className="rounded"
        />
        {t("telegram.enabled")}
      </label>

      <div className="space-y-3">
        <div>
          <label className="block text-xs text-gray-500 mb-1">{t("telegram.botToken")}</label>
          <div className="flex gap-2">
            <input
              type={showToken ? "text" : "password"}
              value={botToken}
              onChange={(e) => setBotToken(e.target.value)}
              className="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm"
              placeholder="123456:ABC-DEF..."
            />
            <button
              type="button"
              onClick={() => setShowToken(!showToken)}
              className="px-2 text-gray-400 hover:text-gray-200"
            >
              {showToken ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </button>
          </div>
          {botName && <p className="text-xs text-gray-500 mt-1">{t("telegram.bot")}: @{botName}</p>}
        </div>

        <div>
          <label className="block text-xs text-gray-500 mb-1">{t("telegram.chatId")}</label>
          <input
            type="text"
            value={chatId}
            onChange={(e) => setChatId(e.target.value)}
            className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm"
            placeholder="-1001234567890"
          />
          {chatName && <p className="text-xs text-gray-500 mt-1">{t("telegram.chat")}: {chatName}</p>}
        </div>
      </div>

      {msg && (
        <p className={`text-sm mt-3 ${msg.type === "ok" ? "text-green-400" : "text-red-400"}`}>
          {msg.text}
        </p>
      )}

      <div className="flex gap-2 mt-4">
        <button
          onClick={save}
          disabled={saving || !botToken || !chatId}
          className="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 px-4 py-1.5 rounded text-sm font-medium"
        >
          {saving ? t("telegram.sending") : t("telegram.save")}
        </button>
        <button
          onClick={test}
          disabled={testing || !enabled}
          className="bg-gray-700 hover:bg-gray-600 disabled:opacity-50 px-4 py-1.5 rounded text-sm font-medium flex items-center gap-1"
        >
          <Send className="w-3 h-3" />
          {testing ? t("telegram.sending") : t("telegram.sendTest")}
        </button>
      </div>
    </Card>
  );
}
