import { useCallback, useEffect, useState, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import { TriangleAlert, Wand2 } from "lucide-react";
import { api } from "../../api";
import type { Template } from "../../types";
import { ActionBanner, Button, Card, ConfirmDialog, Pill, SkeletonRows } from "../ui";
import { useActionCycle } from "../wifi/action";
import { asApplied } from "./shared";
import { CardLoadError } from "./diagnostics";

/**
 * Plantillas de configuración (tools.md §4): aplica sets completos con
 * descripción; las destructivas piden ConfirmDialog danger con consecuencia.
 */
export function TemplatesCard() {
  const { t } = useTranslation();
  const [templates, setTemplates] = useState<Template[]>();
  const [error, setError] = useState(false);
  const [confirmTpl, setConfirmTpl] = useState<Template>();
  const [busyId, setBusyId] = useState<string>();
  const cycle = useActionCycle();

  const load = useCallback(async () => {
    setError(false);
    try {
      const r = await api.templates();
      setTemplates(r.templates ?? []);
    } catch { setError(true); }
  }, []);
  useEffect(() => { load(); }, [load]);

  const apply = (tpl: Template) => {
    setBusyId(tpl.id);
    cycle.run(async () => asApplied(await api.applyTemplate(tpl.id, tpl.destructive)))
      .finally(() => setBusyId(undefined));
  };

  const onApply = (tpl: Template) => {
    if (tpl.destructive) setConfirmTpl(tpl);
    else apply(tpl);
  };

  return (
    <Card variant="subtle" animate={false} title={t("templates.configTitle")} icon={Wand2} iconTone="violet">
      <p className="text-small text-muted mb-3">{t("templates.intro")}</p>

      {error ? (
        <CardLoadError onRetry={load} />
      ) : !templates ? (
        <SkeletonRows rows={2} />
      ) : templates.length === 0 ? (
        <p className="text-small text-muted">{t("templates.empty")}</p>
      ) : (
        <div className="flex flex-col gap-2">
          {templates.map((tpl, i) => (
            <div key={tpl.id} style={{ "--i": Math.min(i, 7) } as CSSProperties}
              className="animate-fade-up flex items-start gap-3 rounded-md border border-border bg-surface px-3 py-2.5">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-body font-medium">{tpl.name}</span>
                  {tpl.destructive && (
                    <Pill tone="warn">
                      <TriangleAlert size={12} aria-hidden="true" />
                      {t("templates.destructive")}
                    </Pill>
                  )}
                </div>
                <p className="text-small text-muted mt-0.5">{tpl.description}</p>
              </div>
              <Button
                variant={tpl.destructive ? "danger" : "secondary"}
                size="sm"
                className="shrink-0"
                loading={cycle.busy && busyId === tpl.id}
                onClick={() => onApply(tpl)}
              >
                {t("templates.apply")}
              </Button>
            </div>
          ))}
        </div>
      )}

      {cycle.phase && (
        <div className="mt-3">
          <ActionBanner
            phase={cycle.phase}
            text={cycle.phase === "done" ? t("templates.applied") : undefined}
            detail={cycle.detail}
            onDone={cycle.clear}
          />
        </div>
      )}

      <ConfirmDialog
        open={!!confirmTpl}
        onClose={() => setConfirmTpl(undefined)}
        onConfirm={() => { const tpl = confirmTpl!; setConfirmTpl(undefined); apply(tpl); }}
        title={t("templates.applyConfirmTitle", { name: confirmTpl?.name ?? "" })}
        consequence={t("templates.applyConsequence")}
        confirmLabel={t("templates.applyGo")}
      />
    </Card>
  );
}
