import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  ArrowUpCircle, Cpu, Globe, LogOut, Network, RefreshCw, ShieldCheck, Users, Wifi,
} from "lucide-react";
import { api } from "../api";
import type { Board, IPv6Probe, Lease, SystemInfo, UpdateCheck, WanStatus, WirelessRadio } from "../types";
import { Card, Pill, Row } from "../components/Card";
import { Toggle } from "../components/Toggle";

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
  const [ipv6, setIpv6] = useState<IPv6Probe>();
  const [ipv6Busy, setIpv6Busy] = useState(false);
  const [ipv6Msg, setIpv6Msg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [loadError, setLoadError] = useState(false);
  const [pwCurrent, setPwCurrent] = useState("");
  const [pwNext, setPwNext] = useState("");
  const [pwConfirm, setPwConfirm] = useState("");
  const [pwBusy, setPwBusy] = useState(false);
  const [pwMsg, setPwMsg] = useState<{ tone: "ok" | "danger"; text: string }>();
  const [update, setUpdate] = useState<UpdateCheck>();
  const [updateBusy, setUpdateBusy] = useState(false);
  const [updateConfirm, setUpdateConfirm] = useState(false);
  const [updateMsg, setUpdateMsg] = useState<{ tone: "ok" | "danger"; text: string }>();

  const load = useCallback(async () => {
    setLoadError(false);
    try {
      const [b, s, w, r, l, v6] = await Promise.all([
        api.board(), api.system(), api.wan(), api.wireless(), api.leases(), api.ipv6(),
      ]);
      setBoard(b); setSystem(s); setWan(w); setRadios(r); setLeases(l); setIpv6(v6);
    } catch {
      setLoadError(true);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  // owut check is slow (network round trip to the ASU server): load it in
  // the background after the dashboard is up.
  useEffect(() => {
    api.updateCheck().then(setUpdate).catch(() => {});
  }, []);

  const recheckUpdate = async () => {
    setUpdateBusy(true);
    try { setUpdate(await api.updateCheck()); } catch { /* keep previous */ }
    setUpdateBusy(false);
  };

  const startUpdate = async () => {
    setUpdateConfirm(false);
    setUpdateBusy(true);
    setUpdateMsg(undefined);
    try {
      await api.startUpdate();
      setUpdateMsg({ tone: "ok", text: t("update.started") });
    } catch (e) {
      setUpdateMsg({ tone: "danger", text: e instanceof Error ? e.message : t("update.failed") });
    } finally {
      setUpdateBusy(false);
    }
  };

  const changePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPwMsg(undefined);
    if (pwNext.length < 8) { setPwMsg({ tone: "danger", text: t("security.tooShort", { count: 8 }) }); return; }
    if (pwNext !== pwConfirm) { setPwMsg({ tone: "danger", text: t("security.mismatch") }); return; }
    setPwBusy(true);
    try {
      await api.setPassword(pwCurrent, pwNext);
      setPwMsg({ tone: "ok", text: t("security.done") });
      setTimeout(onLogout, 2500);
    } catch (err) {
      setPwMsg({ tone: "danger", text: err instanceof Error ? err.message : t("security.failed") });
    } finally {
      setPwBusy(false);
    }
  };

  const toggleIpv6 = async (enabled: boolean) => {
    setIpv6Busy(true);
    setIpv6Msg(undefined);
    try {
      const result = await api.setIpv6(enabled);
      setIpv6(result.state);
      if (result.status === "applied") {
        setIpv6Msg({ tone: "ok", text: t("ipv6.applied") });
      } else if (result.status === "rolled_back") {
        setIpv6Msg({ tone: "danger", text: t("ipv6.rolledBack") });
      }
    } catch {
      setIpv6Msg({ tone: "danger", text: t("ipv6.failed") });
      setIpv6(await api.ipv6());
    } finally {
      setIpv6Busy(false);
    }
  };

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

        <Card title={t("ipv6.title")} icon={Network}>
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <span className="text-sm">{t("ipv6.toggle")}</span>
              {ipv6 && (
                <Pill tone={ipv6.state === "enabled" ? "ok" : ipv6.state === "disabled" ? "muted" : "warn"}>
                  {t(`ipv6.${ipv6.state}`)}
                </Pill>
              )}
            </div>
            <Toggle
              checked={ipv6?.state === "enabled"}
              busy={ipv6Busy}
              disabled={!ipv6}
              onChange={toggleIpv6}
            />
          </div>
          {ipv6 && (
            <p className="text-xs text-muted">
              {t("ipv6.details", {
                odhcpd: ipv6.odhcpd_enabled ? t("ipv6.on") : t("ipv6.off"),
                ra: ipv6.ra_mode || "-",
                dhcpv6: ipv6.dhcpv6_mode || "-",
              })}
            </p>
          )}
          {ipv6Msg && <p className={`text-xs mt-2 ${ipv6Msg.tone === "ok" ? "text-ok" : "text-danger"}`}>{ipv6Msg.text}</p>}
        </Card>

        <Card title={t("update.title")} icon={ArrowUpCircle} action={
          <button onClick={recheckUpdate} disabled={updateBusy} className="text-xs text-muted hover:text-text">
            {updateBusy ? t("update.checking") : t("update.check")}
          </button>
        }>
          <Row label={t("update.current")} value={board?.release && `${board.release.version} (${board.release.revision})`} />
          {update?.owut_present === false ? (
            <p className="text-sm text-muted mt-2">{t("update.noOwut")}</p>
          ) : update ? (
            <>
              {!update.same_version && <Row label={t("update.available")} value={update.version_to} />}
              <Row label="" value={
                !update.same_version
                  ? <Pill tone="warn">{t("update.newVersion")}</Pill>
                  : update.out_of_date_packages > 0
                    ? <Pill tone="warn">{t("update.outOfDate", { count: update.out_of_date_packages })}</Pill>
                    : <Pill tone="ok">{t("update.upToDate")}</Pill>
              } />
              {update.warnings.map((w) => <p key={w} className="text-xs text-warn mt-1">{w}</p>)}
              {!update.safe_to_proceed && <p className="text-xs text-danger mt-2">{t("update.unsafe")}</p>}
              {updateConfirm ? (
                <div className="mt-3 border border-warn/40 rounded-lg p-3">
                  <p className="text-xs font-medium mb-1">{t("update.confirmTitle")}</p>
                  <p className="text-xs text-muted mb-3">
                    {update.same_version
                      ? t("update.confirmBodyPackages")
                      : t("update.confirmBodyFirmware")}
                  </p>
                  <div className="flex gap-2">
                    <button onClick={startUpdate} className="text-xs bg-danger/80 hover:bg-danger rounded-lg px-3 py-1.5 font-medium">
                      {t("update.confirmYes")}
                    </button>
                    <button onClick={() => setUpdateConfirm(false)} className="text-xs bg-border hover:bg-border/70 rounded-lg px-3 py-1.5">
                      {t("update.confirmNo")}
                    </button>
                  </div>
                </div>
              ) : update.available && (
                <button
                  onClick={() => setUpdateConfirm(true)}
                  disabled={!update.safe_to_proceed || updateBusy}
                  className="mt-3 text-sm bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium"
                >
                  {update.same_version
                    ? t("update.upgradePackages", { count: update.out_of_date_packages })
                    : t("update.upgradeFirmware")}
                </button>
              )}
            </>
          ) : (
            <p className="text-sm text-muted mt-2">{t("update.checking")}</p>
          )}
          {updateMsg && <p className={`text-xs mt-2 ${updateMsg.tone === "ok" ? "text-ok" : "text-danger"}`}>{updateMsg.text}</p>}
        </Card>

        <Card title={t("security.title")} icon={ShieldCheck}>
          <form onSubmit={changePassword} className="flex flex-col gap-2">
            <input type="password" value={pwCurrent} onChange={(e) => setPwCurrent(e.target.value)}
              placeholder={t("security.current")} autoComplete="current-password"
              className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
            <input type="password" value={pwNext} onChange={(e) => setPwNext(e.target.value)}
              placeholder={t("security.next")} autoComplete="new-password"
              className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
            <input type="password" value={pwConfirm} onChange={(e) => setPwConfirm(e.target.value)}
              placeholder={t("security.confirm")} autoComplete="new-password"
              className="bg-bg border border-border rounded-lg px-3 py-1.5 text-sm outline-none focus:border-accent" />
            {pwMsg && <p className={`text-xs ${pwMsg.tone === "ok" ? "text-ok" : "text-danger"}`}>{pwMsg.text}</p>}
            <button type="submit" disabled={pwBusy || !pwCurrent || !pwNext}
              className="text-sm bg-accent hover:bg-accent/85 disabled:opacity-40 rounded-lg px-3 py-1.5 font-medium self-start">
              {t("security.submit")}
            </button>
          </form>
        </Card>
      </div>
    </main>
  );
}
