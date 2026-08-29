import { useLayoutEffect, useRef, useState } from "react";

export interface Segment<T extends string> {
  value: T;
  label: React.ReactNode;
  title?: string;
}

/**
 * SegmentedControl §6.19: pastilla surface-2 con indicador surface +
 * sombra deslizante 200ms. Máx 4 segmentos.
 */
export function SegmentedControl<T extends string>({ options, value, onChange, size = "md", ariaLabel }: {
  options: Segment<T>[];
  value: T;
  onChange: (v: T) => void;
  size?: "sm" | "md" | "lg";
  ariaLabel?: string;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [indicator, setIndicator] = useState<{ left: number; width: number }>();

  useLayoutEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const idx = options.findIndex((o) => o.value === value);
    const btn = el.children[idx + 1] as HTMLElement | undefined; // +1: el indicador
    if (btn) setIndicator({ left: btn.offsetLeft, width: btn.offsetWidth });
  }, [value, options]);

  return (
    <div
      ref={containerRef}
      role="tablist"
      aria-label={ariaLabel}
      className="relative inline-flex items-center gap-0.5 rounded-full bg-surface-2 p-0.5 border border-border"
    >
      <span
        aria-hidden="true"
        className="absolute top-0.5 bottom-0.5 rounded-full bg-surface shadow-card transition-all duration-200 ease-[var(--ease-soft)]"
        style={indicator ? { left: indicator.left, width: indicator.width } : { opacity: 0 }}
      />
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="tab"
          aria-selected={o.value === value}
          title={o.title}
          onClick={() => onChange(o.value)}
          className={`relative z-10 rounded-full ring-focus font-medium whitespace-nowrap transition-colors duration-[var(--dur-fast)]
            ${size === "sm" ? "px-2.5 py-1 text-caption" : size === "lg" ? "px-5 py-2 text-body" : "px-3 py-1.5 text-small"}
            ${o.value === value ? "text-text" : "text-muted hover:text-text"}`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
