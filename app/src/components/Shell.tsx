import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { LucideIcon } from "lucide-react";
import { Blocks, LayoutDashboard, LogOut, RefreshCw, Wrench } from "lucide-react";
import { api } from "../api";
import type { Board, DDNSProbe, FwdProbe, IoTProbe, IPv6Probe, Lease, OVPNProbe, PkgUpgrade, SQMProbe, SystemInfo, UpdateCheck, WanStatus, WGProbe, WirelessRadio } from "../types";
import { Overview } from "../pages/Overview";
import { Services } from "../pages/Services";
import { System } from "../pages/System";

type Page = "overview" | "services" | "system";

const NAV: { id: Page; icon: LucideIcon; key: string }[] = [
  { id: "overview", icon: LayoutDashboard, key: "nav.overview" },
  { id: "services", icon: Blocks, key: "nav.services" },
  { id: "system", icon: Wrench, key: "nav.system" },
];

export function Shell({ onLogout }: { onLogout: () => void }) {
  const { t, i18n } = useTranslation();
  const [page, setPage] = useState<Page>("overview");
  const [board, setBoard] = useState<Board>();
  const [system, setSystem] = useState<SystemInfo>();
  const [wan, setWan] = useState<WanStatus>();
  const [radios, setRadios] = useState<WirelessRadio[]>([]);
  const [leases, setLeases] = useState<Lease[]>([]);
  const [ipv6, setIpv6] = useState<IPv6Probe>();
  const [update, setUpdate] = useState<UpdateCheck>();
  const [wg, setWg] = useState<WGProbe>();
  const [ddns, setDdns] = useState<DDNSProbe>();
  const [sqm, setSqm] = useState<SQMProbe>();
  const [ovpn, setOvpn] = useState<OVPNProbe>();
  const [iot, setIot] = useState<IoTProbe>();
  const [fwd, setFwd] = useState<FwdProbe>();
  const [packages, setPackages] = useState<PkgUpgrade[]>();
  const [loadError, setLoadError] = useState(false);

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

  // Slow checks (owut hits the ASU server): load in the background.
  useEffect(() => {
    api.updateCheck().then(setUpdate).catch(() => {});
    api.wireguard().then(setWg).catch(() => {});
    api.ddns().then(setDdns).catch(() => {});
    api.sqm().then(setSqm).catch(() => {});
    api.openvpn().then(setOvpn).catch(() => {});
    api.iotwifi().then(setIot).catch(() => {});
    api.portforward().then(setFwd).catch(() => {});
    api.packages().then((r) => setPackages(r.upgradable)).catch(() => {});
  }, []);

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

  const pageContent = (
    <>
      {loadError && <p className="text-danger text-sm mb-3">{t("error.load")}</p>}
      {page === "overview" && (
        <Overview board={board} system={system} wan={wan} radios={radios} leases={leases} />
      )}
      {page === "services" && (
        <Services wg={wg} onWgChange={setWg} ipv6={ipv6} onIpv6Change={setIpv6} ddns={ddns} onDdnsChange={setDdns} sqm={sqm} onSqmChange={setSqm} ovpn={ovpn} onOvpnChange={setOvpn} iot={iot} onIotChange={setIot} fwd={fwd} onFwdChange={setFwd} />
      )}
      {page === "system" && (
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
          {NAV.map((item) => (
            <button
              key={item.id}
              onClick={() => setPage(item.id)}
              className={`flex items-center gap-2 px-2 py-2 rounded-lg text-sm text-left transition-colors
                ${page === item.id ? "bg-accent/15 text-accent font-medium" : "text-muted hover:text-text hover:bg-card"}`}
            >
              <item.icon size={16} />
              {t(item.key)}
            </button>
          ))}
        </nav>

        {/* Content */}
        <main className="flex-1 p-4 pb-24 md:pb-12">
          {header}
          {pageContent}
        </main>
      </div>

      {/* Bottom nav (mobile) */}
      <nav className="md:hidden fixed bottom-0 inset-x-0 bg-card border-t border-border flex z-10">
        {NAV.map((item) => (
          <button
            key={item.id}
            onClick={() => setPage(item.id)}
            className={`flex-1 flex flex-col items-center gap-0.5 py-2 text-xs
              ${page === item.id ? "text-accent" : "text-muted"}`}
          >
            <item.icon size={18} />
            {t(item.key)}
          </button>
        ))}
      </nav>
    </div>
  );
}
