export interface Board {
  model: string;
  hostname: string;
  kernel: string;
  release?: { distribution: string; version: string; revision: string };
}

export interface SystemInfo {
  uptime: number;
  load: number[];
  memory: { total: number; free: number; available: number; cached: number; buffered: number };
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

/** WireGuard tunnel run by the GL.iNet firmware (read-only for NetGrip). */
export interface GLTunnel {
  iface: string;
  address: string;
  port: string;
  public_key: string;
  peers: WGPeer[];
  running: boolean;
}

export interface WGProbe {
  installed: boolean;
  active: boolean;
  running: boolean;
  port: string;
  address: string;
  public_key: string;
  peers: WGPeer[];
  /** "gl_firmware" when the GL.iNet firmware manages WireGuard here */
  managed_by?: string;
  gl_tunnels: GLTunnel[];
}

export interface ModuleResult<T> {
  status: "applied" | "rolled_back" | "failed";
  rolled_back: boolean;
  state: T;
  error?: string;
}

export interface DDNSEntry {
  section: string;
  enabled: boolean;
  running: boolean;
  service_name: string;
  domain: string;
  lookup_host: string;
  username: string;
  registered_ip: string;
  last_update: string;
}

export interface DDNSProbe {
  installed: boolean;
  entries: DDNSEntry[];
  wan_ip: string;
}

export interface MDNSProbe {
  installed: boolean;
  enabled: boolean;
  running: boolean;
  domain: string;
}

export interface SQMProbe {
  installed: boolean;
  has_wan: boolean;
  active: boolean;
  running: boolean;
  interface: string;
  download: string;
  upload: string;
  profile: string;
  qdisc: string;
  script: string;
}

export interface BufferbloatResult {
  baseline_ms: number;
  loaded_ms: number;
  delta_ms: number;
  grade: string;
  timestamp: string;
  samples_loaded: number[];
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
  /** Stable hostname configured for client configs ("" when unset). */
  public_host: string;
  /** Current WAN IPv4, shown while no stable host is set. */
  wan_ip: string;
  /** DDNS domains already managed by this router, as suggestions. */
  ddns_domains: string[];
}

export interface PkgUpgrade {
  name: string;
  current: string;
  available: string;
}

export interface OptionalPackage {
  id: string;
  packages: string[];
  i18n_key: string;
  module: string;
  installed: boolean;
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
  /** "forward": DNAT to a host on the LAN. "input": a port on the router
   *  itself (a VPN listener, an exposed admin page). */
  kind?: "forward" | "input";
  /** Created by NetGrip, so it can be removed from here. Rules set up in
   *  LuCI or by hand are listed read-only. */
  managed?: boolean;
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

export interface CaptivePortalProbe {
  applicable: boolean;
  installed: boolean;
  active: boolean;
  running: boolean;
  interface: string;
  title: string;
  message: string;
  has_access_code: boolean;
  session_minutes: number;
  custom_html: boolean;
  has_image: boolean;
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
  adguard_installed: boolean;
  adguard_running: boolean;
  adguard_protection: boolean;
  adguard_has_backup: boolean;
  adguard_dns_port?: number;
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

// usteer mesh AP/client types.
export interface UsteerClient {
  mac: string;
  signal: number;
}
export interface UsteerAP {
  bssid: string;
  ssid: string;
  hostname: string;
  iface: string;
  channel: number;
  freq: number;
  util: number;
  num_sta: number;
  local: boolean;
  clients: UsteerClient[];
}


export interface Client {
  name: string;
  ip?: string;
  mac: string;
  type: "wifi24" | "wifi5" | "wifi6" | "cable";
  device_type?: string;
  iface?: string;
  signal?: number;
  rx_bytes: number;
  tx_bytes: number;
  self: boolean;
  reserved: boolean;
  reservable: boolean;
  blocked: boolean;
  blocked_on?: string[];
  blockable: boolean;
  parental_blocked?: boolean;
  parental_next?: string;
  lease_expiry?: number;
  lease_source?: "local" | "gateway";
  ip_source?: "arp";
  quota_used?: number;
  quota_limit?: number;
  quota_remaining?: number;
  quota_period?: string;
  quota_throttled?: boolean;
  quota_exceeded?: boolean;
}

export interface BlockedClient {
  mac: string;
  type: "wifi" | "cable";
  bands?: string[];
  blocked_everywhere: boolean;
}

export type DeviceType =
  | "pc" | "phone" | "tablet" | "camera" | "wearable" | "laptop" | "printer"
  | "audio" | "tv" | "iot" | "gaming" | "gateway" | "nas" | "server"
  | "appliance" | "ac" | "speaker" | "vacuum" | "pool" | "mower" | "other";

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
  /** Tag is newer but the downloadable asset is not published yet (CI window). */
  assets_pending?: boolean;
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

export interface WizardSetupGroup {
  id: string;
  title_key: string;
  packages: string[];
}

export interface WizardSetupProbe {
  manager: "apk" | "opkg";
  groups: WizardSetupGroup[];
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
  /** Port's ingress VLAN (the "*" in UCI's "lan2:u*"); round-tripped untouched. */
  pvid?: boolean;
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

export interface LAGEntry {
  name: string;
  device: string;
  mode: string;
  slaves: string[];
  up: boolean;
}

export interface LAGProbe {
  applicable: boolean;
  installed: boolean;
  lags: LAGEntry[];
  free_ports: string[];
}

export interface LAGConfig {
  name: string;
  mode: string;
  slaves: string[];
}

export interface HistoryEntry {
  ts: number;
  rx: number;
  tx: number;
  load: number;
  clients: number;
}

/** One accounted row from nlbwmon: a device (key = MAC) or a protocol
 *  (key = "HTTPS", "QUIC", "other"…). Down/Up are from the DEVICE's point
 *  of view, unlike Client.rx_bytes where rx is what the AP received. */
/** One core. dropped/squeezed are since boot; the _rate fields are per
 *  second since the previous poll — the ones that say it is happening now. */
export interface CPUCore {
  idx: number;
  usage_pct: number;
  freq_mhz?: number;
  dropped: number;
  dropped_rate: number;
  squeezed: number;
  squeezed_rate: number;
}

export interface CPUProc {
  pid: number;
  name: string;
  /** CPU% between polls; absent on the memory ranking, where only RSS matters. */
  usage_pct?: number;
  /** Resident set in bytes: the memory side of "who is consuming". */
  rss_bytes?: number;
}

export interface CPUProbe {
  cores: CPUCore[];
  usage_pct: number;
  /** The highest single core: on a router this predicts trouble, the
   *  average does not. */
  busiest_pct: number;
  load: number[];
  temp_c?: number;
  /** Which chip the reading came from: a board may expose no CPU sensor,
   *  and a WiFi radio idles warmer than a SoC. */
  temp_source?: string;
  procs: CPUProc[];
  /** Processes holding the most resident memory (point-in-time, top 10). */
  mem_procs?: CPUProc[];
  /** True until there are two samples to compare. */
  warming: boolean;
}

export interface NlbwUsage {
  key: string;
  ip?: string;
  conns: number;
  down_bytes: number;
  up_bytes: number;
}

export interface NlbwmonTop {
  devices: NlbwUsage[];
  apps: NlbwUsage[];
}

export interface NlbwmonProbe {
  installed: boolean;
  running: boolean;
  generations: number;
  commit_interval: number;
  prealloc_days: number;
  protocol_database: boolean;
}

export interface NlbwmonConfig {
  enabled?: boolean;
  generations?: number;
  commit_interval?: number;
  prealloc_days?: number;
}

// banIP (#351). banIP owns its nft table; NetGrip reads UCI + init.d only.
export interface BanipFeed {
  name: string;
  enabled: boolean;
  /** "in" | "out" | "inout" override; "" = feed default */
  direction: "" | "in" | "out" | "inout";
  /** true when the feed exists in the local catalog (banip.feeds / custom) */
  in_catalog: boolean;
  /** catalog defaults, attached by the backend for configured feeds too */
  chain?: "" | "in" | "out" | "inout";
  ipv6?: boolean;
  /** true when the last download failed and the feed's sets are still empty */
  last_download_failed?: boolean;
}

/** One feed available in the local catalog, not yet configured in UCI. */
export interface BanipCatalogFeed {
  name: string;
  descr: string;
  /** default direction from the catalog: "in" | "out" | "inout"; "" = unspecified */
  chain: "" | "in" | "out" | "inout";
  ipv6: boolean;
  custom: boolean;
}

export interface BanipSetStat {
  name: string;
  elements: number;
  packets_in: number;
  packets_out: number;
  local_allow: boolean;
  local_block: boolean;
}

export interface BanipDos {
  syn_packets: number;
  udp_packets: number;
  icmp_packets: number;
  invalid_ct_packets: number;
  invalid_tcp_packets: number;
  syn_limit: number;
  udp_limit: number;
  icmp_limit: number;
}

export interface BanipReport {
  parsed: boolean;
  timestamp: string;
  sets: BanipSetStat[];
  total_ips: number;
  packets_in: number;
  packets_out: number;
  auto_allow: number;
  auto_block: number;
  dos: BanipDos;
}

export interface BanipProbe {
  installed: boolean;
  enabled: boolean;
  running: boolean;
  nft_count: boolean;
  applicable: boolean;
  version: string;
  mem_available_mb: number;
  feeds: BanipFeed[];
  /** available feeds not configured in UCI (from banip.feeds / custom.feeds) */
  catalog: BanipCatalogFeed[];
  report?: BanipReport;
  allowlist: string[];
  blocklist: string[];
}

export interface BanipFeedsConfig {
  feeds: BanipFeed[];
  enabled?: boolean;
  nft_count?: boolean;
}

export interface BanipSearchResult {
  ip: string;
  found: boolean;
  sets: string[];
}

/** Lightweight banIP status for the Services overview card (no report/lists). */
export interface BanipStatus {
  installed: boolean;
  enabled: boolean;
  running: boolean;
  applicable: boolean;
}

export interface FWZone {
  name: string;
  input: string;
  output: string;
  forward: string;
  network: string[];
  masq: boolean;
}

export interface FWRule {
  name: string;
  section: string;
  src: string;
  dest: string;
  proto: string;
  dest_port: string;
  target: string;
}

export interface FirewallProbe {
  applicable: boolean;
  zones: FWZone[];
  rules: FWRule[];
}

export interface FirewallRuleAdd {
  name: string;
  src: string;
  dest: string;
  proto: string;
  dest_port: string;
  target: string;
}

export interface Template {
  id: string;
  name: string;
  description: string;
  destructive: boolean;
}

export interface SwitchPort {
  name: string;
  admin_up: boolean;
  oper_up: boolean;
  speed_mbps: number;
  duplex: string;
  poe_enabled: boolean;
  poe_supported: boolean;
  description: string;
}

export interface SwitchProbe {
  applicable: boolean;
  ports: SwitchPort[];
}

export interface SwitchPortEdit {
  name: string;
  admin_up?: boolean;
  speed_mbps?: number;
  poe_enabled?: boolean;
  description?: string;
}

export interface PortStats {
  name: string;
  rx_bytes: number;
  tx_bytes: number;
  rx_errors: number;
  tx_errors: number;
  rx_drops: number;
  tx_drops: number;
}

export interface PortStatsProbe {
  ports: PortStats[];
  ts: number;
}

export interface SwitchMode {
  id: string;
  name: string;
  description: string;
}

export interface PoEPort {
  name: string;
  enabled: boolean;
  power_w: number;
  class: string;
  status: string;
  schedule_on: string;
  schedule_off: string;
}

export interface PoEProbe {
  applicable: boolean;
  total_budget_w: number;
  used_w: number;
  ports: PoEPort[];
}

export interface PoESchedule {
  port: string;
  on_time: string;
  off_time: string;
}

export interface PoEWatchdogConfig {
  port: string;
  enabled: boolean;
  target: string;
  threshold: number;
  interval_s: number;
  cooldown_s: number;
}

export interface PoEWatchdogState {
  config: PoEWatchdogConfig;
  failures: number;
  last_check: string;
  last_cycle: string;
  cooling: boolean;
}

export interface PortTemplateVLAN {
  vid: number;
  tagged: boolean;
}

export interface PortTemplate {
  name: string;
  description: string;
  vlans: PortTemplateVLAN[];
  admin_up: boolean;
  speed_mbps: number;
}

export interface PortTemplateSave {
  name: string;
  description: string;
  vlans: PortTemplateVLAN[];
  admin_up: boolean;
  speed_mbps: number;
}

export interface RoleProfile {
  id: string;
  name: string;
  description: string;
  vid: number;
  isolated: boolean;
}

export interface DPIProtocol {
  name: string;
  bytes: number;
  flows: number;
  category: string;
}

export interface DPIProbe {
  applicable: boolean;
  total_bytes: number;
  total_flows: number;
  protocols: DPIProtocol[];
}

export interface NetifydApp {
  name: string;
  bytes: number;
  local_bytes: number;
  other_bytes: number;
  packets: number;
  flows: number;
}

export interface NetifydBucket {
  local: number;
  other: number;
  total: number;
}

export interface NetifydTimelineBucket {
  time: string;
  apps: Record<string, NetifydBucket>;
}

export interface NetifydTimeline {
  buckets: NetifydTimelineBucket[];
  top: NetifydApp[];
  totals: NetifydBucket;
}

export interface NetifydProbe {
  installed: boolean;
  enabled: boolean;
  running: boolean;
  applicable: boolean;
  low_end: boolean;
  apps: NetifydApp[];
}

export interface FleetNodeStatus {
  id: string;
  name: string;
  address: string;
  reachable: boolean;
  current_version: string;
  latest_version: string;
  update_available: boolean;
  error?: string;
}

export interface DiscoveredFleetPeer {
  id: string;
  name: string;
  version: string;
  address: string;
  port: number;
  seen_at: string;
}

export interface NftQoSLimit {
  mac: string;
  ip: string;
  download: number;
  upload: number;
}

export interface NftQoSProbe {
  applicable: boolean;
  limits: Record<string, NftQoSLimit>;
}

export interface ParentalRule {
  mac: string;
  enabled: boolean;
  days: number[]; // 0=Sunday..6=Saturday
  start: string; // "HH:MM" local
  end: string; // "HH:MM" local; earlier than start = overnight
  paused: boolean;
}

export interface ParentalProbe {
  rules: Record<string, ParentalRule>;
  ts: number;
}

export interface Quota {
  mac: string;
  ip: string;
  period: "daily" | "monthly";
  limit: number; // bytes
  action: "notify" | "throttle";
  throttle_kbps: number; // kbps, throttle action only
}

export interface QuotaUsage {
  used: number;
  limit: number;
  remaining: number;
  period: string;
  throttled: boolean;
  exceeded: boolean;
}

export interface QuotaProbe {
  applicable: boolean;
  quotas: Record<string, Quota>;
  usage: Record<string, QuotaUsage>;
  ts: number;
}

export interface CableTestResult {
  port: string;
  supported: boolean;
  pair_status?: string;
  length?: string;
  error?: string;
}

export interface CableTestProbe {
  applicable: boolean;
  missing_tool?: string;
  ports: CableTestResult[];
}

export interface DiagnosticsTools {
  ping: boolean;
  traceroute: boolean;
  nslookup: boolean;
  dig: boolean;
}

/** One line of the self-test with the reason behind it. `info` marks
 *  something that could not be proven but is not a fault — a provider that
 *  does not answer pings is the usual case. */
export interface SelfTestCheck {
  key: "gateway" | "wan" | "dns" | "ntp";
  ok: boolean;
  info?: boolean;
  detail?: string;
}

export interface SelfTestResult {
  gateway: boolean;
  wan: boolean;
  dns: boolean;
  ntp: boolean;
  all_ok: boolean;
  /** The same results with their reasons, in display order. */
  checks: SelfTestCheck[];
  tools: DiagnosticsTools;
}

export interface PingSample {
  seq: number;
  time_ms: number;
  error: boolean;
}

export interface PingResult {
  host: string;
  count: number;
  sent: number;
  received: number;
  loss_pct: number;
  min_ms: number;
  avg_ms: number;
  max_ms: number;
  samples: PingSample[];
  missing_tool?: string;
}

export interface TraceHop {
  hop: number;
  host: string;
  rtts: number[];
  raw?: string;
  parsed: boolean;
}

export interface TracerouteResult {
  host: string;
  hops: TraceHop[];
  missing_tool?: string;
}

export interface DNSAnswer {
  name: string;
  type?: string;
  value: string;
  ttl?: number;
}

export interface DNSResult {
  query: string;
  resolver?: string;
  answers: DNSAnswer[];
  raw?: string;
  parsed: boolean;
  missing_tool?: string;
}

export interface TCPResult {
  host: string;
  port: number;
  open: boolean;
  error?: string;
}

export type DiagnosticsResult = PingResult | TracerouteResult | DNSResult | TCPResult;

export interface StormPort {
  port: string;
  link_speed_mbps: number;
  broadcast_kbps: number;
  multicast_kbps: number;
  unknown_unicast_kbps: number;
  active: boolean;
}

export interface StormProbe {
  applicable: boolean;
  ports: StormPort[];
}

export interface StorageDevice {
  name: string;
  path: string;
  fs_type: string;
  size_bytes: number;
  used_bytes: number;
  free_bytes: number;
  mount_point?: string;
}

export interface StorageService {
  name: string;
  running: boolean;
  enabled: boolean;
}

export interface StorageProbe {
  applicable: boolean;
  devices: StorageDevice[];
  services: StorageService[];
}

export interface MACACLPort {
  port: string;
  mode: string;
  macs: string[];
}

export interface MACACLProbe {
  applicable: boolean;
  ports: MACACLPort[];
}

export interface NetPulseAgentStatus {
  running: boolean;
  pushOk: boolean;
  lastPush: string | null;
  lastError: string;
}

export interface NetPulseDiscovery {
  foundServer: string | null;
  lastDiscoveryAt: string | null;
  lastEnrollNote: string;
}

export interface NetPulseState {
  enabled: boolean;
  configured: boolean;
  server: string;
  slug: string;
  phase: string; // "connected" | "searching" (always-on, #146)
  discovery?: NetPulseDiscovery;
  status: NetPulseAgentStatus;
  standaloneReplacedAt: string | null;
}

export interface NetPulseSet {
  server: string;
  slug: string;
  token: string;
  enabled: boolean;
  serverFp?: string;
  interval?: string;
  wanTarget?: string;
  gwTarget?: string;
}

export interface SelfUpdateConfig {
  enabled: boolean;
  intervalHours: number;
  windowStart: number;
  windowEnd: number;
}

export interface SelfUpdateSchedule {
  config: SelfUpdateConfig;
  state: {
    lastCheck: number;
    lastResult: string;
    lastApply: number;
    lastVersion: string;
  };
  status: SelfUpdateStatus;
}

export interface LanService {
  id: string;
  name: string;
  kind: string; // a catalog preset (e.g. "jellyfin") or "custom"
  host?: string;
  port?: number;
  scheme?: string;
  path?: string;
  url?: string;
  alias?: string;
  enabled: boolean;
}

export interface LanServiceStatus extends LanService {
  ok: boolean;
  latency_ms?: number;
  http_status?: number;
  resolved_ip?: string;
  error?: string;
}

export interface LanServiceCatalogEntry {
  kind: string;
  port: number;
  scheme: string;
  path: string;
}

export interface LanHost {
  name: string;
  ip: string;
}

export interface LanServicesProbe {
  services: LanServiceStatus[];
  catalog: LanServiceCatalogEntry[];
  hosts: LanHost[];
  ts: number;
}

export interface LanDiscoverySuggestion {
  host: string;
  ip: string;
  kind: string;
  port: number;
  scheme: string;
  path?: string;
}

export interface LanDiscoveryResult {
  suggestions: LanDiscoverySuggestion[];
  scanned_hosts: number;
  skipped_hosts?: number;
  ts: number;
}
