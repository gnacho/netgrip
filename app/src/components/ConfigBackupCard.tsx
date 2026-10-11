import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { CloudUpload } from "lucide-react";
import { api } from "../api";
import { Button, Card, Input } from "./ui";

export function ConfigBackupCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const [serverURL, setServerURL] = useState("");
  const [routerID, setRouterID] = useState("");
  const [token, setToken] = useState("");
  const [saving, setSaving] = useState(false);
  const [pushing, setPushing] = useState(false);
  const [editing, setEditing] = useState(false);
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
      setEditing(false);
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
    <Card index={index} title={t("backup.title")} icon={CloudUpload}
      action={
        !editing && (
          <Button size="sm" variant="secondary" onClick={() => setEditing(true)}>
            {t("backup.edit")}
          </Button>
        )
      }
    >
      {/* #500: plegada por defecto; los campos solo al pulsar Editar. */}
      <p className="text-small text-muted">{t("backup.description")}</p>

      {editing && (
        <>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-3">
            <div>
              <label className="block text-xs text-muted mb-1">{t("backup.serverURL")}</label>
              <Input
                value={serverURL}
                onChange={(e) => setServerURL(e.target.value)}
                className="w-full"
                placeholder="https://netpulse.example.com"
              />
            </div>
            <div>
              <label className="block text-xs text-muted mb-1">{t("backup.routerID")}</label>
              <Input
                value={routerID}
                onChange={(e) => setRouterID(e.target.value)}
                className="w-full"
                placeholder="rt1"
              />
            </div>
            <div className="sm:col-span-2">
              <label className="block text-xs text-muted mb-1">{t("backup.token")}</label>
              <Input
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                className="w-full"
                placeholder="API token (optional)"
              />
            </div>
          </div>
          {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
          <div className="flex gap-2 mt-3">
            <Button size="sm" onClick={save} loading={saving} disabled={!serverURL || !routerID}>
              {t("backup.save")}
            </Button>
            <Button size="sm" variant="secondary" onClick={push} loading={pushing} disabled={!serverURL || !routerID} icon={CloudUpload}>
              {t("backup.pushNow")}
            </Button>
          </div>
        </>
      )}
    </Card>
  );
}
