import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { LucideIcon } from "lucide-react";
import { ArrowLeftRight, Blocks, Download, HardDrive, LayoutDashboard, LogOut, Menu, Network, Radar, Server, Settings, Smartphone, Wifi, Wrench } from "lucide-react";
import { api, disableDemo, isDemo } from "../api";
import type { Board, Client, DawnAP, DDNSProbe, DriftProbe, EthPort, FwdProbe, GuestProbe, IoTProbe, IPv6Probe, ModeProbe, OVPNProbe, SelfUpdateCheck, SQMProbe, StorageProbe, SystemInfo, TSProbe, UpdateCheck, WanStatus, WGProbe, WirelessRadio } from "../types";
import { useHealthScore } from "../hooks/useHealthScore";
import { Badge, Banner, Button, Drawer, Pill, StatusDot, ThemeToggle, ToastProvider } from "./ui";
import { Logo } from "./ui/illustrations";
import { Overview } from "../pages/Overview";
import { CoveragePage } from "../pages/Coverage";
import { ClientsPage } from "../pages/Clients";
import { WifiPage } from "../pages/Wifi";
import { Services } from "../pages/Services";
import { Ports } from "../pages/Ports";
import { System } from "../pages/System";
import { LanPage } from "../pages/Lan";
import { ToolsPage } from "../pages/Tools";
import { FleetPage } from "../pages/Fleet";
import { StoragePage } from "../pages/Storage";

export type Page = "overview" | "clients" | "coverage" | "wifi" | "lan" | "services" | "ports" | "tools" | "fleet" | "storage" | "system";

const NAV_ICONS: Record<Page, LucideIcon> = {
  overview: LayoutDashboard,
  clients: Smartphone,
  coverage: Radar,
  wifi: Wifi,
  lan: Network,
  services: Blocks,
  ports: ArrowLeftRight,
  tools: Wrench,
  storage: HardDrive,
  fleet: Server,
  system: Settings,
};

/** Nav agrupada §7.1 (tareas, nombres llanos §7.2). */
const NAV_GROUPS: { group: string | null; items: Page[] }[] = [
  { group: null, items: ["overview"] },
  { group: "nav.group.network", items: ["clients", "coverage", "wifi", "lan", "ports"] },
  { group: "nav.group.services", items: ["services"] },
  { group: "nav.group.router", items: ["tools", "storage", "fleet", "system"] },
];

function ShellInner({ onLogout }: { onLogout: () => void }) {
  const { t } = useTranslation();
  const [page, setPage] = useState<Page>("overview");
  const [menuOpen, setMenuOpen] = useState(false);
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
  const [selfUpdate, setSelfUpdate] = useState<SelfUpdateCheck>();
  const [drift, setDrift] = useState<DriftProbe>();
  const [storage, setStorage] = useState<StorageProbe>();
  const [wireless, setWireless] = useState<WirelessRadio[]>();
  const [clients, setClients] = useState<Client[]>();
  const [loadError, setLoadError] = useState(false);
  const [failCount, setFailCount] = useState(0);
  const [demoBannerDismissed, setDemoBannerDismissed] = useState(false);

  const load = useCallback(async () => {
    setLoadError(false);
    try {
      const [b, s, w, v6] = await Promise.all([
        api.board(), api.system(), api.wan(), api.ipv6(),
      ]);
      setBoard(b); setSystem(s); setWan(w); setIpv6(v6);
      setFailCount(0);
    } catch {
      setLoadError(true);
      setFailCount((n) => n + 1);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  // Título de la pestaña (#159): "nombre del router | NetGrip"; antes del
  // primer dato de board se queda en "NetGrip" (igual que en el login).
  useEffect(() => {
    document.title = board?.hostname ? `${board.hostname} | NetGrip` : "NetGrip";
  }, [board?.hostname]);

  // Error de red §11: reintento automático con backoff (5s, 10s, 30s).
  useEffect(() => {
    if (!loadError) return;
    const delay = [5000, 10000, 30000][Math.min(failCount, 2)];
    const id = setTimeout(load, delay);
    return () => clearTimeout(id);
  }, [loadError, failCount, load]);

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
    api.selfUpdateCheck().then(setSelfUpdate).catch(() => {});
    api.drift().then(setDrift).catch(() => {});
    api.storage().then(setStorage).catch(() => {});
    api.wireless().then(setWireless).catch(() => {});
    api.clients().then((r) => setClients(r.clients)).catch(() => {});
  }, []);

  const health = useHealthScore({ system, wan, drift, mode, wireless });

  // In AP mode the router is not the gateway: hide pages that only apply to
  // the gateway (LAN config with dnsmasq, port forwarding).
  // On switches (no WiFi, many ports): hide WiFi and services pages.
  const isSwitch = mode?.hardware_class === "switch";
  const apMode = mode?.mode === "ap" && !isSwitch;
  // Cobertura inalámbrica: solo si DAWN reporta varios routers activos.
  const dawnMultiRouter = useMemo(() => {
    const hosts = new Set<string>();
    for (const a of dawnAps ?? []) hosts.add(a.hostname || a.bssid);
    return hosts.size > 1;
  }, [dawnAps]);
  const visible = (id: Page) => {
    if (isSwitch && (id === "wifi" || id === "services")) return false;
    if (apMode && (id === "lan" || id === "ports")) return false;
    if (id === "storage" && !storage?.applicable) return false;
    if (id === "coverage" && !dawnMultiRouter) return false;
    return true;
  };
  const activePage = NAV_GROUPS.some((g) => g.items.includes(page)) && visible(page) ? page : "overview";

  // Badges §7.1. Sistema (#157): solo cuenta una versión de firmware
  // realmente nueva; los paquetes actualizables y las reconstrucciones
  // same_version NO son alertas (la paquetería vive en LuCI/CLI).
  const servicesActive = [wg?.active, ddns?.active, sqm?.active, ipv6?.state === "enabled", ovpn?.active, ts?.running]
    .filter(Boolean).length;
  const firmwarePending = update?.available && !update.same_version ? 1 : 0;
  const badgeFor = (id: Page): { n: number; tone: "accent" | "warn" } | undefined => {
    if (id === "clients" && clients && clients.length > 0) return { n: clients.length, tone: "accent" };
    if (id === "services" && servicesActive > 0) return { n: servicesActive, tone: "accent" };
    if (id === "system" && firmwarePending > 0) return { n: firmwarePending, tone: "warn" };
    return undefined;
  };

  const navigate = useCallback((p: string) => {
    setPage(p as Page);
    setMenuOpen(false);
  }, []);

  const navList = (compact: boolean, onPick?: () => void) => (
    <div className="flex flex-col gap-0.5">
      {NAV_GROUPS.map((g, gi) => (
        <div key={g.group ?? "top"} className={gi > 0 ? "mt-4" : ""}>
          {g.group && !compact && (
            <p className="text-eyebrow text-faint px-2.5 mb-1">{t(g.group)}</p>
          )}
          {g.group && compact && gi > 0 && <div className="mx-2 my-2 border-t border-border" aria-hidden="true" />}
          {g.items.filter(visible).map((id) => {
            const Icon = NAV_ICONS[id];
            const active = activePage === id;
            const badge = badgeFor(id);
            return (
              <button
                key={id}
                type="button"
                title={t(`nav.desc.${id}`)}
                onClick={() => { setPage(id); onPick?.(); }}
                aria-current={active ? "page" : undefined}
                className={`relative flex items-center gap-2.5 rounded-md px-2.5 py-2 text-body font-medium text-left w-full
                  transition-colors duration-[var(--dur-fast)]
                  ${compact ? "justify-center" : ""}
                  ${active ? "bg-accent-soft text-accent" : "text-muted hover:text-text hover:bg-surface-2"}`}
              >
                {active && (
                  <span aria-hidden="true" className="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-[3px] rounded-r-full bg-accent" />
                )}
                <span className="relative shrink-0">
                  <Icon size={18} />
                  {compact && badge && (
                    <span className={`absolute -top-1.5 -right-1.5 min-w-[14px] h-[14px] px-0.5 rounded-full text-[9px] font-semibold flex items-center justify-center
                      ${badge.tone === "warn" ? "bg-warn text-white" : "bg-accent text-on-accent"}`}>
                      {badge.n}
                    </span>
                  )}
                </span>
                {!compact && <span className="flex-1 truncate">{t(`nav.${id}`)}</span>}
                {!compact && id === "overview" && <StatusDot tone={health.tone} label={t(health.labelKey)} />}
                {!compact && badge && <Badge tone={badge.tone}>{badge.n}</Badge>}
              </button>
            );
          })}
        </div>
      ))}
    </div>
  );

  // Bottom bar móvil §7.4: los 3–4 destinos más usados del modo actual + Menú.
  const bottomItems: Page[] = (
    isSwitch
      ? (["overview", "ports", "tools"] as Page[])
      : (["overview", "wifi", "services", firmwarePending > 0 ? "system" : "tools"] as Page[])
  ).filter(visible);

  const demo = isDemo();
  const exitDemo = () => {
    disableDemo();
    window.location.reload();
  };

  const header = (
    <header className="flex items-center gap-2 mb-4 md:mb-5">
      {/* móvil: logo + hostname; desktop: título de página */}
      <div className="flex-1 min-w-0">
        <div className="md:hidden flex items-center gap-2">
          <span className="text-accent"><Logo size={22} /></span>
          <span className="text-body font-semibold truncate">
            NetGrip <span className="text-muted font-normal font-mono text-small">· {board?.hostname ?? "…"}</span>
          </span>
        </div>
        <div className="hidden md:block">
          <h1 className="text-h1">{t(`nav.${activePage}`)}</h1>
          <p className="text-small text-muted mt-0.5">
            {health.reasons.length === 0 ? t("health.allGood") : t("health.issues", { count: health.reasons.length })}
          </p>
        </div>
      </div>
      <div className="hidden md:block"><ThemeToggle /></div>
      <button
        type="button"
        onClick={onLogout}
        title={t("nav.logout")}
        aria-label={t("nav.logout")}
        className="inline-flex h-9 w-9 items-center justify-center rounded-md text-muted hover:text-danger hover:bg-surface-2 ring-focus transition-colors"
      >
        <LogOut size={18} />
      </button>
    </header>
  );

  const updateBanner = selfUpdate?.available ? (
    <Banner tone="info" icon={Download} className="mb-4"
      action={<Button variant="secondary" size="sm" onClick={() => setPage("system")}>{t("nav.system")}</Button>}>
      {t("selfupdate.bannerText", { version: selfUpdate.latest })}
    </Banner>
  ) : null;

  const demoBanner = demo && !demoBannerDismissed ? (
    <Banner tone="warn" className="mb-4" onDismiss={() => setDemoBannerDismissed(true)}
      action={
        <span className="flex gap-2 shrink-0">
          <Button variant="ghost" size="sm" onClick={exitDemo}>{t("demo.realLogin")}</Button>
          <Button variant="secondary" size="sm" onClick={exitDemo}>{t("demo.exit")}</Button>
        </span>
      }>
      {t("demo.banner")}
    </Banner>
  ) : null;

  const pageContent = (
    <>
      {loadError && (
        <Banner tone="danger" className="mb-4"
          action={<Button variant="secondary" size="sm" onClick={load}>{t("common.retryNow")}</Button>}>
          {t("error.network")}
        </Banner>
      )}
      {activePage === "overview" && (
        <Overview
          board={board} system={system} wan={wan} ethports={ethports}
          drift={drift} onDriftChange={setDrift}
          isSwitch={isSwitch} health={health} mode={mode} onNavigate={navigate}
        />
      )}
      {activePage === "clients" && <ClientsPage />}
      {activePage === "coverage" && <CoveragePage aps={dawnAps} error={dawnError} />}
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
        <System board={board} update={update} onUpdateChange={setUpdate} onLogout={onLogout} />
      )}
    </>
  );

  return (
    <div className="min-h-screen">
      <div className="w-full max-w-[1280px] mx-auto md:flex">
        {/* Sidebar §7.1: 240px desktop, 64px tablet §7.5, oculta en móvil */}
        <nav aria-label={t("app.name")} className="hidden md:flex md:flex-col md:w-16 lg:w-60 md:shrink-0 md:min-h-screen border-r border-border bg-surface/40 p-2 lg:p-3 sticky top-0 max-h-screen overflow-y-auto">
          <div className="flex items-center gap-2 px-1.5 py-2 mb-2">
            <span className="text-accent shrink-0"><Logo size={26} /></span>
            <p className="hidden lg:block text-body font-semibold truncate">
              NetGrip <span className="text-muted font-normal font-mono text-small">· {board?.hostname ?? "…"}</span>
            </p>
          </div>
          <div className="lg:hidden">{navList(true)}</div>
          <div className="hidden lg:block">{navList(false)}</div>
        </nav>

        {/* Contenido */}
        <main className="flex-1 min-w-0 p-4 md:p-6 pb-[calc(84px+env(safe-area-inset-bottom))] md:pb-12">
          {header}
          {demoBanner}
          {updateBanner}
          {pageContent}
        </main>
      </div>

      {/* Bottom bar móvil §7.4 */}
      <nav aria-label={t("nav.menu")} className="md:hidden fixed bottom-0 inset-x-0 z-40 bg-surface border-t border-border flex pb-[env(safe-area-inset-bottom)]">
        {bottomItems.map((id) => {
          const Icon = NAV_ICONS[id];
          const active = activePage === id;
          const badge = badgeFor(id);
          return (
            <button
              key={id}
              type="button"
              onClick={() => setPage(id)}
              aria-current={active ? "page" : undefined}
              className={`flex-1 relative flex flex-col items-center gap-0.5 py-2 text-[10px] font-medium transition-colors
                ${active ? "text-accent" : "text-muted"}`}
            >
              <span className="relative">
                <Icon size={22} />
                {badge && (
                  <span className={`absolute -top-1.5 -right-2 min-w-[15px] h-[15px] px-1 rounded-full text-[9px] font-semibold flex items-center justify-center
                    ${badge.tone === "warn" ? "bg-warn text-white" : "bg-accent text-on-accent"}`}>
                    {badge.n}
                  </span>
                )}
              </span>
              {t(`nav.${id}`)}
            </button>
          );
        })}
        <button
          type="button"
          onClick={() => setMenuOpen(true)}
          aria-expanded={menuOpen}
          className="flex-1 flex flex-col items-center gap-0.5 py-2 text-[10px] font-medium text-muted"
        >
          <Menu size={22} />
          {t("nav.menu")}
        </button>
      </nav>

      {/* Drawer Menú móvil: lista completa agrupada + toggle de tema
          (idioma y densidad viven en Sistema > Opciones, #158) */}
      <Drawer open={menuOpen} onClose={() => setMenuOpen(false)} title={t("nav.menu")}>
        {navList(false, () => setMenuOpen(false))}
        <div className="mt-5 pt-4 border-t border-border flex items-center justify-between">
          <ThemeToggle />
          <Pill tone={health.tone}>{t(health.labelKey)}</Pill>
        </div>
      </Drawer>
    </div>
  );
}

export function Shell(props: { onLogout: () => void }) {
  return (
    <ToastProvider>
      <ShellInner {...props} />
    </ToastProvider>
  );
}
