import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { IconTile, type Tone } from "./IconTile";
import { Toggle } from "./Toggle";
import { HelpTip } from "./HelpTip";

/**
 * SettingRow (design-rev2 §3) — SIEMPRE una única línea horizontal:
 * [IconTile?] [título + descripción (flex-1 min-w-0)] [control].
 * El título hace ellipsis (con title del texto completo); la descripción va
 * debajo, dentro del mismo bloque, con line-clamp-1 en <640px, y NUNCA
 * empuja el control a otra fila.
 */
export function SettingRow({ icon, iconTone = "accent", title, description, help, helpTitle, checked, onChange, busy = false, disabled = false, disabledReason, control }: {
  icon?: LucideIcon;
  iconTone?: Tone;
  title: string;
  description: string;
  /** texto de ayuda (con helpTitle) para el HelpTip opcional */
  help?: string;
  helpTitle?: string;
  checked?: boolean;
  onChange?: (v: boolean) => void;
  busy?: boolean;
  disabled?: boolean;
  /** razón del disabled en small ("Solo disponible en modo router") */
  disabledReason?: string;
  /** control alternativo al Toggle (botón, segmented…) */
  control?: ReactNode;
}) {
  return (
    <div
      className={`flex items-center gap-3 ${busy ? "opacity-70" : ""}`}
      style={{ minHeight: "var(--row-min-h)", paddingBlock: "var(--row-pad-y)" }}
    >
      {icon && <IconTile icon={icon} tone={iconTone} />}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1 min-w-0">
          <p className="text-body font-medium whitespace-nowrap overflow-hidden text-ellipsis" title={title}>
            {title}
          </p>
          {help && helpTitle && <HelpTip title={helpTitle} body={help} />}
        </div>
        <p className="text-small text-muted mt-0.5 line-clamp-1 sm:line-clamp-2">{description}</p>
        {disabled && disabledReason && (
          <p className="text-small text-faint mt-0.5 line-clamp-1">{disabledReason}</p>
        )}
      </div>
      <div className="shrink-0">
        {control ?? (
          <Toggle
            checked={!!checked}
            busy={busy}
            disabled={disabled}
            onChange={(v) => onChange?.(v)}
            label={title}
          />
        )}
      </div>
    </div>
  );
}
