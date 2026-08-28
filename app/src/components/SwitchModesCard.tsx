import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Layers } from "lucide-react";
import { api } from "../api";
import type { SwitchMode } from "../types";
import { Card } from "./Card";

export function SwitchModesCard() {
  const { t } = useTranslation();
  const [modes, setModes] = useState<SwitchMode[]>([]);
  const [busy, setBusy] = useState<string>();
  const [confirmId, setConfirmId] = useState<string>();
  const [uplink, setUplink] = useState("");
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [ports, setPorts] = useState<string[]>([]);

  useEffect(() => {
    api.switchModes().then((r) => setModes(r.modes ?? [])).catch(() => {});
    api.switchPorts().then((r) => {
      if (r.applicable) setPorts(r.ports.map((p) => p.name));
    }).catch(() => {});
  }, []);

  const needsUplink = (id: string) => id === "trunk-uplink" || id === "segmented-home";

  const apply = async (id: string) => {
    if (confirmId !== id) {
      setConfirmId(id);
      return;
    }
    if (needsUplink(id) && !uplink) {
      setMsg({ tone: "danger", text: t("switchModes.uplinkRequired") });
      return;
    }
    setBusy(id); setMsg(undefined);
    try {
      await api.applySwitchMode(id, uplink, true);
      setMsg({ tone: "ok", text: t("switchModes.applied") });
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    } finally { setBusy(undefined); setConfirmId(undefined); }
  };

  return (
    <Card title={t("switchModes.title")} icon={Layers}>
      <p className="text-xs text-muted mb-3">{t("switchModes.intro")}</p>

      {ports.length > 0 && (
        <div className="mb-3">
          <label className="text-xs text-muted">{t("switchModes.uplinkPort")}</label>
          <select value={uplink} onChange={(e) => setUplink(e.target.value)}
            className="bg-bg border border-border rounded-lg px-2 py-1 text-sm mt-1">
            <option value="">{t("switchModes.selectPort")}</option>
            {ports.map((p) => <option key={p} value={p}>{p}</option>)}
          </select>
        </div>
      )}

      <div className="flex flex-col gap-2">
        {modes.map((mode) => (
          <div key={mode.id} className="flex items-start gap-3 p-2 bg-bg/50 border border-border/50 rounded-lg">
            <div className="flex-1">
              <span className="text-sm font-medium">{mode.name}</span>
              <p className="text-xs text-muted mt-0.5">{mode.description}</p>
            </div>
            <div className="shrink-0">
              {confirmId === mode.id ? (
                <div className="flex gap-1">
                  <button onClick={() => apply(mode.id)} disabled={busy === mode.id}
                    className="text-xs bg-warn/20 hover:bg-warn/30 rounded px-2 py-1">
                    {busy === mode.id ? "…" : t("switchModes.confirmApply")}
                  </button>
                  <button onClick={() => setConfirmId(undefined)} className="text-xs text-muted px-1">x</button>
                </div>
              ) : (
                <button onClick={() => apply(mode.id)} disabled={busy === mode.id}
                  className="text-xs bg-accent/15 text-accent px-2 py-1 rounded-lg hover:bg-accent/25 disabled:opacity-50 font-medium">
                  {busy === mode.id ? "…" : t("switchModes.apply")}
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
