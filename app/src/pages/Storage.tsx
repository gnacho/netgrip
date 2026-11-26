import { useCallback, useEffect, useState, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import { CloudOff, FolderSymlink, HardDrive } from "lucide-react";
import { api } from "../api";
import type { StorageDevice, StorageProbe, StorageService } from "../types";
import {
  ActionBanner, Banner, Button, Card, ConfirmDialog, EmptyState, Field, Gauge, HelpTip,
  IconTile, KeyValue, Modal, Pill, Skeleton, SkeletonRows, Toggle,
} from "../components/ui";
import { IlluDisk } from "../components/ui/illustrations";
import { InstallProgress, useInstallJob } from "../components/wizard/common";
import type { InstallJob } from "../api";
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

  // #737: un USB enchufado sin controlador se explica y se arregla desde aquí.
  const { begin, running: driverInstalling, job: driverJob } = useInstallJob();
  const installDriver = async () => {
    try {
      await begin(() => api.wizardPackages(["usbstorage"]));
    } finally {
      refresh();
    }
  };

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
        {probe?.usb_present && !probe.usb_storage_loaded && (
          <Banner tone="warn" className="mb-3"
            action={
              <Button size="sm" variant="secondary" loading={driverInstalling} onClick={installDriver}>
                {t("storage.usbInstallDriver")}
              </Button>
            }>
            {t("storage.usbNoDriver")}
          </Banner>
        )}
        {driverInstalling && <div className="mb-3"><InstallProgress job={driverJob} /></div>}
        {error ? (
          errorBox
        ) : !probe ? (
          <div className="grid gap-3 sm:grid-cols-2">
            <Skeleton className="h-32" />
            <Skeleton className="h-32 hidden sm:block" />
          </div>
        ) : probe.devices.length === 0 ? (
          // Con un USB enchufado sin controlador, el aviso de arriba ya lo
          // explica: decir "no hay nada enchufado" sería mentira (#737).
          probe.usb_present && !probe.usb_storage_loaded ? null : (
            <EmptyState
              illustration={<IlluDisk size={120} />}
              title={t("storage.emptyTitle")}
              body={t("storage.emptyBody")}
            />
          )
        ) : (
          <div className="grid gap-3 sm:grid-cols-2">
            {(probe.devices ?? []).map((d, i) => (
              <DiskCard key={d.path || d.name} disk={d} index={i} onChanged={refresh} />
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
              <ServiceRow key={svc.name} svc={svc} onChanged={refresh}
                mounts={(probe.devices ?? []).map((d) => d.mount_point).filter((m): m is string => !!m)} />
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}

/* ══════════════ Mini-card de disco (storage.md §2) ══════════════ */

function DiskCard({ disk: d, index, onChanged }: { disk: StorageDevice; index: number; onChanged: () => void }) {
  const { t } = useTranslation();
  const [mountOpen, setMountOpen] = useState(false);
  const [target, setTarget] = useState(`/mnt/${d.name}`);
  const [readOnly, setReadOnly] = useState(false);
  const [unmountOpen, setUnmountOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string>();

  const targetError = target.trim() !== "" && !/^\/mnt\/[A-Za-z0-9._-]+$/.test(target.trim())
    ? t("storage.mountTargetInvalid")
    : undefined;

  const mount = async () => {
    setBusy(true);
    setActionError(undefined);
    try {
      await api.mountStorage({ device: d.path, target: target.trim(), read_only: readOnly });
      setMountOpen(false);
      onChanged();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const unmount = async () => {
    if (!d.mount_point) return;
    setBusy(true);
    setActionError(undefined);
    setUnmountOpen(false);
    try {
      await api.unmountStorage(d.mount_point);
      onChanged();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

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
              <div className="flex items-center gap-2">
                <KeyValue items={[{ label: t("storage.mountPoint"), value: d.mount_point, mono: true }]} />
                {d.managed && (
                  <Button size="sm" variant="ghost" loading={busy} onClick={() => setUnmountOpen(true)}>
                    {t("storage.unmount")}
                  </Button>
                )}
              </div>
            ) : (
              <div className="py-1.5 flex flex-col gap-1.5 items-start">
                <Pill tone="warn">{t("storage.notMounted")}</Pill>
                <p className="text-small text-muted">{t("storage.notMountedHint")}</p>
                <Button size="sm" variant="secondary" onClick={() => setMountOpen(true)}>
                  {t("storage.mount")}
                </Button>
              </div>
            )}
          </div>
        </div>
      </div>

      {actionError && <p className="mt-2 text-caption text-danger">{actionError}</p>}

      <Modal
        open={mountOpen}
        onClose={() => setMountOpen(false)}
        title={t("storage.mountTitle")}
        footer={
          <>
            <Button variant="ghost" onClick={() => setMountOpen(false)}>{t("common.cancel")}</Button>
            <Button loading={busy} disabled={!target.trim() || !!targetError} onClick={mount}>
              {t("storage.mountConfirm")}
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <p className="text-small text-muted">{t("storage.mountHint", { device: d.path })}</p>
          <Field label={t("storage.mountTarget")} mono error={targetError} hint={t("storage.mountTargetHint")}
            inputProps={{ value: target, onChange: (e) => setTarget(e.target.value), autoComplete: "off" }} />
          <div className="flex items-center justify-between gap-3 py-1">
            <div className="min-w-0">
              <p className="text-body font-medium">{t("storage.mountReadOnly")}</p>
              <p className="text-small text-muted">{t("storage.mountReadOnlyHint")}</p>
            </div>
            <Toggle checked={readOnly} onChange={setReadOnly} label={t("storage.mountReadOnly")} />
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={unmountOpen}
        onClose={() => setUnmountOpen(false)}
        onConfirm={unmount}
        title={t("storage.unmountTitle", { target: d.mount_point ?? "" })}
        consequence={t("storage.unmountConsequence")}
        confirmLabel={t("storage.unmount")}
        busy={busy}
      />
    </div>
  );
}

/* ══════════════ Fila de servicio (SettingRow + pill de estado) ══════════════ */

function ServiceRow({ svc, onChanged, mounts }: { svc: StorageService; onChanged: () => void; mounts: string[] }) {
  const { t } = useTranslation();
  const { phase, detail, busy, run, clear } = useActionCycle();
  const { begin, running: installing, job } = useInstallJob();
  const [doneText, setDoneText] = useState<string>();
  const [folderOpen, setFolderOpen] = useState(false);
  const [selected, setSelected] = useState<string[]>(svc.paths ?? []);
  const [pathsBusy, setPathsBusy] = useState(false);
  const [pathsError, setPathsError] = useState<string>();

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

  // #720: sin el paquete el interruptor no tiene nada que encender.
  const install = async () => {
    try {
      const res = await begin(async () => {
        const r = await api.installStorageService(svc.name);
        return { job: r.job ?? ({ phase: "done", total: 0, done: 0 } as InstallJob) };
      });
      if (res.phase === "done") {
        setDoneText(t("storage.svcInstalled", { name: label }));
        onChanged();
      }
    } catch {
      /* el progreso del job muestra el fallo */
    }
  };

  // #720: minidlna comparte carpetas concretas; se eligen entre los discos.
  const canPickFolders = (svc.name === "minidlna" || svc.name === "samba4") && svc.installed;
  const toggleFolder = (path: string) =>
    setSelected((cur) => (cur.includes(path) ? cur.filter((p) => p !== path) : [...cur, path]));

  const saveFolders = async () => {
    setPathsBusy(true);
    setPathsError(undefined);
    try {
      await api.setStoragePaths(svc.name, selected);
      setFolderOpen(false);
      onChanged();
    } catch (e) {
      setPathsError(e instanceof Error ? e.message : String(e));
    } finally {
      setPathsBusy(false);
    }
  };

  return (
    <div className="py-3 first:pt-0 last:pb-0">
      <div className={`flex items-start gap-3 ${busy || installing ? "opacity-70" : ""}`}>
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
          {canPickFolders && (
            <div className="mt-1 flex flex-wrap items-center gap-2">
              <span className="text-caption text-faint">
                {svc.paths.length > 0
                  ? t("storage.foldersNow", { list: svc.paths.join(", ") })
                  : t("storage.foldersNone")}
              </span>
              <Button size="sm" variant="ghost" onClick={() => { setSelected(svc.paths ?? []); setFolderOpen(true); }}>
                {t("storage.folders")}
              </Button>
            </div>
          )}
        </div>
        <div className="flex items-center gap-2.5 shrink-0 pt-1">
          {svc.installed ? (
            <>
              <Pill tone={svc.running ? "ok" : "muted"}>
                {svc.running ? t("storage.serviceOn") : t("storage.serviceOff")}
              </Pill>
              <Toggle checked={svc.enabled} busy={busy} onChange={toggle} label={label} />
            </>
          ) : !svc.available ? (
            <Pill tone="muted">{t("storage.svcUnavailable")}</Pill>
          ) : (
            <>
              <Pill tone="muted">{t("storage.svcNotInstalled")}</Pill>
              <Button size="sm" variant="secondary" loading={installing} onClick={install}>
                {t("storage.svcInstall")}
              </Button>
            </>
          )}
        </div>
      </div>
      {installing && <div className="mt-2"><InstallProgress job={job} /></div>}
      <Modal
        open={folderOpen}
        onClose={() => setFolderOpen(false)}
        title={t("storage.foldersTitle")}
        footer={
          <>
            <Button variant="ghost" onClick={() => setFolderOpen(false)}>{t("common.cancel")}</Button>
            <Button loading={pathsBusy} disabled={selected.length === 0} onClick={saveFolders}>
              {t("common.save")}
            </Button>
          </>
        }
      >
        <p className="text-small text-muted">
          {svc.name === "samba4" ? t("storage.foldersGuestHint") : t("storage.foldersHint")}
        </p>
        {mounts.length === 0 ? (
          <p className="mt-3 text-small text-muted">{t("storage.foldersNoMounts")}</p>
        ) : (
          <div className="mt-3 flex flex-col gap-2">
            {mounts.map((m) => (
              <label key={m} className="flex items-center gap-2 text-body">
                <input type="checkbox" className="accent-accent" checked={selected.includes(m)}
                  onChange={() => toggleFolder(m)} />
                <span className="font-mono text-small">{m}</span>
              </label>
            ))}
          </div>
        )}
        {pathsError && <p className="mt-2 text-caption text-danger">{pathsError}</p>}
      </Modal>
      {phase && (
        <div className="mt-2">
          <ActionBanner phase={phase} text={phase === "done" ? doneText : undefined} detail={detail} onDone={clear} />
        </div>
      )}
    </div>
  );
}
