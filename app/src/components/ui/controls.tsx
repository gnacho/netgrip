import { useCallback, useState } from "react";
import { Moon, Rows2, Rows3, Sun } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useTheme } from "../../hooks/useTheme";
import { SegmentedControl } from "./SegmentedControl";

/**
 * Botón único de tema (#158): alterna claro/oscuro en un clic. La elección
 * triple (claro/sistema/oscuro) vive en Sistema > Opciones.
 */
export function ThemeToggle({ className = "" }: { className?: string }) {
  const { isDark, toggle } = useTheme();
  const { t } = useTranslation();
  const label = t("theme.toggle");
  return (
    <button
      type="button"
      onClick={toggle}
      title={label}
      aria-label={label}
      aria-pressed={isDark}
      className={`inline-flex h-9 w-9 items-center justify-center rounded-md text-muted hover:text-text hover:bg-surface-2 ring-focus transition-colors ${className}`}
    >
      {isDark ? <Sun size={18} /> : <Moon size={18} />}
    </button>
  );
}

type Density = "comfortable" | "compact";
const DENSITY_KEY = "netgrip:density";

function currentDensity(): Density {
  return document.documentElement.dataset.density === "compact" ? "compact" : "comfortable";
}

/**
 * Densidad (design-rev2 §2): comfortable por defecto; aplica
 * data-density en <html> y persiste en localStorage("netgrip:density").
 */
export function useDensity() {
  const [density, setDensityState] = useState<Density>(currentDensity);
  const setDensity = useCallback((next: Density) => {
    document.documentElement.dataset.density = next;
    try {
      localStorage.setItem(DENSITY_KEY, next);
    } catch { /* sin persistencia */ }
    setDensityState(next);
  }, []);
  return { density, setDensity };
}

export function DensityToggle({ className = "" }: { className?: string }) {
  const { t } = useTranslation();
  const { density, setDensity } = useDensity();
  const toggle = () => setDensity(density === "compact" ? "comfortable" : "compact");
  const label = t(density === "compact" ? "density.comfortable" : "density.compact");
  return (
    <button
      type="button"
      onClick={toggle}
      title={label}
      aria-label={label}
      className={`inline-flex h-9 w-9 items-center justify-center rounded-md text-muted hover:text-text hover:bg-surface-2 ring-focus transition-colors ${className}`}
    >
      {density === "compact" ? <Rows2 size={18} /> : <Rows3 size={18} />}
    </button>
  );
}

type LangChoice = "auto" | "es" | "en";
const LANG_KEY = "netgrip:lang";

function storedLang(): LangChoice {
  try {
    const v = localStorage.getItem(LANG_KEY);
    if (v === "es" || v === "en") return v;
  } catch { /* sin persistencia */ }
  return "auto";
}

export function useLang() {
  const { i18n } = useTranslation();
  const [choice, setChoice] = useState<LangChoice>(storedLang);

  const apply = useCallback((v: LangChoice) => {
    setChoice(v);
    try { localStorage.setItem(LANG_KEY, v); } catch { /* sin persistencia */ }
    if (v === "auto") {
      const browserLang = navigator.language.startsWith("es") ? "es" : "en";
      i18n.changeLanguage(browserLang);
    } else {
      i18n.changeLanguage(v);
    }
  }, [i18n]);

  return { lang: choice, setLang: apply };
}

export function LangToggle({ className = "" }: { className?: string }) {
  const { lang, setLang } = useLang();
  return (
    <div className={className}>
      <SegmentedControl<LangChoice>
        size="sm"
        ariaLabel="Idioma / Language"
        options={[
          { value: "auto", label: "Auto" },
          { value: "es", label: "ES" },
          { value: "en", label: "EN" },
        ]}
        value={lang}
        onChange={setLang}
      />
    </div>
  );
}
