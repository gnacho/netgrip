export interface Board {
  model: string;
  hostname: string;
  kernel: string;
  release?: { distribution: string; version: string; revision: string };
}

export interface SystemInfo {
  uptime: number;
  load: number[];
  memory: { total: number; free: number; available: number; cached: number };
  root: { total: number; free: number };
}

export interface WanStatus {
  present: boolean;
  up: boolean;
  uptime: number;
  ipv4: string[];
  gateway?: string;
  dns: string[];
}

export interface WirelessClient {
  mac: string;
  signal?: number;
}

export interface WirelessInterface {
  ifname: string;
  ssid: string;
  encryption: string;
  disabled: boolean;
  clients: WirelessClient[];
}

export interface WirelessRadio {
  name: string;
  up: boolean;
  band: string;
  channel: string;
  htmode: string;
  txpower: number;
  interfaces: WirelessInterface[];
}

export interface Lease {
  expires: string;
  mac: string;
  ip: string;
  hostname: string;
}

export interface IPv6Probe {
  state: "enabled" | "disabled" | "partial";
  lan_ipv6: string;
  odhcpd_enabled: boolean;
  ra_mode: string;
  dhcpv6_mode: string;
}

export interface IPv6SetResult {
  status: "applied" | "rolled_back" | "failed";
  rolled_back: boolean;
  state: IPv6Probe;
  error?: string;
}

export interface UpdateCheck {
  available: boolean;
  same_version: boolean;
  owut_present: boolean;
  version_from: string;
  version_to: string;
  out_of_date_packages: number;
  warnings: string[];
  safe_to_proceed: boolean;
  missing_packages: string[];
  safe_with_reinstall: boolean;
}

export interface WGPeer {
  section: string;
  name: string;
  public_key: string;
  allowed_ips: string[];
  admin: boolean;
}

export interface WGProbe {
  installed: boolean;
  active: boolean;
  running: boolean;
  port: string;
  address: string;
  public_key: string;
  peers: WGPeer[];
}

export interface ModuleResult<T> {
  status: "applied" | "rolled_back" | "failed";
  rolled_back: boolean;
  state: T;
  error?: string;
}
