import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Lock, ShieldCheck } from "lucide-react";
import { api, isDemo } from "../../api";
import type { AccessProbe } from "../../types";
import { ActionBanner, Button, Card, ConfirmDialog, HelpTip, Input, SegmentedControl, SkeletonRows, Toggle, useToast } from "../ui";
import { useActionCycle } from "../wifi/action";

/** Which certificate the panel serves. "custom" is a pair somebody
 *  configured by hand; it is offered so that saving anything else in the card
 *  does not quietly rewrite the paths and throw that choice away. */
type CertSource = "panel" | "router" | "custom";

// durationToMin turns a Go time.Duration ("12h0m0s") into minutes.
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

/** Three separate services live in this card: the panel, LuCI and SSH.
 *  Each one saves on its own deliberately - with a single button, turning
 *  LuCI's "force HTTPS" off reapplied SSH as well and could fail with an
 *  error about dropbear, which is not what anybody had touched. */
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
  const [hasRouterCert, setHasRouterCert] = useState(false);
  // What is being served right now, which need not be what is configured:
  // if the configured pair is missing the panel serves another and says so.
  const [servingTls, setServingTls] = useState(false);
  const [serving, setServing] = useState<"" | "panel" | "router" | "custom">("");
  // What is already applied, so Save can tell whether anything really
  // changed: only then is a restart needed, taking the browser with it.
  const [appliedHttps, setAppliedHttps] = useState(false);
  const [appliedCert, setAppliedCert] = useState<CertSource>("panel");
  const [panelHttps, setPanelHttps] = useState(false);
  const [panelCert, setPanelCert] = useState<CertSource>("panel");
  const [confirmScheme, setConfirmScheme] = useState(false);

  const panel = useActionCycle();
  const luci = useActionCycle();
  const ssh = useActionCycle();

  const reload = () => {
    api.access().then((p) => {
      setProbe(p);
      setLuciHttp(p.luci.http_port);
      setLuciHttps(p.luci.https_port);
      setLuciForce(p.luci.force_https);
      setSshEnabled(p.ssh.enabled);
      setSshPort(p.ssh.port || "22");
      setTtlMin(durationToMin(p.panel.session_ttl));
    }).catch(() => {});
    api.httpsState().then((s) => {
      setHasCert(s.has_cert);
      setHasRouterCert(s.router_cert);
      setPanelHttps(s.enabled);
      setAppliedHttps(s.enabled);
      setPanelCert(s.cert);
      setAppliedCert(s.cert);
      setServingTls(s.serving);
      setServing(s.serving_source ?? "");
    }).catch(() => {});
  };
  useEffect(reload, []);

  const schemeTarget = (https: boolean) =>
    `${https ? "https" : "http"}://${window.location.host}${window.location.pathname}`;

  // Anything that restarts the panel is asked about first rather than done
  // underneath the person who pressed Save. A certificate change counts: it
  // restarts too, and it often lands the browser on an interstitial for a
  // certificate it has never been shown.
  const savePanel = () => {
    if (panelHttps !== appliedHttps || (panelHttps && panelCert !== appliedCert)) {
      setConfirmScheme(true);
      return;
    }
    applyPanel();
  };

  // Everything in the panel section is applied here, on Save. The HTTPS
  // switch used to apply on the toggle itself, which is exactly what a Save
  // button says will not happen.
  const applyPanel = () =>
    panel.run(async () => {
      await api.setPanelSessionTtl(ttlMin);
      const schemeChanged = panelHttps !== appliedHttps;
      const certChanged = panelHttps && panelCert !== appliedCert;
      if (!schemeChanged && !certChanged) return { status: "applied" as const };

      const res = await api.setPanelHttps(panelHttps, panelCert);
      setAppliedHttps(panelHttps);
      setAppliedCert(panelCert);
      // The server decides whether a restart is owed; it answers
      // "unchanged" when the running state already matches. Waiting for a
      // restart that was never scheduled would time out and report a
      // failure that did not happen.
      if (!res.restarting) return { status: "applied" as const };
      // Changing the certificate without changing the scheme restarts too,
      // but the URL does not move: waiting for it to come back is enough.
      push({ tone: "ok", text: t("access.panelHttpsRestarting") });
      const target = schemeTarget(panelHttps);
      if (!(await waitForRestart())) {
        // The restart never happened, so the old scheme is still the live
        // one. Following the new URL here would land on a port that is not
        // speaking it; say so and leave the tab where it works.
        push({ tone: "danger", text: t("access.panelHttpsNoRestart", { url: target }) });
        return { status: "applied" as const };
      }
      window.location.href = target;
      return { status: "applied" as const };
    }).then(() => reload());

  const saveLuci = () =>
    luci.run(() =>
      api.setLuciAccess({ http_port: luciHttp, https_port: luciHttps, force_https: luciForce, enabled: true }),
    ).then((res) => { if (res?.status === "applied") reload(); });

  const saveSsh = () =>
    ssh.run(() => api.setSshAccess({ enabled: sshEnabled, port: sshPort })).then((res) => {
      if (res?.status === "applied") reload();
    });

  // Changing the panel's own scheme means restarting it, so the tab that
  // asked is left on a URL that no longer answers.
  //
  // What can be observed from here is only that the OLD process goes away.
  // The new one cannot be asked whether it is up: on the old scheme it no
  // longer answers, and on the new one the request is cross-origin and, with
  // a self-signed certificate the browser has not been shown yet, fails for
  // a reason indistinguishable from "not up". So the old listener dying is
  // the signal, and it is the one that matters - if it never dies the
  // restart did not happen and following the new URL would land on a port
  // still speaking the old scheme.
  //
  // Returns whether the restart was actually observed. A fixed timer used to
  // stand in for this and fired early, so the browser asked for HTTP from a
  // listener that was still TLS: "Client sent an HTTP request to an HTTPS
  // server".
  const waitForRestart = async (): Promise<boolean> => {
    if (isDemo()) return true; // no process to wait for, and no /api/me either
    const deadline = Date.now() + 15000;
    while (Date.now() < deadline) {
      try {
        await fetch(`/api/me?probe=${Date.now()}`, { cache: "no-store" });
      } catch {
        // Gone. Now give the replacement time to bind before following it:
        // it has the whole of main() to get through - collectors, monitors,
        // the certificate - before it listens, and on a router that is
        // seconds. Navigating on the death signal alone lands on
        // connection-refused, which is the same error by another route.
        await new Promise((r) => setTimeout(r, 2500));
        return true;
      }
      await new Promise((r) => setTimeout(r, 300));
    }
    return false;
  };


  // The router's option appears only when that pair can be served: offering
  // one that saving would refuse is a promise the card cannot keep. The
  // current value is always among the options, or the control renders with
  // nothing selected and the card stops saying what is in use.
  const certOptions = [
    { value: "panel" as const, label: t("access.certOwn") },
    ...(hasRouterCert || panelCert === "router"
      ? [{ value: "router" as const, label: t("access.certRouter") }]
      : []),
    ...(panelCert === "custom" || appliedCert === "custom"
      ? [{ value: "custom" as const, label: t("access.certCustomOption") }]
      : []),
  ];

  // Chip (i) del certificado (#499): la explicación específica de la
  // opción elegida, más el estado del certificado propio si aplica.
  const certHintBody = [
    panelCert === "router"
      ? t("access.certRouterHint")
      : panelCert === "custom"
        ? t("access.certCustomHint")
        : t("access.certOwnHint"),
    ...(panelCert === "panel" ? [t(hasCert ? "access.ownCertReady" : "access.ownCertNone")] : []),
  ].join(" ");

  const numPort = (value: number, onChange: (v: number) => void, ariaLabel: string) => (
    <Input
      type="number" mono min={1} max={65535} value={value || ""} aria-label={ariaLabel}
      onChange={(e) => onChange(Number(e.target.value))}
      className="w-20"
    />
  );

  /** Sección de UNA fila (#500): título, controles en línea y Guardar. */
  const sectionRow = (title: string, controls: React.ReactNode, save: () => void, cycle: ReturnType<typeof useActionCycle>, disabled?: boolean) => (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border border-border/60 px-3 py-2.5">
      <span className="text-caption font-semibold uppercase tracking-wide text-muted w-full sm:w-48 shrink-0">{title}</span>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 flex-1 min-w-0">{controls}</div>
      <div className="flex items-center gap-3 shrink-0">
        <Button size="sm" onClick={save} loading={cycle.busy} disabled={disabled}>{t("access.save")}</Button>
        {cycle.phase && (
          <ActionBanner phase={cycle.phase} text={cycle.phase === "done" ? t("access.saved") : undefined}
            detail={cycle.detail} onDone={cycle.clear} />
        )}
      </div>
    </div>
  );

  return (
    <Card index={index} title={t("access.title")} icon={Lock} help="access">
      <ConfirmDialog
        open={confirmScheme}
        onClose={() => setConfirmScheme(false)}
        onConfirm={() => { setConfirmScheme(false); applyPanel(); }}
        title={t("access.panelSection")}
        consequence={t("access.panelHttpsConfirm", { url: schemeTarget(panelHttps) })}
        confirmLabel={t("access.save")}
      />
      {!probe ? (
        <SkeletonRows rows={3} />
      ) : (
        <div className="flex flex-col gap-2">
          {sectionRow(
            t("access.panelSection"),
            <>
              <span className="text-small text-muted whitespace-nowrap">{t("access.sessionTtl")}</span>
              <div className="flex items-center gap-1.5">
                <Input type="number" mono min={1} max={100000} value={ttlMin || ""}
                  aria-label={t("access.sessionTtl")}
                  onChange={(e) => setTtlMin(Number(e.target.value))} className="w-16" />
                <span className="text-small text-muted">{t("access.minutes")}</span>
              </div>
              <span className="text-border-strong" aria-hidden="true">·</span>
              <span className="text-small whitespace-nowrap">{t("access.panelHttps")}</span>
              <HelpTip title={t("access.panelHttps")} body={t("access.panelHttpsHint")} />
              <Toggle checked={panelHttps} onChange={setPanelHttps} label={t("access.panelHttps")} />
              {panelHttps && (
                <>
                  <span className="text-border-strong" aria-hidden="true">·</span>
                  <span className="text-small text-muted whitespace-nowrap">{t("access.certSource")}</span>
                  <HelpTip title={t("access.certSource")} body={certHintBody} />
                  <SegmentedControl
                    size="sm"
                    ariaLabel={t("access.certSource")}
                    value={panelCert}
                    onChange={(v) => setPanelCert(v)}
                    options={certOptions}
                  />
                </>
              )}
              <span className="flex items-center gap-1.5 min-w-0">
                <ShieldCheck size={14} className={servingTls ? "text-ok" : "text-faint"} aria-hidden="true" />
                <span className="text-caption text-muted truncate">
                  {servingTls
                    ? t("access.servingWith", {
                        // An unknown source still means TLS is on: say so
                        // rather than claiming plain HTTP, which is the one
                        // thing that must never be shown wrongly.
                        cert:
                          serving === "panel" ? t("access.certOwn")
                            : serving === "router" ? t("access.certRouter")
                              : t("access.certCustom"),
                      })
                    : t("access.servingPlain")}
                </span>
              </span>
            </>,
            savePanel, panel, ttlMin <= 0,
          )}

          {sectionRow(
            t("access.luciSection"),
            <>
              <span className="text-small text-muted whitespace-nowrap">{t("access.luciPorts")}</span>
              <div className="flex items-center gap-1.5">
                {numPort(luciHttp, setLuciHttp, t("access.luciHttp"))}
                <span className="text-small text-muted">/</span>
                {numPort(luciHttps, setLuciHttps, t("access.luciHttps"))}
              </div>
              <span className="text-border-strong" aria-hidden="true">·</span>
              <span className="text-small whitespace-nowrap">{t("access.forceHttps")}</span>
              <Toggle checked={luciForce} onChange={setLuciForce} label={t("access.forceHttps")} />
            </>,
            saveLuci, luci,
          )}

          {sectionRow(
            t("access.sshSection"),
            <>
              <span className="text-small whitespace-nowrap">{t("access.enableSsh")}</span>
              <HelpTip title={t("access.enableSsh")} body={t("access.sshHint")} />
              <Toggle checked={sshEnabled} onChange={setSshEnabled} label={t("access.enableSsh")} />
              <span className="text-border-strong" aria-hidden="true">·</span>
              <span className="text-small text-muted whitespace-nowrap">{t("access.sshPort")}</span>
              <Input type="number" mono min={1} max={65535} value={sshPort}
                aria-label={t("access.sshPort")} onChange={(e) => setSshPort(e.target.value)} className="w-20" />
            </>,
            saveSsh, ssh,
          )}
        </div>
      )}
    </Card>
  );
}
