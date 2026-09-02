export class UnauthorizedError extends Error {}

/**
 * Modo demo §9: activo con `?demo=1` en la URL, localStorage
 * "netgrip:demo"="1" o build con VITE_DEMO=1. Cuando está activo, cada
 * método de `api` delega en `src/demo` (escenario García).
 */
export function isDemo(): boolean {
  try {
    if (import.meta.env.VITE_DEMO === "1") return true;
    if (new URLSearchParams(window.location.search).get("demo") === "1") {
      localStorage.setItem("netgrip:demo", "1");
      return true;
    }
    return localStorage.getItem("netgrip:demo") === "1";
  } catch {
    return false;
  }
}

export function enableDemo() {
  try {
    localStorage.setItem("netgrip:demo", "1");
  } catch { /* sin persistencia */ }
}

export function disableDemo() {
  try {
    localStorage.removeItem("netgrip:demo");
  } catch { /* sin persistencia */ }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (res.status === 401) throw new UnauthorizedError();
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error || `HTTP ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

const realApi = {
  login: (password: string) =>
    request<void>("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password }),
    }),
  logout: () => request<void>("/api/logout", { method: "POST" }),
  me: () => request<void>("/api/me"),
  board: () => request<import("./types").Board>("/api/board"),
  system: () => request<import("./types").SystemInfo>("/api/system"),
  wan: () => request<import("./types").WanStatus>("/api/wan"),
  wireless: () => request<import("./types").WirelessRadio[]>("/api/wireless"),
  leases: () => request<import("./types").Lease[]>("/api/leases"),
  clients: () => request<{ clients: import("./types").Client[]; bands: string[]; ts: number }>("/api/clients"),
  blockedClients: () => request<{ blocked: import("./types").BlockedClient[]; ts: number }>("/api/clients/blocked"),
  reserveClient: (mac: string, ip: string, reserved: boolean) =>
    request<{ status: string }>("/api/clients/reserve", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mac, ip, reserved }),
    }),
  blockClient: (mac: string, type: string, blocked: boolean, band?: string) =>
    request<{ status: string }>("/api/clients/block", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mac, type, blocked, band }),
    }),
  clientMeta: () => request<{ meta: Record<string, { name: string; device_type: string }> }>("/api/clients/meta"),
  setClientMeta: (mac: string, name: string, device_type: string) =>
    request<{ meta: Record<string, { name: string; device_type: string }> }>("/api/clients/meta", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mac, name, device_type }),
    }),
  ipv6: () => request<import("./types").IPv6Probe>("/api/ipv6"),
  setIpv6: (enabled: boolean) =>
    request<import("./types").IPv6SetResult>("/api/ipv6", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }),
  setPassword: (current: string, next: string) =>
    request<void>("/api/password", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ current, next }),
    }),
  updateCheck: () => request<import("./types").UpdateCheck>("/api/update"),
  startUpdate: () =>
    request<{ started: boolean; reboot_pending: boolean }>("/api/update", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ confirm: true }),
    }),
  wireguard: () => request<import("./types").WGProbe>("/api/wireguard"),
  setWireguard: (action: "enable" | "disable") =>
    request<import("./types").ModuleResult<import("./types").WGProbe>>("/api/wireguard", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action }),
    }),
  addWgPeer: (name: string, publicKey: string, admin: boolean) =>
    request<import("./types").ModuleResult<import("./types").WGProbe>>("/api/wireguard/peers", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, public_key: publicKey, admin }),
    }),
  addWgPeerQr: (name: string, admin: boolean) =>
    request<{ config: string; state: import("./types").WGProbe }>("/api/wireguard/peers/qr", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, admin }),
    }),
  deleteWgPeer: (publicKey: string) =>
    request<import("./types").ModuleResult<import("./types").WGProbe>>(
      "/api/wireguard/peers/delete",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ public_key: publicKey }),
      },
    ),
  ddns: () => request<import("./types").DDNSProbe>("/api/ddns"),
  netdev: () => request<{ counters: import("./types").IfaceCounters[]; ts: number }>("/api/netdev"),
  mode: () => request<import("./types").ModeProbe>("/api/mode"),
  setMode: (target: "router" | "ap") =>
    request<import("./types").ModuleResult<import("./types").ModeProbe>>("/api/mode", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ target, confirm: true }),
    }),
  access: () => request<import("./types").AccessProbe>("/api/access"),
  setLuciAccess: (luci: import("./types").LuciAccess) =>
    request<import("./types").ModuleResult<import("./types").AccessProbe>>("/api/access", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ target: "luci", luci }),
    }),
  setSshAccess: (ssh: import("./types").SSHAccess) =>
    request<import("./types").ModuleResult<import("./types").AccessProbe>>("/api/access", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ target: "ssh", ssh }),
    }),
  setPanelSessionTtl: (minutes: number) =>
    request<{ status: string; panel: import("./types").PanelAccess }>("/api/access", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ target: "panel_session", session_ttl_minutes: minutes }),
    }),
  remoteAccess: () => request<import("./types").RemoteAccess>("/api/remoteaccess"),
  setRemoteAccess: (opts: { ping_wan?: boolean; remote_https?: boolean; remote_ssh?: boolean }) =>
    request<import("./types").ModuleResult<import("./types").RemoteAccess>>("/api/remoteaccess", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(opts),
    }),
  offload: () => request<import("./types").OffloadProbe>("/api/offload"),
  setOffload: (enabled: boolean) =>
    request<import("./types").ModuleResult<import("./types").OffloadProbe>>("/api/offload", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }),
  mdns: () => request<import("./types").MDNSProbe>("/api/mdns"),
  setMdns: (enabled: boolean) =>
    request<import("./types").ModuleResult<import("./types").MDNSProbe>>("/api/mdns", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }),
  wifi: () => request<{ interfaces: import("./types").WifiUI[] }>("/api/wifi"),
  wifiKey: (section: string) => request<{ key: string }>(`/api/wifi/key?section=${encodeURIComponent(section)}`),
  setWifi: (edit: { section: string; ssid?: string; key?: string; encryption?: string; hidden?: boolean; disabled?: boolean; mac?: string }) =>
    request<import("./types").ModuleResult<import("./types").WifiUI>>("/api/wifi", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(edit),
    }),
  lan: () => request<import("./types").LANConfig>("/api/lan"),
  setLan: (opts: { ipaddr?: string; netmask?: string; ap_isolation?: boolean }) =>
    request<import("./types").ModuleResult<import("./types").LANConfig>>("/api/lan", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(opts),
    }),
  setDhcp: (cfg: import("./types").DHCPConfig) =>
    request<import("./types").ModuleResult<import("./types").LANConfig>>("/api/lan/dhcp", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    }),
  setReservation: (mac: string, ip: string, name: string, reserved: boolean) =>
    request<import("./types").ModuleResult<import("./types").LANConfig>>("/api/lan/reservation", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mac, ip, name, reserved }),
    }),
  clearReservations: () =>
    request<import("./types").ModuleResult<import("./types").LANConfig>>("/api/lan/reservations/clear", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({}),
    }),
  dns: () => request<import("./types").DNSConfig>("/api/dns"),
  setDns: (opts: { rebind_protection?: boolean; override_dns?: boolean; dns_vpn?: boolean }) =>
    request<import("./types").ModuleResult<import("./types").DNSConfig>>("/api/dns", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(opts),
    }),
  setDnsHost: (ip: string, hostname: string, remove: boolean) =>
    request<import("./types").ModuleResult<import("./types").DNSConfig>>("/api/dns/hosts", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ip, hostname, remove }),
    }),
  ethports: () => request<{ ports: import("./types").EthPort[] }>("/api/ethports"),
  usteer: () => request<{ aps: import("./types").UsteerAP[] }>("/api/usteer"),
  guestwifi: () => request<import("./types").GuestProbe>("/api/guestwifi"),
  setGuestwifi: (cfg: { enabled: boolean; ssid?: string; key?: string; band?: string }) =>
    request<import("./types").ModuleResult<import("./types").GuestProbe>>("/api/guestwifi", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    }),
  tailscale: () => request<import("./types").TSProbe>("/api/tailscale"),
  setTailscale: (enabled: boolean) =>
    request<import("./types").ModuleResult<import("./types").TSProbe>>("/api/tailscale", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }),
  portforward: () => request<import("./types").FwdProbe>("/api/portforward"),
  addFwdRule: (src_dport: string, dest_ip: string, dest_port: string, proto: string) =>
    request<import("./types").ModuleResult<import("./types").FwdProbe>>("/api/portforward", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ src_dport, dest_ip, dest_port, proto }),
    }),
  deleteFwdRule: (section: string) =>
    request<import("./types").ModuleResult<import("./types").FwdProbe>>("/api/portforward/delete", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ section }),
    }),
  iotwifi: () => request<import("./types").IoTProbe>("/api/iotwifi"),
  setIotwifi: (cfg: { enabled: boolean; ssid?: string; key?: string; band?: string }) =>
    request<import("./types").ModuleResult<import("./types").IoTProbe>>("/api/iotwifi", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    }),
  packages: () => request<{ upgradable: import("./types").PkgUpgrade[] }>("/api/packages"),
  optionalPackages: () =>
    request<{ packages: import("./types").OptionalPackage[] }>("/api/packages/optional"),
  wizardPackages: (ids: string[]) =>
    request<{ installed: string[] }>("/api/wizard/packages", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ids }),
    }),
  upgradePackage: (name: string) =>
    request<{ upgradable: import("./types").PkgUpgrade[] }>("/api/packages/upgrade", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    }),
  sqm: () => request<import("./types").SQMProbe>("/api/sqm"),
  openvpn: () => request<import("./types").OVPNProbe>("/api/openvpn"),
  setOpenvpn: (action: "enable" | "disable") =>
    request<import("./types").ModuleResult<import("./types").OVPNProbe>>("/api/openvpn", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action }),
    }),
  addOvpnClient: (name: string) =>
    request<{ config: string; state: import("./types").OVPNProbe }>("/api/openvpn/clients", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    }),
  deleteOvpnClient: (name: string) =>
    request<{ state: import("./types").OVPNProbe }>("/api/openvpn/clients/delete", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    }),
  setSqm: (cfg: { enabled: boolean; download?: string; upload?: string }) =>
    request<import("./types").ModuleResult<import("./types").SQMProbe>>("/api/sqm", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    }),
  runBufferbloatTest: () =>
    request<import("./types").BufferbloatResult>("/api/sqm/test", { method: "POST" }),
  bufferbloatHistory: () =>
    request<{ entries: import("./types").BufferbloatResult[] }>("/api/sqm/history"),
  setDdns: (cfg: {
    enabled: boolean;
    service_name?: string;
    domain?: string;
    lookup_host?: string;
    username?: string;
    password?: string;
  }) =>
    request<import("./types").ModuleResult<import("./types").DDNSProbe>>("/api/ddns", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    }),
  deleteDdns: (domain: string) =>
    request<import("./types").ModuleResult<import("./types").DDNSProbe>>("/api/ddns", {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ domain }),
    }),
  snapshots: () =>
    request<{ snapshots: import("./types").ConfigSnapshot[] }>("/api/config/snapshots"),
  createSnapshot: () =>
    request<import("./types").ConfigSnapshot>("/api/config/snapshot", { method: "POST" }),
  deleteSnapshot: (id: string) =>
    request<void>(`/api/config/snapshot?id=${encodeURIComponent(id)}`, { method: "DELETE" }),
  snapshotDiff: (from: string, to: string) =>
    request<{ diffs: import("./types").ConfigDiff[] }>(`/api/config/diff?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`),
  rollbackSnapshot: (id: string) =>
    request<{ status: string }>("/api/config/rollback", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id }),
    }),
  bouncePort: (iface: string) =>
    request<{ iface: string; ok: boolean }>("/api/ports/bounce", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ iface }),
    }),
  blockPort: (iface: string, blocked: boolean) =>
    request<{ iface: string; ok: boolean }>("/api/ports/block", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ iface, blocked }),
    }),
  igmp: () => request<import("./types").IGMPProbe>("/api/igmp"),
  setIgmp: (enabled: boolean) =>
    request<import("./types").IGMPProbe>("/api/igmp", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }),
  loops: () => request<import("./types").LoopResult>("/api/loops"),
  selfUpdateCheck: () =>
    request<import("./types").SelfUpdateCheck>("/api/selfupdate"),
  selfUpdateApply: () =>
    request<{ status: string; restarting: boolean }>("/api/selfupdate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ confirm: true }),
    }),
  selfUpdateStatus: () =>
    request<import("./types").SelfUpdateStatus>("/api/selfupdate/status"),
  wizardState: () =>
    request<import("./types").WizardState>("/api/wizard"),
  wizardSetup: () =>
    request<import("./types").WizardSetupProbe>("/api/wizard/setup"),
  installWizardSetup: (mode: string, groups?: string[]) =>
    request<{ installed: string[] }>("/api/wizard/setup", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mode, groups }),
    }),
  wizardComplete: () =>
    request<{ status: string }>("/api/wizard/complete", { method: "POST" }),
  drift: () =>
    request<import("./types").DriftProbe>("/api/drift"),
  vlans: () =>
    request<import("./types").VLANProbe>("/api/vlans"),
  setVlan: (edit: import("./types").VLANEdit) =>
    request<import("./types").ModuleResult<import("./types").VLANProbe>>("/api/vlans", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(edit),
    }),
  deleteVlan: (vid: number) =>
    request<import("./types").ModuleResult<import("./types").VLANProbe>>("/api/vlans", {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ vid }),
    }),
  lag: () => request<import("./types").LAGProbe>("/api/lag"),
  setLag: (cfg: import("./types").LAGConfig) =>
    request<import("./types").ModuleResult<import("./types").LAGProbe>>("/api/lag", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    }),
  deleteLag: (name: string) =>
    request<import("./types").ModuleResult<import("./types").LAGProbe>>("/api/lag", {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    }),
  history: () =>
    request<{ entries: import("./types").HistoryEntry[] }>("/api/history"),
  httpsState: () =>
    request<{ has_cert: boolean }>("/api/https"),
  enableHttps: () =>
    request<{ status: string }>("/api/https", { method: "POST" }),
  wakeOnLan: (mac: string) =>
    request<{ status: string }>("/api/wol", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mac }),
    }),
  telegramGet: () =>
    request<{ botToken: string; chatId: string; enabled: boolean }>("/api/telegram"),
  telegramSet: (botToken: string, chatId: string, enabled: boolean) =>
    request<{ ok: boolean; botName: string; chatName: string }>("/api/telegram", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ botToken, chatId, enabled }),
    }),
  telegramTest: () =>
    request<{ ok: boolean }>("/api/telegram/test", { method: "POST" }),
  nlbwmon: () =>
    request<import("./types").NlbwmonProbe>("/api/nlbwmon"),
  setNlbwmon: (cfg: import("./types").NlbwmonConfig) =>
    request<import("./types").ModuleResult<import("./types").NlbwmonProbe>>("/api/nlbwmon", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    }),
  firewall: () =>
    request<import("./types").FirewallProbe>("/api/firewall"),
  addFirewallRule: (rule: import("./types").FirewallRuleAdd) =>
    request<import("./types").ModuleResult<import("./types").FirewallProbe>>("/api/firewall", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(rule),
    }),
  deleteFirewallRule: (section: string) =>
    request<import("./types").ModuleResult<import("./types").FirewallProbe>>("/api/firewall", {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ section }),
    }),
  templates: () =>
    request<{ templates: import("./types").Template[] }>("/api/templates"),
  applyTemplate: (id: string, confirm: boolean) =>
    request<{ status: string }>("/api/templates", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id, confirm }),
    }),
  switchPorts: () =>
    request<import("./types").SwitchProbe>("/api/switch"),
  setSwitchPort: (edit: import("./types").SwitchPortEdit) =>
    request<import("./types").ModuleResult<import("./types").SwitchProbe>>("/api/switch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(edit),
    }),
  portStats: () =>
    request<import("./types").PortStatsProbe>("/api/port-stats"),
  switchModes: () =>
    request<{ modes: import("./types").SwitchMode[] }>("/api/switch/modes"),
  applySwitchMode: (id: string, uplinkPort: string, confirm: boolean) =>
    request<{ status: string }>("/api/switch/modes", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id, uplink_port: uplinkPort, confirm }),
    }),
  poe: () =>
    request<import("./types").PoEProbe>("/api/poe"),
  setPoESchedule: (sched: import("./types").PoESchedule) =>
    request<{ status: string; state: import("./types").PoEProbe }>("/api/poe/schedule", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(sched),
    }),
  poeWatchdogs: () =>
    request<{ watchdogs: import("./types").PoEWatchdogState[] }>("/api/poe/watchdog"),
  setPoEWatchdog: (cfg: import("./types").PoEWatchdogConfig) =>
    request<{ status: string; watchdogs: import("./types").PoEWatchdogState[] }>("/api/poe/watchdog", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    }),
  portTemplates: () =>
    request<{ templates: import("./types").PortTemplate[] }>("/api/port-templates"),
  savePortTemplate: (tpl: import("./types").PortTemplateSave) =>
    request<{ status: string }>("/api/port-templates", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(tpl),
    }),
  deletePortTemplate: (name: string) =>
    request<{ status: string }>("/api/port-templates", {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    }),
  applyPortTemplate: (template: string, ports: string[]) =>
    request<{ status: string }>("/api/port-templates/apply", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ template, ports }),
    }),
  roleProfiles: () =>
    request<{ roles: import("./types").RoleProfile[] }>("/api/roles"),
  applyRoleProfile: (roleId: string, port: string) =>
    request<{ status: string }>("/api/roles", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ role_id: roleId, port }),
    }),
  dpi: () =>
    request<import("./types").DPIProbe>("/api/dpi"),
  fleet: () =>
    request<{ nodes: import("./types").FleetNodeStatus[] }>("/api/fleet"),
  addFleetNode: (node: { id: string; name: string; address: string; password: string }) =>
    request<{ status: string }>("/api/fleet", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(node),
    }),
  deleteFleetNode: (id: string) =>
    request<{ status: string }>("/api/fleet", {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id }),
    }),
  checkFleetNode: (id: string) =>
    request<import("./types").FleetNodeStatus>("/api/fleet/check", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id }),
    }),
  checkAllFleet: () =>
    request<{ nodes: import("./types").FleetNodeStatus[] }>("/api/fleet/check-all", {
      method: "POST",
    }),
  discoveredFleet: () =>
    request<{ peers: import("./types").DiscoveredFleetPeer[] }>("/api/fleet/discovered"),
  adoptFleetPeer: (peer: { id: string; name: string; address: string; password: string }) =>
    request<{ status: string }>("/api/fleet/adopt", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(peer),
    }),
  updateFleetNode: (id: string) =>
    request<{ status: string }>("/api/fleet/update", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id }),
    }),
  fleetDiscoveryConfig: () =>
    request<{ enabled: boolean }>("/api/fleet/discovery-config"),
  setFleetDiscoveryConfig: (enabled: boolean) =>
    request<{ enabled: boolean }>("/api/fleet/discovery-config", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled }),
    }),
  cableTest: () =>
    request<import("./types").CableTestProbe>("/api/cable-test"),
  stormControl: () =>
    request<import("./types").StormProbe>("/api/storm"),
  setStormControl: (port: string, percent: number) =>
    request<{ status: string }>("/api/storm", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ port, percent }),
    }),
  storage: () =>
    request<import("./types").StorageProbe>("/api/storage"),
  setStorageService: (name: string, action: "enable" | "disable") =>
    request<{ status: string }>("/api/storage", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, action }),
    }),
  macAcl: () =>
    request<import("./types").MACACLProbe>("/api/mac-acl"),
  setMacAcl: (port: string, mode: string, macs: string[]) =>
    request<{ status: string }>("/api/mac-acl", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ port, mode, macs }),
    }),
  netpulse: () =>
    request<import("./types").NetPulseState>("/api/netpulse"),
  setNetPulse: (cfg: import("./types").NetPulseSet) =>
    request<import("./types").NetPulseState>("/api/netpulse", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(cfg),
    }),
  pushConfigGet: () =>
    request<{ server_url: string; router_id: string; token: string }>("/api/push-config"),
  pushConfigSet: (server_url: string, router_id: string, token: string) =>
    request<{ status: string }>("/api/push-config", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ server_url, router_id, token }),
    }),
  pushSnapshot: () =>
    request<{ ok: boolean; snapshot_id?: string; error?: string }>("/api/push-config/push", { method: "POST" }),

};

/** API pública: delega en `src/demo` cuando el modo demo está activo.
 *  El demo se importa de forma perezosa (dynamic import) para que las
 *  fixtures NO viajen en el bundle de producción embebido en el router. */
type DemoModule = typeof import("./demo");
let demoPromise: Promise<DemoModule> | undefined;
const loadDemo = () => (demoPromise ??= import("./demo"));

export const api = new Proxy(realApi, {
  get(target, prop) {
    const real = (target as Record<string | symbol, unknown>)[prop];
    if (typeof real !== "function") return real;
    return (...args: unknown[]) => {
      if (isDemo()) {
        return loadDemo().then((m) => {
          const demo = m.demoApi as unknown as Record<string | symbol, (...a: unknown[]) => unknown>;
          return demo[prop](...args);
        });
      }
      return (real as (...a: unknown[]) => unknown)(...args);
    };
  },
});
