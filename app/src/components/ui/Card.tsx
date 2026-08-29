import type { CSSProperties, ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { useTranslation } from "react-i18next";
import { IconTile, type Tone } from "./IconTile";
import { HelpTip } from "./HelpTip";

/**
 * Card §6.1. Título siempre llano (tarea/concepto), nunca jerga.
 * `help` es una clave bajo help.* (help.<help>.title / help.<help>.body).
 */
export function Card({ eyebrow, title, icon, iconTone = "accent", help, action, variant = "default", index = 0, id, className = "", children, animate = true }: {
  eyebrow?: string;
  title?: ReactNode;
  icon?: LucideIcon;
  iconTone?: Tone;
  help?: string;
  action?: ReactNode;
  variant?: "default" | "subtle";
  /** stagger fade-up: --i * 40ms */
  index?: number;
  id?: string;
  className?: string;
  children: ReactNode;
  animate?: boolean;
}) {
  const { t } = useTranslation();
  const style = { "--i": Math.min(index, 7) } as CSSProperties;
  return (
    <section
      id={id}
      style={style}
      className={`rounded-lg p-[var(--card-pad)] ${animate ? "animate-fade-up" : ""} ${
        variant === "subtle"
          ? "bg-surface-2 border border-border"
          : "bg-surface border border-border shadow-card"
      } ${className}`}
    >
      {(title || action || eyebrow) && (
        <header className="mb-3">
          {eyebrow && <p className="text-eyebrow text-faint mb-1.5">{eyebrow}</p>}
          <div className="flex items-center gap-2.5">
            {icon && <IconTile icon={icon} tone={iconTone} />}
            {title && <h2 className="text-h2 flex-1 min-w-0">{title}</h2>}
            {help && <HelpTip title={t(`help.${help}.title`)} body={t(`help.${help}.body`)} />}
            {action}
          </div>
        </header>
      )}
      {children}
    </section>
  );
}
