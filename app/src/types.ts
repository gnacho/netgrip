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
  hidden: boolean;
  bssid: string;
  rname: string;
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

export interface EthDevice {
  mac: string;
  name?: string;
}

export interface EthPort {
  name: string;
  wan: boolean;
  up: boolean;
  speed_mbps: number;
  devices: EthDevice[];
}

export interface ModeProbe {
  mode: "router" | "ap";
  hardware_class: "router" | "ap" | "switch";
  wan_in_bridge: boolean;
  wan_configured: boolean;
  dnsmasq_on: boolean;
  firewall_on: boolean;
  has_wifi: boolean;
  port_count: number;
}

export interface PanelAccess {
  http_port: number;
  https_enabled: boolean;
  force_https: boolean;
  session_ttl: string;
}

export interface LuciAccess {
  http_port: number;
  https_port: number;
  force_https: boolean;
  enabled: boolean;
}

export interface SSHAccess {
  enabled: boolean;
  port: string;
}

export interface AccessProbe {
  panel: PanelAccess;
  luci: LuciAccess;
  ssh: SSHAccess;
}

export interface RemoteAccess {
  applicable: boolean;
  ping_wan: boolean;
  remote_https: boolean;
  remote_ssh: boolean;
}

export interface Reservation {
  mac: string;
  ip: string;
  name?: string;
}

export interface DHCPConfig {
  enabled: boolean;
  start: number;
  limit: number;
  lease_time: number;
  gateway?: string;
  dns1?: string;
  dns2?: string;
}

export interface LANConfig {
  applicable: boolean;
  ipaddr: string;
  netmask: string;
  ap_isolation: boolean;
  dhcp: DHCPConfig;
  reservations: Reservation[];
}

export interface HostEntry {
  ip: string;
  hostname: string;
}

export interface DNSConfig {
  applicable: boolean;
  rebind_protection: boolean;
  override_dns: boolean;
  dns_vpn: boolean;
  adguard_active: boolean;
  hosts: HostEntry[];
}

export interface WifiUI {
  section: string;
  radio: string;
  ifname: string;
  band: string;
  ssid: string;
  encryption: string;
  has_key: boolean;
  hidden: boolean;
  mac: string;
  bssid: string;
  disabled: boolean;
  clients: { mac: string; signal?: number }[];
}

export interface OffloadProbe {
  applicable: boolean;
  software: boolean;
  hardware: boolean;
  hardware_known: boolean;
  hardware_active: boolean;
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

export interface Client {
  name: string;
  ip?: string;
  mac: string;
  type: "wifi24" | "wifi5" | "cable";
  iface?: string;
  signal?: number;
  rx_bytes: number;
  tx_bytes: number;
  self: boolean;
  reserved: boolean;
  reservable: boolean;
  blocked: boolean;
  blockable: boolean;
}

export interface ConfigSnapshot {
  id: string;
  timestamp: number;
  configs: number;
}

export interface ConfigDiff {
  config: string;
  before: string;
  after: string;
}

export interface IGMPProbe {
  applicable: boolean;
  enabled: boolean;
}

export interface LoopEntry {
  mac: string;
  ports: string[];
}

export interface LoopResult {
  loops: LoopEntry[];
  has_hub: boolean;
}

export interface SelfUpdateCheck {
  current: string;
  latest: string;
  available: boolean;
  notes: string;
  asset_url?: string;
  asset_size?: number;
}

export interface SelfUpdateStatus {
  phase: "idle" | "downloading" | "installing" | "restarting" | "error";
  progress: number;
  message?: string;
}

export interface WizardState {
  completed: boolean;
  mode: "router" | "ap";
}

export interface DriftLine {
  kind: "added" | "removed";
  text: string;
}

export interface DriftConfig {
  config: string;
  lines: DriftLine[];
}

export interface DriftProbe {
  has_baseline: boolean;
  snapshot_id: string;
  snapshot_ts: number;
  changes: number;
  configs: DriftConfig[];
}

export interface VLANPort {
  port: string;
  tagged: boolean;
}

export interface VLAN {
  vid: number;
  name: string;
  device: string;
  ports: VLANPort[];
  default: boolean;
}

export interface VLANProbe {
  applicable: boolean;
  bridge: string;
  vlans: VLAN[];
  ports: string[];
}

export interface VLANEdit {
  vid: number;
  ports: VLANPort[];
}
