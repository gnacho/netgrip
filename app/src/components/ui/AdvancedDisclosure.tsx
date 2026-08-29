import type { ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import { useTranslation } from "react-i18next";

/**
 * AdvancedDisclosure §6.20: <details> estilizado, chevron rota 180° en
 * 200ms, contenido con transición de altura vía grid-template-rows.
 */
export function AdvancedDisclosure({ label, children, className = "" }: {
  label?: string;
  children: ReactNode;
  className?: string;
}) {
  const { t } = useTranslation();
  return (
    <details className={`group ${className}`}>
      <summary className="flex cursor-pointer list-none items-center gap-1.5 py-2 text-small text-muted hover:text-text transition-colors [&::-webkit-details-marker]:hidden">
        <ChevronDown size={16} className="transition-transform duration-200 ease-[var(--ease-soft)] group-open:rotate-180" aria-hidden="true" />
        {label ?? t("common.advanced")}
      </summary>
      <div className="grid grid-rows-[0fr] transition-[grid-template-rows] duration-200 ease-[var(--ease-soft)] group-open:grid-rows-[1fr]">
        <div className="overflow-hidden">
          <div className="pt-1 pb-2">{children}</div>
        </div>
      </div>
    </details>
  );
}
