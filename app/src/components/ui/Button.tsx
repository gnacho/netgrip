import type { ButtonHTMLAttributes, ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

type Variant = "primary" | "secondary" | "ghost" | "danger";

const VARIANTS: Record<Variant, string> = {
  primary: "bg-accent text-on-accent hover:bg-accent-hover",
  secondary: "bg-surface border border-border hover:bg-surface-2 text-text",
  ghost: "text-muted hover:text-text hover:bg-surface-2",
  danger: "bg-danger text-white hover:opacity-90",
};

/** Button §6.22. `loading` reemplaza al icono por un spinner. */
export function Button({ variant = "primary", size = "md", loading = false, icon: Icon, className = "", children, disabled, ...rest }: {
  variant?: Variant;
  size?: "sm" | "md";
  loading?: boolean;
  icon?: LucideIcon;
  children: ReactNode;
} & ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      {...rest}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={`inline-flex items-center justify-center gap-2 rounded-md font-medium ring-focus
        transition-[background-color,color,transform] duration-[var(--dur-fast)] ease-[var(--ease-soft)]
        active:scale-[0.98] disabled:opacity-50 disabled:pointer-events-none
        ${size === "sm" ? "h-9 px-3 text-small" : "h-10 px-4 text-body"}
        ${VARIANTS[variant]} ${className}`}
    >
      {loading
        ? <span aria-hidden="true" className="h-4 w-4 rounded-full border-2 border-current/30 border-t-current animate-spin-loop" />
        : Icon && <Icon size={size === "sm" ? 16 : 18} aria-hidden="true" />}
      {children}
    </button>
  );
}
