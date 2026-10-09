import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { LucideIcon } from "lucide-react";
import { Activity, ArrowLeftRight, Blocks, ChartColumn, ChartPie, CircuitBoard, Download, Forward, Globe, HardDrive, Info, Layers, LayoutDashboard, LogOut, Menu, Network, Radar, Server, Settings, ShieldBan, Smartphone, Wifi, Wrench } from "lucide-react";
import { api, disableDemo, isDemo } from "../api";
import type { Board, DDNSProbe, DriftProbe, EthPort, FwdProbe, GuestProbe, IoTProbe, IPv6Probe, MDNSProbe, MultiWanProbe, OVPNProbe, SelfUpdateCheck, SQMProbe, StorageProbe, SystemInfo, TSProbe, UsteerAP, UpdateCheck, WanStatus, WGProbe, WirelessRadio } from "../types";
import { useHealthScore } from "../hooks/useHealthScore";
import { useMode } from "../hooks/useMode";
import { AnnouncementBanner } from "./AnnouncementBanner";
import { Banner, Button, Drawer, Pill, StatusDot, ThemeToggle, ToastProvider } from "./ui";
import { Logo } from "./ui/illustrations";
import { Overview } from "../pages/Overview";
import { TrafficPage } from "../pages/Traffic";
import { CoveragePage } from "../pages/Coverage";
import { ClientsPage } from "../pages/Clients";
import { WifiPage } from "../pages/Wifi";
import { InternetPage } from "../pages/Internet";
import { Services } from "../pages/Services";
import { Ports } from "../pages/Ports";
import { System } from "../pages/System";
import { LanPage } from "../pages/Lan";
import { ToolsPage } from "../pages/Tools";
import { DiagnosticsPage } from "../pages/Diagnostics";
import { AdvancedNetworkPage } from "../pages/AdvancedNetwork";
import { AdvancedSwitchPage } from "../pages/AdvancedSwitch";
import { FleetPage } from "../pages/Fleet";
import { StoragePage } from "../pages/Storage";
import { AboutPage } from "../pages/About";
import { SelfUpdateDialog } from "../components/system/SelfUpdateDialog";

export type Page = "overview" | "internet" | "wan" | "clients" | "coverage" | "wifi" | "lan" | "services" | "traffic" | "banip" | "ports" | "forwards" | "tools" | "diagnostics" | "adv-network" | "adv-switch" | "fleet" | "storage" | "system" | "dpi" | "about";

const NAV_ICONS: Record<Page, LucideIcon> = {
  overview: LayoutDashboard,
  internet: Globe,
  wan: Globe,
  clients: Smartphone,
  coverage: Radar,
  wifi: Wifi,
  lan: Network,
  services: Blocks,
  traffic: ChartPie,
  banip: ShieldBan,
  ports: ArrowLeftRight,
  forwards: Forward,
  tools: Wrench,
  diagnostics: Activity,
  "adv-network": Layers,
  "adv-switch": CircuitBoard,
  storage: HardDrive,
  fleet: Server,
  system: Settings,
  dpi: ChartColumn,
  about: Info,
};

/** Badge de rol en la cabecera del sidebar (#487): colores del mockup de
 *  rediseño. Solo se pinta cuando useMode confirma el rol (modeReady). */
const ROLE_BADGE: Record<"router" | "ap" | "switch", { labelKey: string; cls: string; dot: string }> = {
  router: { labelKey: "nav.role.router", cls: "bg-accent-soft text-accent", dot: "bg-accent" },
  ap: { labelKey: "nav.role.ap", cls: "bg-success-soft text-success", dot: "bg-success" },
  switch: { labelKey: "nav.role.switch", cls: "bg-violet-soft text-violet", dot: "bg-violet" },
};

/** IA de navegación por roles (#487): misma agrupación que el mockup de
 *  rediseño. "wan", "forwards", "dpi" y "banip" siguen existiendo como
 *  páginas pero ya no son entradas directas: se componen dentro de
 *  "internet" (solo gateway). */
const NAV_GROUPS: { group: string | null; items: Page[] }[] = [
  { group: null, items: ["overview"] },
  { group: "nav.group.yourNetwork", items: ["clients", "coverage", "traffic"] },
  { group: "nav.group.connection", items: ["internet", "wifi", "lan", "ports"] },
  { group: "nav.group.services", items: ["services"] },
  { group: "nav.group.system", items: ["tools", "diagnostics", "storage", "fleet", "system", "about"] },
  { group: "nav.group.advanced", items: ["adv-network", "adv-switch"] },
];

function ShellInner({ onLogout }: { onLogout: () => void }) {
  const { t } = useTranslation();
  const [page, setPage] = useState<Page>("overview");
  const [menuOpen, setMenuOpen] = useState(false);
  const [board, setBoard] = useState<Board>();
  const [system, setSystem] = useState<SystemInfo>();
  const [wan, setWan] = useState<WanStatus>();
  const [ipv6, setIpv6] = useState<IPv6Probe>();
  const [update, setUpdate] = useState<UpdateCheck>();
  const [wg, setWg] = useState<WGProbe>();
  const [ddns, setDdns] = useState<DDNSProbe>();
  const [mdns, setMdns] = useState<MDNSProbe>();
  const [sqm, setSqm] = useState<SQMProbe>();
  const [ovpn, setOvpn] = useState<OVPNProbe>();
  const [iot, setIot] = useState<IoTProbe>();
  const [fwd, setFwd] = useState<FwdProbe>();
  const [mwan, setMwan] = useState<MultiWanProbe>();
  const [ts, setTs] = useState<TSProbe>();
  const [guest, setGuest] = useState<GuestProbe>();
  const [ethports, setEthports] = useState<EthPort[]>();
  const [usteerAps, setUsteerAps] = useState<UsteerAP[]>();
  const [usteerError, setUsteerError] = useState(false);
  const [selfUpdate, setSelfUpdate] = useState<SelfUpdateCheck>();
  const [drift, setDrift] = useState<DriftProbe>();
  const [storage, setStorage] = useState<StorageProbe>();
  const [wireless, setWireless] = useState<WirelessRadio[]>();
  const [advanced, setAdvanced] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [failCount, setFailCount] = useState(0);
  const [demoBannerDismissed, setDemoBannerDismissed] = useState(false);
  const [selfUpdateDialogOpen, setSelfUpdateDialogOpen] = useState(false);

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
    api.wireguard().then(setWg).catch(() => {});
    api.ddns().then(setDdns).catch(() => {});
    api.mdns().then(setMdns).catch(() => {});
    api.sqm().then(setSqm).catch(() => {});
    api.openvpn().then(setOvpn).catch(() => {});
    api.iotwifi().then(setIot).catch(() => {});
    api.portforward().then(setFwd).catch(() => {});
    api.multiwan().then(setMwan).catch(() => {});
    api.tailscale().then(setTs).catch(() => {});
    api.guestwifi().then(setGuest).catch(() => {});
    api.ethports().then((r) => setEthports(r.ports)).catch(() => {});
    api.usteer().then((r) => { setUsteerAps(r.aps); setUsteerError(false); }).catch(() => setUsteerError(true));
    api.selfUpdateCheck().then(setSelfUpdate).catch(() => {});
    api.drift().then(setDrift).catch(() => {});
    api.storage().then(setStorage).catch(() => {});
    api.wireless().then(setWireless).catch(() => {});
    api.advanced().then((p) => setAdvanced(p.advanced)).catch(() => {});
  }, []);

  const { mode, modeReady } = useMode();

  const health = useHealthScore({ system, wan, drift, mode, wireless });

  // In AP mode the router is not the gateway: hide pages that only apply to
  // the gateway (LAN config with dnsmasq, port forwarding).
  // On switches (no WiFi, many ports): hide WiFi and services pages.
  // Role is first-class (#447): a device with only LAN ports is a managed
  // switch whatever its port count; hardware_class stays as fallback for
  // older probes.
  // Los grupos de área se gobiernan por rol (#447) y hardware: las páginas
  // de gateway exigen role=router, las de WiFi exigen radios y la de Switch
  // exige bocas ethernet. El rol llega por useMode (#484): caché en
  // localStorage para el primer paint + refetch en segundo plano.
  // Matriz de roles (#484): gateway ve todo; ap ve bocas LAN + radios;
  // switch solo bocas LAN + opciones de switch: sin Servicios ni Consumo
  // (DNS blocker, VPN, QoS... nada de eso aplica a un L2 puro).
  // Hasta que modeReady es falso NADA gateado se pinta: solo overview. Así
  // las entradas del rol nunca parpadean: o salen cacheadas (recarga) o no
  // salen hasta que el probe confirma el rol (primera visita).
  const role = mode?.role ?? (mode?.mode === "ap" ? "ap" : "router");
  const isRouter = role === "router";
  const isSwitch = role === "switch";
  // Fallbacks por campo solo cuando modeReady: si el probe trae un campo
  // concreto se usa tal cual; si falta (probes antiguos sin role), el
  // fallback actual aplica solo a ese campo.
  const hasWifiRadios = mode?.has_wifi ?? true;
  const hasSwitchPorts = (mode?.port_count ?? 1) > 0;
  // Cobertura inalámbrica: solo si usteer reporta varios routers activos.
  const usteerMultiRouter = useMemo(() => {
    const hosts = new Set<string>();
    for (const a of usteerAps ?? []) hosts.add(a.hostname || a.bssid);
    return hosts.size > 1;
  }, [usteerAps]);
  const visible = (id: Page) => {
    if (!modeReady) return id === "overview";
    // La línea de salida y todo lo que cuelga de ella (reenvío de puertos,
    // DPI, banIP) solo aplica a la puerta de enlace.
    if ((id === "internet" || id === "wan" || id === "forwards" || id === "dpi" || id === "banip") && !isRouter) return false;
    // Red local (DHCP/DNS/reservas): sin dnsmasq la página quedaría vacía
    // (p. ej. un switch gestionado), así que la entrada no se ofrece.
    if (id === "lan" && !mode?.dnsmasq_on) return false;
    if ((id === "wifi" || id === "coverage") && !hasWifiRadios) return false;
    if (id === "ports" && !hasSwitchPorts) return false;
    if (id === "storage" && !storage?.applicable) return false;
    if (id === "coverage" && !usteerMultiRouter) return false;
    if ((id === "adv-network" || id === "adv-switch") && !advanced) return false;
    if (id === "services" && isSwitch) return false;
    return true;
  };
  const activePage = NAV_GROUPS.some((g) => g.items.includes(page)) && visible(page) ? page : "overview";

  const navigate = useCallback((p: string) => {
    setPage(p as Page);
    setMenuOpen(false);
  }, []);

  const navList = (compact: boolean, onPick?: () => void) => (
    <div className="flex flex-col gap-0.5">
      {NAV_GROUPS.map((g, gi) => {
        const items = g.items.filter(visible);
        // Grupo vacío tras el filtro por rol (p.ej. Router en un AP, o
        // WiFi y Servicios en un switch): no se pinta la cabecera suelta.
        if (items.length === 0) return null;
        return (
        <div key={g.group ?? "top"} className={gi > 0 ? "mt-4" : ""}>
          {g.group && !compact && (
            <p className="text-eyebrow text-faint px-2.5 mb-1">{t(g.group)}</p>
          )}
          {g.group && compact && gi > 0 && <div className="mx-2 my-2 border-t border-border" aria-hidden="true" />}
          {items.map((id) => {
            const Icon = NAV_ICONS[id];
            const active = activePage === id;
            // En un switch la entrada de puertos cambia de nombre y lleva
            // el número de bocas (#487), como en el mockup de rediseño.
            const label = id === "ports" && isSwitch ? t("nav.portsSwitch") : t(`nav.${id}`);
            const portCount = id === "ports" && isSwitch ? mode?.port_count : undefined;
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
                </span>
                {!compact && <span className="flex-1 truncate">{label}</span>}
                {!compact && portCount !== undefined && portCount > 0 && (
                  <span className="px-1.5 py-0.5 rounded-md bg-accent-soft text-accent text-[10px] font-bold tabular-nums">
                    {portCount}
                  </span>
                )}
                {!compact && id === "overview" && <StatusDot tone={health.tone} label={t(health.labelKey)} />}
              </button>
            );
          })}
        </div>
        );
      })}
    </div>
  );

  // Bottom bar móvil §7.4: los 3-4 destinos más usados del modo actual + Menú.
  const bottomItems: Page[] = (
    isSwitch
      ? (["overview", "ports", "tools"] as Page[])
      : !isRouter
        ? (["overview", "wifi", "ports", "tools"] as Page[])
        : (["overview", "wifi", "traffic", "tools"] as Page[])
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
      action={<Button variant="secondary" size="sm" onClick={() => setSelfUpdateDialogOpen(true)}>{t("selfupdate.update")}</Button>}>
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
          board={board} system={system} wan={wan}
          drift={drift} onDriftChange={setDrift}
          isSwitch={role === "switch"} health={health} mode={mode}
          wireless={wireless} ethports={ethports} onNavigate={navigate}
        />
      )}
      {activePage === "traffic" && <TrafficPage onNavigate={navigate} />}
      {activePage === "clients" && <ClientsPage />}
      {activePage === "internet" && (
        <InternetPage mwan={mwan} onMwanChange={setMwan} fwd={fwd} onFwdChange={setFwd} />
      )}
      {activePage === "coverage" && <CoveragePage aps={usteerAps} error={usteerError} />}
      {activePage === "wifi" && (
        <WifiPage iot={iot} onIotChange={setIot} guest={guest} onGuestChange={setGuest} />
      )}
      {activePage === "lan" && (
        <LanPage ipv6={ipv6} onIpv6Change={setIpv6} />
      )}
      {activePage === "services" && (
        <Services wg={wg} onWgChange={setWg} ddns={ddns} onDdnsChange={setDdns} mdns={mdns} onMdnsChange={setMdns} sqm={sqm} onSqmChange={setSqm} ovpn={ovpn} onOvpnChange={setOvpn} ts={ts} onTsChange={setTs} apMode={!isRouter} onNavigate={navigate} />
      )}
      {activePage === "ports" && (
        <Ports ethports={ethports ?? []} />
      )}
      {activePage === "tools" && (
        <ToolsPage ethports={ethports ?? []} />
      )}
      {activePage === "diagnostics" && <DiagnosticsPage />}
      {activePage === "adv-network" && <AdvancedNetworkPage />}
      {activePage === "adv-switch" && <AdvancedSwitchPage />}
      {activePage === "fleet" && (
        <FleetPage />
      )}
      {activePage === "storage" && (
        <StoragePage />
      )}
      {activePage === "about" && <AboutPage />}
      {activePage === "system" && (
        <System board={board} update={update} onUpdateChange={setUpdate} onLogout={onLogout} />
      )}
      <SelfUpdateDialog
        open={selfUpdateDialogOpen}
        onClose={() => setSelfUpdateDialogOpen(false)}
        initialCheck={selfUpdate}
      />
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
          {/* Badge de rol (#487): debajo del nombre, solo cuando el rol es
              fiable (caché o probe confirmado); nunca en la versión compacta. */}
          {modeReady && (
            <div className="hidden lg:block px-1.5 pb-2 -mt-1">
              <span className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-semibold ${ROLE_BADGE[role].cls}`}>
                <span aria-hidden="true" className={`h-1.5 w-1.5 rounded-full ${ROLE_BADGE[role].dot}`} />
                {t(ROLE_BADGE[role].labelKey)}
              </span>
            </div>
          )}
          <div className="lg:hidden">{navList(true)}</div>
          <div className="hidden lg:block">{navList(false)}</div>
        </nav>

        {/* Contenido */}
        <main className="flex-1 min-w-0 p-4 md:p-6 pb-[calc(84px+env(safe-area-inset-bottom))] md:pb-12">
          {header}
          {demoBanner}
          {updateBanner}
          <AnnouncementBanner />
          {pageContent}
        </main>
      </div>

      {/* Bottom bar móvil §7.4 */}
      <nav aria-label={t("nav.menu")} className="md:hidden fixed bottom-0 inset-x-0 z-40 bg-surface border-t border-border flex pb-[env(safe-area-inset-bottom)]">
        {bottomItems.map((id) => {
          const Icon = NAV_ICONS[id];
          const active = activePage === id;
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
