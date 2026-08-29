import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Shield, ExternalLink } from "lucide-react";
import { api } from "../../api";
import type { DNSConfig } from "../../types";
import { Card, Pill, SettingRow, SkeletonRows } from "../ui";
import { TechName } from "./shared";

export function AdguardCard({ index = 0 }: { index?: number }) {
  const { t } = useTranslation();
  const [cfg, setCfg] = useState<DNSConfig>();

  useEffect(() => {
    api.dns().then(setCfg).catch(() => {});
  }, []);

  if (!cfg) return <Card index={index} icon={Shield} iconTone="success" title={t("adguard.title")}><SkeletonRows rows={2} /></Card>;
  if (!cfg.applicable && !cfg.adguard_active) return null;

  const dashboardUrl = `http://${window.location.hostname}:3000`;

  return (
    <Card index={index} icon={Shield} iconTone="success" title={t("adguard.title")}>
      <TechName>AdGuard Home</TechName>

      <SettingRow
        title={t("adguard.protection")}
        description={cfg.adguard_active ? t("adguard.activeDesc") : t("adguard.inactiveDesc")}
        checked={cfg.adguard_active}
        disabled
      />

      {cfg.adguard_active && (
        <div className="mt-3 flex items-center gap-2">
          <Pill tone="ok">{t("adguard.filtering")}</Pill>
          <a
            href={dashboardUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-small text-accent hover:text-accent-hover ring-focus rounded-sm"
          >
            {t("adguard.openDashboard")}
            <ExternalLink size={12} aria-hidden="true" />
          </a>
        </div>
      )}

      {!cfg.adguard_active && (
        <p className="mt-3 text-caption text-muted">{t("adguard.howToInstall")}</p>
      )}
    </Card>
  );
}
