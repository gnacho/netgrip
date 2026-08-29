import { useCallback, useEffect, useState, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import { CloudOff, FolderSymlink, HardDrive } from "lucide-react";
import { api } from "../api";
import type { StorageDevice, StorageProbe, StorageService } from "../types";
import {
  ActionBanner, Button, Card, EmptyState, Gauge, HelpTip, IconTile, KeyValue, Pill,
  Skeleton, SkeletonRows, Toggle,
} from "../components/ui";
import { IlluDisk } from "../components/ui/illustrations";
import { useActionCycle } from "../components/wifi/action";
import { fmtBytes } from "../lib/format";

/** Mapeo de nombre técnico → bloque i18n llano (storage.service.*). */
function serviceKey(name: string): "samba" | "minidlna" | "nfs" | undefined {
  const n = name.toLowerCase();
  if (n === "samba" || n === "samba4") return "samba";
  if (n === "minidlna" || n === "dlna") return "minidlna";
  if (n === "nfs") return "nfs";
  return undefined;
}

export function StoragePage() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<StorageProbe>();
  const [error, setError] = useState(false);

  const load = useCallback(async () => {
    setError(false);
    try {
      setProbe(await api.storage());
    } catch {
      setError(true);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const refresh = useCallback(() => {
    api.storage().then(setProbe).catch(() => {});
  }, []);

  // Página entera no aplica (hardware sin USB o feature off) y se entra por URL.
  if (probe && !probe.applicable) {
    return (
      <Card index={0}>
        <EmptyState
          illustration={<IlluDisk size={120} />}
          title={t("storage.notApplicable")}
        />
      </Card>
    );
  }

  const errorBox = (
    <EmptyState
      small
      illustration={<CloudOff size={24} />}
      title={t("common.loadError")}
      action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retry")}</Button>}
    />
  );

  return (
    <div className="flex flex-col gap-[var(--card-gap)]">
      {/* ════ Discos conectados (storage.md §2) ════ */}
      <Card
        index={0}
        title={t("storage.disksTitle")}
        icon={HardDrive}
        help="storage"
        action={probe && (
          <Pill tone="muted">{t("storage.diskCount", { count: probe.devices.length })}</Pill>
        )}
      >
        {error ? (
          errorBox
        ) : !probe ? (
          <div className="grid gap-3 sm:grid-cols-2">
            <Skeleton className="h-32" />
            <Skeleton className="h-32 hidden sm:block" />
          </div>
        ) : probe.devices.length === 0 ? (
          <EmptyState
            illustration={<IlluDisk size={120} />}
            title={t("storage.emptyTitle")}
            body={t("storage.emptyBody")}
          />
        ) : (
          <div className="grid gap-3 sm:grid-cols-2">
            {(probe.devices ?? []).map((d, i) => (
              <DiskCard key={d.path || d.name} disk={d} index={i} />
            ))}
          </div>
        )}
      </Card>

      {/* ════ Compartir en tu red (storage.md §3) ════ */}
      <Card index={1} title={t("storage.shareTitle")} icon={FolderSymlink}>
        {error ? (
          errorBox
        ) : !probe ? (
          <SkeletonRows rows={2} />
        ) : probe.services.length === 0 ? (
          <p className="text-small text-muted">{t("storage.noServices")}</p>
        ) : (
          <div className="flex flex-col divide-y divide-border/60">
            {(probe.services ?? []).map((svc) => (
              <ServiceRow key={svc.name} svc={svc} onChanged={refresh} />
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}

/* ══════════════ Mini-card de disco (storage.md §2) ══════════════ */

function DiskCard({ disk: d, index }: { disk: StorageDevice; index: number }) {
  const { t } = useTranslation();
  const pct = d.size_bytes > 0 ? Math.round((d.used_bytes / d.size_bytes) * 100) : 0;
  const usage = t("storage.usedOf", {
    used: fmtBytes(d.used_bytes),
    total: fmtBytes(d.size_bytes),
    pct,
  });
  const free = t("storage.freeOf", {
    free: fmtBytes(Math.max(0, d.size_bytes - d.used_bytes)),
    total: fmtBytes(d.size_bytes),
  });

  return (
    <div
      style={{ "--i": index, animationDelay: `${index * 60}ms` } as CSSProperties}
      className="animate-fade-up rounded-lg border border-border bg-surface-2 p-4"
    >
      <div className="flex items-center gap-2">
        <p title={d.name} className="flex-1 min-w-0 truncate text-body font-semibold">{d.name}</p>
        {d.fs_type && (
          <Pill tone="muted"><span className="font-mono">{d.fs_type}</span></Pill>
        )}
      </div>

      {/* Gauge circular (design-rev2 §5): % usado dentro; libre/total al lado */}
      <div className="mt-3 flex items-center gap-4">
        <Gauge value={pct} size="sm" label="%" ariaLabel={usage} />
        <div className="flex-1 min-w-0">
          <p title={free} className="text-small text-muted tabular-nums truncate">{free}</p>
          <div className="mt-1">
            {d.mount_point ? (
              <KeyValue items={[{ label: t("storage.mountPoint"), value: d.mount_point, mono: true }]} />
            ) : (
              <div className="py-1.5 flex flex-col gap-1.5 items-start">
                <Pill tone="warn">{t("storage.notMounted")}</Pill>
                <p className="text-small text-muted">{t("storage.notMountedHint")}</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

/* ══════════════ Fila de servicio (SettingRow + pill de estado) ══════════════ */

function ServiceRow({ svc, onChanged }: { svc: StorageService; onChanged: () => void }) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const [doneText, setDoneText] = useState<string>();

  const key = serviceKey(svc.name);
  const label = key ? t(`storage.service.${key}.label`) : svc.name;
  const desc = key ? t(`storage.service.${key}.desc`) : t("storage.serviceGeneric");

  const toggle = (on: boolean) => {
    setDoneText(t(on ? "storage.svcOnOk" : "storage.svcOffOk", { name: label }));
    run(async () => {
      const r = await api.setStorageService(svc.name, on ? "enable" : "disable");
      // El backend responde "ok"; el ciclo del banner entiende "applied".
      return { ...r, status: r.status === "ok" ? "applied" : r.status };
    }).then((res) => {
      if (res?.status === "applied") onChanged();
    });
  };

  return (
    <div className="py-3 first:pt-0 last:pb-0">
      <div className={`flex items-start gap-3 ${busy ? "opacity-70" : ""}`}>
        <IconTile icon={FolderSymlink} tone="accent" />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-1.5 flex-wrap">
            <p className="text-body font-medium">{label}</p>
            <span className="font-mono text-caption text-faint">{svc.name}</span>
            {key === "samba" && (
              <HelpTip title={t("help.samba.title")} body={t("help.samba.body")} />
            )}
          </div>
          <p className="text-small text-muted mt-0.5">{desc}</p>
        </div>
        <div className="flex items-center gap-2.5 shrink-0 pt-1">
          <Pill tone={svc.running ? "ok" : "muted"}>
            {svc.running ? t("storage.serviceOn") : t("storage.serviceOff")}
          </Pill>
          <Toggle checked={svc.enabled} busy={busy} onChange={toggle} label={label} />
        </div>
      </div>
      {phase && (
        <div className="mt-2">
          <ActionBanner phase={phase} text={phase === "done" ? doneText : undefined} detail={detail} onDone={clear} />
        </div>
      )}
    </div>
  );
}
