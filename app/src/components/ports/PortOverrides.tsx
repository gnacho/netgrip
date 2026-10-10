import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { AdvancedDisclosure, Button } from "../ui";

/**
 * Patron compartido "ajuste global + excepciones por boca" (#487) usado por
 * el control de tormentas, los ajustes STP por boca y las listas MAC: una
 * seccion colapsable que solo lista las bocas con ajuste propio y un
 * picker para anadir bocas una a una. Nunca lista las 52 bocas del chasis.
 */

/** Seccion colapsable con las bocas que se salen del ajuste global. */
export function PortOverrides({ count, children }: { count: number; children: ReactNode }) {
  const { t } = useTranslation();
  return (
    <AdvancedDisclosure label={t("ports.overrideTitle", { count })}>
      <div className="flex flex-col gap-2 pt-1">
        {children}
      </div>
    </AdvancedDisclosure>
  );
}

/** Select + boton para anadir una boca a la lista de excepciones. */
export function PortAddPicker({ candidates, onAdd }: {
  candidates: string[];
  onAdd: (port: string) => void;
}) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState("");
  if (candidates.length === 0) return null;

  const add = () => {
    if (!selected) return;
    onAdd(selected);
    setSelected("");
  };

  return (
    <div className="flex items-center gap-2">
      <select
        value={selected}
        onChange={(e) => setSelected(e.target.value)}
        aria-label={t("ports.pickPort")}
        className="h-9 w-44 max-w-full rounded-sm border border-border bg-surface-2 px-3 text-small outline-none focus:border-accent ring-focus"
      >
        <option value="">{t("ports.pickPort")}</option>
        {candidates.map((p) => <option key={p} value={p}>{p}</option>)}
      </select>
      <Button variant="secondary" size="sm" icon={Plus} disabled={!selected} onClick={add}>
        {t("ports.addPort")}
      </Button>
    </div>
  );
}
