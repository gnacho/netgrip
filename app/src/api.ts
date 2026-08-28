export class UnauthorizedError extends Error {}

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

export const api = {
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
  clients: () => request<{ clients: import("./types").Client[]; ts: number }>("/api/clients"),
  reserveClient: (mac: string, ip: string, reserved: boolean) =>
    request<{ status: string }>("/api/clients/reserve", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mac, ip, reserved }),
    }),
  blockClient: (mac: string, type: string, blocked: boolean) =>
    request<{ status: string }>("/api/clients/block", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mac, type, blocked }),
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
  wifi: () => request<{ interfaces: import("./types").WifiUI[] }>("/api/wifi"),
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
  dawn: () => request<{ aps: import("./types").DawnAP[] }>("/api/dawn"),
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
};
