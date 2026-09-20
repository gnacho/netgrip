import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Download, ExternalLink, Play, Shield, Square } from "lucide-react";
import { api } from "../../api";
import type { DNSConfig } from "../../types";
import { Button, Card, ConfirmDialog, Pill, SkeletonRows, useToast } from "../ui";
import { InstallProgress, useInstallJob } from "../wizard/common";
import { TechName } from "./shared";

/**
 * AdGuard Home (#359): instalación desde el catálogo opcional, arranque y
 * parada del servicio, y atajo a la config DNS (dnsmasq) cuando la
 * integración no está activa. La escritura dnsmasq sigue viviendo en la
 * página Red local; aquí solo se enlaza.
 */
export function AdguardCard({ index = 0, onNavigate }: { index?: number; onNavigate?: (p: string) => void }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const { begin, running: installing, job } = useInstallJob();
  const [cfg, setCfg] = useState<DNSConfig>();
  const [busy, setBusy] = useState(false);
  const [confirmInstall, setConfirmInstall] = useState(false);

  const load = () => api.dns().then(setCfg).catch(() => {});
  useEffect(() => { load(); }, []);

  if (!cfg) return <Card index={index} icon={Shield} iconTone="success" title={t("adguard.title")}><SkeletonRows rows={2} /></Card>;
  if (!cfg.applicable && !cfg.adguard_active) return null;

  const dashboardUrl = `http://${window.location.hostname}:3000`;

  const install = async () => {
    setConfirmInstall(false);
    const j = await begin(() => api.wizardPackages(["adguard"]));
    if (j.phase === "done") {
      push({ tone: "ok", text: t("adguard.installOk") });
      load();
    } else {
      push({ tone: "danger", text: j.error || t("adguard.installFailed") });
    }
  };

  const action = async (a: "start" | "stop") => {
    setBusy(true);
    try {
      const res = await api.adguardAction(a);
      if (res.error) push({ tone: "danger", text: t("adguard.actionFailed"), detail: res.error });
      else {
        setCfg(res.state);
        push({ tone: "ok", text: t("adguard.actionOk") });
      }
    } catch (e) {
      push({ tone: "danger", text: t("adguard.actionFailed"), detail: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
      load();
    }
  };

  return (
    <Card index={index} icon={Shield} iconTone="success" title={t("adguard.title")}>
      <TechName>AdGuard Home</TechName>

      {!cfg.adguard_installed ? (
        <div className="mt-3 flex flex-col gap-2">
          <p className="text-caption text-muted">{t("adguard.notInstalledDesc")}</p>
          <div>
            <Button variant="primary" size="sm" loading={installing} onClick={() => setConfirmInstall(true)}>
              <Download size={14} aria-hidden="true" /> {t("adguard.install")}
            </Button>
          </div>
          <InstallProgress job={installing ? job : null} />
        </div>
      ) : (
        <>
          <div className="mt-3 flex items-center gap-2">
            {cfg.adguard_running ? (
              <>
                <Pill tone="ok" live>{t("adguard.running")}</Pill>
                <a
                  href={dashboardUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 text-small text-accent hover:text-accent-hover ring-focus rounded-sm"
                >
                  {t("adguard.openDashboard")}
                  <ExternalLink size={12} aria-hidden="true" />
                </a>
              </>
            ) : (
              <>
                <Pill tone="muted">{t("adguard.stopped")}</Pill>
                <Button variant="secondary" size="sm" disabled={busy} onClick={() => action("start")}>
                  <Play size={14} aria-hidden="true" /> {t("adguard.start")}
                </Button>
              </>
            )}
            {cfg.adguard_running && (
              <Button variant="ghost" size="sm" className="text-danger hover:text-danger" disabled={busy} onClick={() => action("stop")}>
                <Square size={14} aria-hidden="true" /> {t("adguard.stop")}
              </Button>
            )}
          </div>

          {/* Integración dnsmasq: si el DNS no apunta a AdGuard, atajo a la config */}
          {!cfg.adguard_active && (
            <div className="mt-3 flex items-center gap-2">
              <p className="text-caption text-muted flex-1">{t("adguard.dnsNotActive")}</p>
              {onNavigate && (
                <Button variant="secondary" size="sm" onClick={() => onNavigate("lan")}>
                  {t("adguard.configureDns")}
                </Button>
              )}
            </div>
          )}
        </>
      )}

      <ConfirmDialog
        open={confirmInstall}
        onClose={() => setConfirmInstall(false)}
        onConfirm={install}
        title={t("adguard.installConfirmTitle")}
        consequence={t("adguard.installConfirmBody")}
        confirmLabel={t("adguard.install")}
        busy={installing}
      />
    </Card>
  );
}
