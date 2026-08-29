import { useState, type InputHTMLAttributes } from "react";
import { Eye, EyeOff, type LucideIcon } from "lucide-react";
import { HelpTip } from "./HelpTip";

/**
 * Input (design-rev2 §4) — estilo "filled": fondo fill, borde transparente en
 * reposo, radio 10px, altura var(--input-h). Hover → borde strong; focus →
 * fondo surface + anillo 2px accent (sin outline nativo). Error → danger;
 * disabled → 60 % opacity. `icon` = prefix 12px faint; `mono` = fuente mono
 * tabular (IP/MAC/claves).
 */
export function Input({ mono = false, error = false, icon: Icon, type = "text", className = "", ...rest }: {
  mono?: boolean;
  error?: boolean;
  icon?: LucideIcon;
} & InputHTMLAttributes<HTMLInputElement>) {
  const [show, setShow] = useState(false);
  const isPassword = type === "password";
  return (
    <span className="relative block">
      {Icon && (
        <Icon
          size={16}
          aria-hidden="true"
          className="absolute left-3 top-1/2 -translate-y-1/2 text-faint pointer-events-none"
        />
      )}
      <input
        {...rest}
        type={isPassword && show ? "text" : type}
        aria-invalid={error || undefined}
        className={`w-full h-[var(--input-h)] rounded-[10px] border bg-fill px-3 text-body outline-none
          placeholder:text-faint
          transition-[background-color,border-color,box-shadow] duration-[var(--dur-fast)]
          disabled:opacity-60 disabled:cursor-not-allowed
          ${mono ? "font-mono text-small tabular-nums" : ""}
          ${Icon ? "pl-9" : ""}
          ${isPassword ? "pr-10" : ""}
          ${error
            ? "border-danger focus:bg-surface focus:shadow-[0_0_0_2px_var(--color-danger)]"
            : "border-transparent hover:border-border-strong focus:bg-surface focus:shadow-[0_0_0_2px_var(--color-accent)]"}
          ${className}`}
      />
      {isPassword && (
        <button
          type="button"
          tabIndex={-1}
          aria-hidden="true"
          onClick={() => setShow((s) => !s)}
          className="absolute right-2.5 top-1/2 -translate-y-1/2 text-faint hover:text-text transition-opacity duration-[var(--dur-fast)]"
        >
          {show ? <EyeOff size={16} /> : <Eye size={16} />}
        </button>
      )}
    </span>
  );
}

/** Field §4: label caption muted + control + ayuda/error debajo en caption. */
export function Field({ label, hint, error, help, helpTitle, mono, icon, children, inputProps }: {
  label: string;
  hint?: string;
  error?: string;
  /** clave de ayuda larga (help.*) o texto directo con helpTitle */
  help?: string;
  helpTitle?: string;
  mono?: boolean;
  icon?: LucideIcon;
  children?: React.ReactNode;
  inputProps?: Parameters<typeof Input>[0];
}) {
  return (
    <label className="block">
      <span className="mb-1.5 flex items-center gap-1 text-caption font-medium text-muted">
        {label}
        {help && helpTitle && <HelpTip title={helpTitle} body={help} />}
      </span>
      {children ?? <Input mono={mono} icon={icon} error={!!error} {...inputProps} />}
      {error
        ? <span className="mt-1 block text-caption text-danger">{error}</span>
        : hint && <span className="mt-1 block text-caption text-muted">{hint}</span>}
    </label>
  );
}
