import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { LucideIcon } from "lucide-react";
import { ArrowLeftRight, Blocks, Download, HardDrive, LayoutDashboard, LogOut, Network, RefreshCw, Server, Wifi, Wrench } from "lucide-react";
import { api } from "../api";
import type { Board, DawnAP, DDNSProbe, DriftProbe, EthPort, FwdProbe, GuestProbe, IoTProbe, IPv6Probe, ModeProbe, OVPNProbe, PkgUpgrade, SelfUpdateCheck, SQMProbe, StorageProbe, SystemInfo, TSProbe, UpdateCheck, WanStatus, WGProbe } from "../types";
import { Overview } from "../pages/Overview";
import { WifiPage } from "../pages/Wifi";
import { Services } from "../pages/Services";
import { Ports } from "../pages/Ports";
import { System } from "../pages/System";
import { LanPage } from "../pages/Lan";
import { ToolsPage } from "../pages/Tools";
import { FleetPage } from "../pages/Fleet";
import { StoragePage } from "../pages/Storage";

type Page = "overview" | "wifi" | "lan" | "services" | "ports" | "tools" | "fleet" | "storage" | "system";

const NAV: { id: Page; icon: LucideIcon; key: string }[] = [
  { id: "overview", icon: LayoutDashboard, key: "nav.overview" },
  { id: "wifi", icon: Wifi, key: "nav.wifi" },
  { id: "lan", icon: Network, key: "nav.lan" },
  { id: "services", icon: Blocks, key: "nav.services" },
  { id: "ports", icon: ArrowLeftRight, key: "nav.ports" },
  { id: "tools", icon: RefreshCw, key: "nav.tools" },
  { id: "storage", icon: HardDrive, key: "nav.storage" },
  { id: "fleet", icon: Server, key: "nav.fleet" },
  { id: "system", icon: Wrench, key: "nav.system" },
];

export function Shell({ onLogout }: { onLogout: () => void }) {
  const { t, i18n } = useTranslation();
  const [page, setPage] = useState<Page>("overview");
  const [board, setBoard] = useState<Board>();
  const [system, setSystem] = useState<SystemInfo>();
  const [wan, setWan] = useState<WanStatus>();
  const [mode, setMode] = useState<ModeProbe>();
  const [ipv6, setIpv6] = useState<IPv6Probe>();
  const [update, setUpdate] = useState<UpdateCheck>();
  const [wg, setWg] = useState<WGProbe>();
  const [ddns, setDdns] = useState<DDNSProbe>();
  const [sqm, setSqm] = useState<SQMProbe>();
  const [ovpn, setOvpn] = useState<OVPNProbe>();
  const [iot, setIot] = useState<IoTProbe>();
  const [fwd, setFwd] = useState<FwdProbe>();
  const [ts, setTs] = useState<TSProbe>();
  const [guest, setGuest] = useState<GuestProbe>();
  const [ethports, setEthports] = useState<EthPort[]>();
  const [dawnAps, setDawnAps] = useState<DawnAP[]>();
  const [dawnError, setDawnError] = useState(false);
  const [packages, setPackages] = useState<PkgUpgrade[]>();
  const [selfUpdate, setSelfUpdate] = useState<SelfUpdateCheck>();
  const [drift, setDrift] = useState<DriftProbe>();
  const [storage, setStorage] = useState<StorageProbe>();
  const [loadError, setLoadError] = useState(false);

  const load = useCallback(async () => {
    setLoadError(false);
    try {
      const [b, s, w, v6] = await Promise.all([
        api.board(), api.system(), api.wan(), api.ipv6(),
      ]);
      setBoard(b); setSystem(s); setWan(w); setIpv6(v6);
    } catch {
      setLoadError(true);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  // Slow checks (owut hits the ASU server): load in the background.
  useEffect(() => {
    api.updateCheck().then(setUpdate).catch(() => {});
    api.mode().then(setMode).catch(() => {});
    api.wireguard().then(setWg).catch(() => {});
    api.ddns().then(setDdns).catch(() => {});
    api.sqm().then(setSqm).catch(() => {});
    api.openvpn().then(setOvpn).catch(() => {});
    api.iotwifi().then(setIot).catch(() => {});
    api.portforward().then(setFwd).catch(() => {});
    api.tailscale().then(setTs).catch(() => {});
    api.guestwifi().then(setGuest).catch(() => {});
    api.ethports().then((r) => setEthports(r.ports)).catch(() => {});
    api.dawn().then((r) => { setDawnAps(r.aps); setDawnError(false); }).catch(() => setDawnError(true));
    api.packages().then((r) => setPackages(r.upgradable)).catch(() => {});
    api.selfUpdateCheck().then(setSelfUpdate).catch(() => {});
    api.drift().then(setDrift).catch(() => {});
    api.storage().then(setStorage).catch(() => {});
  }, []);

  // In AP mode the router is not the gateway: hide pages that only apply to
  // the gateway (LAN config with dnsmasq, port forwarding).
  // On switches (no WiFi, many ports): hide WiFi and services pages.
  const isSwitch = mode?.hardware_class === "switch";
  const apMode = mode?.mode === "ap" && !isSwitch;
  const navItems = NAV.filter((n) => {
    if (isSwitch && (n.id === "wifi" || n.id === "services")) return false;
    if (apMode && (n.id === "lan" || n.id === "ports")) return false;
    if (n.id === "storage" && !storage?.applicable) return false;
    return true;
  });
  const activePage = navItems.some((n) => n.id === page) ? page : "overview";

  const header = (
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
  );

  const updateBanner = selfUpdate?.available ? (
    <button onClick={() => setPage("system")}
      className="w-full mb-3 py-2 px-4 bg-accent/15 hover:bg-accent/25 border border-accent/30 rounded-lg text-sm flex items-center gap-2 transition-colors">
      <Download size={16} className="text-accent" />
      <span>{t("selfupdate.bannerText", { version: selfUpdate.latest })}</span>
    </button>
  ) : null;

  const pageContent = (
    <>
      {loadError && <p className="text-danger text-sm mb-3">{t("error.load")}</p>}
      {activePage === "overview" && (
        <Overview board={board} system={system} wan={wan} ethports={ethports} dawnAps={dawnAps} dawnError={dawnError} drift={drift} onDriftChange={setDrift} isSwitch={isSwitch} />
      )}
      {activePage === "wifi" && (
        <WifiPage iot={iot} onIotChange={setIot} guest={guest} onGuestChange={setGuest} />
      )}
      {activePage === "lan" && (
        <LanPage />
      )}
      {activePage === "services" && (
        <Services wg={wg} onWgChange={setWg} ipv6={ipv6} onIpv6Change={setIpv6} ddns={ddns} onDdnsChange={setDdns} sqm={sqm} onSqmChange={setSqm} ovpn={ovpn} onOvpnChange={setOvpn} ts={ts} onTsChange={setTs} />
      )}
      {activePage === "ports" && (
        <Ports fwd={fwd} onFwdChange={setFwd} />
      )}
      {activePage === "tools" && (
        <ToolsPage ethports={ethports ?? []} />
      )}
      {activePage === "fleet" && (
        <FleetPage />
      )}
      {activePage === "storage" && (
        <StoragePage />
      )}
      {activePage === "system" && (
        <System board={board} update={update} onUpdateChange={setUpdate} packages={packages} onPackagesChange={setPackages} onLogout={onLogout} />
      )}
    </>
  );

  return (
    <div className="min-h-screen">
      <div className="w-full max-w-[1280px] mx-auto md:flex">
        {/* Sidebar (desktop) */}
        <nav className="hidden md:flex md:flex-col md:w-44 md:shrink-0 border-r border-border p-3 gap-1">
          <p className="text-sm font-semibold px-2 py-2 mb-1">{t("app.name")}</p>
          {navItems.map((item) => (
            <button
              key={item.id}
              onClick={() => setPage(item.id)}
              className={`flex items-center gap-2 px-2 py-2 rounded-lg text-sm text-left transition-colors
                ${activePage === item.id ? "bg-accent/15 text-accent font-medium" : "text-muted hover:text-text hover:bg-card"}`}
            >
              <item.icon size={16} />
              {t(item.key)}
            </button>
          ))}
        </nav>

        {/* Content */}
        <main className="flex-1 p-4 pb-24 md:pb-12">
          {header}
          {updateBanner}
          {pageContent}
        </main>
      </div>

      {/* Bottom nav (mobile) */}
      <nav className="md:hidden fixed bottom-0 inset-x-0 bg-card border-t border-border flex z-10">
        {navItems.map((item) => (
          <button
            key={item.id}
            onClick={() => setPage(item.id)}
            className={`flex-1 flex flex-col items-center gap-0.5 py-2 text-xs
              ${activePage === item.id ? "text-accent" : "text-muted"}`}
          >
            <item.icon size={18} />
            {t(item.key)}
          </button>
        ))}
      </nav>
    </div>
  );
}
