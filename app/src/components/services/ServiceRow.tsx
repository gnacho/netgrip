import type { ReactNode } from "react";

/**
 * ServiceRow (#260): estado "no instalado" de un servicio en UNA sola fila —
 * icono + título + descripción a la izquierda y el CTA de instalar a la
 * derecha — en vez de la EmptyState apilada que ocupa toda la card.
 */
export function ServiceRow({ icon, title, description, action }: {
  icon?: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex items-center gap-3 py-1">
      {icon && <span className="shrink-0 text-faint" aria-hidden="true">{icon}</span>}
      <div className="min-w-0 flex-1">
        <p className="text-body font-semibold truncate">{title}</p>
        {description && <p className="text-small text-muted truncate">{description}</p>}
      </div>
      {action && <span className="shrink-0">{action}</span>}
    </div>
  );
}
