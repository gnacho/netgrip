import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { CircleCheck } from "lucide-react";
import { createPortal } from "react-dom";
import { api } from "../../api";
import { Button } from "../ui";

/**
 * Overlay a pantalla completa para procesos que tumban la red
 * (actualización de firmware, reinicio) — system.md §5.
 * NO se cierra con Esc ni click fuera. El botón ghost "Seguir esperando
 * en segundo plano" lo oculta pero el sondeo a /api/me continúa.
 */
export function WaitOverlay({ open, title, subtitle, steps, step, countdown, onBackground }: {
  open: boolean;
  title: string;
  subtitle?: string;
  /** pasos humanos con check que se van marcando (opcional) */
  steps?: string[];
  /** índice del paso activo */
  step?: number;
  /** segundos restantes (reinicio) */
  countdown?: number;
  onBackground: () => void;
}) {
  const { t } = useTranslation();
  if (!open) return null;
  return createPortal(
    <div className="fixed inset-0 z-[70] bg-bg flex items-center justify-center p-4" role="alert">
      <div className="w-full max-w-md rounded-lg bg-surface border border-border shadow-elevated p-6 text-center animate-fade-up">
        <h2 className="text-h2">
          {title}
          <span aria-hidden="true" className="inline-flex gap-0.5 ml-1 align-baseline">
            {[0, 1, 2].map((i) => (
              <span key={i} className="animate-pulse-dot inline-block" style={{ animationDelay: `${i * 260}ms` }}>.</span>
            ))}
          </span>
        </h2>
        {subtitle && <p className="text-small text-muted mt-1.5">{subtitle}</p>}
        {countdown !== undefined && (
          <p className="mt-3 text-h1 font-semibold tabular-nums text-accent">{countdown}&nbsp;s</p>
        )}
        {steps && steps.length > 0 && (
          <ol className="mt-4 text-left flex flex-col gap-2.5">
            {steps.map((label, i) => {
              const done = step !== undefined && i < step;
              const active = step !== undefined && i === step;
              return (
                <li key={label} className={`flex items-center gap-2.5 text-small ${done ? "text-ok" : active ? "text-text font-medium" : "text-faint"}`}>
                  {done ? (
                    <CircleCheck size={16} className="shrink-0" aria-hidden="true" />
                  ) : (
                    <span aria-hidden="true" className={`h-4 w-4 shrink-0 rounded-full border-2 ${active ? "border-accent/40 border-t-accent animate-spin-loop" : "border-border-strong"}`} />
                  )}
                  {label}
                </li>
              );
            })}
          </ol>
        )}
        {/* barra indeterminada fina (shimmer reutiliza .skeleton) */}
        <div aria-hidden="true" className="skeleton h-1 w-full mt-5 rounded-full" />
        <div className="mt-5">
          <Button variant="ghost" size="sm" onClick={onBackground}>{t("system.waitBackground")}</Button>
        </div>
      </div>
    </div>,
    document.body,
  );
}

/**
 * Sondeo a /api/me cada 5 s: llama a `onBack` cuando el router responde,
 * nunca antes de `minElapsedMs` (en demo /api/me responde al instante).
 */
export function useMeBack(active: boolean, minElapsedMs: number, onBack: () => void) {
  const startRef = useRef(0);
  const backRef = useRef(onBack);
  backRef.current = onBack;

  useEffect(() => {
    if (!active) return;
    startRef.current = Date.now();
    let stopped = false;
    const check = () => {
      api.me().then(() => {
        if (stopped || Date.now() - startRef.current < minElapsedMs) return;
        stopped = true;
        clearInterval(id);
        clearTimeout(to);
        backRef.current();
      }).catch(() => { /* el router aún no vuelve */ });
    };
    const id = setInterval(check, 5000);
    const to = setTimeout(check, minElapsedMs);
    return () => { stopped = true; clearInterval(id); clearTimeout(to); };
  }, [active, minElapsedMs]);
}

/** Cuenta atrás en segundos mientras `active`; no baja de 0. */
export function useCountdown(active: boolean, from: number) {
  const [left, setLeft] = useState(from);
  useEffect(() => {
    if (!active) return;
    setLeft(from);
    const id = setInterval(() => setLeft((s) => Math.max(0, s - 1)), 1000);
    return () => clearInterval(id);
  }, [active, from]);
  return left;
}
