import { useTranslation } from "react-i18next";
import { Wrench } from "lucide-react";
import type { EthPort } from "../types";
import { BounceTool, CableTestTool, LoopsTool } from "../components/tools/diagnostics";
import { Card } from "../components/ui";

/**
 * Herramientas (#500): UNA tarjeta a ancho completo con las tres
 * herramientas físicas en columnas internas (cable, bucles, bounce).
 * La sección de mantenimiento (copias y plantillas) se retiró: el backup
 * vive en Ajustes y el executor hace snapshot automático en cada apply;
 * los endpoints /api/snapshots y /api/templates siguen vivos.
 */
export function ToolsPage({ ethports }: { ethports: EthPort[] }) {
  const { t } = useTranslation();
  return (
    <Card title={t("tools.sectionDiagnostics")} icon={Wrench}>
      <div className="grid grid-cols-1 gap-6 md:grid-cols-3 md:gap-5">
        <CableTestTool />
        <LoopsTool />
        <BounceTool ethports={ethports} />
      </div>
    </Card>
  );
}
