import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Globe } from "lucide-react";
import { api } from "../api";
import type { RemoteAccess } from "../types";
import { Card } from "./Card";

export function RemoteAccessCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<RemoteAccess>();
  const [busy, setBusy] = useState<string>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  useEffect(() => {
    api.remoteAccess().then(setProbe).catch(() => {});
  }, []);

  const toggle = async (key: "ping_wan" | "remote_https" | "remote_ssh", value: boolean) => {
    setBusy(key); setMsg(undefined);
    try {
      await api.setRemoteAccess({ [key]: value });
      setProbe(await api.remoteAccess());
      setMsg({ tone: "ok", text: t("access.saved") });
    } catch (err) {
      setMsg({ tone: "danger", text: err instanceof Error ? err.message : String(err) });
    } finally {
      setBusy(undefined);
    }
  };

  const items: { key: "ping_wan" | "remote_https" | "remote_ssh"; label: string; hint: string }[] = [
    { key: "ping_wan", label: t("remote.pingWan"), hint: t("remote.pingWanHint") },
    { key: "remote_https", label: t("remote.https"), hint: t("remote.httpsHint") },
    { key: "remote_ssh", label: t("remote.ssh"), hint: t("remote.sshHint") },
  ];

  return (
    <Card title={t("remote.title")} icon={Globe}>
      {!probe ? (
        <p className="text-sm text-muted">…</p>
      ) : !probe.applicable ? (
        <p className="text-sm text-warn">{t("remote.notApplicable")}</p>
      ) : (
        <div className="flex flex-col gap-3 text-sm">
          <p className="text-xs text-muted">{t("remote.intro")}</p>
          {items.map((item) => (
            <div key={item.key} className="flex items-center justify-between gap-2">
              <div className="flex-1">
                <div>{item.label}</div>
                <div className="text-xs text-muted">{item.hint}</div>
              </div>
              <input
                type="checkbox" checked={probe[item.key]}
                disabled={busy === item.key}
                onChange={(e) => toggle(item.key, e.target.checked)}
                className="accent-accent"
              />
            </div>
          ))}
          <p className="text-xs text-danger">{t("remote.warn")}</p>
          {msg && <p className={`text-xs ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
        </div>
      )}
    </Card>
  );
}
