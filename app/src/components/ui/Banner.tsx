import type { ReactNode } from "react";
import { CircleAlert, CircleCheck, Info, TriangleAlert, X } from "lucide-react";
import type { LucideIcon } from "lucide-react";

type BannerTone = "info" | "ok" | "warn" | "danger";

const TONES: Record<BannerTone, { cls: string; icon: LucideIcon }> = {
  info: { cls: "bg-accent-soft text-accent", icon: Info },
  ok: { cls: "bg-ok-soft text-ok", icon: CircleCheck },
  warn: { cls: "bg-warn-soft text-warn", icon: TriangleAlert },
  danger: { cls: "bg-danger-soft text-danger", icon: CircleAlert },
};

/** Banner §6.13: franja a lo ancho con icono + texto + acción opcional. */
export function Banner({ tone = "info", icon, action, onDismiss, children, className = "" }: {
  tone?: BannerTone;
  icon?: LucideIcon;
  action?: ReactNode;
  onDismiss?: () => void;
  children: ReactNode;
  className?: string;
}) {
  const t = TONES[tone];
  const Icon = icon ?? t.icon;
  return (
    <div role={tone === "danger" || tone === "warn" ? "alert" : "status"}
      className={`flex items-center gap-2.5 rounded-md px-3.5 py-2.5 text-small animate-banner-in ${t.cls} ${className}`}>
      <Icon size={16} className="shrink-0" aria-hidden="true" />
      <div className="flex-1 min-w-0 text-text/90 dark:text-text">{children}</div>
      {action}
      {onDismiss && (
        <button type="button" onClick={onDismiss} aria-label="×"
          className="shrink-0 opacity-70 hover:opacity-100 ring-focus rounded-sm">
          <X size={16} />
        </button>
      )}
    </div>
  );
}
