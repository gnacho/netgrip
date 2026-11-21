import { useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { CircleHelp } from "lucide-react";
import { useTranslation } from "react-i18next";

type Position = { left: number; top: number; width: number };

/**
 * HelpTip §6.7 — "¿Qué es esto?". Botón circular 20px y popover en un portal
 * fijo: las tarjetas usan transformaciones y contextos de apilado propios,
 * así que un popover absoluto quedaba detrás de la tarjeta siguiente.
 */
export function HelpTip({ title, body, more }: {
  title: string;
  body: string;
  more?: { label: string; href: string };
}) {
  const { t } = useTranslation();
  const id = useId();
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState<Position>();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const tipRef = useRef<HTMLDivElement>(null);

  const place = () => {
    const anchor = buttonRef.current;
    if (!anchor) return;
    const rect = anchor.getBoundingClientRect();
    const width = Math.min(280, window.innerWidth - 32);
    const height = tipRef.current?.offsetHeight ?? 120;
    const left = Math.min(
      Math.max(16, rect.right - width),
      Math.max(16, window.innerWidth - width - 16),
    );
    let top = rect.bottom + 8;
    if (top + height > window.innerHeight - 16) {
      top = Math.max(16, rect.top - height - 8);
    }
    setPosition({ left, top, width });
  };

  useLayoutEffect(() => {
    if (!open) {
      setPosition(undefined);
      return;
    }
    place();
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onDown = (event: MouseEvent) => {
      const target = event.target as Node;
      if (buttonRef.current?.contains(target) || tipRef.current?.contains(target)) return;
      setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    const reposition = () => place();
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    window.addEventListener("resize", reposition);
    window.addEventListener("scroll", reposition, true);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("resize", reposition);
      window.removeEventListener("scroll", reposition, true);
    };
  }, [open]);

  return (
    <span className="inline-flex shrink-0">
      <button
        ref={buttonRef}
        type="button"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        aria-label={title}
        onClick={() => setOpen((current) => !current)}
        className="inline-flex h-5 w-5 items-center justify-center rounded-full text-faint hover:text-accent ring-focus transition-colors duration-[var(--dur-fast)]"
      >
        <CircleHelp size={16} />
      </button>
      {open && createPortal(
        <div
          ref={tipRef}
          id={id}
          role="tooltip"
          style={{
            position: "fixed",
            left: position?.left ?? -9999,
            top: position?.top ?? -9999,
            width: position?.width ?? Math.min(280, window.innerWidth - 32),
          }}
          className="z-[100] rounded-md border border-border bg-surface p-3 text-left shadow-elevated animate-banner-in"
        >
          <span className="block text-small font-semibold mb-1">{title}</span>
          <span className="block text-small text-muted">{body}</span>
          {more && (
            <a href={more.href} className="mt-2 inline-block text-small text-accent hover:text-accent-hover">
              {more.label} →
            </a>
          )}
          <span className="sr-only">{t("common.escToClose")}</span>
        </div>,
        document.body,
      )}
    </span>
  );
}
