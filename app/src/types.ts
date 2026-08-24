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

export interface DDNSProbe {
  installed: boolean;
  active: boolean;
  running: boolean;
  service_name: string;
  domain: string;
  lookup_host: string;
  username: string;
  registered_ip: string;
  last_update: string;
}

export interface SQMProbe {
  installed: boolean;
  has_wan: boolean;
  active: boolean;
  running: boolean;
  interface: string;
  download: string;
  upload: string;
}

export interface OVPNClient {
  name: string;
}

export interface OVPNProbe {
  installed: boolean;
  has_pki: boolean;
  active: boolean;
  running: boolean;
  port: string;
  subnet: string;
  clients: OVPNClient[];
}

export interface PkgUpgrade {
  name: string;
  current: string;
  available: string;
}

export interface IoTProbe {
  active: boolean;
  ssid: string;
  band: string;
  isolated: boolean;
  ifaces: string[];
  clients: number;
}

export interface FwdRule {
  section: string;
  name: string;
  src_dport: string;
  dest_ip: string;
  dest_port: string;
  proto: string;
}

export interface FwdProbe {
  has_wan: boolean;
  firewall: boolean;
  rules: FwdRule[];
}

export interface TSProbe {
  installed: boolean;
  running: boolean;
  state: string;
  auth_url?: string;
  ips: string[];
}

export interface GuestProbe {
  gateway: boolean;
  active: boolean;
  ssid: string;
  subnet: string;
  ifaces: string[];
  clients: number;
  gl_conflict: boolean;
}

export interface IfaceCounters {
  name: string;
  rx_bytes: number;
  tx_bytes: number;
}

export interface EthPort {
  name: string;
  up: boolean;
  speed_mbps: number;
  macs: string[];
}

export interface DawnClient {
  mac: string;
  signal: number;
}

export interface DawnAP {
  bssid: string;
  ssid: string;
  hostname: string;
  iface: string;
  channel: number;
  freq: number;
  util: number;
  num_sta: number;
  local: boolean;
  clients: DawnClient[];
}
