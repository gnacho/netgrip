import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Activity, Globe, Plug, Route, Send } from "lucide-react";
import { api } from "../../api";
import type { DNSResult, PingResult, SelfTestResult, TCPResult, TracerouteResult } from "../../types";
import { Banner, Button, Card, Input, Pill, StatusDot } from "../ui";

/**
 * Diagnóstico de red (issue #305): cinco pruebas guiadas con formulario y
 * tabla de resultados. Cada tarjeta informa su resultado a la página vía
 * `onResult` para poder montar el resumen compartible.
 */

function ToolMissingBanner({ tool }: { tool: string }) {
  const { t } = useTranslation();
  return <Banner tone="warn" className="mt-3">{t("diagnostics.toolMissing", { tool })}</Banner>;
}

function ErrBanner({ error, onRetry }: { error?: string; onRetry: () => void }) {
  const { t } = useTranslation();
  if (!error) return null;
  return (
    <Banner tone="danger" className="mt-3"
      action={<Button variant="secondary" size="sm" onClick={onRetry}>{t("common.retry")}</Button>}>
      {error}
    </Banner>
  );
}

/** Autodiagnóstico (`/api/diagnostics/selftest`): resumen verde/rojo. */
export function SelfTestCard({ onResult }: { onResult: (r: SelfTestResult) => void }) {
  const { t } = useTranslation();
  const [result, setResult] = useState<SelfTestResult>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  const run = async () => {
    setBusy(true);
    setError(undefined);
    try {
      const r = await api.selftest();
      setResult(r);
      onResult(r);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally { setBusy(false); }
  };

  useEffect(() => { run(); /* eslint-disable-line react-hooks/exhaustive-deps */ }, []);

  const checks: { key: string; ok: boolean }[] = result
    ? [
        { key: t("diagnostics.gateway"), ok: result.gateway },
        { key: t("diagnostics.wan"), ok: result.wan },
        { key: t("diagnostics.dns"), ok: result.dns },
        { key: t("diagnostics.ntp"), ok: result.ntp },
      ]
    : [];

  const missingTools = result
    ? (["traceroute", "nslookup", "dig"] as const).filter((k) => !result.tools[k])
    : [];

  return (
    <Card title={t("diagnostics.selfTestTitle")} icon={Activity} iconTone="accent" help="diagnostics">
      <p className="text-small text-muted mb-3">{t("diagnostics.selfTestDesc")}</p>
      <Button onClick={run} loading={busy}>
        {busy ? t("diagnostics.selfTestRunning") : t("diagnostics.selfTestRun")}
      </Button>

      <ErrBanner error={error} onRetry={run} />

      {result && !error && (
        <div className="mt-3 flex flex-col gap-2">
          {checks.map((c) => (
            <div key={c.key} className="flex items-center gap-2">
              <StatusDot tone={c.ok ? "ok" : "danger"} label={c.ok ? t("diagnostics.ok") : t("diagnostics.fail")} />
              <span className="text-small">{c.key}</span>
              {c.key === t("diagnostics.ntp") && <span className="text-caption text-faint">{t("diagnostics.ntpApprox")}</span>}
            </div>
          ))}
          <div className="mt-1">
            <Pill tone={result.all_ok ? "ok" : "warn"}>
              {result.all_ok ? t("diagnostics.allGood") : t("diagnostics.someFail")}
            </Pill>
          </div>
          {missingTools.length > 0 && (
            <p className="text-caption text-faint">
              {t("diagnostics.missingTools", { tools: missingTools.join(", ") })}
            </p>
          )}
        </div>
      )}
    </Card>
  );
}

/** Ping (`POST /api/diagnostics` test=ping): latencia + pérdida. */
export function PingCard({ onResult }: { onResult: (r: PingResult) => void }) {
  const { t } = useTranslation();
  const [host, setHost] = useState("8.8.8.8");
  const [count, setCount] = useState("4");
  const [result, setResult] = useState<PingResult>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  const run = async () => {
    setBusy(true);
    setError(undefined);
    try {
      const r = await api.runDiagnostics("ping", { host: host.trim(), count: Number(count) || 4 }) as PingResult;
      setResult(r);
      onResult(r);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally { setBusy(false); }
  };

  return (
    <Card title={t("diagnostics.pingTitle")} icon={Send} iconTone="teal" help="diagnostics">
      <p className="text-small text-muted mb-3">{t("diagnostics.pingDesc")}</p>
      <div className="flex flex-wrap items-end gap-2">
        <label className="flex-1 min-w-40 block">
          <span className="mb-1 block text-caption font-medium text-muted">{t("diagnostics.host")}</span>
          <Input mono value={host} onChange={(e) => setHost(e.target.value)} placeholder="8.8.8.8" />
        </label>
        <label className="w-24 block">
          <span className="mb-1 block text-caption font-medium text-muted">{t("diagnostics.count")}</span>
          <Input mono value={count} onChange={(e) => setCount(e.target.value.replace(/\D/g, ""))} />
        </label>
        <Button onClick={run} loading={busy}>{t("diagnostics.run")}</Button>
      </div>

      <ErrBanner error={error} onRetry={run} />

      {result && !error && (
        result.missing_tool ? <ToolMissingBanner tool={result.missing_tool} /> : (
          <div className="mt-3 flex flex-col gap-2">
            <div className="flex flex-wrap gap-1.5">
              <Pill tone={result.loss_pct > 0 ? "warn" : "ok"}>
                {t("diagnostics.sentReceived")}: {result.sent}/{result.received}
              </Pill>
              <Pill tone={result.loss_pct > 0 ? "warn" : "ok"}>{t("diagnostics.loss")}: {result.loss_pct}%</Pill>
              <Pill tone="muted">
                {t("diagnostics.minMaxAvg")}: {result.min_ms}/{result.avg_ms}/{result.max_ms} ms
              </Pill>
            </div>
            {result.samples.length > 0 && (
              <table className="w-full text-small">
                <thead>
                  <tr className="text-caption text-faint">
                    <th className="text-left font-medium py-1">{t("diagnostics.seq")}</th>
                    <th className="text-right font-medium py-1">{t("diagnostics.latency")}</th>
                  </tr>
                </thead>
                <tbody>
                  {result.samples.map((s) => (
                    <tr key={s.seq} className="border-t border-border/60">
                      <td className="py-1 font-mono text-faint">{s.seq}</td>
                      <td className="py-1 text-right font-mono tabular-nums">{s.time_ms} ms</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )
      )}
    </Card>
  );
}

/** Traceroute (`POST /api/diagnostics` test=traceroute): tabla de saltos. */
export function TracerouteCard({ onResult }: { onResult: (r: TracerouteResult) => void }) {
  const { t } = useTranslation();
  const [host, setHost] = useState("8.8.8.8");
  const [result, setResult] = useState<TracerouteResult>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  const run = async () => {
    setBusy(true);
    setError(undefined);
    try {
      const r = await api.runDiagnostics("traceroute", { host: host.trim() }) as TracerouteResult;
      setResult(r);
      onResult(r);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally { setBusy(false); }
  };

  return (
    <Card title={t("diagnostics.tracerouteTitle")} icon={Route} iconTone="accent" help="diagnostics">
      <p className="text-small text-muted mb-3">{t("diagnostics.tracerouteDesc")}</p>
      <div className="flex flex-wrap items-end gap-2">
        <label className="flex-1 min-w-40 block">
          <span className="mb-1 block text-caption font-medium text-muted">{t("diagnostics.host")}</span>
          <Input mono value={host} onChange={(e) => setHost(e.target.value)} placeholder="8.8.8.8" />
        </label>
        <Button onClick={run} loading={busy}>{t("diagnostics.run")}</Button>
      </div>

      <ErrBanner error={error} onRetry={run} />

      {result && !error && (
        result.missing_tool ? <ToolMissingBanner tool={result.missing_tool} /> : (
          <table className="mt-3 w-full text-small">
            <thead>
              <tr className="text-caption text-faint">
                <th className="text-left font-medium py-1">{t("diagnostics.hop")}</th>
                <th className="text-left font-medium py-1">{t("diagnostics.host")}</th>
                <th className="text-right font-medium py-1">{t("diagnostics.latency")}</th>
              </tr>
            </thead>
            <tbody>
              {result.hops.map((h) => (
                <tr key={h.hop} className="border-t border-border/60">
                  <td className="py-1 font-mono text-faint">{h.hop}</td>
                  <td className="py-1 font-mono text-faint">{h.host}</td>
                  <td className="py-1 text-right font-mono tabular-nums">
                    {h.parsed ? h.rtts.map((x) => `${x} ms`).join(" ") : t("diagnostics.unanswered")}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )
      )}
    </Card>
  );
}

/** Consulta DNS (`POST /api/diagnostics` test=dns): lista de respuestas. */
export function DnsCard({ onResult }: { onResult: (r: DNSResult) => void }) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("example.com");
  const [resolver, setResolver] = useState("");
  const [result, setResult] = useState<DNSResult>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  const run = async () => {
    setBusy(true);
    setError(undefined);
    try {
      const r = await api.runDiagnostics("dns", { query: query.trim(), resolver: resolver.trim() || undefined }) as DNSResult;
      setResult(r);
      onResult(r);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally { setBusy(false); }
  };

  return (
    <Card title={t("diagnostics.dnsTitle")} icon={Globe} iconTone="accent" help="diagnostics">
      <p className="text-small text-muted mb-3">{t("diagnostics.dnsDesc")}</p>
      <div className="flex flex-wrap items-end gap-2">
        <label className="flex-1 min-w-40 block">
          <span className="mb-1 block text-caption font-medium text-muted">{t("diagnostics.query")}</span>
          <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="example.com" />
        </label>
        <label className="flex-1 min-w-40 block">
          <span className="mb-1 block text-caption font-medium text-muted">{t("diagnostics.resolver")}</span>
          <Input mono value={resolver} onChange={(e) => setResolver(e.target.value)} placeholder="1.1.1.1" />
        </label>
        <Button onClick={run} loading={busy}>{t("diagnostics.run")}</Button>
      </div>

      <ErrBanner error={error} onRetry={run} />

      {result && !error && (
        result.missing_tool ? <ToolMissingBanner tool={result.missing_tool} /> : (
          <div className="mt-3 flex flex-col gap-2">
            {result.answers.length === 0 ? (
              <Banner tone="info">{t("diagnostics.noAnswers")}</Banner>
            ) : (
              <table className="w-full text-small">
                <thead>
                  <tr className="text-caption text-faint">
                    <th className="text-left font-medium py-1">{t("diagnostics.query")}</th>
                    <th className="text-left font-medium py-1">{t("diagnostics.type")}</th>
                    <th className="text-right font-medium py-1">{t("diagnostics.value")}</th>
                  </tr>
                </thead>
                <tbody>
                  {result.answers.map((a, i) => (
                    <tr key={i} className="border-t border-border/60">
                      <td className="py-1 text-faint">{a.name}</td>
                      <td className="py-1 font-mono text-faint">{a.type ?? "-"}</td>
                      <td className="py-1 text-right font-mono tabular-nums">{a.value}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )
      )}
    </Card>
  );
}

/** Test de puerto TCP (`POST /api/diagnostics` test=tcp): open/closed. */
export function TcpCard({ onResult }: { onResult: (r: TCPResult) => void }) {
  const { t } = useTranslation();
  const [host, setHost] = useState("");
  const [port, setPort] = useState("443");
  const [result, setResult] = useState<TCPResult>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  const run = async () => {
    setBusy(true);
    setError(undefined);
    try {
      const r = await api.runDiagnostics("tcp", { host: host.trim(), port: Number(port) }) as TCPResult;
      setResult(r);
      onResult(r);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally { setBusy(false); }
  };

  return (
    <Card title={t("diagnostics.tcpTitle")} icon={Plug} iconTone="teal" help="diagnostics">
      <p className="text-small text-muted mb-3">{t("diagnostics.tcpDesc")}</p>
      <div className="flex flex-wrap items-end gap-2">
        <label className="flex-1 min-w-40 block">
          <span className="mb-1 block text-caption font-medium text-muted">{t("diagnostics.host")}</span>
          <Input mono value={host} onChange={(e) => setHost(e.target.value)} placeholder="example.com" />
        </label>
        <label className="w-24 block">
          <span className="mb-1 block text-caption font-medium text-muted">{t("diagnostics.port")}</span>
          <Input mono value={port} onChange={(e) => setPort(e.target.value.replace(/\D/g, ""))} />
        </label>
        <Button onClick={run} loading={busy}>{t("diagnostics.run")}</Button>
      </div>

      <ErrBanner error={error} onRetry={run} />

      {result && !error && (
        <div className="mt-3">
          <Pill tone={result.open ? "ok" : "danger"}>
            {result.host}:{result.port} - {result.open ? t("diagnostics.open") : t("diagnostics.closed")}
          </Pill>
          {!result.open && result.error && (
            <p className="mt-1.5 text-caption text-faint">{result.error}</p>
          )}
        </div>
      )}
    </Card>
  );
}
