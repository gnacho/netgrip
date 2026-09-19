import type { CSSProperties, ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Maximize2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { IconTile, type Tone } from "./IconTile";
import { HelpTip } from "./HelpTip";

/**
 * Card §6.1. Título siempre llano (tarea/concepto), nunca jerga.
 * `help` es una clave bajo help.* (help.<help>.title / help.<help>.body).
 * `onExpand` renders an expand button next to the help tip for cards that
 * keep their detail in a modal, so the summary takes as little vertical
 * space as possible.
 */
export function Card({ eyebrow, title, icon, iconTone = "accent", help, action, onExpand, expandLabel, variant = "default", index = 0, id, className = "", children, animate = true }: {
  eyebrow?: string;
  title?: ReactNode;
  icon?: LucideIcon;
  iconTone?: Tone;
  help?: string;
  action?: ReactNode;
  onExpand?: () => void;
  /** Required when onExpand is set: the button's aria-label (i18n). */
  expandLabel?: string;
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
            {onExpand && (
              <button type="button" onClick={onExpand} aria-label={expandLabel}
                className="shrink-0 text-muted hover:text-text ring-focus rounded-sm">
                <Maximize2 size={14} />
              </button>
            )}
            {action}
          </div>
        </header>
      )}
      {children}
    </section>
  );
}
