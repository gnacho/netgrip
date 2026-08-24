import { useState } from "react";
import { useTranslation } from "react-i18next";
import { PackageOpen, RefreshCw } from "lucide-react";
import { api } from "../api";
import type { PkgUpgrade } from "../types";
import { Card, Pill } from "./Card";

export function PackagesCard({ upgradable, onChange }: {
  upgradable: PkgUpgrade[] | undefined;
  onChange: (p: PkgUpgrade[]) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState<string>();
  const [refreshing, setRefreshing] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  const refresh = async () => {
    setRefreshing(true);
    setMsg(undefined);
    try {
      const result = await api.packages();
      onChange(result.upgradable);
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("pkg.failed") });
    } finally {
      setRefreshing(false);
    }
  };

  const upgrade = async (name: string) => {
    setBusy(name);
    setMsg(undefined);
    try {
      const result = await api.upgradePackage(name);
      onChange(result.upgradable);
      setMsg({ tone: "ok", text: t("pkg.done", { name }) });
    } catch (e) {
      setMsg({ tone: "danger", text: e instanceof Error ? e.message : t("pkg.failed") });
    } finally {
      setBusy(undefined);
    }
  };

  return (
    <Card title={t("pkg.title")} icon={PackageOpen} action={
      <button onClick={refresh} disabled={refreshing} className="text-muted hover:text-text" title={t("nav.refresh")}>
        <RefreshCw size={14} className={refreshing ? "animate-spin" : ""} />
      </button>
    }>
      {upgradable === undefined ? (
        <p className="text-sm text-muted">{t("update.checking")}</p>
      ) : upgradable.length === 0 ? (
        <Pill tone="ok">{t("update.upToDate")}</Pill>
      ) : (
        <>
          <Pill tone="warn">{t("pkg.count", { count: upgradable.length })}</Pill>
          <div className="mt-2 max-h-64 overflow-y-auto">
            {upgradable.map((p) => (
              <div key={p.name} className="flex items-center gap-2 py-1.5 border-b border-border/50 last:border-0 text-sm">
                <div className="flex-1 min-w-0">
                  <span className="block truncate">{p.name}</span>
                  <span className="block text-xs text-muted">{p.current} → {p.available}</span>
                </div>
                <button onClick={() => upgrade(p.name)} disabled={busy !== undefined}
                  className="text-xs bg-border hover:bg-border/70 disabled:opacity-40 rounded-lg px-2.5 py-1 shrink-0">
                  {busy === p.name ? "…" : t("pkg.upgrade")}
                </button>
              </div>
            ))}
          </div>
        </>
      )}
      {msg && <p className={`text-xs mt-2 ${msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{msg.text}</p>}
    </Card>
  );
}
