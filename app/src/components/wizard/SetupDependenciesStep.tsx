import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertTriangle, Check, ChevronDown, ChevronUp, Download, Info, ListChecks, Package, ShieldCheck,
  Sparkles, Wrench,
} from "lucide-react";
import { Banner, Pill } from "../ui";
import { IlluRouter } from "../ui/illustrations";
import type { WizardSetupGroup, WizardSetupProbe } from "../../types";
import { Reveal, StepFooter, StepShell } from "./common";

type InstallMode = "full" | "minimal" | "custom";

/**
 * Paso inicial de preparación del router.
 * Ofrece tres niveles: recomendado, mínimo y personalizado.
 */
export function SetupDependenciesStep({ probe, onInstall, onBack, onSkip }: {
  probe: WizardSetupProbe;
  onInstall: (mode: InstallMode, groups: string[]) => void;
  onBack: () => void;
  onSkip: () => void;
}) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<InstallMode>("full");
  const [showDetails, setShowDetails] = useState(false);
  const [busy, setBusy] = useState(false);
  const [installError, setInstallError] = useState<string | null>(null);

  const groupByID = (id: string) => probe.groups.find((g) => g.id === id);
  const selectedGroupIDs = mode === "minimal"
    ? ["core"]
    : mode === "full"
      ? probe.groups.map((g) => g.id)
      : [];
  const selectedPkgs = selectedGroupIDs.flatMap((id) => groupByID(id)?.packages ?? []);

  const modes: { id: InstallMode; icon: typeof Package; title: string; desc: string; pill?: string }[] = [
    {
      id: "full",
      icon: Sparkles,
      title: t("wizard.setup.fullTitle"),
      desc: t("wizard.setup.fullDesc"),
      pill: t("wizard.setup.recommended"),
    },
    {
      id: "minimal",
      icon: ShieldCheck,
      title: t("wizard.setup.minimalTitle"),
      desc: t("wizard.setup.minimalDesc"),
    },
    {
      id: "custom",
      icon: ListChecks,
      title: t("wizard.setup.customTitle"),
      desc: t("wizard.setup.customDesc"),
    },
  ];

  const install = async () => {
    setBusy(true);
    setInstallError(null);
    try {
      await onInstall(mode, selectedGroupIDs);
    } catch (e) {
      setInstallError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const allInstalled = selectedPkgs.length === 0;
  const nextLabel = mode === "custom"
    ? t("wizard.setup.customize")
    : allInstalled
      ? t("wizard.continue")
      : t("wizard.setup.installNow");

  return (
    <StepShell
      illustration={<IlluRouter size={140} />}
      title={t("wizard.setup.title")}
      body={t("wizard.setup.body")}
      footer={
        <div className="mt-8 space-y-4">
          <Banner tone="info">
            <span className="flex items-start gap-2">
              <Info size={16} className="mt-0.5 shrink-0" />
              <span>{t("wizard.setup.apkOrOpkg")}</span>
            </span>
          </Banner>
          <StepFooter
            onBack={onBack}
            onNext={install}
            busy={busy}
            nextDisabled={mode === "custom"}
            nextLabel={nextLabel}
            onSkip={onSkip}
          />
        </div>
      }
    >
      <div className="space-y-4">
        {installError && (
          <Banner tone="danger">
            <span className="flex items-start gap-2">
              <AlertTriangle size={16} className="mt-0.5 shrink-0" />
              <span>
                <span className="block font-medium">{t("wizard.setup.installFailed")}</span>
                <span className="mt-0.5 block font-mono text-caption break-all">{installError}</span>
              </span>
            </span>
          </Banner>
        )}
        <div className="grid grid-cols-1 gap-3" role="radiogroup" aria-label={t("wizard.setup.title")}>
          {modes.map((m) => {
            const active = mode === m.id;
            const Icon = m.icon;
            return (
              <button
                key={m.id}
                type="button"
                role="radio"
                aria-checked={active}
                onClick={() => setMode(m.id)}
                disabled={busy}
                className={`flex items-start gap-3 rounded-xl border p-4 text-left ring-focus transition-all duration-[var(--dur-fast)]
                  ${active
                    ? "border-accent bg-accent-soft shadow-sm"
                    : "border-border bg-surface hover:bg-surface-2"}`}
              >
                <span
                  aria-hidden="true"
                  className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border-2 transition-colors duration-200
                    ${active ? "border-accent" : "border-border-strong"}`}
                >
                  {active && <span className="h-2.5 w-2.5 rounded-full bg-accent" />}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="flex items-center gap-2 text-body font-medium">
                    <Icon size={18} className={active ? "text-accent" : "text-faint"} />
                    {m.title}
                    {m.pill && <Pill tone="accent">{m.pill}</Pill>}
                  </span>
                  <span className="mt-0.5 block text-small text-muted">{m.desc}</span>
                </span>
              </button>
            );
          })}
        </div>

        {mode !== "custom" && (
          <button
            type="button"
            onClick={() => setShowDetails((s) => !s)}
            className="flex w-full items-center justify-between gap-2 rounded-lg border border-border bg-surface p-3 text-left text-small font-medium text-text hover:bg-surface-2 transition-colors"
          >
            <span className="flex items-center gap-2">
              <Package size={16} className="text-faint" />
              {allInstalled
                ? t("wizard.continue")
                : t("wizard.setup.seeWhatInstalls", { count: selectedPkgs.length })}
            </span>
            {showDetails ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
          </button>
        )}

        <Reveal open={showDetails}>
          <div className="space-y-3 rounded-xl border border-border bg-surface p-4">
            {selectedGroupIDs.map((id) => {
              const g = groupByID(id);
              if (!g) return null;
              return <PkgList key={id} group={g} manager={probe.manager} />;
            })}
          </div>
        </Reveal>

        <div className="flex items-start gap-2 rounded-lg border border-warn/30 bg-warn/5 p-3 text-small text-muted">
          <Wrench size={16} className="mt-0.5 shrink-0 text-warn" />
          <span>{t("wizard.setup.canInstallLater")}</span>
        </div>
      </div>
    </StepShell>
  );
}

function PkgList({ group, manager }: { group: WizardSetupGroup; manager: string }) {
  const { t } = useTranslation();
  const command = manager === "apk" ? `apk add ${group.packages.join(" ")}` : `opkg install ${group.packages.join(" ")}`;
  return (
    <div>
      <h3 className="text-small font-medium text-text mb-2">{t(group.title_key)}</h3>
      {group.packages.length === 0 ? (
        <p className="text-small text-ok flex items-center gap-1.5">
          <Check size={14} />
          {t("wizard.packages.installed")}
        </p>
      ) : (
        <>
          <ul className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            {group.packages.map((pkg) => (
              <li
                key={pkg}
                className="flex items-center gap-2 rounded-lg border border-ok/30 bg-ok/5 px-3 py-2 text-small text-text"
              >
                <Download size={14} className="text-ok" />
                <span className="font-mono">{pkg}</span>
              </li>
            ))}
          </ul>
          <p className="mt-2 font-mono text-caption text-faint">{command}</p>
        </>
      )}
    </div>
  );
}
