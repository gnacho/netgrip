import { useTranslation } from "react-i18next";
import type { EthPort } from "../types";
import { BounceCard, CableTestCard, LoopsCard } from "../components/tools/diagnostics";

/**
 * Herramientas (#500): solo diagnóstico físico en tres cards gemelas.
 * La sección de mantenimiento (copias de seguridad y plantillas) se
 * retiró: el backup vive en Ajustes (tarjeta remota) y el executor hace
 * snapshot automático en cada apply; las plantillas del backend eran
 * tres presets fijos que ya cubren el asistente y las páginas de cada
 * función. Los endpoints /api/snapshots y /api/templates siguen vivos.
 * La antigua tarjeta de paquetes opcionales (#484) también se retiró:
 * la paquetería se gestiona desde LuCI o CLI.
 */
export function ToolsPage({ ethports }: { ethports: EthPort[] }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      <section aria-label={t("tools.sectionDiagnostics")}>
        <div className="grid grid-cols-1 gap-[var(--card-gap)] md:grid-cols-2 xl:grid-cols-3">
          <CableTestCard index={1} />
          <LoopsCard index={2} />
          <BounceCard ethports={ethports} index={3} />
        </div>
      </section>

    </div>
  );
}
