import { useEffect, useState } from "react";
import { CircleCheck, TriangleAlert } from "lucide-react";
import { useTranslation } from "react-i18next";

export type ActionPhase = "applying" | "checking" | "done" | "failed";

const BG: Record<ActionPhase, string> = {
  applying: "bg-accent-soft",
  checking: "bg-accent-soft",
  done: "bg-ok-soft",
  failed: "bg-danger-soft",
};

/**
 * ActionBanner §6.11: traduce snapshot→apply→healthcheck→rollback a
 * lenguaje humano, con tres puntos-progreso. Mapea ModuleResult.status.
 */
export function ActionBanner({ phase, text, detail, onDone }: {
  phase: ActionPhase;
  /** texto small; si no se da, se usa el copy por defecto de la fase */
  text?: string;
  /** error crudo (se muestra bajo "Ver detalle técnico") */
  detail?: string;
  /** se llama cuando el banner "done" se funde (4s) */
  onDone?: () => void;
}) {
  const { t } = useTranslation();
  const [showDetail, setShowDetail] = useState(false);

  const step = phase === "applying" ? 1 : phase === "checking" ? 2 : 3;
  const defaultText = phase === "applying" ? t("action.applying")
    : phase === "checking" ? t("action.checking")
    : phase === "done" ? t("action.done")
    : t("action.failed");

  useEffect(() => {
    if (phase !== "done" || !onDone) return;
    const id = setTimeout(onDone, 4000);
    return () => clearTimeout(id);
  }, [phase, onDone]);

  return (
    <div role="status" className={`rounded-md px-3.5 py-3 animate-banner-in ${BG[phase]}`}>
      <div className="flex items-center gap-3">
        {phase === "done" ? (
          <CircleCheck size={16} className="text-ok shrink-0" aria-hidden="true" />
        ) : phase === "failed" ? (
          <TriangleAlert size={16} className="text-danger shrink-0" aria-hidden="true" />
        ) : (
          <span className="flex gap-1 shrink-0" aria-hidden="true">
            {[1, 2, 3].map((i) => (
              <span key={i}
                className={`h-1.5 w-1.5 rounded-full transition-colors duration-200 ${i <= step ? "bg-accent" : "bg-border-strong"}`} />
            ))}
          </span>
        )}
        <p className="text-small flex-1 min-w-0">{text ?? defaultText}</p>
      </div>
      {phase === "failed" && detail && (
        <div className="mt-2 pl-7">
          <button type="button" onClick={() => setShowDetail((s) => !s)}
            className="text-small text-accent hover:text-accent-hover ring-focus rounded-sm">
            {t("action.detail")}
          </button>
          {showDetail && (
            <pre className="mt-1.5 max-h-40 overflow-auto rounded-sm bg-surface/60 border border-border p-2 font-mono text-caption whitespace-pre-wrap">{detail}</pre>
          )}
        </div>
      )}
    </div>
  );
}
