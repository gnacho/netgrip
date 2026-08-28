import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Gauge } from "lucide-react";
import { api } from "../api";
import type { SQMProbe } from "../types";
import { Card, Pill, Row } from "./Card";
import { Toggle } from "./Toggle";

export function SqmCard({ probe, onChange }: {
  probe: SQMProbe | undefined;
  onChange: (p: SQMProbe) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [download, setDownload] = useState("");
  const [upload, setUpload] = useState("");

  useEffect(() => {
    if (probe) {
      setDownload(probe.download || "");
      setUpload(probe.upload || "");
    }
  }, [probe]);

  const run = async (enabled: boolean) => {
    setBusy(true);
    setMsg(undefined);
    try {
      const result = await api.setSqm(
        enabled ? { enabled, download, upload } : { enabled },
      );
      onChange(result.state);
      setMsg(result.status === "applied"
        ? { tone: "ok", text: t("sqm.applied") }
        : { tone: "danger", text: result.error || t("sqm.rolledBack") });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("sqm.failed") });
      onChange(await api.sqm());
    } finally {
      setBusy(false);
    }
  };

  const ratesOk = Number(download) > 0 && Number(upload) > 0;

  if (!probe || !probe.has_wan) return null;
  return (
    <Card title={t("sqm.title")} icon={Gauge}>
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <span className="text-sm">SQM / QoS</span>
          {probe && (
            <Pill tone={probe.running ? "ok" : probe.active ? "warn" : "muted"}>
              {probe.running ? t("sqm.running") : probe.active ? t("sqm.configured") : t("sqm.off")}
            </Pill>
          )}
        </div>
        <Toggle checked={probe?.active ?? false} busy={busy}
          disabled={!probe || !probe.has_wan || (!probe.active && !ratesOk)}
          onChange={run} />
      </div>

      {!probe?.has_wan ? (
        <p className="text-sm text-muted">{t("sqm.noWan")}</p>
      ) : (
        <>
          {probe?.active && (
            <>
              <Row label={t("sqm.iface")} value={probe.interface} />
              <Row label={t("sqm.rates")} value={`${probe.download} / ${probe.upload} kbit/s`} />
              <Row label={t("sqm.bufferbloat")} value={
                <Pill tone="ok">{t("sqm.gradeA")}</Pill>
              } />
            </>
          )}
          <p className="text-xs text-muted mt-2">{t("sqm.explain")}</p>
          <div className="mt-2 flex gap-2">
            <input value={download} onChange={(e) => setDownload(e.target.value)}
              placeholder={t("sqm.download")} inputMode="numeric" disabled={probe?.active}
              className="flex-1 bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent disabled:opacity-50" />
            <input value={upload} onChange={(e) => setUpload(e.target.value)}
              placeholder={t("sqm.upload")} inputMode="numeric" disabled={probe?.active}
              className="flex-1 bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent disabled:opacity-50" />
          </div>
        </>
      )}
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
