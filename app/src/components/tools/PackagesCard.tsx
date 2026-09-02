import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Package, Trash2 } from "lucide-react";
import { api } from "../../api";
import type { OptionalPackage } from "../../types";
import { Button, Card, ConfirmDialog, Pill, SkeletonRows, useToast } from "../ui";

/**
 * Paquetes opcionales (issue #202): catálogo con estado real, instalación y
 * desinstalación por entrada (y "quitar todos"). Solo entradas del catálogo:
 * el backend nunca toca paquetes del sistema.
 */
export function PackagesCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const { push } = useToast();
  const [pkgs, setPkgs] = useState<OptionalPackage[]>();
  const [busyId, setBusyId] = useState<string>();
  const [delTarget, setDelTarget] = useState<OptionalPackage>();
  const [delAll, setDelAll] = useState(false);

  const load = useCallback(() => {
    api.optionalPackages().then((r) => setPkgs(r.packages)).catch(() => {});
  }, []);

  useEffect(() => { load(); }, [load]);

  if (!pkgs) return <Card index={index} title={t("packages.cardTitle")} icon={Package}><SkeletonRows rows={3} /></Card>;

  const installed = pkgs.filter((p) => p.installed);

  const install = async (id: string) => {
    setBusyId(id);
    try {
      await api.wizardPackages([id]);
      load();
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally { setBusyId(undefined); }
  };

  const remove = async (ids: string[]) => {
    setBusyId(ids.join(","));
    try {
      const res = await api.removePackages(ids);
      if (res.error) push({ tone: "danger", text: res.error });
      else push({ tone: "ok", text: t("packages.removed") });
      setPkgs(res.packages);
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusyId(undefined);
      setDelTarget(undefined);
      setDelAll(false);
    }
  };

  return (
    <Card index={index} title={t("packages.cardTitle")} icon={Package}
      action={installed.length > 0 ? <Pill tone="accent">{t("packages.installedCount", { count: installed.length })}</Pill> : undefined}>
      <p className="text-small text-muted -mt-1 mb-2">{t("packages.desc")}</p>
      <div className="divide-y divide-border/60">
        {pkgs.map((p) => (
          <div key={p.id} className="flex items-center gap-3 py-2.5">
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="font-medium">{t(p.i18n_key)}</span>
                {p.installed && <Pill tone="ok">{t("packages.installed")}</Pill>}
              </div>
              <p className="text-caption text-muted mt-0.5 font-mono truncate">{p.packages.join(", ")}</p>
            </div>
            {p.installed ? (
              <Button variant="ghost" size="sm"
                className="text-danger hover:text-danger hover:bg-danger/10"
                disabled={busyId === p.id}
                onClick={() => setDelTarget(p)}
                aria-label={t("packages.remove", { name: t(p.i18n_key) })}>
                <Trash2 size={16} />
              </Button>
            ) : (
              <Button variant="secondary" size="sm"
                loading={busyId === p.id}
                onClick={() => install(p.id)}>
                {t("packages.install")}
              </Button>
            )}
          </div>
        ))}
      </div>

      {installed.length > 0 && (
        <div className="mt-3 pt-3 border-t border-border/60">
          <Button variant="ghost" size="sm"
            className="text-danger hover:text-danger hover:bg-danger/10"
            loading={busyId === installed.map((p) => p.id).join(",")}
            onClick={() => setDelAll(true)}>
            <Trash2 size={16} /> {t("packages.removeAll")}
          </Button>
        </div>
      )}

      <ConfirmDialog
        open={!!delTarget}
        title={t("packages.remove", { name: delTarget ? t(delTarget.i18n_key) : "" })}
        consequence={t("packages.removeBody", { pkgs: delTarget?.packages.join(", ") ?? "" })}
        confirmLabel={t("packages.removeConfirm")}
        busy={!!busyId}
        onClose={() => setDelTarget(undefined)}
        onConfirm={() => delTarget && remove([delTarget.id])}
      />
      <ConfirmDialog
        open={delAll}
        title={t("packages.removeAll")}
        consequence={t("packages.removeAllBody", { count: installed.length })}
        confirmLabel={t("packages.removeAllConfirm")}
        busy={!!busyId}
        onClose={() => setDelAll(false)}
        onConfirm={() => remove(installed.map((p) => p.id))}
      />
    </Card>
  );
}
