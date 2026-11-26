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
export function Card({ eyebrow, title, status, icon, iconTone = "accent", help, action, onExpand, expandLabel, variant = "default", index = 0, id, className = "", children, animate = true, headerOnly = false }: {
  eyebrow?: string;
  title?: ReactNode;
  /** Badge that describes the title (state pill). Rendered right after it. */
  status?: ReactNode;
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
  children?: ReactNode;
  animate?: boolean;
  /** The header is the whole card: no body or header margin below it. */
  headerOnly?: boolean;
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
        <header className={headerOnly ? "" : "mb-3"}>
          {eyebrow && <p className="text-eyebrow text-faint mb-1.5">{eyebrow}</p>}
          {/* #726: si el título y la acción no caben, la acción baja a su
              propia línea en vez de exprimir el título. */}
          <div className="flex flex-wrap items-center gap-2.5">
            {icon && <IconTile icon={icon} tone={iconTone} />}
            <div className="flex min-w-[10rem] flex-1 items-center gap-2.5">
              {title && <h2 className="text-h2 min-w-0">{title}</h2>}
              {status}
              {help && <HelpTip title={t(`help.${help}.title`)} body={t(`help.${help}.body`)} />}
              {onExpand && (
                <button type="button" onClick={onExpand} aria-label={expandLabel}
                  className="shrink-0 text-muted hover:text-text ring-focus rounded-sm">
                  <Maximize2 size={14} />
                </button>
              )}
            </div>
            {action}
          </div>
        </header>
      )}
      {children}
    </section>
  );
}
