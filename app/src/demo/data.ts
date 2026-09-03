import type * as T from "../types";

/**
 * Escenario demo §9.2 — familia García. Un único mundo consistente que se
 * comparte entre TODAS las páginas. Estado mutable: los writes del modo
 * demo actualizan estos objetos para que la UI reaccione de forma coherente.
 */

const MIB = 1048576;
const now = () => Math.floor(Date.now() / 1000);

export const demoBoard: T.Board = {
  model: "Redmi AX6 (IPQ807x)",
  hostname: "casa",
  kernel: "6.6.89",
  release: { distribution: "OpenWrt", version: "25.12.5", revision: "r28417" },
};

export const demoSystem: T.SystemInfo = {
  uptime: 12 * 86400 + 4 * 3600,
  load: [0.42, 0.38, 0.31],
  // RAM 148/395 MB (37 % usada)
  memory: {
    total: 395 * MIB,
    free: 96 * MIB,
    available: 247 * MIB,
    cached: 88 * MIB,
  },
  // root en kB: 96/128 MB (75 % libre)
  root: { total: 128 * 1024, free: 96 * 1024 },
};

export const demoWan: T.WanStatus = {
  present: true,
  up: true,
  uptime: 3 * 86400 + 1 * 3600,
  ipv4: ["82.158.44.21"],
  gateway: "82.158.44.1",
  dns: ["1.1.1.1", "9.9.9.9"],
};

export const demoWireless: T.WirelessRadio[] = [
  {
    name: "radio0", up: true, band: "2g", channel: "6", htmode: "HE40", txpower: 20,
    interfaces: [
      { ifname: "phy0-ap0", ssid: "CasaGarcia", encryption: "sae-mixed", disabled: false, hidden: false, bssid: "8C:53:C3:11:22:30", rname: "radio0", clients: [{ mac: "3A:7B:1C:44:20:11", signal: -58 }] },
      { ifname: "phy0-ap1", ssid: "CasaGarcia-Invitados", encryption: "sae-mixed", disabled: false, hidden: false, bssid: "8C:53:C3:11:22:31", rname: "radio0", clients: [{ mac: "FA:22:9C:71:08:55", signal: -66 }] },
      { ifname: "phy0-ap2", ssid: "CasaGarcia-IoT", encryption: "psk2", disabled: false, hidden: false, bssid: "8C:53:C3:11:22:32", rname: "radio0", clients: [{ mac: "B4:E6:2D:90:14:07", signal: -61 }, { mac: "C8:2B:96:33:77:19", signal: -72 }, { mac: "DC:A6:32:08:51:90", signal: -69 }] },
    ],
  },
  {
    name: "radio1", up: true, band: "5g", channel: "44", htmode: "HE80", txpower: 23,
    interfaces: [
      { ifname: "phy1-ap0", ssid: "CasaGarcia", encryption: "sae-mixed", disabled: false, hidden: false, bssid: "8C:53:C3:11:22:40", rname: "radio1", clients: [{ mac: "9C:B6:D0:12:AB:77", signal: -52 }, { mac: "F0:18:98:5A:31:C2", signal: -47 }] },
    ],
  },
];

export const demoClients: T.Client[] = [
  { name: "Pixel 8 Pro (Marta)", ip: "192.168.8.112", mac: "9C:B6:D0:12:AB:77", type: "wifi5", device_type: "phone", iface: "phy1-ap0", signal: -52, rx_bytes: 4210 * MIB, tx_bytes: 380 * MIB, self: false, reserved: false, reservable: true, blocked: false, blockable: true },
  { name: "MacBook Air (Nacho)", ip: "192.168.8.120", mac: "F0:18:98:5A:31:C2", type: "wifi5", device_type: "laptop", iface: "phy1-ap0", signal: -47, rx_bytes: 2870 * MIB, tx_bytes: 640 * MIB, self: false, reserved: false, reservable: true, blocked: false, blockable: true, lease_expiry: now() + 2 * 3600, lease_source: "gateway" },
  { name: "TV Samsung Salón", ip: "192.168.8.131", ip_source: "arp", mac: "70:2A:D5:44:09:E1", type: "cable", device_type: "tv", rx_bytes: 9130 * MIB, tx_bytes: 410 * MIB, self: false, reserved: false, reservable: true, blocked: false, blockable: true },
  { name: "NAS", ip: "192.168.8.10", mac: "00:11:32:9A:BC:10", type: "cable", device_type: "nas", rx_bytes: 1820 * MIB, tx_bytes: 5240 * MIB, self: false, reserved: true, reservable: true, blocked: false, blockable: true },
  { name: "Echo Dot Cocina", ip: "192.168.8.172", mac: "B4:E6:2D:90:14:07", type: "wifi24", device_type: "speaker", iface: "phy0-ap2", signal: -61, rx_bytes: 96 * MIB, tx_bytes: 34 * MIB, self: false, reserved: false, reservable: true, blocked: false, blockable: true },
  { name: "Galaxy Tab S9 (invitados)", ip: "192.168.9.108", mac: "FA:22:9C:71:08:55", type: "wifi24", device_type: "tablet", iface: "phy0-ap1", signal: -66, rx_bytes: 540 * MIB, tx_bytes: 88 * MIB, self: false, reserved: false, reservable: false, blocked: false, blockable: true },
  { name: "Impresora HP", ip: "192.168.8.180", mac: "C8:2B:96:33:77:19", type: "wifi24", device_type: "printer", iface: "phy0-ap2", signal: -72, rx_bytes: 12 * MIB, tx_bytes: 4 * MIB, self: false, reserved: false, reservable: true, blocked: false, blockable: true, lease_expiry: now() + 42 * 60, lease_source: "gateway" },
  { name: "Robot aspiradora", ip: "192.168.8.183", mac: "DC:A6:32:08:51:90", type: "wifi24", device_type: "iot", iface: "phy0-ap2", signal: -69, rx_bytes: 31 * MIB, tx_bytes: 9 * MIB, self: false, reserved: false, reservable: true, blocked: false, blockable: true },
];

export const demoLeases: T.Lease[] = demoClients.filter((c) => c.ip).map((c) => ({
  expires: "11h 32m", mac: c.mac, ip: c.ip!, hostname: c.name.split(" (")[0],
}));

export const demoIpv6: T.IPv6Probe = {
  state: "enabled",
  lan_ipv6: "fd5f:3c4a:9e00::1/60",
  odhcpd_enabled: true,
  ra_mode: "server",
  dhcpv6_mode: "server",
};

export const demoUpdate: T.UpdateCheck = {
  available: true,
  same_version: false,
  owut_present: true,
  version_from: "25.12.5",
  version_to: "25.12.6",
  out_of_date_packages: 3,
  warnings: [],
  safe_to_proceed: true,
  missing_packages: [],
  safe_with_reinstall: true,
};

export const demoWg: T.WGProbe = {
  installed: true,
  active: true,
  running: true,
  port: "51820",
  address: "10.9.0.1/24",
  public_key: "gR8sTxV3uK2mN7pQ4wY6zA1bC5dE9fH0jL3nM8oP2qU=",
  peers: [
    { section: "peer_nacho", name: "movil-nacho", public_key: "hK9tYwR4vL3nO8qS5xZ7aB2cD6eF0gI1kM4oN9pR3vA=", allowed_ips: ["10.9.0.2/32"], admin: true },
    { section: "peer_marta", name: "portatil-marta", public_key: "pL2mN5bV8cX1zA4sD7fG0hJ3kL6nM9qW2eR5tY8uI1o=", allowed_ips: ["10.9.0.3/32"], admin: false },
  ],
};

export const demoDdns: T.DDNSProbe = {
  installed: true,
  entries: [
    {
      section: "casa_duckdns_org",
      enabled: true,
      running: true,
      service_name: "duckdns.org",
      domain: "casa.duckdns.org",
      lookup_host: "casa.duckdns.org",
      username: "casa",
      registered_ip: "82.158.44.21",
      last_update: new Date(Date.now() - 42 * 60000).toISOString(),
    },
  ],
};

export const demoMdns: T.MDNSProbe = {
  installed: true,
  enabled: true,
  running: true,
  domain: "casa.local",
};

export const demoSqm: T.SQMProbe = {
  installed: true,
  has_wan: true,
  active: true,
  running: true,
  interface: "wan",
  download: "280000",
  upload: "280000",
};

export const demoOvpn: T.OVPNProbe = {
  installed: true,
  has_pki: true,
  active: false,
  running: false,
  port: "1194",
  subnet: "10.8.0.0/24",
  clients: [{ name: "portatil-nacho" }],
};

export const demoPackages: T.PkgUpgrade[] = [
  { name: "luci-app-firewall", current: "25.118.1", available: "25.201.4" },
  { name: "wpad-basic-mbedtls", current: "2025.08.26~1", available: "2025.09.12~1" },
  { name: "kmod-usb-storage", current: "6.6.89-1", available: "6.6.93-1" },
];

export const demoOptionalPackages: T.OptionalPackage[] = [
  { id: "wireguard", packages: ["wireguard-tools", "kmod-wireguard"], i18n_key: "wizard.packages.wireguard", module: "wireguard", installed: true },
  { id: "nlbwmon", packages: ["nlbwmon"], i18n_key: "wizard.packages.nlbwmon", module: "nlbwmon", installed: false },
  { id: "tailscale", packages: ["tailscale"], i18n_key: "wizard.packages.tailscale", module: "tailscale", installed: false },
  { id: "adguard", packages: ["adguardhome"], i18n_key: "wizard.packages.adguard", module: "adguard", installed: false },
];

export const demoIot: T.IoTProbe = {
  active: true,
  ssid: "CasaGarcia-IoT",
  band: "2g",
  isolated: true,
  ifaces: ["phy0-ap2"],
  clients: 3,
};

export const demoFwd: T.FwdProbe = {
  has_wan: true,
  firewall: true,
  rules: [
    { section: "fwd_https_nas", name: "HTTPS al NAS", src_dport: "443", dest_ip: "192.168.8.10", dest_port: "443", proto: "tcp" },
  ],
};

export const demoTailscale: T.TSProbe = {
  installed: true,
  running: false,
  state: "stopped",
  ips: [],
};

export const demoGuest: T.GuestProbe = {
  gateway: true,
  active: true,
  ssid: "CasaGarcia-Invitados",
  subnet: "192.168.9.0/24",
  ifaces: ["phy0-ap1"],
  clients: 1,
  gl_conflict: false,
};

export const demoEthPorts: T.EthPort[] = [
  { name: "wan", wan: true, up: true, speed_mbps: 1000, devices: [] },
  { name: "lan1", wan: false, up: true, speed_mbps: 1000, devices: [{ mac: "00:11:32:9A:BC:10", name: "NAS" }] },
  { name: "lan2", wan: false, up: true, speed_mbps: 1000, devices: [{ mac: "70:2A:D5:44:09:E1", name: "TV Samsung Salón" }] },
  { name: "lan3", wan: false, up: false, speed_mbps: 0, devices: [] },
  { name: "lan4", wan: false, up: true, speed_mbps: 100, devices: [{ mac: "C8:2B:96:33:77:19", name: "Impresora HP" }] },
];

export const demoMode: T.ModeProbe = {
  mode: "router",
  hardware_class: "router",
  wan_in_bridge: false,
  wan_configured: true,
  dnsmasq_on: true,
  firewall_on: true,
  has_wifi: true,
  port_count: 5,
};

export const demoAccess: T.AccessProbe = {
  panel: { http_port: 8080, https_enabled: true, force_https: false, session_ttl: "60m" },
  luci: { http_port: 80, https_port: 443, force_https: false, enabled: true },
  ssh: { enabled: true, port: "22" },
};

export const demoRemoteAccess: T.RemoteAccess = {
  applicable: true,
  ping_wan: false,
  remote_https: false,
  remote_ssh: false,
};

export const demoOffload: T.OffloadProbe = {
  applicable: true,
  software: true,
  hardware: false,
  hardware_known: true,
  hardware_active: false,
};

export const demoWifi: T.WifiUI[] = [
  { section: "wifinet0", radio: "radio0", ifname: "phy0-ap0", band: "2g", ssid: "CasaGarcia", encryption: "sae-mixed", has_key: true, hidden: false, mac: "", bssid: "8C:53:C3:11:22:30", disabled: false, clients: [{ mac: "3A:7B:1C:44:20:11", signal: -58 }] },
  { section: "wifinet1", radio: "radio1", ifname: "phy1-ap0", band: "5g", ssid: "CasaGarcia", encryption: "sae-mixed", has_key: true, hidden: false, mac: "", bssid: "8C:53:C3:11:22:40", disabled: false, clients: [{ mac: "9C:B6:D0:12:AB:77", signal: -52 }, { mac: "F0:18:98:5A:31:C2", signal: -47 }] },
  { section: "wifinet2", radio: "radio0", ifname: "phy0-ap1", band: "2g", ssid: "CasaGarcia-Invitados", encryption: "sae-mixed", has_key: true, hidden: false, mac: "", bssid: "8C:53:C3:11:22:31", disabled: false, clients: [{ mac: "FA:22:9C:71:08:55", signal: -66 }] },
  { section: "wifinet3", radio: "radio0", ifname: "phy0-ap2", band: "2g", ssid: "CasaGarcia-IoT", encryption: "psk2", has_key: true, hidden: false, mac: "", bssid: "8C:53:C3:11:22:32", disabled: false, clients: [{ mac: "B4:E6:2D:90:14:07", signal: -61 }, { mac: "C8:2B:96:33:77:19", signal: -72 }, { mac: "DC:A6:32:08:51:90", signal: -69 }] },
];

export const demoLan: T.LANConfig = {
  applicable: true,
  ipaddr: "192.168.8.1",
  netmask: "255.255.255.0",
  ap_isolation: false,
  dhcp: { enabled: true, start: 100, limit: 150, lease_time: 43200, gateway: "192.168.8.1", dns1: "1.1.1.1", dns2: "9.9.9.9" },
  reservations: [{ mac: "00:11:32:9A:BC:10", ip: "192.168.8.10", name: "NAS" }],
};

export const demoDns: T.DNSConfig = {
  applicable: true,
  rebind_protection: true,
  override_dns: false,
  dns_vpn: false,
  adguard_active: true,
  hosts: [{ ip: "192.168.8.10", hostname: "nas" }],
};

export const demoUsteer: T.UsteerAP[] = [
  { bssid: "8C:53:C3:11:22:30", ssid: "CasaGarcia", hostname: "casa", iface: "hostapd.phy0-ap0", channel: 6, freq: 2437, util: 18, num_sta: 4, local: true, clients: [{ mac: "3A:7B:1C:44:20:11", signal: -58 }] },
  { bssid: "8C:53:C3:11:22:40", ssid: "CasaGarcia", hostname: "casa", iface: "hostapd.phy1-ap0", channel: 44, freq: 5220, util: 26, num_sta: 2, local: true, clients: [{ mac: "9C:B6:D0:12:AB:77", signal: -52 }, { mac: "F0:18:98:5A:31:C2", signal: -47 }] },
  { bssid: "A4:91:B1:66:40:10", ssid: "CasaGarcia", hostname: "ap-atico", iface: "hostapd.phy0-ap0", channel: 11, freq: 2462, util: 34, num_sta: 2, local: false, clients: [{ mac: "B4:E6:2D:90:14:07", signal: -61 }, { mac: "DC:A6:32:08:51:90", signal: -69 }] },
  { bssid: "A4:91:B1:66:40:11", ssid: "CasaGarcia", hostname: "ap-atico", iface: "hostapd.phy1-ap0", channel: 48, freq: 5240, util: 12, num_sta: 1, local: false, clients: [{ mac: "FA:22:9C:71:08:55", signal: -66 }] },
];

export const demoSnapshots: T.ConfigSnapshot[] = [
  { id: "20260825T030012", timestamp: now() - 3 * 86400, configs: 14 },
  { id: "20260818T030008", timestamp: now() - 10 * 86400, configs: 14 },
];

/** Drift feliz (captura principal): 0 cambios. */
export const demoDriftClean: T.DriftProbe = {
  has_baseline: true,
  snapshot_id: demoSnapshots[0].id,
  snapshot_ts: demoSnapshots[0].timestamp,
  changes: 0,
  configs: [],
};

/** Fixture alternativa §9.2: 2 cambios en network (estado warn). */
export const demoDriftWarn: T.DriftProbe = {
  has_baseline: true,
  snapshot_id: demoSnapshots[0].id,
  snapshot_ts: demoSnapshots[0].timestamp,
  changes: 2,
  configs: [
    {
      config: "network",
      lines: [
        { kind: "added", text: "option dns '8.8.8.8'" },
        { kind: "removed", text: "option mtu '1500'" },
      ],
    },
  ],
};

export const demoVlans: T.VLANProbe = {
  applicable: true,
  bridge: "br-lan",
  vlans: [{ vid: 1, name: "lan", device: "br-lan", ports: [
    { port: "wan", tagged: false }, { port: "lan1", tagged: false }, { port: "lan2", tagged: false },
    { port: "lan3", tagged: false }, { port: "lan4", tagged: false },
  ], default: true }],
  ports: ["wan", "lan1", "lan2", "lan3", "lan4"],
};

export const demoLag: T.LAGProbe = {
  applicable: true,
  installed: true,
  lags: [
    { name: "lag0", device: "bond-lag0", mode: "802.3ad", slaves: ["lan3", "lan4"], up: true },
  ],
  free_ports: ["wan", "lan1", "lan2"],
};

export const demoNlbwmon: T.NlbwmonProbe = {
  installed: true,
  running: true,
  generations: 30,
  commit_interval: 600,
  prealloc_days: 2,
  protocol_database: true,
};

export const demoFirewall: T.FirewallProbe = {
  applicable: true,
  zones: [
    { name: "lan", input: "ACCEPT", output: "ACCEPT", forward: "ACCEPT", network: ["lan"], masq: false },
    { name: "wan", input: "REJECT", output: "ACCEPT", forward: "REJECT", network: ["wan"], masq: true },
    { name: "guest", input: "REJECT", output: "ACCEPT", forward: "REJECT", network: ["guest"], masq: false },
  ],
  rules: [],
};

// Mismos datos que internal/modules/templates.go (ListTemplates), para que el
// demo refleje fielmente lo que devuelve el backend real — incluido el flujo de
// confirmación de la plantilla destructiva.
export const demoTemplates: T.Template[] = [
  { id: "hardened", name: "Hardened router", description: "Disable IPv6, enable rebind protection, block WAN ping, disable remote SSH", destructive: false },
  { id: "iot-ready", name: "IoT ready", description: "Enable IoT WiFi (2.4 GHz isolated), block IoT from LAN", destructive: false },
  { id: "reset-defaults", name: "Reset to defaults", description: "Remove all custom firewall rules, reset WiFi to defaults, clear DHCP reservations", destructive: true },
];

export const demoSwitch: T.SwitchProbe = { applicable: false, ports: [] };
export const demoSwitchModes: T.SwitchMode[] = [];

export const demoPortStats: T.PortStatsProbe = {
  ts: Date.now(),
  ports: demoEthPorts.map((p) => ({
    name: p.name,
    rx_bytes: p.up ? 42 * 1024 * MIB : 0,
    tx_bytes: p.up ? 7 * 1024 * MIB : 0,
    rx_errors: 0, tx_errors: 0, rx_drops: 0, tx_drops: 0,
  })),
};

export const demoPoe: T.PoEProbe = { applicable: false, total_budget_w: 0, used_w: 0, ports: [] };
export const demoPoeWatchdogs: T.PoEWatchdogState[] = [];
export const demoPortTemplates: T.PortTemplate[] = [];
export const demoRoleProfiles: T.RoleProfile[] = [];

export const demoDpi: T.DPIProbe = {
  applicable: true,
  total_bytes: 1520 * MIB,
  total_flows: 842,
  protocols: [
    { name: "YouTube", bytes: 720 * MIB, flows: 96, category: "streaming" },
    { name: "Netflix", bytes: 410 * MIB, flows: 54, category: "streaming" },
    { name: "HTTP", bytes: 180 * MIB, flows: 402, category: "web" },
    { name: "WhatsApp", bytes: 42 * MIB, flows: 118, category: "chat" },
    { name: "Zoom", bytes: 96 * MIB, flows: 12, category: "voip" },
    { name: "DNS", bytes: 8 * MIB, flows: 48, category: "dns" },
    { name: "MQTT", bytes: 4 * MIB, flows: 64, category: "iot" },
  ],
};

export const demoNetifyd: T.NetifydProbe = {
  installed: true,
  enabled: true,
  running: true,
  applicable: true,
  low_end: false,
  apps: [
    { name: "YouTube", bytes: 720 * MIB, local_bytes: 120 * MIB, other_bytes: 600 * MIB, packets: 9600, flows: 96 },
    { name: "Netflix", bytes: 410 * MIB, local_bytes: 80 * MIB, other_bytes: 330 * MIB, packets: 5400, flows: 54 },
    { name: "HTTPS", bytes: 180 * MIB, local_bytes: 60 * MIB, other_bytes: 120 * MIB, packets: 4020, flows: 402 },
    { name: "WhatsApp", bytes: 42 * MIB, local_bytes: 12 * MIB, other_bytes: 30 * MIB, packets: 1180, flows: 118 },
    { name: "Zoom", bytes: 96 * MIB, local_bytes: 32 * MIB, other_bytes: 64 * MIB, packets: 120, flows: 12 },
    { name: "DNS", bytes: 8 * MIB, local_bytes: 4 * MIB, other_bytes: 4 * MIB, packets: 480, flows: 48 },
    { name: "MQTT", bytes: 4 * MIB, local_bytes: 1 * MIB, other_bytes: 3 * MIB, packets: 640, flows: 64 },
  ],
};

function makeDemoTimeline(): T.NetifydTimeline {
  const apps = ["YouTube", "Netflix", "HTTPS", "WhatsApp", "Zoom", "DNS", "MQTT", "QUIC", "BitTorrent", "ICMPv6"];
  const buckets: T.NetifydTimelineBucket[] = [];
  const now = new Date();
  now.setMinutes(Math.floor(now.getMinutes() / 5) * 5, 0, 0);
  const totalTotals = { local: 0, other: 0, total: 0 };
  const appTotals: Record<string, T.NetifydBucket> = {};
  for (let i = 23; i >= 0; i--) {
    const t = new Date(now.getTime() - i * 5 * 60 * 1000);
    const bucketApps: Record<string, T.NetifydBucket> = {};
    for (const name of apps) {
      const base = (apps.indexOf(name) + 1) * 0.5 * MIB;
      const local = Math.round(base * (0.8 + Math.random() * 0.4));
      const other = Math.round(base * (0.6 + Math.random() * 0.6));
      const total = local + other;
      bucketApps[name] = { local, other, total };
      const agg = appTotals[name] ?? { local: 0, other: 0, total: 0 };
      agg.local += local;
      agg.other += other;
      agg.total += total;
      appTotals[name] = agg;
      totalTotals.local += local;
      totalTotals.other += other;
      totalTotals.total += total;
    }
    buckets.push({ time: t.toISOString(), apps: bucketApps });
  }
  const top = Object.entries(appTotals)
    .map(([name, b]) => ({ name, bytes: b.total, local_bytes: b.local, other_bytes: b.other, packets: 0, flows: 0 }))
    .sort((a, b) => b.bytes - a.bytes)
    .slice(0, 10);
  return { buckets, top, totals: totalTotals };
}

export const demoNetifydTimeline = makeDemoTimeline();

export const demoFleet: T.FleetNodeStatus[] = [
  { id: "ap-atico", name: "ap-atico", address: "192.168.8.2", reachable: true, current_version: "0.1.2", latest_version: "0.1.2", update_available: false },
  { id: "switch-garaje", name: "switch-garaje", address: "192.168.8.3", reachable: true, current_version: "0.1.1", latest_version: "0.1.2", update_available: true },
];

export const demoDiscoveredFleet: T.DiscoveredFleetPeer[] = [
  { id: "ap-jardin", name: "ap-jardin", version: "0.1.2", address: "192.168.8.4", port: 8080, seen_at: new Date().toISOString() },
];

export const demoCableTest: T.CableTestProbe = {
  applicable: true,
  ports: [
    { port: "wan", supported: true, pair_status: "ok", length: "3m" },
    { port: "lan1", supported: true, pair_status: "ok", length: "8m" },
    { port: "lan2", supported: true, pair_status: "ok", length: "5m" },
    { port: "lan3", supported: true, pair_status: "open" },
    { port: "lan4", supported: true, pair_status: "ok", length: "12m" },
  ],
};

export const demoStorm: T.StormProbe = { applicable: false, ports: [] };

export const demoStorage: T.StorageProbe = {
  applicable: true,
  devices: [{
    name: "sda1", path: "/dev/sda1", fs_type: "ext4",
    size_bytes: 120 * 1024 * MIB,
    used_bytes: Math.round(120 * 0.41 * 1024) * MIB,
    free_bytes: Math.round(120 * 0.59 * 1024) * MIB,
    mount_point: "/mnt/backups",
  }],
  services: [
    { name: "samba", running: true, enabled: true },
    { name: "nfs", running: false, enabled: false },
  ],
};

export const demoMacAcl: T.MACACLProbe = { applicable: false, ports: [] };

export const demoSelfUpdate: T.SelfUpdateCheck = {
  current: "0.1.2",
  latest: "0.1.2",
  available: false,
  notes: "",
};

export const demoWizard: T.WizardState = { completed: true, mode: "router" };

export const demoWizardSetup: T.WizardSetupProbe = {
  manager: "apk",
  groups: [
    { id: "core", title_key: "wizard.setup.required", packages: ["rpcd-mod-file"] },
    { id: "netpulse", title_key: "wizard.setup.netpulse", packages: ["tailscale"] },
    { id: "diagnostics", title_key: "wizard.setup.diagnostics", packages: ["ethtool-full", "tcpdump-mini"] },
    { id: "extras", title_key: "wizard.setup.extras", packages: [] },
  ],
};

export const demoIgmp: T.IGMPProbe = { applicable: true, enabled: true };

export const demoLoops: T.LoopResult = { loops: [], has_hub: false };

export const demoTelegram = { botToken: "", chatId: "", enabled: false };

/**
 * Histórico 24 h (muestras cada 5 min, contadores acumulados): día tranquilo
 * con pico de streaming a las 21:00.
 */
export function buildDemoHistory(): T.HistoryEntry[] {
  const entries: T.HistoryEntry[] = [];
  const n = 288; // 24h * 12 muestras/h
  const start = now() - 24 * 3600;
  let rx = 38 * 1024 * MIB; // contadores acumulados (~38 GB)
  let tx = 5 * 1024 * MIB;
  for (let i = 0; i < n; i++) {
    const ts = start + i * 300;
    const hour = new Date(ts * 1000).getHours();
    // ritmo base por franja + pico de streaming 21:00–23:00
    let rate = 0.4 * MIB; // B/s aprox de fondo
    if (hour >= 8 && hour <= 22) rate = 1.6 * MIB;
    if (hour >= 21 && hour <= 23) rate = 12 * MIB; // streaming
    const jitter = 0.7 + 0.6 * Math.abs(Math.sin(i * 0.9) + Math.sin(i * 0.37));
    rx += rate * jitter * 300;
    tx += rate * jitter * 300 * 0.14;
    entries.push({
      ts,
      rx: Math.round(rx),
      tx: Math.round(tx),
      load: Math.round((0.3 + 0.2 * Math.abs(Math.sin(i * 0.5))) * 100) / 100,
      clients: 6 + Math.round(2 * Math.abs(Math.sin(i * 0.21))),
    });
  }
  return entries;
}
