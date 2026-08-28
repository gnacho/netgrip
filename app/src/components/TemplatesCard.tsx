import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Wand2, AlertTriangle } from "lucide-react";
import { api } from "../api";
import type { Template } from "../types";
import { Card } from "./Card";

export function TemplatesCard() {
  const { t } = useTranslation();
  const [templates, setTemplates] = useState<Template[]>([]);
  const [busy, setBusy] = useState<string>();
  const [confirmId, setConfirmId] = useState<string>();
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  useEffect(() => { api.templates().then((r) => setTemplates(r.templates ?? [])).catch(() => {}); }, []);

  const apply = async (id: string, destructive: boolean) => {
    if (destructive && confirmId !== id) {
      setConfirmId(id);
      return;
    }
    setBusy(id); setMsg(undefined);
    try {
      await api.applyTemplate(id, destructive);
      setMsg({ tone: "ok", text: t("templates.applied") });
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    } finally { setBusy(undefined); setConfirmId(undefined); }
  };

  return (
    <Card title={t("templates.title")} icon={Wand2}>
      <p className="text-xs text-muted mb-3">{t("templates.intro")}</p>

      <div className="flex flex-col gap-2">
        {templates.map((tpl) => (
          <div key={tpl.id} className="flex items-start gap-3 p-2 bg-bg/50 border border-border/50 rounded-lg">
            <div className="flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">{tpl.name}</span>
                {tpl.destructive && <AlertTriangle size={12} className="text-warn" />}
              </div>
              <p className="text-xs text-muted mt-0.5">{tpl.description}</p>
            </div>
            <div className="shrink-0">
              {confirmId === tpl.id ? (
                <div className="flex gap-1">
                  <button onClick={() => apply(tpl.id, true)} disabled={busy === tpl.id}
                    className="text-xs bg-danger/20 hover:bg-danger/30 rounded px-2 py-1">
                    {busy === tpl.id ? "…" : t("templates.confirmApply")}
                  </button>
                  <button onClick={() => setConfirmId(undefined)} className="text-xs text-muted px-1">x</button>
                </div>
              ) : (
                <button onClick={() => apply(tpl.id, tpl.destructive)} disabled={busy === tpl.id}
                  className={`text-xs rounded-lg px-2 py-1 font-medium ${
                    tpl.destructive
                      ? "bg-warn/20 text-warn hover:bg-warn/30"
                      : "bg-accent/15 text-accent hover:bg-accent/25"
                  } disabled:opacity-50`}>
                  {busy === tpl.id ? "…" : t("templates.apply")}
                </button>
              )}
            </div>
          </div>
        ))}
      </div>

      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
