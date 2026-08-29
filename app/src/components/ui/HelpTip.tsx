import { useEffect, useRef, useState } from "react";
import { CircleHelp } from "lucide-react";
import { useTranslation } from "react-i18next";

/**
 * HelpTip §6.7 — "¿Qué es esto?". Botón circular 20px; popover propio
 * (useState + useRef), cierra con click fuera, Esc o segundo click.
 */
export function HelpTip({ title, body, more }: {
  title: string;
  body: string;
  more?: { label: string; href: string };
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <span ref={ref} className="relative inline-flex shrink-0">
      <button
        type="button"
        aria-expanded={open}
        aria-label={title}
        onClick={() => setOpen((o) => !o)}
        className="inline-flex h-5 w-5 items-center justify-center rounded-full text-faint hover:text-accent ring-focus transition-colors duration-[var(--dur-fast)]"
      >
        <CircleHelp size={16} />
      </button>
      {open && (
        <span
          role="tooltip"
          className="absolute right-0 top-6 z-40 block w-[min(280px,calc(100vw-48px))] rounded-md border border-border bg-surface p-3 text-left shadow-elevated animate-banner-in"
        >
          <span className="block text-small font-semibold mb-1">{title}</span>
          <span className="block text-small text-muted">{body}</span>
          {more && (
            <a href={more.href} className="mt-2 inline-block text-small text-accent hover:text-accent-hover">
              {more.label} →
            </a>
          )}
          <span className="sr-only">{t("common.escToClose")}</span>
        </span>
      )}
    </span>
  );
}
