import { useEffect, useState, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import { Ban, CloudOff, RefreshCw, Repeat, RotateCcw, Stethoscope } from "lucide-react";
import { api } from "../../api";
import type { CableTestProbe, EthPort, LoopResult } from "../../types";
import { Banner, Button, Card, ConfirmDialog, EmptyState, Pill, StatusDot } from "../ui";

/**
 * Diagnóstico (tools.md §3): tres cards gemelas "pulsar y entender" —
 * explicación llana + botón de acción + resultado con semáforo.
 */

/** Test de cable (`/api/cable-test`). */
export function CableTestCard({ index = 1 }: { index?: number }) {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<CableTestProbe>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(false);

  const run = async () => {
    setBusy(true);
    setError(false);
    try { setProbe(await api.cableTest()); }
    catch { setError(true); }
    finally { setBusy(false); }
  };

  return (
    <Card index={index} title={t("tools.cableTestTitle")} icon={Stethoscope} iconTone="teal" help="cabletest">
      <p className="text-small text-muted mb-3">{t("tools.cableTestDesc")}</p>
      <Button onClick={run} loading={busy}>
        {busy ? t("tools.testing") : t("tools.cableTestRunAll")}
      </Button>

      {error && (
        <Banner tone="danger" className="mt-3"
          action={<Button variant="secondary" size="sm" onClick={run}>{t("common.retry")}</Button>}>
          {t("tools.loadError")}
        </Banner>
      )}

      {probe && !error && (
        probe.ports.length === 0 ? (
          <p className="text-small text-muted mt-3">{t("tools.cableNotApplicable")}</p>
        ) : (
          <div className="mt-3 flex flex-col gap-2">
            {probe.ports.map((p, i) => (
              <div key={p.port} style={{ "--i": i * 1.5 } as CSSProperties}
                className="animate-fade-up flex flex-wrap items-center gap-x-3 gap-y-1">
                <span className="w-12 shrink-0 font-mono text-small">{p.port}</span>
                {!p.supported ? (
                  <Pill tone="muted">{t("tools.cableUnsupported")}</Pill>
                ) : p.pair_status === "ok" ? (
                  <Pill tone="ok">{t("tools.cablePerfect")}{p.length ? ` · ${p.length}` : ""}</Pill>
                ) : (
                  <Pill tone={p.pair_status === "short" ? "danger" : "warn"}>
                    {t("tools.cableProblem")}{p.length ? ` · ${t("tools.cableFailsAt", { length: p.length })}` : ""}
                  </Pill>
                )}
              </div>
            ))}
          </div>
        )
      )}
    </Card>
  );
}

/** Bucles de red (`/api/loops` + bloqueo de boca). */
export function LoopsCard({ index = 2 }: { index?: number }) {
  const { t } = useTranslation();
  const [result, setResult] = useState<LoopResult>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(false);
  const [blockBusy, setBlockBusy] = useState<string>();
  const [blocked, setBlocked] = useState<Record<string, boolean>>({});
  const [confirmPort, setConfirmPort] = useState<string>();
  const [msg, setMsg] = useState<{ text: string; tone: "ok" | "danger" }>();

  const check = async () => {
    setBusy(true);
    setError(false);
    try { setResult(await api.loops()); }
    catch { setError(true); }
    finally { setBusy(false); }
  };

  const setBlock = async (iface: string, on: boolean) => {
    setBlockBusy(iface);
    setMsg(undefined);
    try {
      await api.blockPort(iface, on);
      setBlocked((prev) => ({ ...prev, [iface]: on }));
      setMsg({ text: on ? t("tools.portBlocked") : t("tools.portUnblocked"), tone: "ok" });
    } catch (e) {
      setMsg({ text: e instanceof Error ? e.message : String(e), tone: "danger" });
    } finally { setBlockBusy(undefined); setConfirmPort(undefined); }
  };

  return (
    <Card index={index} title={t("tools.loopsTitle")} icon={Repeat} iconTone="warn">
      <p className="text-small text-muted mb-3">{t("tools.loopsDesc")}</p>
      <Button onClick={check} loading={busy}>
        {busy ? t("tools.loopsSearching") : t("tools.loopsRun")}
      </Button>

      {error && (
        <Banner tone="danger" className="mt-3"
          action={<Button variant="secondary" size="sm" onClick={check}>{t("common.retry")}</Button>}>
          {t("tools.loadError")}
        </Banner>
      )}

      {result && !error && (
        <div className="mt-3 flex flex-col gap-2">
          {result.loops.length === 0 && !result.has_hub && (
            <Banner tone="ok">{t("tools.loopsClean")}</Banner>
          )}
          {result.has_hub && <Banner tone="warn">{t("tools.hubWarning")}</Banner>}
          {result.loops.map((l, i) => (
            <div key={l.mac} style={{ "--i": i * 1.5 } as CSSProperties}
              className="animate-fade-up rounded-md border border-border px-3 py-2">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-mono text-small">{l.mac}</span>
                <span className="text-caption text-muted">{l.ports.join(", ")}</span>
              </div>
              <div className="mt-1.5 flex flex-wrap gap-1.5">
                {l.ports.map((port) =>
                  blocked[port] ? (
                    <div key={port} className="flex items-center gap-1.5">
                      <Pill tone="muted">{t("tools.blockedPill", { port })}</Pill>
                      <Button variant="ghost" size="sm" icon={RotateCcw}
                        loading={blockBusy === port} onClick={() => setBlock(port, false)}>
                        {t("tools.loopUnblock")}
                      </Button>
                    </div>
                  ) : (
                    <Button key={port} variant="danger" size="sm" icon={Ban}
                      loading={blockBusy === port} onClick={() => setConfirmPort(port)}>
                      {t("tools.loopBlock", { port })}
                    </Button>
                  ),
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {msg && (
        <Banner tone={msg.tone} className="mt-3" onDismiss={() => setMsg(undefined)}>{msg.text}</Banner>
      )}

      <ConfirmDialog
        open={!!confirmPort}
        onClose={() => setConfirmPort(undefined)}
        onConfirm={() => setBlock(confirmPort!, true)}
        title={t("tools.loopBlockTitle", { port: confirmPort ?? "" })}
        consequence={t("tools.loopBlockConsequence")}
        confirmLabel={t("tools.loopBlockGo")}
        busy={!!blockBusy}
      />
    </Card>
  );
}

/** Reiniciar una boca (`/api/ports/bounce`). */
export function BounceCard({ ethports, index = 3 }: { ethports: EthPort[]; index?: number }) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState<string>();
  const [justBounced, setJustBounced] = useState<string>();
  const [failMsg, setFailMsg] = useState<string>();

  const wired = ethports.filter((p) => !p.wan && p.name.startsWith("lan"));

  useEffect(() => {
    if (!justBounced) return;
    const id = setTimeout(() => setJustBounced(undefined), 6000);
    return () => clearTimeout(id);
  }, [justBounced]);

  const bounce = async (iface: string) => {
    setBusy(iface);
    setFailMsg(undefined);
    try {
      await api.bouncePort(iface);
      setJustBounced(iface);
    } catch (e) {
      setFailMsg(e instanceof Error ? e.message : String(e));
    } finally { setBusy(undefined); }
  };

  if (wired.length === 0) return null;

  return (
    <Card index={index} title={t("tools.bounceTitle")} icon={RefreshCw} iconTone="muted">
      <p className="text-small text-muted mb-3">{t("tools.bounceDesc")}</p>
      <div className="flex flex-wrap gap-2">
        {wired.map((p) => (
          <div key={p.name} className="flex items-center gap-2">
            <Button variant="secondary" size="sm" loading={busy === p.name} onClick={() => bounce(p.name)}>
              <StatusDot tone={p.up ? "ok" : "muted"} label={p.up ? t("tools.linkUp") : t("tools.linkDown")} />
              {p.name}
            </Button>
            {justBounced === p.name && <Pill tone="ok">{t("tools.bouncedOk")}</Pill>}
          </div>
        ))}
      </div>
      {failMsg && (
        <Banner tone="danger" className="mt-3" onDismiss={() => setFailMsg(undefined)}>{failMsg}</Banner>
      )}
    </Card>
  );
}

/** Estado de error reutilizable dentro del cajón avanzado. */
export function CardLoadError({ onRetry }: { onRetry: () => void }) {
  const { t } = useTranslation();
  return (
    <EmptyState
      small
      illustration={<CloudOff size={24} />}
      title={t("tools.loadError")}
      action={<Button variant="secondary" size="sm" onClick={onRetry}>{t("common.retry")}</Button>}
    />
  );
}
