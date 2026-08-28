import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Gauge, Play } from "lucide-react";
import { api } from "../api";
import type { BufferbloatResult, SQMProbe } from "../types";
import { Card, Pill, Row } from "./Card";
import { Toggle } from "./Toggle";

function GradePill({ grade }: { grade: string }) {
  const tone = grade === "A" || grade === "B" ? "ok" : grade === "C" ? "warn" : "danger";
  return <Pill tone={tone}>{grade}</Pill>;
}

function Sparkline({ values }: { values: number[] }) {
  if (values.length < 2) return null;
  const w = 200;
  const h = 32;
  const pad = 2;
  const max = Math.max(...values, 1);
  const step = (w - pad * 2) / (values.length - 1);
  const points = values.map((v, i) => `${pad + i * step},${h - pad - (v / max) * (h - pad * 2)}`).join(" ");
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-full h-8 mt-1" preserveAspectRatio="none">
      <polyline points={points} fill="none" stroke="currentColor" strokeWidth="1.5" className="text-accent" />
    </svg>
  );
}

export function SqmCard({ probe, onChange }: {
  probe: SQMProbe | undefined;
  onChange: (p: SQMProbe) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [download, setDownload] = useState("");
  const [upload, setUpload] = useState("");
  const [latest, setLatest] = useState<BufferbloatResult>();
  const [history, setHistory] = useState<BufferbloatResult[]>([]);
  const [testing, setTesting] = useState(false);

  useEffect(() => {
    if (probe) {
      setDownload(probe.download || "");
      setUpload(probe.upload || "");
    }
  }, [probe]);

  useEffect(() => {
    api.bufferbloatHistory().then((r) => {
      setHistory(r.entries ?? []);
      const entries = r.entries ?? [];
      if (entries.length > 0) setLatest(entries[entries.length - 1]);
    }).catch(() => {});
  }, []);

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

  const runTest = async () => {
    setTesting(true);
    try {
      const result = await api.runBufferbloatTest();
      setLatest(result);
      setHistory((prev) => [...prev, result].slice(-50));
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("sqm.testFailed") });
    } finally {
      setTesting(false);
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

          <div className="mt-3 pt-3 border-t border-border/50">
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs font-medium uppercase tracking-wider text-muted">{t("sqm.bufferbloat")}</span>
              {latest ? (
                <GradePill grade={latest.grade} />
              ) : (
                <span className="text-xs text-muted">{t("sqm.notTested")}</span>
              )}
            </div>
            {latest && (
              <div className="grid grid-cols-3 gap-2 text-xs mb-1">
                <div><span className="text-muted">{t("sqm.baseline")}:</span> {latest.baseline_ms} ms</div>
                <div><span className="text-muted">{t("sqm.loaded")}:</span> {latest.loaded_ms} ms</div>
                <div><span className="text-muted">{t("sqm.delta")}:</span> {latest.delta_ms} ms</div>
              </div>
            )}
            <button
              onClick={runTest}
              disabled={testing}
              className="flex items-center gap-1 text-xs bg-accent/15 hover:bg-accent/25 text-accent px-2 py-1 rounded disabled:opacity-50 transition-colors"
            >
              <Play size={12} />
              {testing ? t("sqm.testing") : t("sqm.runTest")}
            </button>
            {history.length >= 2 && (
              <div className="mt-2">
                <span className="text-[10px] text-muted">{t("sqm.lastTests")}</span>
                <Sparkline values={history.slice(-10).map((h) => h.delta_ms)} />
              </div>
            )}
          </div>
        </>
      )}
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
