import type { Board, UpdateCheck } from "../types";
import { UpdateCard } from "../components/UpdateCard";
import { SecurityCard } from "../components/SecurityCard";

export function System({ board, update, onUpdateChange, onLogout }: {
  board: Board | undefined;
  update: UpdateCheck | undefined;
  onUpdateChange: (u: UpdateCheck) => void;
  onLogout: () => void;
}) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <UpdateCard board={board} update={update} onChange={onUpdateChange} />
      <SecurityCard onLogout={onLogout} />
    </div>
  );
}
