import type { Board, PkgUpgrade, UpdateCheck } from "../types";
import { UpdateCard } from "../components/UpdateCard";
import { SecurityCard } from "../components/SecurityCard";
import { PackagesCard } from "../components/PackagesCard";

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
      <PackagesCard upgradable={packages} onChange={onPackagesChange} />
    </div>
  );
}
