/** Switch iOS 44×26 §6.4. */
export function Toggle({ checked, disabled, busy, onChange, label }: {
  checked: boolean;
  disabled?: boolean;
  busy?: boolean;
  onChange: (value: boolean) => void;
  label?: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      aria-busy={busy || undefined}
      disabled={disabled || busy}
      onClick={() => onChange(!checked)}
      className={`relative inline-flex h-[26px] w-11 shrink-0 items-center rounded-full ring-focus
        transition-[background-color] duration-200 ease-[var(--ease-soft)]
        ${checked ? "bg-accent" : "bg-border-strong"}
        ${disabled || busy ? "opacity-60 cursor-not-allowed" : "cursor-pointer"}`}
    >
      <span
        aria-hidden="true"
        className={`absolute left-[2px] top-[2px] flex h-[22px] w-[22px] items-center justify-center rounded-full bg-white
          shadow-[0_1px_2px_rgba(0,0,0,0.2)] transition-transform duration-200 ease-[var(--ease-soft)]
          ${checked ? "translate-x-[18px]" : "translate-x-0"}`}
      >
        {busy && (
          <span className="h-3 w-3 rounded-full border-2 border-muted/40 border-t-muted animate-spin-loop" />
        )}
      </span>
    </button>
  );
}
