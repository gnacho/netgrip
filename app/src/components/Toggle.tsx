export function Toggle({ checked, disabled, onChange, busy }: {
  checked: boolean;
  disabled?: boolean;
  busy?: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <button
      role="switch"
      aria-checked={checked}
      disabled={disabled || busy}
      onClick={() => onChange(!checked)}
      className={`relative w-11 h-6 rounded-full transition-colors shrink-0
        ${checked ? "bg-accent" : "bg-border"}
        ${disabled || busy ? "opacity-50 cursor-not-allowed" : "cursor-pointer"}`}
    >
      <span
        className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white transition-transform
          ${checked ? "translate-x-5" : "translate-x-0"}
          ${busy ? "animate-pulse" : ""}`}
      />
    </button>
  );
}
