import { useState } from "react";
import { Monitor, Moon, Rows2, Rows3, Sun } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useTheme, type ThemeChoice } from "../../hooks/useTheme";
import { SegmentedControl } from "./SegmentedControl";

export function ThemeToggle({ className = "" }: { className?: string }) {
  const { theme, setTheme } = useTheme();
  return (
    <div className={className}>
      <SegmentedControl<ThemeChoice>
        size="sm"
        ariaLabel="Theme"
        value={theme}
        onChange={setTheme}
        options={[
          { value: "light", label: <Sun size={14} /> },
          { value: "auto", label: <Monitor size={14} /> },
          { value: "dark", label: <Moon size={14} /> },
        ]}
      />
    </div>
  );
}

type Density = "comfortable" | "compact";
const DENSITY_KEY = "netgrip:density";

function currentDensity(): Density {
  return document.documentElement.dataset.density === "compact" ? "compact" : "comfortable";
}

/**
 * Toggle de densidad (design-rev2 §2): comfortable por defecto; aplica
 * data-density en <html> y persiste en localStorage("netgrip:density").
 */
export function DensityToggle({ className = "" }: { className?: string }) {
  const { t } = useTranslation();
  const [density, setDensity] = useState<Density>(currentDensity);
  const toggle = () => {
    const next: Density = currentDensity() === "compact" ? "comfortable" : "compact";
    document.documentElement.dataset.density = next;
    try {
      localStorage.setItem(DENSITY_KEY, next);
    } catch { /* sin persistencia */ }
    setDensity(next);
  };
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

export function LangToggle({ className = "" }: { className?: string }) {
  const { i18n } = useTranslation();
  const [choice, setChoice] = useState<LangChoice>(storedLang);

  const apply = (v: LangChoice) => {
    setChoice(v);
    try { localStorage.setItem(LANG_KEY, v); } catch { /* sin persistencia */ }
    if (v === "auto") {
      const browserLang = navigator.language.startsWith("es") ? "es" : "en";
      i18n.changeLanguage(browserLang);
    } else {
      i18n.changeLanguage(v);
    }
  };

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
        value={choice}
        onChange={apply}
      />
    </div>
  );
}
