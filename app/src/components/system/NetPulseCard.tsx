import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Activity, Server } from "lucide-react";
import { api } from "../../api";
import type { NetPulseState } from "../../types";
import { Button, Card, Field, SettingRow, StatusDot, useToast } from "../ui";

const STATUS_POLL_MS = 10_000;

/** relTime: "hace 3 min" / "3 min ago" a partir de un ts ISO (o null). */
function relTime(iso: string | null, t: (k: string, opts?: Record<string, unknown>) => string): string {
  if (!iso) return t("netpulse.never");
  const secs = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 1000));
  if (secs < 60) return t("netpulse.relSec", { n: secs });
  const mins = Math.round(secs / 60);
  if (mins < 60) return t("netpulse.relMin", { n: mins });
  return t("netpulse.relHour", { n: Math.round(mins / 60) });
}

export function NetPulseCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [state, setState] = useState<NetPulseState>();
  const [server, setServer] = useState("");
  const [slug, setSlug] = useState("");
  const [token, setToken] = useState("");
  const [enabled, setEnabled] = useState(false);
  const [saving, setSaving] = useState(false);
  const [, forceTick] = useState(0);

  const refresh = () => {
    api.netpulse().then((s) => {
      setState(s);
      setServer((prev) => (prev === "" ? s.server : prev));
      setSlug((prev) => (prev === "" ? s.slug : prev));
      setEnabled(s.enabled);
    }).catch(() => {});
  };

  useEffect(() => {
    refresh();
    const poll = setInterval(refresh, STATUS_POLL_MS);
    const tick = setInterval(() => forceTick((n) => n + 1), 30_000);
    return () => { clearInterval(poll); clearInterval(tick); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const save = async () => {
    setSaving(true);
    try {
      const s = await api.setNetPulse({ server, slug, token, enabled });
      setState(s);
      setToken("");
      push({ tone: "ok", text: t("netpulse.saved") });
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setSaving(false);
    }
  };

  const status = state?.status;
  const dotTone = !status || !status.running ? "muted" : status.pushOk ? "ok" : "danger";
  const statusText = !status || !status.running
    ? t("netpulse.idle")
    : status.pushOk
      ? t("netpulse.pushOk", { time: relTime(status.lastPush, t) })
      : t("netpulse.pushFail", { time: relTime(status.lastPush, t) });

  return (
    <Card index={index} title={t("netpulse.title")} icon={Activity}>
      <SettingRow
        title={t("netpulse.enabled")}
        description={t("netpulse.description")}
        checked={enabled}
        onChange={setEnabled}
      />

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-1">
        <Field
          label={t("netpulse.server")}
          icon={Server}
          inputProps={{
            mono: true, value: server, placeholder: "http://192.168.1.226:3000",
            onChange: (e) => setServer(e.target.value),
          }}
        />
        <Field
          label={t("netpulse.slug")}
          inputProps={{
            mono: true, value: slug, placeholder: "patio",
            onChange: (e) => setSlug(e.target.value),
          }}
        />
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-3">
        <Field
          label={t("netpulse.token")}
          hint={state?.configured ? t("netpulse.tokenSet") : undefined}
          inputProps={{
            type: "password", mono: true, value: token, autoComplete: "new-password",
            placeholder: state?.configured ? "········" : "64 hex",
            onChange: (e) => setToken(e.target.value),
          }}
        />
        <div className="flex items-end pb-1">
          <div className="flex items-center gap-2 min-w-0">
            <StatusDot tone={dotTone} live={!!status?.running} label={statusText} />
            <span className="text-small truncate">{statusText}</span>
          </div>
        </div>
      </div>
      {status?.lastError && (
        <p className="text-caption text-danger mt-2 truncate" title={status.lastError}>
          {status.lastError}
        </p>
      )}

      <div className="flex gap-2 mt-4">
        <Button onClick={save} loading={saving} disabled={!server || !slug || (enabled && !state?.configured && !token)}>
          {t("netpulse.save")}
        </Button>
      </div>
    </Card>
  );
}
