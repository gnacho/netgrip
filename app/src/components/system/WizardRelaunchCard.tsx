import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ListRestart } from "lucide-react";
import { api, isDemo } from "../../api";
import { Button, Card, ConfirmDialog, useToast } from "../ui";

/**
 * #415: relaunch the first-run wizard on demand. Resetting the flag has no
 * side effects on its own: the panel reloads and the wizard gates on
 * wizardState().completed again. Steps read live probes, so an already
 * configured router shows its current state (no false "not installed").
 * In demo it reuses the ?wizard=1 preview path (App.tsx).
 */
export function WizardRelaunchCard({ index = 1 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [confirm, setConfirm] = useState(false);
  const [busy, setBusy] = useState(false);

  const relaunch = async () => {
    setConfirm(false);
    setBusy(true);
    try {
      if (isDemo()) {
        const url = new URL(window.location.href);
        url.searchParams.set("wizard", "1");
        window.location.href = url.toString();
        return;
      }
      await api.wizardReset();
      window.location.reload();
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : t("system.wizardFailed") });
      setBusy(false);
    }
  };

  return (
    <Card index={index} title={t("system.wizardTitle")} icon={ListRestart}>
      <p className="text-small text-muted">{t("system.wizardDesc")}</p>
      <div className="mt-4">
        <Button variant="secondary" icon={ListRestart} loading={busy} onClick={() => setConfirm(true)}>
          {t("system.wizardRelaunch")}
        </Button>
      </div>
      <ConfirmDialog
        open={confirm}
        onClose={() => setConfirm(false)}
        onConfirm={relaunch}
        title={t("system.wizardConfirmTitle")}
        consequence={t("system.wizardConsequence")}
        confirmLabel={t("system.wizardConfirmBtn")}
      />
    </Card>
  );
}
