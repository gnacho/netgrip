import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Cpu, Globe } from "lucide-react";
import type { Board, DawnAP, DriftProbe, EthPort, SystemInfo, WanStatus } from "../types";
import { Card, Pill, Row } from "../components/Card";
import { TrafficCard } from "../components/TrafficCard";
import { EthPortsCard } from "../components/EthPortsCard";
import { DawnCard } from "../components/DawnCard";
import { ClientsCard } from "../components/ClientsCard";
import { DriftCard } from "../components/DriftCard";

function fmtUptime(t: TFunction, secs: number): string {
  const d = Math.floor(secs / 86400);
  const h = Math.floor((secs % 86400) / 3600);
  const m = Math.floor((secs % 3600) / 60);
  if (d > 0) return `${t("time.days", { count: d })} ${t("time.hours", { count: h })}`;
  if (h > 0) return `${t("time.hours", { count: h })} ${t("time.minutes", { count: m })}`;
  return t("time.minutes", { count: m });
}

function fmtMB(bytes: number): string {
  return `${Math.round(bytes / 1048576)} MB`;
}

export function Overview({ board, system, wan, ethports, dawnAps, dawnError, drift, onDriftChange, isSwitch }: {
  board: Board | undefined;
  system: SystemInfo | undefined;
  wan: WanStatus | undefined;
  ethports: EthPort[] | undefined;
  dawnAps: DawnAP[] | undefined;
  dawnError: boolean;
  drift: DriftProbe | undefined;
  onDriftChange: (d: DriftProbe) => void;
  isSwitch: boolean;
}) {
  const { t } = useTranslation();
  const ramUsed = system ? system.memory.total - system.memory.available : 0;
  const ramPct = system ? Math.round((ramUsed / system.memory.total) * 100) : 0;

  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Card title={t("system.title")} icon={Cpu}>
        <Row label={t("system.model")} value={board?.model} />
        <Row label={t("system.firmware")} value={board?.release && `${board.release.distribution} ${board.release.version}`} />
        <Row label={t("system.uptime")} value={system && fmtUptime(t, system.uptime)} />
        <Row label={t("system.load")} value={system?.load.map((l) => l.toFixed(2)).join(" · ")} />
        <Row label={t("system.ram")} value={system && `${fmtMB(ramUsed)} / ${fmtMB(system.memory.total)} (${ramPct}%)`} />
        <Row label={t("system.flash")} value={system && `${fmtMB(system.root.free * 1024)} ${t("system.free")}`} />
      </Card>

      {!isSwitch && (
        <Card title={t("wan.title")} icon={Globe}>
          {!wan?.present ? (
            <p className="text-sm text-muted">{t("wan.absent")}</p>
          ) : (
            <>
              <Row label={t("wan.title")} value={<Pill tone={wan.up ? "ok" : "danger"}>{wan.up ? t("wan.up") : t("wan.down")}</Pill>} />
              <Row label={t("wan.ip")} value={wan.ipv4.join(", ")} />
              <Row label={t("wan.gateway")} value={wan.gateway} />
              <Row label={t("wan.dns")} value={wan.dns.join(", ")} />
            </>
          )}
        </Card>
      )}

      <DriftCard drift={drift} onChange={onDriftChange} />

      <div className="sm:col-span-2 xl:col-span-4">
        <TrafficCard />
      </div>

      <div className="sm:col-span-2 xl:col-span-4">
        <EthPortsCard ports={ethports} />
      </div>

      {!isSwitch && (
        <div className="sm:col-span-2 xl:col-span-4">
          <DawnCard aps={dawnAps} error={dawnError} />
        </div>
      )}

      <div className="sm:col-span-2 xl:col-span-4">
        <ClientsCard />
      </div>
    </div>
  );
}
