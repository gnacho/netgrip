import { Check } from "lucide-react";
import { useTranslation } from "react-i18next";

/**
 * Stepper §6.14 (wizard): píldora "Paso 2 de 5" + barra fina 4px; en
 * desktop, lista lateral opcional de pasos con check en completados.
 */
export function Stepper({ step, total, steps, current, onSelect }: {
  /** paso actual 1-based (para la píldora/barra) */
  step: number;
  total: number;
  /** lista lateral desktop: títulos llanos; current 0-based */
  steps?: string[];
  current?: number;
  onSelect?: (i: number) => void;
}) {
  const { t } = useTranslation();
  const pct = Math.max(0, Math.min(100, (step / total) * 100));
  return (
    <div>
      <div className="flex items-center gap-3">
        <span className="rounded-full bg-accent-soft text-accent px-2.5 py-0.5 text-caption font-semibold whitespace-nowrap">
          {t("wizard.stepOf", { step, total })}
        </span>
        <div className="h-1 flex-1 rounded-full bg-border overflow-hidden">
          <div className="h-full rounded-full bg-accent transition-[width] duration-[var(--dur-slow)] ease-[var(--ease-soft)]"
            style={{ width: `${pct}%` }} />
        </div>
      </div>
      {steps && steps.length > 0 && (
        <ol className="hidden md:block mt-4 space-y-1">
          {steps.map((s, i) => {
            const done = current !== undefined && i < current;
            const active = current === i;
            return (
              <li key={s}>
                <button type="button" onClick={() => onSelect?.(i)} disabled={!onSelect}
                  className={`flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-small text-left ring-focus
                    ${active ? "text-accent font-medium" : done ? "text-muted" : "text-faint"}`}>
                  <span className={`flex h-[18px] w-[18px] items-center justify-center rounded-full border
                    ${done ? "bg-ok border-ok text-white" : active ? "border-accent" : "border-border-strong"}`}>
                    {done && <Check size={12} />}
                  </span>
                  {s}
                </button>
              </li>
            );
          })}
        </ol>
      )}
    </div>
  );
}
