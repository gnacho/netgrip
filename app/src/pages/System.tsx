import type { ReactNode } from "react";
import type { Board, PkgUpgrade, UpdateCheck } from "../types";
import { UpdateCard } from "../components/system/UpdateCard";
import { SecurityCard } from "../components/system/SecurityCard";
import { RemoteAccessCard } from "../components/system/RemoteAccessCard";
import { AccessCard } from "../components/system/AccessCard";
import { TelegramCard } from "../components/system/TelegramCard";
import { ModeCard } from "../components/system/ModeCard";
import { IdentityCard } from "../components/system/IdentityCard";
import { useTranslation } from "react-i18next";

function GroupLabel({ children }: { children: ReactNode }) {
  return <p className="text-eyebrow text-faint mb-2 animate-fade-up">{children}</p>;
}

export function System({ board, update, onUpdateChange, packages: _packages, onPackagesChange, onLogout }: {
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
      {/* Mantenerse al día */}
      <section>
        <GroupLabel>{t("system.groupFresh")}</GroupLabel>
        <UpdateCard board={board} update={update} onChange={onUpdateChange} onPackagesChange={onPackagesChange} />
      </section>

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
        <ModeCard index={0} />
        <IdentityCard index={1} board={board} />
      </section>
    </div>
  );
}
