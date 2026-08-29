import type { ReactNode } from "react";

export interface KVItem {
  label: string;
  value: ReactNode;
  /** dato de red (IP, MAC, host…) → mono */
  mono?: boolean;
}

/** KeyValue §6.24: solo para detalle técnico dentro de cards. */
export function KeyValue({ items, className = "" }: { items: KVItem[]; className?: string }) {
  return (
    <dl className={className}>
      {items.map((it) => (
        <div key={it.label} className="flex items-baseline justify-between gap-4 py-2.5 border-b border-border/60 last:border-0">
          <dt className="text-small text-muted shrink-0">{it.label}</dt>
          <dd className={`text-body font-medium text-right min-w-0 break-all ${it.mono ? "font-mono text-small" : ""}`}>
            {it.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}
