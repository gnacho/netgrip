import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { HardDrive } from "lucide-react";
import { api } from "../api";
import { Card } from "../components/Card";
import type { StorageProbe } from "../types";

function fmtBytes(b: number): string {
  if (b > 1099511627776) return `${(b / 1099511627776).toFixed(1)} TB`;
  if (b > 1073741824) return `${(b / 1073741824).toFixed(1)} GB`;
  if (b > 1048576) return `${(b / 1048576).toFixed(0)} MB`;
  return `${b} B`;
}

export function StoragePage() {
  const { t } = useTranslation();
  const [probe, setProbe] = useState<StorageProbe>();
  const [busy, setBusy] = useState<string>();

  useEffect(() => { api.storage().then(setProbe).catch(() => {}); }, []);

  if (!probe?.applicable) {
    return <p className="text-muted text-sm">{t("storage.noUsb")}</p>;
  }

  const toggle = async (name: string, enabled: boolean) => {
    const key = `${name}-${enabled ? "enable" : "disable"}`;
    setBusy(key);
    try {
      await api.setStorageService(name, enabled ? "disable" : "enable");
      setProbe(await api.storage());
    } catch {
    } finally { setBusy(undefined); }
  };

  const serviceLabel: Record<string, string> = {
    samba4: "Samba (SMB)",
    minidlna: "DLNA (MiniDLNA)",
  };

  return (
    <div className="flex flex-col gap-4">
      <Card title={t("storage.devices")} icon={HardDrive}>
        {probe.devices.length === 0 ? (
          <p className="text-sm text-muted">{t("storage.noDevices")}</p>
        ) : (
          <div className="flex flex-col gap-3">
            {probe.devices.map((d) => {
              const pct = d.size_bytes > 0 ? (d.used_bytes / d.size_bytes) * 100 : 0;
              return (
                <div key={d.name} className="border border-border/50 rounded-lg p-3">
                  <div className="flex items-center justify-between mb-2">
                    <span className="font-medium text-sm">{d.name}</span>
                    <span className="text-xs text-muted">{d.fs_type || "?"}</span>
                  </div>
                  {d.mount_point && (
                    <p className="text-xs text-muted mb-2">{d.mount_point}</p>
                  )}
                  <div className="h-2 bg-bg rounded overflow-hidden mb-1">
                    <div className="h-full bg-accent/60 rounded transition-all" style={{ width: `${pct}%` }} />
                  </div>
                  <div className="flex justify-between text-[10px] text-muted">
                    <span>{fmtBytes(d.used_bytes)} / {fmtBytes(d.size_bytes)}</span>
                    <span>{fmtBytes(d.free_bytes)} {t("storage.free")}</span>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </Card>

      <Card title={t("storage.services")} icon={HardDrive}>
        <p className="text-xs text-muted mb-3">{t("storage.servicesIntro")}</p>
        <div className="flex flex-col gap-2">
          {probe.services.map((svc) => {
            const key = `${svc.name}-${svc.enabled ? "enable" : "disable"}`;
            return (
              <div key={svc.name} className="flex items-center justify-between py-2 border-b border-border/50 last:border-0">
                <div>
                  <span className="text-sm font-medium">{serviceLabel[svc.name] || svc.name}</span>
                  <span className={`ml-2 text-xs ${svc.running ? "text-ok" : "text-muted"}`}>
                    {svc.running ? t("storage.running") : t("storage.stopped")}
                  </span>
                </div>
                <input type="checkbox" checked={svc.enabled} disabled={busy === key}
                  onChange={() => toggle(svc.name, svc.enabled)}
                  className="accent-accent" />
              </div>
            );
          })}
        </div>
      </Card>
    </div>
  );
}
