import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ShieldBan } from "lucide-react";
import { api } from "../../api";
import type { BanipStatus } from "../../types";
import { Button, Card, ConfirmDialog, Pill, SkeletonRows, Toggle, useToast } from "../ui";
import { TechName } from "./shared";

/**
 * banIP en Servicios (#351): switch que instala/activa y desinstala el
 * servicio, más atajo a su página. El switch refleja el estado completo
 * (instalado + activado + en marcha): si falta algo, OFF y al deslizarlo
 * se instala o se activa según corresponda.
 */
export function BanipCard({ index = 0, onNavigate }: { index?: number; onNavigate?: (p: string) => void }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [st, setSt] = useState<BanipStatus>();
  const [busy, setBusy] = useState(false);
  const [confirmInstall, setConfirmInstall] = useState(false);
  const [confirmUninstall, setConfirmUninstall] = useState(false);

  const load = useCallback(() => {
    api.banipStatus().then(setSt).catch(() => {});
  }, []);

  useEffect(() => { load(); }, [load]);

  if (!st) {
    return <Card index={index} icon={ShieldBan} title={t("banip.cardTitle")}><SkeletonRows rows={2} /></Card>;
  }
  // Solo aplica en el gateway (mismo criterio que la entrada del menú).
  if (!st.applicable) return null;

  const active = st.installed && st.enabled && st.running;

  const fail = (e: unknown) =>
    push({ tone: "danger", text: t("banip.cardActionFailed"), detail: e instanceof Error ? e.message : String(e) });

  // OFF -> ON
  const turnOn = async () => {
    setConfirmInstall(false);
    setBusy(true);
    try {
      if (!st.installed) {
        // Sincrónico y lento en mipsle (opkg update + install): await sin
        // timeout propio, con busy marcado.
        await api.banipInstall();
        push({ tone: "ok", text: t("banip.installOk") });
      } else if (!st.enabled) {
        await api.banipAction("enable");
        push({ tone: "ok", text: t("banip.actionOk") });
      } else if (!st.running) {
        await api.banipAction("start");
        push({ tone: "ok", text: t("banip.actionOk") });
      }
    } catch (e) {
      fail(e);
    } finally {
      setBusy(false);
      load();
    }
  };

  // ON -> OFF: solo tiene sentido cuando está instalado (deshabilitar se
  // hace desde la página; aquí el OFF desinstala, como pide el usuario).
  const turnOff = async () => {
    setConfirmUninstall(false);
    setBusy(true);
    try {
      await api.banipUninstall();
      push({ tone: "ok", text: t("banip.uninstallOk") });
    } catch (e) {
      fail(e);
    } finally {
      setBusy(false);
      load();
    }
  };

  const onToggle = (next: boolean) => {
    if (busy) return;
    if (next) {
      if (!st.installed) setConfirmInstall(true);
      else turnOn();
    } else if (st.installed) {
      setConfirmUninstall(true);
    }
  };

  return (
    <Card
      index={index}
      icon={ShieldBan}
      iconTone={active ? "success" : "muted"}
      title={t("banip.cardTitle")}
      action={onNavigate && (
        <Button variant="secondary" size="sm" onClick={() => onNavigate("banip")}>
          {t("banip.cardManage")}
        </Button>
      )}
    >
      <TechName>banIP</TechName>
      <div className="mt-2 flex items-center gap-3">
        <Toggle
          checked={active}
          disabled={busy}
          busy={busy}
          onChange={onToggle}
          label={t("banip.cardTitle")}
        />
        <Pill tone={!st.installed ? "muted" : active ? "ok" : "warn"} live={active}>
          {!st.installed ? t("banip.cardStateNotInstalled") : active ? t("banip.cardStateActive") : t("banip.cardStateInactive")}
        </Pill>
      </div>
      <p className="mt-2 text-caption text-muted">{t("banip.cardDesc")}</p>

      <ConfirmDialog
        open={confirmInstall}
        onClose={() => setConfirmInstall(false)}
        onConfirm={turnOn}
        title={t("banip.installConfirmTitle")}
        consequence={t("banip.installConfirmBody")}
        confirmLabel={t("banip.install")}
        busy={busy}
      />
      <ConfirmDialog
        open={confirmUninstall}
        onClose={() => setConfirmUninstall(false)}
        onConfirm={turnOff}
        title={t("banip.uninstallConfirmTitle")}
        consequence={t("banip.uninstallConfirmBody")}
        confirmLabel={t("banip.uninstall")}
        busy={busy}
      />
    </Card>
  );
}
