import { FlaskConical, Languages, Monitor, Moon, Rows3, Settings2, Sun } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { ReactNode } from "react";
import { useTheme, type ThemeChoice } from "../../hooks/useTheme";
import { useLabs } from "../../hooks/useLabs";
import { Card, HelpTip, SegmentedControl, Toggle } from "../ui";
import { useDensity, useLang } from "../ui/controls";

function Row({ icon, label, control }: { icon: ReactNode; label: ReactNode; control: ReactNode }) {
  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
      <span className="flex items-center gap-2 text-body font-medium text-text">
        <span className="text-muted">{icon}</span>
        {label}
      </span>
      <div className="sm:shrink-0">{control}</div>
    </div>
  );
}

/**
 * Preferencias de la interfaz (#158): idioma, densidad y tema. Las tres
 * elecciones viven aquí; en la barra superior solo queda el toggle
 * rápido claro/oscuro.
 */
export function OptionsCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const { lang, setLang } = useLang();
  const { density, setDensity } = useDensity();
  const { theme, setTheme } = useTheme();
  const { labs, setLabs } = useLabs();

  return (
    <Card index={index} title={t("options.title")} icon={Settings2}>
      <div className="flex flex-col gap-4">
        <Row
          icon={<Languages size={16} aria-hidden="true" />}
          label={t("options.language")}
          control={
            <SegmentedControl<"auto" | "es" | "en">
              ariaLabel={t("options.language")}
              value={lang}
              onChange={setLang}
              size="lg"
              options={[
                { value: "auto", label: t("options.langAuto") },
                { value: "es", label: "ES" },
                { value: "en", label: "EN" },
              ]}
            />
          }
        />
        <Row
          icon={<Rows3 size={16} aria-hidden="true" />}
          label={t("options.density")}
          control={
            <SegmentedControl<"comfortable" | "compact">
              ariaLabel={t("options.density")}
              value={density}
              onChange={setDensity}
              size="lg"
              options={[
                { value: "comfortable", label: t("options.comfortable") },
                { value: "compact", label: t("options.compact") },
              ]}
            />
          }
        />
        <Row
          icon={theme === "dark" ? <Moon size={16} aria-hidden="true" /> : theme === "light" ? <Sun size={16} aria-hidden="true" /> : <Monitor size={16} aria-hidden="true" />}
          label={t("options.theme")}
          control={
            <SegmentedControl<ThemeChoice>
              ariaLabel={t("options.theme")}
              value={theme}
              onChange={setTheme}
              size="lg"
              options={[
                { value: "light", label: t("theme.light") },
                { value: "auto", label: t("theme.system") },
                { value: "dark", label: t("theme.dark") },
              ]}
            />
          }
        />
        <Row
          icon={<FlaskConical size={16} aria-hidden="true" />}
          label={
            <span className="flex items-center gap-1.5">
              {t("options.labs")}
              <HelpTip title={t("options.labs")} body={t("options.labsHint")} />
            </span>
          }
          control={<Toggle checked={labs} onChange={setLabs} label={t("options.labs")} />}
        />
      </div>
    </Card>
  );
}
