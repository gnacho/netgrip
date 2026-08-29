import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Lock, ShieldCheck } from "lucide-react";
import { api } from "../../api";
import type { AccessProbe } from "../../types";
import { ActionBanner, Button, Card, Input, SkeletonRows, Toggle, useToast } from "../ui";
import { useActionCycle } from "../wifi/action";

// durationToMin convierte un time.Duration de Go ("12h0m0s") a minutos.
function durationToMin(d: string): number {
  if (!d) return 720;
  let total = 0;
  const re = /(\d+)([hms])/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(d)) !== null) {
    const n = Number(m[1]);
    if (m[2] === "h") total += n * 60;
    else if (m[2] === "m") total += n;
    else total += n / 60;
  }
  return Math.round(total) || 720;
}

export function AccessCard({ index = 2 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [probe, setProbe] = useState<AccessProbe>();
  const [luciHttp, setLuciHttp] = useState(80);
  const [luciHttps, setLuciHttps] = useState(443);
  const [luciForce, setLuciForce] = useState(false);
  const [sshEnabled, setSshEnabled] = useState(true);
  const [sshPort, setSshPort] = useState("22");
  const [ttlMin, setTtlMin] = useState(720);
  const [hasCert, setHasCert] = useState(false);
  const [certBusy, setCertBusy] = useState(false);
  const { phase, detail, busy, run, clear } = useActionCycle();

  useEffect(() => {
    api.access().then((p) => {
      setProbe(p);
      setLuciHttp(p.luci.http_port);
      setLuciHttps(p.luci.https_port);
      setLuciForce(p.luci.force_https);
      setSshEnabled(p.ssh.enabled);
      setSshPort(p.ssh.port || "22");
      setTtlMin(durationToMin(p.panel.session_ttl));
    }).catch(() => {});
    api.httpsState().then((s) => setHasCert(s.has_cert)).catch(() => {});
  }, []);

  const saveAll = () => {
    run(async () => {
      await api.setPanelSessionTtl(ttlMin);
      const results = await Promise.all([
        api.setLuciAccess({ http_port: luciHttp, https_port: luciHttps, force_https: luciForce, enabled: true }),
        api.setSshAccess({ enabled: sshEnabled, port: sshPort }),
      ]);
      const failed = results.find((r) => r.status !== "applied");
      return failed ?? { status: "applied" as const };
    }).then((res) => {
      if (res?.status === "applied") api.access().then(setProbe).catch(() => {});
    });
  };

  const genCert = async () => {
    setCertBusy(true);
    try {
      await api.enableHttps();
      setHasCert(true);
      push({ tone: "ok", text: t("access.httpsGenerated") });
    } catch (err) {
      push({ tone: "danger", text: err instanceof Error ? err.message : String(err) });
    } finally {
      setCertBusy(false);
    }
  };

  const numPort = (value: number, onChange: (v: number) => void, ariaLabel: string) => (
    <Input
      type="number" mono min={1} max={65535} value={value || ""} aria-label={ariaLabel}
      onChange={(e) => onChange(Number(e.target.value))}
      className="w-24"
    />
  );

  return (
    <Card index={index} title={t("access.title")} icon={Lock}>
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : (
        <div className="flex flex-col gap-3">
          <p className="text-caption text-muted">{t("access.disclaimer")}</p>

          <div className="grid gap-x-6 gap-y-1 sm:grid-cols-2">
            <div className="flex items-center gap-3 py-2 border-b border-border/60">
              <span className="text-body font-medium flex-1 min-w-0">{t("access.sessionTtl")}</span>
              <div className="flex items-center gap-1.5 shrink-0">
                <Input type="number" mono min={1} max={100000} value={ttlMin || ""}
                  onChange={(e) => setTtlMin(Number(e.target.value))} className="w-20" />
                <span className="text-small text-muted">{t("access.minutes")}</span>
              </div>
            </div>

            <div className="flex items-center gap-3 py-2 border-b border-border/60">
              <span className="text-body font-medium flex-1 min-w-0">{t("access.luciPorts")}</span>
              <div className="flex items-center gap-1.5 shrink-0">
                {numPort(luciHttp, setLuciHttp, t("access.luciHttp"))}
                <span className="text-muted text-caption">/</span>
                {numPort(luciHttps, setLuciHttps, t("access.luciHttps"))}
              </div>
            </div>

            <div className="flex items-center justify-between gap-3 py-2 border-b border-border/60">
              <span className="text-body font-medium">{t("access.forceHttps")}</span>
              <Toggle checked={luciForce} onChange={setLuciForce} label={t("access.forceHttps")} />
            </div>

            <div className="flex items-center justify-between gap-3 py-2 border-b border-border/60">
              <div className="min-w-0">
                <span className="text-body font-medium">{t("access.enableSsh")}</span>
                <p className="text-caption text-muted">{t("access.sshHint")}</p>
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                <Input type="number" mono min={1} max={65535} value={sshPort} disabled={!sshEnabled}
                  aria-label={t("access.sshPort")} onChange={(e) => setSshPort(e.target.value)} className="w-20" />
                <Toggle checked={sshEnabled} onChange={setSshEnabled} label={t("access.enableSsh")} />
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2 pt-1">
            <ShieldCheck size={14} className={hasCert ? "text-ok" : "text-faint"} aria-hidden="true" />
            <span className="text-small flex-1">{hasCert ? t("access.httpsReady") : t("access.httpsNone")}</span>
            {!hasCert && (
              <Button variant="secondary" size="sm" onClick={genCert} loading={certBusy}>
                {t("access.httpsGenerate")}
              </Button>
            )}
          </div>

          <div className="flex items-center gap-3">
            <Button onClick={saveAll} loading={busy} disabled={ttlMin <= 0}>{t("access.save")}</Button>
          </div>
          {phase && (
            <ActionBanner phase={phase} text={phase === "done" ? t("access.saved") : undefined} detail={detail} onDone={clear} />
          )}
        </div>
      )}
    </Card>
  );
}
