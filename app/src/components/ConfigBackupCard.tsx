import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { CloudUpload } from "lucide-react";
import { api } from "../api";
import { Card } from "./Card";

export function ConfigBackupCard() {
  const { t } = useTranslation();
  const [serverURL, setServerURL] = useState("");
  const [routerID, setRouterID] = useState("");
  const [token, setToken] = useState("");
  const [saving, setSaving] = useState(false);
  const [pushing, setPushing] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  useEffect(() => {
    api.pushConfigGet().then((d) => {
      setServerURL(d.server_url || "");
      setRouterID(d.router_id || "");
      setToken(d.token || "");
    }).catch(() => {});
  }, []);

  const save = async () => {
    setSaving(true);
    setMsg(undefined);
    try {
      await api.pushConfigSet(serverURL, routerID, token);
      setMsg({ tone: "ok", text: t("backup.saved") });
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    }
    setSaving(false);
  };

  const push = async () => {
    setPushing(true);
    setMsg(undefined);
    try {
      const r = await api.pushSnapshot();
      if (r.ok) {
        setMsg({ tone: "ok", text: t("backup.pushed", { id: r.snapshot_id }) });
      } else {
        setMsg({ tone: "danger", text: r.error || t("backup.pushFailed") });
      }
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    }
    setPushing(false);
  };

  return (
    <Card title={t("backup.title")} icon={CloudUpload}>
      <p className="text-xs text-muted mb-3">{t("backup.description")}</p>
      <div className="space-y-2">
        <div>
          <label className="block text-xs text-muted mb-1">{t("backup.serverURL")}</label>
          <input
            value={serverURL}
            onChange={(e) => setServerURL(e.target.value)}
            className="input w-full"
            placeholder="https://netpulse.example.com"
          />
        </div>
        <div>
          <label className="block text-xs text-muted mb-1">{t("backup.routerID")}</label>
          <input
            value={routerID}
            onChange={(e) => setRouterID(e.target.value)}
            className="input w-full"
            placeholder="rt1"
          />
        </div>
        <div>
          <label className="block text-xs text-muted mb-1">{t("backup.token")}</label>
          <input
            type="password"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            className="input w-full"
            placeholder="API token (optional)"
          />
        </div>
      </div>
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
      <div className="flex gap-2 mt-3">
        <button
          onClick={save}
          disabled={saving || !serverURL || !routerID}
          className="bg-accent hover:bg-accent/85 disabled:opacity-50 text-white px-3 py-1.5 rounded text-sm font-medium"
        >
          {saving ? t("backup.saving") : t("backup.save")}
        </button>
        <button
          onClick={push}
          disabled={pushing || !serverURL || !routerID}
          className="bg-card border border-border hover:bg-border/50 disabled:opacity-50 px-3 py-1.5 rounded text-sm font-medium flex items-center gap-1"
        >
          <CloudUpload size={14} />
          {pushing ? t("backup.pushing") : t("backup.pushNow")}
        </button>
      </div>
    </Card>
  );
}
