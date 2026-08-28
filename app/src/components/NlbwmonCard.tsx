import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { BarChart3 } from "lucide-react";
import { api } from "../api";
import type { NlbwmonProbe } from "../types";
import { Card } from "./Card";
import { Toggle } from "./Toggle";

export function NlbwmonCard() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<NlbwmonProbe>();
  const [busy, setBusy] = useState(false);
  const [generations, setGenerations] = useState(30);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  useEffect(() => {
    api.nlbwmon().then((p) => {
      setProbe(p);
      if (p.generations > 0) setGenerations(p.generations);
    }).catch(() => {});
  }, []);

  if (!probe || !probe.installed) return null;

  const save = async () => {
    setBusy(true); setMsg(undefined);
    try {
      const res = await api.setNlbwmon({ generations });
      setProbe(res.state);
      setMsg({ tone: "ok", text: t("nlbwmon.saved") });
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    } finally {
      setBusy(false);
    }
  };

  const toggle = async (enabled: boolean) => {
    setBusy(true); setMsg(undefined);
    try {
      const res = await api.setNlbwmon({ enabled });
      setProbe(res.state);
      setMsg({ tone: "ok", text: enabled ? t("nlbwmon.enabled") : t("nlbwmon.disabled") });
    } catch (e: any) {
      setMsg({ tone: "danger", text: e.message });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card title={t("nlbwmon.title")} icon={BarChart3}>
      <p className="text-xs text-muted mb-3">{t("nlbwmon.intro")}</p>

      <div className="flex items-center justify-between gap-2 mb-3">
        <span className="text-sm">{probe.running ? t("nlbwmon.running") : t("nlbwmon.stopped")}</span>
        <Toggle checked={probe.running} busy={busy} onChange={toggle} />
      </div>

      <div className="space-y-2">
        <div>
          <label className="text-xs text-muted">{t("nlbwmon.generations")}</label>
          <div className="flex items-center gap-2">
            <input type="number" value={generations} min={1} max={365}
              onChange={(e) => setGenerations(Number(e.target.value))}
              className="bg-bg border border-border rounded-lg px-2 py-1 text-sm outline-none focus:border-accent w-20" />
            <span className="text-xs text-muted">{t("nlbwmon.generationsHint")}</span>
          </div>
        </div>

        <div className="text-xs text-muted">
          {t("nlbwmon.interval")}: {probe.commit_interval}s
          {" · "}
          {t("nlbwmon.prealloc")}: {probe.prealloc_days}d
        </div>
      </div>

      <div className="flex justify-end mt-3">
        <button onClick={save} disabled={busy || generations === probe.generations}
          className="text-sm bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium">
          {busy ? "…" : t("nlbwmon.save")}
        </button>
      </div>

      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
