import type { ReactNode } from "react";
import type { Board, PkgUpgrade, UpdateCheck } from "../types";
import { SecurityCard } from "../components/system/SecurityCard";
import { RemoteAccessCard } from "../components/system/RemoteAccessCard";
import { AccessCard } from "../components/system/AccessCard";
import { TelegramCard } from "../components/system/TelegramCard";
import { ModeCard } from "../components/system/ModeCard";
import { IdentityCard } from "../components/system/IdentityCard";
import { NetPulseCard } from "../components/system/NetPulseCard";
import { NetPulseStandaloneBanner, NetPulseStatusChip } from "../components/system/NetPulseStatus";
import { useTranslation } from "react-i18next";

// Hidden by design (#146): zero-touch NetPulse integration. The embedded
// agent is always on and self-enrolls, so the manual configuration card has
// no place in the UI for now. Code (component, i18n keys, API) is kept
// compiled; flip this flag to bring the card back.
const NETPULSE_CARD_HIDDEN = true;

function GroupLabel({ children }: { children: ReactNode }) {
  return <p className="text-eyebrow text-faint mb-2 animate-fade-up">{children}</p>;
}

export function System({ board, update: _update, onUpdateChange: _onUpdateChange, packages: _packages, onPackagesChange: _onPackagesChange, onLogout }: {
  board: Board | undefined;
  update: UpdateCheck | undefined;
  onUpdateChange: (u: UpdateCheck) => void;
  packages: PkgUpgrade[] | undefined;
  onPackagesChange: (p: PkgUpgrade[]) => void;
  onLogout: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-6">
      {/* Protección */}
      <section className="flex flex-col gap-[var(--card-gap)]">
        <GroupLabel>{t("system.groupProtection")}</GroupLabel>
        <SecurityCard index={0} onLogout={onLogout} />
        <RemoteAccessCard index={1} />
        <AccessCard index={2} />
        <TelegramCard index={3} />
      </section>

      {/* Este equipo */}
      <section className="flex flex-col gap-[var(--card-gap)]">
        <GroupLabel>{t("system.groupDevice")}</GroupLabel>
        <NetPulseStandaloneBanner />
        <NetPulseStatusChip />
        <ModeCard index={0} />
        <IdentityCard index={1} board={board} />
        {!NETPULSE_CARD_HIDDEN && <NetPulseCard index={2} />}
      </section>
    </div>
  );
}
