import { useTranslation } from "react-i18next";
import type { EthPort } from "../types";
import { SnapshotsCard } from "../components/tools/snapshots";
import { BounceCard, CableTestCard, LoopsCard } from "../components/tools/diagnostics";
import { TemplatesCard } from "../components/tools/TemplatesCard";

/**
 * Herramientas (tools.md): copias de seguridad como héroe, diagnóstico físico
 * en tres cards gemelas y plantillas. La antigua tarjeta de paquetes
 * opcionales (#484) se retiró: la paquetería se gestiona desde LuCI o CLI;
 * el endpoint /api/packages/optional sigue vivo para el asistente inicial.
 */
export function ToolsPage({ ethports }: { ethports: EthPort[] }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <section aria-label={t("tools.sectionMaintenance")}>
        <p className="text-eyebrow text-faint mb-2">{t("tools.sectionMaintenance")}</p>
        <div className="flex flex-col gap-[var(--card-gap)]">
          <SnapshotsCard />
          <TemplatesCard />
        </div>
      </section>

      <section aria-label={t("tools.sectionDiagnostics")}>
        <p className="text-eyebrow text-faint mb-2">{t("tools.sectionDiagnostics")}</p>
        <div className="grid grid-cols-1 gap-[var(--card-gap)] md:grid-cols-2 xl:grid-cols-3">
          <CableTestCard index={1} />
          <LoopsCard index={2} />
          <BounceCard ethports={ethports} index={3} />
        </div>
      </section>

    </div>
  );
}
