import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  Cpu, Globe, LogOut, RefreshCw, Users, Wifi,
} from "lucide-react";
import { api } from "../api";
import type { Board, Lease, SystemInfo, WanStatus, WirelessRadio } from "../types";
import { Card, Pill, Row } from "../components/Card";

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

export function Dashboard({ onLogout }: { onLogout: () => void }) {
  const { t, i18n } = useTranslation();
  const [board, setBoard] = useState<Board>();
  const [system, setSystem] = useState<SystemInfo>();
  const [wan, setWan] = useState<WanStatus>();
  const [radios, setRadios] = useState<WirelessRadio[]>([]);
  const [leases, setLeases] = useState<Lease[]>([]);
  const [loadError, setLoadError] = useState(false);

  const load = useCallback(async () => {
    setLoadError(false);
    try {
      const [b, s, w, r, l] = await Promise.all([
        api.board(), api.system(), api.wan(), api.wireless(), api.leases(),
      ]);
      setBoard(b); setSystem(s); setWan(w); setRadios(r); setLeases(l);
    } catch {
      setLoadError(true);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const ramUsed = system ? system.memory.total - system.memory.available : 0;
  const ramPct = system ? Math.round((ramUsed / system.memory.total) * 100) : 0;

  return (
    <main className="max-w-3xl mx-auto p-4 pb-12">
      <header className="flex items-center gap-3 mb-4">
        <h1 className="text-lg font-semibold flex-1">
          {t("app.name")} <span className="text-muted font-normal">· {board?.hostname ?? "…"}</span>
        </h1>
        <button
          onClick={() => i18n.changeLanguage(i18n.language === "es" ? "en" : "es")}
          className="text-sm text-muted hover:text-text px-2 py-1"
          title="Language"
        >
          {i18n.language === "es" ? "🇪🇸 Español" : "🇬🇧 English"}
        </button>
        <button onClick={load} className="text-muted hover:text-text p-2" title={t("nav.refresh")}>
          <RefreshCw size={16} />
        </button>
        <button onClick={onLogout} className="text-muted hover:text-danger p-2" title={t("nav.logout")}>
          <LogOut size={16} />
        </button>
      </header>

      {loadError && <p className="text-danger text-sm mb-3">{t("error.load")}</p>}

      <div className="grid gap-4 sm:grid-cols-2">
        <Card title={t("system.title")} icon={Cpu}>
          <Row label={t("system.model")} value={board?.model} />
          <Row label={t("system.firmware")} value={board?.release && `${board.release.distribution} ${board.release.version}`} />
          <Row label={t("system.uptime")} value={system && fmtUptime(t, system.uptime)} />
          <Row label={t("system.load")} value={system?.load.map((l) => l.toFixed(2)).join(" · ")} />
          <Row label={t("system.ram")} value={system && `${fmtMB(ramUsed)} / ${fmtMB(system.memory.total)} (${ramPct}%)`} />
          <Row label={t("system.flash")} value={system && `${fmtMB(system.root.free * 1024)} ${t("system.free")}`} />
        </Card>

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

        <Card title={t("wifi.title")} icon={Wifi} action={
          <Pill tone="muted">
            {t("wifi.clients", { count: radios.reduce((n, r) => n + r.interfaces.reduce((m, i) => m + i.clients.length, 0), 0) })}
          </Pill>
        }>
          {radios.map((radio) => (
            <div key={radio.name} className="mb-3 last:mb-0">
              <div className="flex items-center gap-2 text-sm font-medium mb-1">
                <span>{radio.band === "5g" ? t("wifi.band5") : t("wifi.band24")}</span>
                {!radio.up && <Pill tone="danger">{t("wifi.down")}</Pill>}
                <span className="text-muted text-xs">
                  {t("wifi.channel")} {radio.channel} · {radio.htmode} · {radio.txpower} dBm
                </span>
              </div>
              {radio.interfaces.map((iface) => (
                <div key={iface.ifname} className="ml-2 text-sm">
                  <div className="flex justify-between py-0.5">
                    <span>{iface.ssid}</span>
                    <span className="text-muted">{t("wifi.clients", { count: iface.clients.length })}</span>
                  </div>
                  {iface.clients.map((c) => (
                    <div key={c.mac} className="flex justify-between text-xs text-muted ml-2 py-0.5">
                      <span className="font-mono">{c.mac}</span>
                      <span>{c.signal} dBm</span>
                    </div>
                  ))}
                </div>
              ))}
            </div>
          ))}
        </Card>

        <Card title={t("clients.title")} icon={Users}>
          {leases.length === 0 ? (
            <p className="text-sm text-muted">{t("clients.empty")}</p>
          ) : (
            leases.map((l) => (
              <Row key={l.mac} label={l.hostname || l.mac} value={l.ip} />
            ))
          )}
        </Card>

      </div>
    </main>
  );
}
