import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Info, RotateCcw, Star } from "lucide-react";
import { api, isDemo } from "../../api";
import type { Board, SelfUpdateCheck } from "../../types";
import { Banner, Button, Card, ConfirmDialog, KeyValue, useToast } from "../ui";
import { WaitOverlay, useCountdown, useMeBack } from "./WaitOverlay";

// El repo es lo único que la interfaz sigue enlazando tras retirar Acerca
// de (#681).
const STARGAZERS_URL = "https://github.com/gnacho/netgrip/stargazers";

const REBOOT_SECONDS = 60;
const MIN_ELAPSED_MS = 6000;

/**
 * POST /api/reboot directo: el endpoint existe en el backend (system.md)
 * pero no tiene binding en api.ts, que no podemos modificar. En demo se
 * simula (solo cuenta atrás + sondeo).
 */
async function rebootRouter(): Promise<void> {
  if (isDemo()) return;
  await fetch("/api/reboot", { method: "POST" }).then((r) => {
    if (!r.ok) throw new Error(`HTTP ${r.status}`);
  });
}

export function IdentityCard({ board, index = 1 }: { board: Board | undefined; index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [confirm, setConfirm] = useState(false);
  const [phase, setPhase] = useState<"idle" | "overlay" | "background">("idle");
  const [starting, setStarting] = useState(false);
  const [selfUpdate, setSelfUpdate] = useState<SelfUpdateCheck>();

  useEffect(() => {
    api.selfUpdateCheck().then(setSelfUpdate).catch(() => {});
  }, []);

  const left = useCountdown(phase !== "idle", REBOOT_SECONDS);

  useMeBack(phase !== "idle", MIN_ELAPSED_MS, () => {
    // el router volvió: recarga automática
    window.location.reload();
  });

  const reboot = async () => {
    setConfirm(false);
    setStarting(true);
    try {
      await rebootRouter();
      setPhase("overlay");
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : t("system.rebootFailed") });
    } finally {
      setStarting(false);
    }
  };

  return (
    <Card index={index} title={t("system.identityTitle")} icon={Info}>
      <div className="grid gap-x-6 sm:grid-cols-2">
        <KeyValue items={[
          { label: t("system.model"), value: board?.model ?? "…" },
          { label: t("system.firmware"), value: board?.release ? `${board.release.distribution} ${board.release.version} (${board.release.revision})` : "…" },
          { label: t("system.kernel"), value: board?.kernel ?? "…", mono: true },
        ]} />
        <div>
          <KeyValue items={[
            { label: t("system.hostname"), value: board?.hostname ?? "…", mono: true },
            { label: "NetGrip", value: selfUpdate?.current ?? "…", mono: true },
          ]} />
          {/* #723: la estrella vive bajo la versión del producto, no en la
              fila del reinicio. */}
          <a
            href={STARGAZERS_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-2 inline-flex items-center gap-1 text-small text-accent hover:text-accent-hover ring-focus rounded-sm"
          >
            <Star size={14} aria-hidden="true" />
            {t("system.starCta")}
          </a>
        </div>
      </div>

      <div className="mt-4 flex flex-wrap items-center gap-x-4 gap-y-2">
        <Button variant="secondary" icon={RotateCcw} loading={starting}
          className="text-danger border-danger/40 hover:bg-danger-soft"
          onClick={() => setConfirm(true)}>
          {t("system.reboot")}
        </Button>
      </div>

      {phase === "background" && (
        <Banner tone="info" className="mt-3">{t("system.rebootBanner")}</Banner>
      )}

      <ConfirmDialog
        open={confirm}
        onClose={() => setConfirm(false)}
        onConfirm={reboot}
        title={t("system.rebootConfirmTitle")}
        consequence={t("system.rebootConsequence")}
        confirmLabel={t("system.rebootConfirm")}
      />

      <WaitOverlay
        open={phase === "overlay"}
        title={t("system.rebootOverlayTitle")}
        subtitle={t("system.rebootOverlayBody")}
        countdown={left}
        onBackground={() => setPhase("background")}
      />
    </Card>
  );
}
