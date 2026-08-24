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
};
