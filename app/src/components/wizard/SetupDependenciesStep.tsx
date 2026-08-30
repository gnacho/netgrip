import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Check, ChevronDown, ChevronUp, Info, ListChecks, Package, ShieldCheck,
  Sparkles, Wrench,
} from "lucide-react";
import { Banner, Pill } from "../ui";
import { IlluRouter } from "../ui/illustrations";
import { Reveal, StepFooter, StepShell } from "./common";

type InstallMode = "full" | "minimal" | "custom";

interface PkgGroup {
  id: string;
  title: string;
  pkgs: string[];
}

const REQUIRED: PkgGroup = {
  id: "required",
  title: "wizard.setup.required",
  pkgs: ["curl", "ca-certificates", "rpcd-mod-file"],
};

const RECOMMENDED: PkgGroup[] = [
  { id: "netpulse", title: "wizard.setup.netpulse", pkgs: ["tailscale", "wireguard-tools"] },
  { id: "diagnostics", title: "wizard.setup.diagnostics", pkgs: ["ethtool-full", "tcpdump-mini"] },
  { id: "extras", title: "wizard.setup.extras", pkgs: ["sqm-scripts", "nlbwmon"] },
];

/**
 * Mockup del paso inicial de preparación del router.
 * Ofrece tres niveles: recomendado, mínimo y personalizado.
 * Las acciones reales (apk/opkg) vendrán en la implementación final.
 */
export function SetupDependenciesStep({ onNext, onBack, onSkip }: {
  onNext: () => void;
  onBack: () => void;
  onSkip: () => void;
}) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<InstallMode>("full");
  const [showDetails, setShowDetails] = useState(false);
  const [busy] = useState(false); // reservado para la acción real

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

  const selectedPkgs = mode === "minimal"
    ? REQUIRED.pkgs
    : mode === "full"
      ? [...REQUIRED.pkgs, ...RECOMMENDED.flatMap((g) => g.pkgs)]
      : [];

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
            onNext={onNext}
            busy={busy}
            nextLabel={mode === "custom" ? t("wizard.setup.customize") : t("wizard.setup.installNow")}
            onSkip={onSkip}
          />
        </div>
      }
    >
      <div className="space-y-4">
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

        <button
          type="button"
          onClick={() => setShowDetails((s) => !s)}
          className="flex w-full items-center justify-between gap-2 rounded-lg border border-border bg-surface p-3 text-left text-small font-medium text-text hover:bg-surface-2 transition-colors"
        >
          <span className="flex items-center gap-2">
            <Package size={16} className="text-faint" />
            {mode === "custom"
              ? t("wizard.setup.seeCatalog")
              : t("wizard.setup.seeWhatInstalls", { count: selectedPkgs.length })}
          </span>
          {showDetails ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        </button>

        <Reveal open={showDetails}>
          <div className="space-y-3 rounded-xl border border-border bg-surface p-4">
            <PkgList group={REQUIRED} included={selectedPkgs} />
            {mode !== "minimal" && RECOMMENDED.map((g) => (
              <PkgList key={g.id} group={g} included={selectedPkgs} />
            ))}
            {mode === "custom" && (
              <div className="rounded-lg border border-dashed border-border bg-bg p-4 text-center text-small text-muted">
                {t("wizard.setup.customHint")}
              </div>
            )}
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

function PkgList({ group, included }: { group: PkgGroup; included: string[] }) {
  const { t } = useTranslation();
  return (
    <div>
      <h3 className="text-small font-medium text-text mb-2">{t(group.title)}</h3>
      <ul className="grid grid-cols-1 sm:grid-cols-2 gap-2">
        {group.pkgs.map((pkg) => {
          const checked = included.includes(pkg);
          return (
            <li
              key={pkg}
              className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-small
                ${checked ? "border-ok/30 bg-ok/5 text-text" : "border-border bg-surface-2 text-faint"}`}
            >
              <Check size={14} className={checked ? "text-ok" : "text-border-strong"} />
              <span className="font-mono">{pkg}</span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
