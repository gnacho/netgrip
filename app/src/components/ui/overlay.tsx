import { useEffect, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { TriangleAlert, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "./Button";
import { IconTile } from "./IconTile";

function useOverlayA11y(open: boolean, onClose: () => void, ref: React.RefObject<HTMLDivElement | null>) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
      if (e.key === "Tab" && ref.current) {
        // foco atrapado
        const els = ref.current.querySelectorAll<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
        );
        if (els.length === 0) return;
        const first = els[0];
        const last = els[els.length - 1];
        if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
        else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
      }
    };
    document.addEventListener("keydown", onKey);
    const t = setTimeout(() => {
      ref.current?.querySelector<HTMLElement>("button, input, [tabindex]")?.focus();
    }, 30);
    return () => { document.removeEventListener("keydown", onKey); clearTimeout(t); };
  }, [open, onClose, ref]);
}

/**
 * Modal §6.16: centrado en desktop (max-w 480, r-lg, sombra elevada,
 * fondo bg/70 + blur). En móvil se comporta como hoja inferior (Drawer).
 */
export function Modal({ open, onClose, title, children, footer, wide = false }: {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  wide?: boolean;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useOverlayA11y(open, onClose, ref);
  if (!open) return null;
  return createPortal(
    <div className="fixed inset-0 z-50 flex items-end md:items-center justify-center md:p-4"
      role="dialog" aria-modal="true">
      <div className="absolute inset-0 bg-bg/70 backdrop-blur-[4px]" onClick={onClose} />
      <div
        ref={ref}
        className={`relative w-full ${wide ? "md:max-w-2xl" : "md:max-w-[480px]"} max-h-[92vh] overflow-y-auto
          rounded-t-lg md:rounded-lg bg-surface border border-border shadow-elevated
          p-4 md:p-5 animate-fade-up`}
      >
        <div className="mx-auto mb-3 h-1 w-9 rounded-full bg-border-strong md:hidden" aria-hidden="true" />
        {(title || true) && (
          <div className="flex items-center gap-2 mb-3">
            {title && <h2 className="text-h2 flex-1 min-w-0">{title}</h2>}
            <button type="button" onClick={onClose} aria-label="×"
              className="ml-auto shrink-0 text-muted hover:text-text ring-focus rounded-sm">
              <X size={18} />
            </button>
          </div>
        )}
        {children}
        {footer && <div className="mt-4 flex justify-end gap-2">{footer}</div>}
      </div>
    </div>,
    document.body,
  );
}

/** Drawer §6.16: hoja inferior móvil (también usable en desktop como panel lateral simple). */
export function Drawer({ open, onClose, title, children }: {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useOverlayA11y(open, onClose, ref);
  if (!open) return null;
  return createPortal(
    <div className="fixed inset-0 z-50 flex items-end" role="dialog" aria-modal="true">
      <div className="absolute inset-0 bg-bg/70 backdrop-blur-[4px]" onClick={onClose} />
      <div
        ref={ref}
        className="relative w-full max-h-[88vh] overflow-y-auto rounded-t-lg bg-surface border-t border-border shadow-elevated p-4 pb-8 animate-banner-in"
      >
        <div className="mx-auto mb-3 h-1 w-9 rounded-full bg-border-strong" aria-hidden="true" />
        {title && <h2 className="text-h2 mb-3">{title}</h2>}
        {children}
      </div>
    </div>,
    document.body,
  );
}

/** ConfirmDialog §6.17: destructivas con consecuencia explicada. */
export function ConfirmDialog({ open, onClose, onConfirm, title, consequence, confirmLabel, busy = false }: {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  /** la consecuencia, en una frase (obligatoria) */
  consequence: string;
  /** verbo concreto: "Cambiar contraseña" */
  confirmLabel: string;
  busy?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Modal open={open} onClose={onClose}>
      <div className="flex items-start gap-3">
        <IconTile icon={TriangleAlert} tone="danger" />
        <div className="min-w-0">
          <h2 className="text-h2">{title}</h2>
          <p className="text-small text-muted mt-1">{consequence}</p>
        </div>
      </div>
      <div className="mt-5 flex justify-end gap-2">
        <Button variant="ghost" onClick={onClose}>{t("common.cancel")}</Button>
        <Button variant="danger" onClick={onConfirm} loading={busy}>{confirmLabel}</Button>
      </div>
    </Modal>
  );
}
