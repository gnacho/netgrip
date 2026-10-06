import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Wrench } from "lucide-react";
import { api, isDemo } from "../../api";
import { Card, ConfirmDialog, HelpTip, Toggle, useToast } from "../ui";

/**
 * #441: advanced mode toggle. The flag only gates visibility of the
 * Advanced section and its endpoints; enabling asks for an explicit
 * confirmation because the features behind it (VLAN, IGMP, storm control,
 * MAC ACL, and later SNMP and switch mirroring) can lock you out of the
 * network if misused.
 */
export function AdvancedCard({ index = 1 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [on, setOn] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (isDemo()) {
      setLoaded(true);
      return;
    }
    api.advanced().then((p) => { setOn(p.advanced); setLoaded(true); }).catch(() => {});
  }, []);

  const apply = async (enabled: boolean) => {
    setConfirm(false);
    setBusy(true);
    try {
      if (isDemo()) {
        setOn(enabled);
        return;
      }
      const r = await api.setAdvanced(enabled);
      setOn(r.state.advanced);
      if (r.state.advanced !== enabled) {
        push({ tone: "danger", text: t("advanced.toggleFailed") });
      }
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : t("advanced.toggleFailed") });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card index={index} title={t("advanced.toggleTitle")} icon={Wrench}>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <span className="flex items-center gap-2 text-body font-medium text-text">
          {t("advanced.toggleLabel")}
          <HelpTip title={t("advanced.toggleTitle")} body={t("advanced.toggleHint")} />
        </span>
        <div className="sm:shrink-0">
          <Toggle
            checked={on}
            disabled={!loaded || busy}
            onChange={(v) => (v ? setConfirm(true) : void apply(false))}
            label={t("advanced.toggleLabel")}
          />
        </div>
      </div>
      <ConfirmDialog
        open={confirm}
        onClose={() => setConfirm(false)}
        onConfirm={() => void apply(true)}
        title={t("advanced.confirmTitle")}
        consequence={t("advanced.confirmConsequence")}
        confirmLabel={t("advanced.confirmBtn")}
      />
    </Card>
  );
}
