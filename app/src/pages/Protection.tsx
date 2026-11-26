import { AdguardCard } from "../components/services/AdguardCard";
import { BanipPage } from "./Banip";

/**
 * Protección (#687): lo que se bloquea y lo que no llega a entrar. AdGuard
 * Home filtra el DNS y banIP corta direcciones de mala reputación, así que
 * viven juntos y fuera de Internet, con sus listas de bloqueo.
 */
export function ProtectionPage() {
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <AdguardCard index={0} />
      <div id="page-banip"><BanipPage /></div>
    </div>
  );
}
