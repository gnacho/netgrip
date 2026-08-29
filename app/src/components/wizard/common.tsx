import type { ReactNode } from "react";
import { ChevronLeft } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "../ui";

/** Lo que el wizard ha aplicado en esta sesión (alimenta el resumen final). */
export interface WizardRecord {
  mode?: "router" | "ap";
  password?: boolean;
  wifi?: { ssid: string; enc: string };
  guest?: string;
  iot?: string;
  wg?: boolean;
  pkgs?: string[];
}

/**
 * Esqueleto de paso (wizard.md §1): ilustración centrada (fade-up con delay
 * 60ms), título display, una frase llana de contexto y el contenido.
 */
export function StepShell({ illustration, title, body, children, footer }: {
  illustration: ReactNode;
  title: string;
  body?: string;
  children?: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <div>
      <div
        className="flex justify-center text-faint animate-fade-up"
        style={{ animationDelay: "60ms" }}
      >
        {illustration}
      </div>
      <h1 className="text-display text-center mt-4">{title}</h1>
      {body && (
        <p className="text-body text-muted text-center mt-2 max-w-[52ch] mx-auto">{body}</p>
      )}
      {children && <div className="mt-7">{children}</div>}
      {footer}
    </div>
  );
}

/** Pie estándar: Atrás (ghost) + Continuar (primary con spinner) + "Saltar este paso →". */
export function StepFooter({ onBack, onNext, busy = false, nextDisabled = false, nextLabel, onSkip, fails = 0 }: {
  onBack?: () => void;
  onNext: () => void;
  busy?: boolean;
  nextDisabled?: boolean;
  nextLabel?: string;
  onSkip?: () => void;
  /** nº de fallos del paso; ≥2 muestra el consuelo "configúralo luego" (wizard.md §5) */
  fails?: number;
}) {
  const { t } = useTranslation();
  return (
    <div className="mt-8">
      <div className="flex items-center justify-between gap-3">
        {onBack ? (
          <Button variant="ghost" icon={ChevronLeft} onClick={onBack} disabled={busy}>
            {t("wizard.back")}
          </Button>
        ) : (
          <span />
        )}
        <Button onClick={onNext} loading={busy} disabled={nextDisabled}>
          {nextLabel ?? t("wizard.continue")}
        </Button>
      </div>
      {onSkip && (
        <div className="mt-4 text-center">
          {fails >= 2 && (
            <p className="text-small text-muted mb-1">{t("wizard.skipLater")}</p>
          )}
          <button
            type="button"
            onClick={onSkip}
            disabled={busy}
            className="text-small text-muted hover:text-text ring-focus rounded-sm transition-colors duration-[var(--dur-fast)] disabled:opacity-50"
          >
            {t("wizard.skip")} →
          </button>
        </div>
      )}
    </div>
  );
}

/**
 * Medidor de fuerza propio (wizard.md §3.3): barra 4px en 3 tramos
 * (débil danger / correcta warn / buena ok), heurística local por
 * longitud y variedad, sin librería.
 */
export function StrengthMeter({ password }: { password: string }) {
  const { t } = useTranslation();
  if (!password) return null;
  let variety = 0;
  if (/[a-z]/.test(password)) variety++;
  if (/[A-Z]/.test(password)) variety++;
  if (/\d/.test(password)) variety++;
  if (/[^a-zA-Z0-9]/.test(password)) variety++;
  let score = 0;
  if (password.length >= 8) score++;
  if (password.length >= 12) score++;
  if (variety >= 3) score++;
  const level = score <= 1 ? 1 : score === 2 ? 2 : 3;
  const color = level === 1 ? "bg-danger" : level === 2 ? "bg-warn" : "bg-ok";
  const label = level === 1
    ? t("wizard.password.weak")
    : level === 2
      ? t("wizard.password.fair")
      : t("wizard.password.good");
  return (
    <div className="mt-1.5">
      <div className="flex gap-1" aria-hidden="true">
        {[1, 2, 3].map((i) => (
          <span
            key={i}
            className={`h-1 flex-1 rounded-full transition-colors duration-200 ${i <= level ? color : "bg-border"}`}
          />
        ))}
      </div>
      <p className="text-caption text-muted mt-1">{label}</p>
    </div>
  );
}

/** Aparición con altura animada 200ms (truco grid-rows, como AdvancedDisclosure). */
export function Reveal({ open, children }: { open: boolean; children: ReactNode }) {
  return (
    <div
      className={`grid transition-[grid-template-rows] duration-200 ease-[var(--ease-soft)] ${
        open ? "grid-rows-[1fr]" : "grid-rows-[0fr]"
      }`}
    >
      <div className="overflow-hidden">{children}</div>
    </div>
  );
}

/** Generador local de claves (16 chars, sin caracteres ambiguos, sin dependencias). */
export function genKey(len = 16): string {
  const chars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789";
  const buf = new Uint32Array(len);
  crypto.getRandomValues(buf);
  return Array.from(buf, (n) => chars[n % chars.length]).join("");
}

/** encryption cruda de hostapd → etiqueta llana para el resumen. */
export function encLabel(enc: string): string {
  if (enc === "sae") return "WPA3";
  if (enc === "sae-mixed") return "WPA2/WPA3";
  if (enc.startsWith("psk")) return "WPA2";
  return enc.toUpperCase();
}
