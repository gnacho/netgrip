import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

export function Card({ title, icon: Icon, children, action }: {
  title: string;
  icon: LucideIcon;
  children: ReactNode;
  action?: ReactNode;
}) {
  return (
    <section className="bg-card border border-border rounded-xl p-4">
      <header className="flex items-center gap-2 mb-3">
        <Icon size={16} className="text-muted" />
        <h2 className="text-xs font-medium uppercase tracking-wider text-muted flex-1">{title}</h2>
        {action}
      </header>
      {children}
    </section>
  );
}

export function Row({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="flex justify-between gap-4 py-1 border-b border-border/50 last:border-0 text-sm">
      <span className="text-muted">{label}</span>
      <span className="text-right">{value}</span>
    </div>
  );
}

export function Pill({ tone, children }: { tone: "ok" | "warn" | "danger" | "muted"; children: ReactNode }) {
  const colors = {
    ok: "bg-ok/15 text-ok",
    warn: "bg-warn/15 text-warn",
    danger: "bg-danger/15 text-danger",
    muted: "bg-muted/15 text-muted",
  } as const;
  return <span className={`text-xs px-2 py-0.5 rounded-full ${colors[tone]}`}>{children}</span>;
}
