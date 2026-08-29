import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "../../api";
import type { NetPulseState } from "../../types";
import { Banner, StatusDot } from "../ui";

const STATUS_POLL_MS = 10_000;
const STANDALONE_DISMISS_KEY = "netpulse-standalone-dismissed";

/** relTime: "hace 3 min" / "3 min ago" a partir de un ts ISO (o null). */
export function relTime(iso: string | null, t: (k: string, opts?: Record<string, unknown>) => string): string {
  if (!iso) return t("netpulse.never");
  const secs = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 1000));
  if (secs < 60) return t("netpulse.relSec", { n: secs });
  const mins = Math.round(secs / 60);
  if (mins < 60) return t("netpulse.relMin", { n: mins });
  return t("netpulse.relHour", { n: Math.round(mins / 60) });
}

/** useNetPulseState: poll ligero del estado del agente (chip + banner). */
function useNetPulseState() {
  const [state, setState] = useState<NetPulseState>();
  const [, forceTick] = useState(0);

  useEffect(() => {
    const refresh = () => api.netpulse().then(setState).catch(() => {});
    refresh();
    const poll = setInterval(refresh, STATUS_POLL_MS);
    const tick = setInterval(() => forceTick((n) => n + 1), 30_000);
    return () => { clearInterval(poll); clearInterval(tick); };
  }, []);

  return state;
}

/**
 * NetPulseStandaloneBanner: aviso de standalone sustituido (#143), INDEPENDIENTE
 * de la NetPulseCard (que está oculta por diseño, #146). Dismissible; reaparece
 * solo si hay una detección NUEVA (timestamp posterior al descartado).
 */
export function NetPulseStandaloneBanner() {
  const { t } = useTranslation();
  const state = useNetPulseState();
  const [dismissed, setDismissed] = useState<string>(() =>
    localStorage.getItem(STANDALONE_DISMISS_KEY) ?? "",
  );

  const show =
    !!state?.standaloneReplacedAt &&
    new Date(state.standaloneReplacedAt).getTime() > new Date(dismissed || 0).getTime();

  if (!show) return null;

  const dismiss = () => {
    const now = new Date().toISOString();
    localStorage.setItem(STANDALONE_DISMISS_KEY, now);
    setDismissed(now);
  };

  return (
    <Banner tone="warn" className="mb-3" onDismiss={dismiss}>
      {t("netpulse.standaloneNotice")}
    </Banner>
  );
}

/**
 * NetPulseStatusChip: línea de estado discreta del agente always-on (#146):
 * conectado al server (con antigüedad del último push) o buscando server.
 */
export function NetPulseStatusChip() {
  const { t } = useTranslation();
  const state = useNetPulseState();

  const connected = state?.phase === "connected" && state.status?.pushOk;
  const text = connected
    ? t("netpulse.chipConnected", {
        server: state?.server || state?.discovery?.foundServer || "",
        time: relTime(state?.status?.lastPush ?? null, t),
      })
    : t("netpulse.chipSearching");

  return (
    <div className="flex items-center gap-2 min-w-0 animate-fade-up">
      <StatusDot tone={connected ? "ok" : "warn"} live={!connected} label={text} />
      <span className="text-small text-faint truncate">{text}</span>
    </div>
  );
}
