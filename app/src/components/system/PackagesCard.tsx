import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Package, RefreshCw } from "lucide-react";
import { api } from "../../api";
import type { PkgUpgrade } from "../../types";
import { ActionBanner, Button, Card, Pill, SkeletonRows, useToast } from "../ui";
import { useActionCycle } from "../wifi/action";

export function PackagesCard({ upgradable, onChange, index = 1 }: {
  upgradable: PkgUpgrade[] | undefined;
  onChange: (p: PkgUpgrade[]) => void;
  index?: number;
}) {
  const { t } = useTranslation();
  const { push } = useToast();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [busyOne, setBusyOne] = useState<string>();
  const [refreshing, setRefreshing] = useState(false);

  const refresh = async () => {
    setRefreshing(true);
    try {
      const result = await api.packages();
      onChange(result.upgradable);
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : t("pkg.failed") });
    } finally {
      setRefreshing(false);
    }
  };

  const upgradeOne = async (name: string) => {
    setBusyOne(name);
    try {
      const result = await api.upgradePackage(name);
      onChange(result.upgradable);
      push({ tone: "ok", text: t("pkg.done", { name }) });
    } catch (e) {
      push({ tone: "danger", text: e instanceof Error ? e.message : t("pkg.failed") });
    } finally {
      setBusyOne(undefined);
    }
  };

  const upgradeAll = () => {
    const names = (upgradable ?? []).map((p) => p.name);
    run(async () => {
      let remaining = upgradable ?? [];
      for (const name of names) {
        const r = await api.upgradePackage(name);
        remaining = r.upgradable;
      }
      if (remaining.length > 0) {
        return { status: "failed", error: remaining.map((p) => p.name).join(", ") };
      }
      return { status: "applied" as const };
    }).then((res) => {
      if (res?.status === "applied") onChange([]);
    });
  };

  return (
    <Card index={index} title={t("pkg.title")} icon={Package} help="packages"
      action={upgradable && upgradable.length > 0
        ? <Pill tone="warn">{t("pkg.count", { count: upgradable.length })}</Pill>
        : upgradable ? <Pill tone="ok">{t("pkg.upToDate")}</Pill> : undefined}
    >
      {upgradable === undefined ? (
        <SkeletonRows rows={3} />
      ) : upgradable.length === 0 ? (
        <div className="flex items-center justify-between gap-2">
          <p className="text-small text-muted">{t("pkg.upToDate")}</p>
          <Button variant="ghost" size="sm" icon={RefreshCw} onClick={refresh} loading={refreshing}>
            {t("update.checkNow")}
          </Button>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          <div className="max-h-56 overflow-y-auto">
            {upgradable.map((p) => (
              <div key={p.name} className="flex items-center gap-2 py-2 border-b border-border/60 last:border-0">
                <div className="flex-1 min-w-0">
                  <span className="block truncate font-mono text-small">{p.name}</span>
                  <span className="block text-caption text-muted font-mono">{p.current} → {p.available}</span>
                </div>
                <Button variant="secondary" size="sm" onClick={() => upgradeOne(p.name)}
                  disabled={busy || busyOne !== undefined} loading={busyOne === p.name}>
                  {t("pkg.upgrade")}
                </Button>
              </div>
            ))}
          </div>
          <div>
            <Button onClick={upgradeAll} disabled={busy || busyOne !== undefined}>{t("pkg.upgradeAll")}</Button>
          </div>
          {phase && (
            <ActionBanner
              phase={phase}
              text={phase === "done" ? t("pkg.allDone") : undefined}
              detail={detail}
              onDone={clear}
            />
          )}
        </div>
      )}
    </Card>
  );
}
