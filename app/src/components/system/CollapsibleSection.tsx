import { useState, type ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { IconTile, type Tone } from "../ui/IconTile";

/**
 * Fila compacta que se despliega (#499): la cabecera es UNA línea (icono,
 * título, estado opcional, chevron) y el contenido solo ocupa sitio al
 * abrir. El contenido se mantiene montado mientras exista el padre
 * (oculto con `hidden`), de modo que los formularios conservan su estado
 * y no repiten peticiones al abrir y cerrar.
 */
export function CollapsibleSection({ icon, iconTone = "muted", title, status, defaultOpen = false, onOpenChange, children }: {
  icon?: LucideIcon;
  iconTone?: Tone;
  title: string;
  /** Resumen en la cabecera (Pill, punto de estado...); opcional. */
  status?: ReactNode;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="rounded-lg border border-border/60">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => { const next = !open; setOpen(next); onOpenChange?.(next); }}
        className="w-full flex items-center gap-3 px-3 py-2.5 text-left rounded-lg ring-focus hover:bg-surface-2 transition-colors duration-150"
      >
        {icon && <IconTile icon={icon} tone={iconTone} size={28} />}
        <span className="flex-1 min-w-0 text-body font-medium">{title}</span>
        {status}
        <ChevronDown
          size={16}
          aria-hidden="true"
          className={`shrink-0 text-muted transition-transform duration-200 ${open ? "rotate-180" : ""}`}
        />
      </button>
      <div className={open ? "flex flex-col gap-2 px-3 pb-3" : "hidden"}>{children}</div>
    </div>
  );
}
