import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { CircleAlert, CircleCheck, Copy, Info, TriangleAlert, X } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useTranslation } from "react-i18next";

export type ToastTone = "ok" | "info" | "warn" | "danger";

export interface ToastItem {
  id: number;
  tone: ToastTone;
  text: string;
  detail?: string;
}

const ICONS: Record<ToastTone, LucideIcon> = {
  ok: CircleCheck,
  info: Info,
  warn: TriangleAlert,
  danger: CircleAlert,
};

const TONE_CLS: Record<ToastTone, string> = {
  ok: "text-ok",
  info: "text-accent",
  warn: "text-warn",
  danger: "text-danger",
};

interface ToastCtx {
  push: (t: Omit<ToastItem, "id">) => void;
}

const Ctx = createContext<ToastCtx>({ push: () => {} });

export const useToast = () => useContext(Ctx);

/** Toast §6.18 + host: esquina inferior derecha (móvil: centrado abajo), máx 3. */
export function ToastProvider({ children }: { children: ReactNode }) {
  const { t } = useTranslation();
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const nextId = useRef(1);

  const dismiss = useCallback((id: number) => {
    setToasts((ts) => ts.filter((x) => x.id !== id));
  }, []);

  const push = useCallback((item: Omit<ToastItem, "id">) => {
    const id = nextId.current++;
    setToasts((ts) => [...ts.slice(-2), { ...item, id }]);
    setTimeout(() => dismiss(id), item.tone === "danger" ? 8000 : 4000);
  }, [dismiss]);

  const ctx = useMemo(() => ({ push }), [push]);

  return (
    <Ctx.Provider value={ctx}>
      {children}
      {createPortal(
        <div aria-live="polite" className="fixed z-[60] bottom-20 md:bottom-5 inset-x-4 md:inset-x-auto md:right-5 md:w-80 flex flex-col gap-2 items-stretch">
          {toasts.map((toast) => {
            const Icon = ICONS[toast.tone];
            return (
              <div key={toast.id}
                className="flex items-start gap-2.5 rounded-md border border-border bg-surface px-3.5 py-3 shadow-elevated animate-fade-up">
                <Icon size={16} className={`mt-0.5 shrink-0 ${TONE_CLS[toast.tone]}`} aria-hidden="true" />
                <p className="flex-1 min-w-0 text-small">{toast.text}</p>
                {toast.detail && (
                  <button type="button"
                    title={t("common.copyDetail")}
                    onClick={() => navigator.clipboard?.writeText(toast.detail ?? "").catch(() => {})}
                    className="shrink-0 text-muted hover:text-text ring-focus rounded-sm">
                    <Copy size={14} />
                  </button>
                )}
                <button type="button" onClick={() => dismiss(toast.id)} aria-label="×"
                  className="shrink-0 text-muted hover:text-text ring-focus rounded-sm">
                  <X size={14} />
                </button>
              </div>
            );
          })}
        </div>,
        document.body,
      )}
    </Ctx.Provider>
  );
}
