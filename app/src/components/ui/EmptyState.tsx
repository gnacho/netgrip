import type { ReactNode } from "react";

/** EmptyState §6.12: ilustración SVG + h2 + small muted + acción opcional. */
export function EmptyState({ illustration, title, body, action, small = false }: {
  illustration?: ReactNode;
  title: string;
  body?: string;
  action?: ReactNode;
  /** variante compacta (dentro de cards) */
  small?: boolean;
}) {
  return (
    <div className={`flex flex-col items-center text-center ${small ? "py-4 px-2" : "py-8 px-4"}`}>
      {illustration && (
        <div className="mb-3 text-faint" aria-hidden="true">{illustration}</div>
      )}
      <h2 className={small ? "text-body font-semibold" : "text-h2"}>{title}</h2>
      {body && <p className="text-small text-muted mt-1 max-w-sm">{body}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
