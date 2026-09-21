import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Download, ExternalLink, Shield, Square } from "lucide-react";
import { api } from "../../api";
import type { DNSConfig } from "../../types";
import { Button, Card, ConfirmDialog, Pill, SkeletonRows, Toggle, useToast } from "../ui";
import { InstallProgress, useInstallJob } from "../wizard/common";
import { TechName } from "./shared";

/**
 * AdGuard Home (#359, #360): un único toggle de "protección DNS" que hace el
 * handoff completo. ON = paquete instalado, servicio en marcha y dnsmasq
 * entregándole toda la red (con backup y healthcheck, en el backend). OFF =
 * config DNS previa restaurada y servicio parado, sin desinstalar. El estado
 * intermedio "en marcha pero sin filtrado" lleva su propia pill.
 */
export function AdguardCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const { begin, running: installing, job } = useInstallJob();
  const [cfg, setCfg] = useState<DNSConfig>();
  const [busyWith, setBusyWith] = useState<"enable" | "disable" | null>(null);
  const busy = busyWith !== null;
  const [confirmEnable, setConfirmEnable] = useState(false);
  const [confirmDisable, setConfirmDisable] = useState(false);

  const load = () => api.dns().then(setCfg).catch(() => {});
  useEffect(() => { load(); }, []);

  if (!cfg) return <Card index={index} icon={Shield} iconTone="success" title={t("adguard.title")}><SkeletonRows rows={2} /></Card>;
  if (!cfg.applicable && !cfg.adguard_active) return null;

  const dashboardUrl = `http://${window.location.hostname}:3000`;
  const protection = cfg.adguard_protection;

  const doEnable = async () => {
    setConfirmEnable(false);
    setBusyWith("enable");
    try {
      if (!cfg.adguard_installed) {
        const j = await begin(() => api.wizardPackages(["adguard"]));
        if (j.phase !== "done") {
          push({ tone: "danger", text: j.error || t("adguard.installFailed") });
          return;
        }
      }
      const res = await api.adguardProtection(true);
      if (res.error) push({ tone: "danger", text: t("adguard.protectionFailed"), detail: res.error });
      else {
        if (res.state) setCfg(res.state);
        push({ tone: "ok", text: t("adguard.protectionEnabled") });
      }
    } catch (e) {
      push({ tone: "danger", text: t("adguard.protectionFailed"), detail: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusyWith(null);
      load();
    }
  };

  const doDisable = async () => {
    setConfirmDisable(false);
    setBusyWith("disable");
    try {
      const res = await api.adguardProtection(false);
      if (res.error) push({ tone: "danger", text: t("adguard.protectionFailed"), detail: res.error });
      else {
        if (res.state) setCfg(res.state);
        push({ tone: "ok", text: t("adguard.protectionDisabled") });
      }
    } catch (e) {
      push({ tone: "danger", text: t("adguard.protectionFailed"), detail: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusyWith(null);
      load();
    }
  };

  const stopService = async () => {
    setBusyWith("disable");
    try {
      const res = await api.adguardAction("stop");
      if (res.error) push({ tone: "danger", text: t("adguard.actionFailed"), detail: res.error });
      else {
        if (res.state) setCfg(res.state);
        load();
      }
    } finally {
      setBusyWith(null);
    }
  };

  return (
    <Card index={index} icon={Shield} iconTone="success" title={t("adguard.title")}>
      <TechName>AdGuard Home</TechName>

      {!cfg.adguard_installed ? (
        <div className="mt-3 flex flex-col gap-2">
          <p className="text-caption text-muted">{t("adguard.notInstalledDesc")}</p>
          <div>
            <Button variant="secondary" size="sm" loading={installing} onClick={() => setConfirmEnable(true)}>
              <Download size={14} aria-hidden="true" /> {t("adguard.install")}
            </Button>
          </div>
          <InstallProgress job={installing ? job : null} />
        </div>
      ) : (
        <>
          <div className="mt-3 flex items-center gap-2">
            {protection ? (
              <Pill tone="ok" live>{t("adguard.protectionOn")}</Pill>
            ) : cfg.adguard_running ? (
              <Pill tone="warn">{t("adguard.runningNoFilter")}</Pill>
            ) : (
              <Pill tone="muted">{t("adguard.stopped")}</Pill>
            )}
            {cfg.adguard_running && (
              <a
                href={dashboardUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 text-small text-accent hover:text-accent-hover ring-focus rounded-sm"
              >
                {t("adguard.openDashboard")}
                <ExternalLink size={12} aria-hidden="true" />
              </a>
            )}
            {cfg.adguard_running && !protection && (
              <Button variant="ghost" size="sm" className="text-danger hover:text-danger" disabled={busy} onClick={stopService}>
                <Square size={14} aria-hidden="true" /> {t("adguard.stop")}
              </Button>
            )}
          </div>

          <div className="mt-3 flex items-center gap-2">
            <Toggle
              checked={protection}
              busy={busy}
              onChange={(on) => (on ? setConfirmEnable(true) : setConfirmDisable(true))}
              label={t("adguard.protectionSwitch")}
            />
            <span className="text-small text-secondary">{t("adguard.protectionSwitch")}</span>
          </div>
          <p className="mt-2 text-caption text-muted">
            {protection
              ? t("adguard.protectionOnDesc")
              : cfg.adguard_has_backup
                ? t("adguard.protectionOffBackupDesc")
                : t("adguard.protectionOffDesc")}
          </p>
          {busyWith && (
            <div className="mt-3 flex flex-col gap-1.5" role="status">
              <span className="text-small text-muted">
                {busyWith === "enable" ? t("adguard.protectionEnabling") : t("adguard.protectionDisabling")}
              </span>
              <div aria-hidden="true" className="h-1.5 w-full overflow-hidden rounded-full bg-border">
                <div className="h-full w-1/3 rounded-full bg-accent" style={{ animation: "progress-slide 1.1s ease-in-out infinite" }} />
              </div>
            </div>
          )}
        </>
      )}

      <ConfirmDialog
        open={confirmEnable}
        onClose={() => setConfirmEnable(false)}
        onConfirm={doEnable}
        title={cfg.adguard_installed ? t("adguard.enableTitle") : t("adguard.enableInstallTitle")}
        consequence={cfg.adguard_installed ? t("adguard.enableBody") : t("adguard.enableInstallBody")}
        confirmLabel={cfg.adguard_installed ? t("adguard.enableConfirm") : t("adguard.install")}
        busy={busy}
      />
      <ConfirmDialog
        open={confirmDisable}
        onClose={() => setConfirmDisable(false)}
        onConfirm={doDisable}
        title={t("adguard.disableTitle")}
        consequence={t("adguard.disableBody")}
        confirmLabel={t("adguard.disableConfirm")}
        busy={busy}
      />
    </Card>
  );
}
