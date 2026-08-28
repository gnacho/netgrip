import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Route } from "lucide-react";
import { api } from "../api";
import type { ModeProbe } from "../types";
import { Card, Pill, Row } from "./Card";

export function ModeCard() {
  const { t } = useTranslation();
  const [mode, setMode] = useState<ModeProbe>();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string } | undefined>();

  useEffect(() => {
    api.mode().then(setMode).catch(() => {});
  }, []);

  const switchMode = async (target: "router" | "ap") => {
    const warning = target === "router"
      ? t("mode.confirmRouter")
      : t("mode.confirmAp");
    if (!confirm(warning)) return;
    setBusy(true); setMsg(undefined);
    try {
      const res = await api.setMode(target);
      setMode(res.state);
      setMsg(res.status === "applied"
        ? { tone: "ok", text: t("mode.changed") }
        : { tone: "danger", text: res.error || t("mode.reverted") });
    } catch (err) {
      setMsg({ tone: "danger", text: err instanceof Error ? err.message : String(err) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card title={t("mode.title")} icon={Route}>
      {!mode ? (
        <p className="text-sm text-muted">…</p>
      ) : (
        <>
          <Row label={t("mode.title")} value={
            mode.hardware_class === "switch" ? (
              <Pill tone="muted">{t("mode.switch")}</Pill>
            ) : (
              <Pill tone={mode.mode === "router" ? "ok" : "warn"}>
                {mode.mode === "router" ? t("mode.router") : t("mode.ap")}
              </Pill>
            )
          } />
          {mode.mode === "ap" && mode.hardware_class !== "switch" && <p className="text-xs text-warn mt-1">{t("mode.wanBridge")}</p>}
          {mode.hardware_class === "switch" && <p className="text-xs text-muted mt-1">{t("mode.switchInfo", { ports: mode.port_count })}</p>}
          <Row label="dnsmasq" value={mode.dnsmasq_on ? t("mode.on") : t("mode.off")} />
          <Row label="Firewall" value={mode.firewall_on ? t("mode.on") : t("mode.off")} />

          {mode.hardware_class !== "switch" && (
            <div className="mt-2 flex flex-col gap-2">
              {mode.mode === "ap" ? (
                <button onClick={() => switchMode("router")} disabled={busy}
                  className="text-sm bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium self-start">
                  {busy ? "…" : t("mode.toRouter")}
                </button>
              ) : (
                <button onClick={() => switchMode("ap")} disabled={busy}
                  className="text-sm bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium self-start">
                  {busy ? "…" : t("mode.toAp")}
                </button>
              )}
              {msg && <p className={`text-xs ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
            </div>
          )}
        </>
      )}
    </Card>
  );
}
