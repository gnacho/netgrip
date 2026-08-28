import type { Board, PkgUpgrade, UpdateCheck } from "../types";
import { UpdateCard } from "../components/UpdateCard";
import { SecurityCard } from "../components/SecurityCard";
import { PackagesCard } from "../components/PackagesCard";
import { ModeCard } from "../components/ModeCard";
import { AccessCard } from "../components/AccessCard";
import { RemoteAccessCard } from "../components/RemoteAccessCard";
import { OffloadCard } from "../components/OffloadCard";
import { SelfUpdateCard } from "../components/SelfUpdateCard";
import { TelegramCard } from "../components/TelegramCard";

export function System({ board, update, onUpdateChange, packages, onPackagesChange, onLogout }: {
  board: Board | undefined;
  update: UpdateCheck | undefined;
  onUpdateChange: (u: UpdateCheck) => void;
  packages: PkgUpgrade[] | undefined;
  onPackagesChange: (p: PkgUpgrade[]) => void;
  onLogout: () => void;
}) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <UpdateCard board={board} update={update} onChange={onUpdateChange} />
      <SecurityCard onLogout={onLogout} />
      <ModeCard />
      <RemoteAccessCard />
      <OffloadCard />
      <SelfUpdateCard />
      <TelegramCard />
      <div className="sm:col-span-2 xl:col-span-3">
        <AccessCard />
      </div>
      <div className="sm:col-span-2 xl:col-span-3">
        <PackagesCard upgradable={packages} onChange={onPackagesChange} />
      </div>
    </div>
  );
}
