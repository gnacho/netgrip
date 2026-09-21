import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Radio, Router } from "lucide-react";
import { api } from "../../api";
import type { ModeProbe, UsteerAP } from "../../types";
import { ActionBanner, Card, ConfirmDialog, Pill, SegmentedControl, SkeletonRows, useToast } from "../ui";
import { useActionCycle } from "../wifi/action";

type Mode = "router" | "ap";

export function ModeCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [mode, setMode] = useState<ModeProbe>();
  const [confirmTarget, setConfirmTarget] = useState<Mode>();
  const [usteerAps, setUsteerAps] = useState<UsteerAP[]>();
  const [usteerError, setUsteerError] = useState(false);
  const { phase, detail, busy, run, clear } = useActionCycle();

  useEffect(() => {
    api.mode().then(setMode).catch(() => {});
  }, []);

  useEffect(() => {
    api.usteer().then((r) => { setUsteerAps(r.aps); setUsteerError(false); }).catch(() => setUsteerError(true));
  }, []);

  const switchMode = (target: Mode) => {
    setConfirmTarget(undefined);
    run(() => api.setMode(target)).then((res) => {
      if (res?.status === "applied") {
        setMode(res.state);
        push({ tone: "ok", text: t(target === "ap" ? "mode.changedToastAp" : "mode.changedToastRouter") });
      }
    });
  };

  const isSwitch = mode?.hardware_class === "switch";

  // Repeater: usteer neighbors detected (peers only, not the local router).
  const neighborHosts = new Set<string>();
  for (const a of usteerAps ?? []) if (!a.local) neighborHosts.add(a.hostname || a.bssid);
  const neighborCount = neighborHosts.size;

  return (
    <Card index={index} title={t("mode.title")} icon={Router}>
      {!mode ? (
        <SkeletonRows rows={3} />
      ) : isSwitch ? (
        <>
          <Pill tone="muted">{t("mode.switch")}</Pill>
          <p className="text-small text-muted mt-2">{t("mode.switchInfo", { ports: mode.port_count })}</p>
        </>
      ) : (
        <div className="flex flex-col items-start gap-3">
          <SegmentedControl<Mode>
            ariaLabel={t("mode.title")}
            value={mode.mode}
            onChange={(v) => { if (v !== mode.mode && !busy) setConfirmTarget(v); }}
            size="lg"
            options={[
              { value: "router", label: <span className="inline-flex items-center gap-1.5"><Router size={16} aria-hidden="true" />{t("mode.router")}</span> },
              { value: "ap", label: <span className="inline-flex items-center gap-1.5"><Radio size={16} aria-hidden="true" />{t("mode.ap")}</span> },
            ]}
          />
          <p className="text-small text-muted">
            {mode.mode === "router" ? t("mode.descRouter") : t("mode.descAp")}
          </p>
          {/* pills técnicas faint: el admin las entiende, el familiar las ignora */}
          <div className="flex flex-wrap gap-1.5">
            <Pill tone="muted">dnsmasq · {mode.dnsmasq_on ? t("mode.on") : t("mode.off")}</Pill>
            <Pill tone="muted">firewall · {mode.firewall_on ? t("mode.on") : t("mode.off")}</Pill>
          </div>
          {mode.mode === "ap" && (
            <div className="flex flex-wrap items-center gap-1.5">
              {!usteerError && neighborCount > 0 ? (
                <Pill tone="ok">{t("mode.usteerActive", { count: neighborCount })}</Pill>
              ) : (
                <p className="text-small text-faint">{t("mode.usteerHint")}</p>
              )}
            </div>
          )}
          {phase && (
            <ActionBanner
              phase={phase}
              text={phase === "done" ? t("mode.changed") : undefined}
              detail={detail}
              onDone={clear}
            />
          )}
        </div>
      )}

      <ConfirmDialog
        open={confirmTarget !== undefined}
        onClose={() => setConfirmTarget(undefined)}
        onConfirm={() => confirmTarget && switchMode(confirmTarget)}
        title={t("mode.confirmTitle")}
        consequence={t(confirmTarget === "ap" ? "mode.confirmToAp" : "mode.confirmToRouter")}
        confirmLabel={t("mode.confirmBtn")}
      />
    </Card>
  );
}
