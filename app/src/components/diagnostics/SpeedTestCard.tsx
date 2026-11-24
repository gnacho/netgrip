import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowDown, ArrowUp, Gauge } from "lucide-react";
import { api } from "../../api";
import type { SpeedTestResult } from "../../types";
import { Button, Pill, SkeletonRows } from "../ui";

/**
 * Test de velocidad M-Lab NDT7 (#637), el mismo motor que NetPulse.
 *
 * Se ejecuta en el router, así que mide la línea del operador y no el WiFi del
 * móvil que abre el panel. Cada fase dura unos 10 s: el botón avisa antes de
 * gastar 20 s de ancho de banda.
 */
export function SpeedTestCard() {
  const { t } = useTranslation();
  const [state, setState] = useState<SpeedTestResult>();

  const load = useCallback(() => {
    api.speedtest().then(setState).catch(() => setState({ state: "error", down_mbps: 0, up_mbps: 0, error: t("common.loadError") } as SpeedTestResult));
  }, [t]);

  useEffect(() => { load(); }, [load]);

  // Mientras corre, se pregunta cada 2 s hasta que termina.
  useEffect(() => {
    if (state?.state !== "running") return;
    const id = setInterval(load, 2000);
    return () => clearInterval(id);
  }, [state?.state, load]);

  const start = async () => {
    try {
      setState(await api.startSpeedtest());
    } catch (e) {
      setState({ state: "error", down_mbps: 0, up_mbps: 0, error: e instanceof Error ? e.message : String(e) });
    }
  };

  const running = state?.state === "running";
  const mbps = (v: number) => (v >= 100 ? v.toFixed(0) : v.toFixed(1));

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <span className="text-small font-medium">{t("diagnostics.speedTitle")}</span>
        <Button size="sm" icon={Gauge} loading={running} disabled={running} onClick={start}>
          {running ? t("diagnostics.speedRunning") : t("diagnostics.speedStart")}
        </Button>
      </div>

      {!state ? (
        <SkeletonRows rows={1} />
      ) : state.state === "running" ? (
        <p className="text-small text-muted" role="status">
          {state.phase === "upload" ? t("diagnostics.speedUploading") : t("diagnostics.speedDownloading")}
        </p>
      ) : state.state === "error" ? (
        <p className="text-small text-danger">{state.error || t("diagnostics.speedFailed")}</p>
      ) : state.state === "done" ? (
        <>
          <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
            <span className="inline-flex items-center gap-1.5 text-ok">
              <ArrowDown size={16} aria-hidden="true" />
              <span className="text-h2 font-semibold" style={{ fontVariantNumeric: "tabular-nums" }}>{mbps(state.down_mbps)}</span>
              <span className="text-small">Mbps</span>
            </span>
            <span className="inline-flex items-center gap-1.5 text-accent">
              <ArrowUp size={16} aria-hidden="true" />
              <span className="text-h2 font-semibold" style={{ fontVariantNumeric: "tabular-nums" }}>{mbps(state.up_mbps)}</span>
              <span className="text-small">Mbps</span>
            </span>
            {state.ping_ms ? (
              <span className="inline-flex items-center gap-1.5 text-small text-muted">
                {t("diagnostics.speedPing")}
                <span className="font-mono text-body text-text">{state.ping_ms.toFixed(1)} ms</span>
              </span>
            ) : null}
            {state.server && <Pill tone="muted">{state.server}</Pill>}
          </div>
          <p className="text-caption text-faint">{t("diagnostics.speedNote")}</p>
        </>
      ) : (
        <p className="text-small text-muted">{t("diagnostics.speedIdle")}</p>
      )}
    </div>
  );
}
