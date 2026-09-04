import type * as T from "../types";
import type { api } from "../api";
import * as D from "./data";

/**
 * Modo demo §9: objeto con la MISMA interfaz que `api`. Los GET resuelven
 * con latencia simulada (150–400 ms); los writes devuelven ModuleResult tras
 * 800–1500 ms. Las series de tráfico evolucionan entre llamadas.
 */

const wait = (min: number, max: number) =>
  new Promise<void>((r) => setTimeout(r, min + Math.random() * (max - min)));

const get = async <V>(v: V): Promise<V> => { await wait(150, 400); return v; };
const write = async <V>(state: V): Promise<T.ModuleResult<V>> => {
  await wait(800, 1500);
  return { status: "applied", rolled_back: false, state };
};

// ── Estado mutable del escenario ──────────────────────────────────────────
const state = {
  clients: structuredClone(D.demoClients),
  wireless: D.demoWireless,
  wifi: D.demoWifi,
  wg: structuredClone(D.demoWg),
  ddns: { ...D.demoDdns },
  mdns: { ...D.demoMdns },
  sqm: { ...D.demoSqm },
  ovpn: structuredClone(D.demoOvpn),
  ipv6: { ...D.demoIpv6 },
  guest: { ...D.demoGuest },
  iot: { ...D.demoIot },
  fwd: structuredClone(D.demoFwd),
  ts: { ...D.demoTailscale },
  lan: structuredClone(D.demoLan),
  dns: structuredClone(D.demoDns),
  packages: [...D.demoPackages],
  optionalPkgs: structuredClone(D.demoOptionalPackages),
  drift: structuredClone(D.demoDriftClean),
  snapshots: [...D.demoSnapshots],
  vlans: structuredClone(D.demoVlans),
  lag: structuredClone(D.demoLag),
  firewall: structuredClone(D.demoFirewall),
  nlbwmon: { ...D.demoNlbwmon },
  offload: { ...D.demoOffload },
  mode: { ...D.demoMode },
  igmp: { ...D.demoIgmp },
  access: structuredClone(D.demoAccess),
  remote: { ...D.demoRemoteAccess },
  storage: structuredClone(D.demoStorage),
  fleet: structuredClone(D.demoFleet),
  discoveredFleet: structuredClone(D.demoDiscoveredFleet),
  telegram: { ...D.demoTelegram },
  netpulse: {
    enabled: true,
    configured: true,
    server: "https://netpulse.example.com",
    slug: "garcia-gw",
  },
  nftqos: {
    applicable: true,
    limits: {
      "00:11:22:33:44:55": { mac: "00:11:22:33:44:55", ip: "192.168.1.100", download: 20, upload: 5 },
    } as Record<string, T.NftQoSLimit>,
  },
  portTemplates: [...D.demoPortTemplates],
  hasCert: true,
  history: D.buildDemoHistory(),
  netifyd: { ...D.demoNetifyd },
};

// ── Contadores de tráfico que evolucionan con ruido suave acotado ─────────
const rates: Record<string, { rx: number; tx: number }> = {
  "br-lan": { rx: 5.2e6, tx: 1.1e6 },
  eth0: { rx: 5.4e6, tx: 1.15e6 },
  wlan0: { rx: 2.1e6, tx: 0.4e6 },
  wlan1: { rx: 2.8e6, tx: 0.6e6 },
};
const counters: Record<string, { rx_bytes: number; tx_bytes: number }> = Object.fromEntries(
  Object.keys(rates).map((k) => [k, { rx_bytes: 38e9 * Math.random() + 5e9, tx_bytes: 6e9 }]),
);
let countersTs = Date.now();

function nextNetdev() {
  const ts = Date.now();
  const dt = Math.max(0.5, (ts - countersTs) / 1000);
  countersTs = ts;
  for (const [name, base] of Object.entries(rates)) {
    // ruido suave acotado ±35 % con deriva lenta
    const noise = 1 + 0.35 * Math.sin(ts / 7000 + name.length) * Math.random();
    counters[name].rx_bytes += base.rx * noise * dt;
    counters[name].tx_bytes += base.tx * (2 - noise) * dt;
  }
  return {
    ts,
    counters: Object.entries(counters).map(([name, c]) => ({
      name, rx_bytes: Math.round(c.rx_bytes), tx_bytes: Math.round(c.tx_bytes),
    })),
  };
}

let clientsTs = Date.now();
const DEMO_BANDS = ["2g", "5g"];
function nextClients() {
  const ts = Date.now();
  const dt = Math.max(0.5, (ts - clientsTs) / 1000);
  clientsTs = ts;
  for (const c of state.clients) {
    const factor = c.type === "cable" ? 2.2 : c.type === "wifi5" ? 1.4 : 0.15;
    const noise = 1 + 0.5 * Math.sin(ts / 9000 + c.mac.length) * Math.random();
    c.rx_bytes += Math.round(6e5 * factor * noise * dt);
    c.tx_bytes += Math.round(1.2e5 * factor * (2 - noise) * dt);
  }
  return { ts, clients: state.clients, bands: DEMO_BANDS };
}

export const demoApi: typeof api = {
  // sesión
  login: async () => { await wait(400, 800); },
  logout: async () => { await wait(100, 200); },
  me: () => get(undefined as void),
  wizardState: () => get(D.demoWizard),
  wizardSetup: () => get(D.demoWizardSetup),
  installWizardSetup: async () => { await wait(800, 1500); return { job: { phase: "done", total: 1, done: 1, installed: ["ethtool-full"] } }; },
  wizardComplete: async () => { await wait(400, 800); return { status: "ok" }; },

  // núcleo
  board: () => get(D.demoBoard),
  system: () => get(D.demoSystem),
  wan: () => get(D.demoWan),
  wireless: () => get(state.wireless),
  leases: () => get(D.demoLeases),
  clients: async () => { await wait(150, 400); return nextClients(); },
  blockedClients: async () => {
    await wait(80, 200);
    const blocked = state.clients
      .filter((c) => c.blocked || (c.blocked_on?.length ?? 0) > 0)
      .map((c): import("../types").BlockedClient => ({
        mac: c.mac,
        type: c.type === "cable" ? "cable" : "wifi",
        bands: c.blocked_on,
        blocked_everywhere: !!c.blocked,
      }));
    return { blocked, ts: Date.now() };
  },
  reserveClient: async (mac, _ip, reserved) => {
    await wait(800, 1200);
    const c = state.clients.find((x) => x.mac === mac);
    if (c) c.reserved = reserved;
    return { status: "ok" };
  },
  blockClient: async (mac, _type, blocked, band) => {
    await wait(800, 1200);
    const c = state.clients.find((x) => x.mac === mac);
    if (!c) return { status: "ok" };
    if (band) {
      const on = new Set(c.blocked_on ?? []);
      if (blocked) on.add(band); else on.delete(band);
      const list = [...on].sort();
      c.blocked_on = list.length ? list : undefined;
      c.blocked = DEMO_BANDS.every((b) => on.has(b));
    } else {
      c.blocked = blocked;
      c.blocked_on = blocked ? [...DEMO_BANDS] : undefined;
    }
    return { status: "ok" };
  },
  clientMeta: async () => {
    await wait(100, 250);
    const meta: Record<string, { name: string; device_type: string }> = {};
    for (const c of state.clients) if (c.device_type) meta[c.mac] = { name: "", device_type: c.device_type };
    return { meta };
  },
  setClientMeta: async (mac, name, device_type) => {
    await wait(500, 900);
    const c = state.clients.find((x) => x.mac === mac);
    if (c) { if (name) c.name = name; c.device_type = device_type; }
    return { meta: { [mac]: { name, device_type } } };
  },
  wakeOnLan: async () => { await wait(400, 900); return { status: "ok" }; },
  netdev: async () => { await wait(150, 400); return nextNetdev(); },
  history: () => get({ entries: state.history }),
  mode: () => get(state.mode),
  setMode: async (target) => { state.mode.mode = target; return write({ ...state.mode }); },

  // servicios
  ipv6: () => get(state.ipv6),
  setIpv6: async (enabled) => {
    await wait(800, 1500);
    state.ipv6.state = enabled ? "enabled" : "disabled";
    return { status: "applied" as const, rolled_back: false, state: { ...state.ipv6 } };
  },
  wireguard: () => get(state.wg),
  setWireguard: async (action) => {
    state.wg.active = state.wg.running = action === "enable";
    return write(state.wg);
  },
  addWgPeer: async (name, publicKey, admin) => {
    state.wg.peers.push({ section: `peer_${state.wg.peers.length}`, name, public_key: publicKey, allowed_ips: [], admin });
    return write(state.wg);
  },
  addWgPeerQr: async (name, admin) => {
    await wait(800, 1500);
    state.wg.peers.push({ section: `peer_${state.wg.peers.length}`, name, public_key: "DEMO" + "x".repeat(39) + "=", allowed_ips: [], admin });
    return { config: "[Interface]\nPrivateKey = <demo>\nAddress = 10.9.0.9/32\n\n[Peer]\nPublicKey = " + state.wg.public_key + "\nAllowedIPs = 0.0.0.0/0\nEndpoint = casa.duckdns.org:51820\n", state: state.wg };
  },
  deleteWgPeer: async (publicKey) => {
    state.wg.peers = state.wg.peers.filter((p) => p.public_key !== publicKey);
    return write(state.wg);
  },
  ddns: () => get(state.ddns),
  setDdns: async (cfg) => {
    await wait(800, 1500);
    const existing = state.ddns.entries.find((e) => e.domain === cfg.domain);
    if (existing) {
      existing.enabled = cfg.enabled;
      if (cfg.service_name) existing.service_name = cfg.service_name;
      if (cfg.domain) existing.domain = cfg.domain;
      if (cfg.lookup_host !== undefined) existing.lookup_host = cfg.lookup_host;
      if (cfg.username !== undefined) existing.username = cfg.username;
      existing.running = cfg.enabled;
    } else if (cfg.enabled && cfg.domain) {
      state.ddns.entries.push({
        section: cfg.domain.replace(/[^a-zA-Z0-9]+/g, "_").replace(/^_+|_+$/g, "").replace(/^[0-9]/, "d$&") || "d",
        enabled: true,
        running: true,
        service_name: cfg.service_name || "duckdns.org",
        domain: cfg.domain,
        lookup_host: cfg.lookup_host || "",
        username: cfg.username || "",
        registered_ip: D.demoWan.ipv4[0] || "",
        last_update: new Date().toISOString(),
      });
    }
    return { status: "applied" as const, rolled_back: false, state: state.ddns };
  },
  deleteDdns: async (section) => {
    await wait(800, 1500);
    state.ddns.entries = state.ddns.entries.filter((e) => e.section !== section);
    return { status: "applied" as const, rolled_back: false, state: state.ddns };
  },
  mdns: () => get(state.mdns),
  setMdns: async (enabled) => {
    state.mdns.enabled = state.mdns.running = enabled;
    return write(state.mdns);
  },
  sqm: () => get(state.sqm),
  setSqm: async (cfg) => {
    await wait(800, 1500);
    // §9.1: 10 % de rollbacks simulados en SQM para fotografiar el ActionBanner
    if (Math.random() < 0.1) {
      return { status: "rolled_back" as const, rolled_back: true, state: state.sqm, error: "rtnetlink: file exists (cake no pudo recrearse en wan)" };
    }
    state.sqm.active = state.sqm.running = cfg.enabled;
    if (cfg.download) state.sqm.download = cfg.download;
    if (cfg.upload) state.sqm.upload = cfg.upload;
    return { status: "applied" as const, rolled_back: false, state: state.sqm };
  },
  openvpn: () => get(state.ovpn),
  setOpenvpn: async (action) => {
    state.ovpn.active = state.ovpn.running = action === "enable";
    return write(state.ovpn);
  },
  addOvpnClient: async (name) => {
    await wait(800, 1500);
    state.ovpn.clients.push({ name });
    return { config: "# demo .ovpn para " + name + "\nclient\ndev tun\nproto udp\nremote casa.duckdns.org 1194\n", state: state.ovpn };
  },
  deleteOvpnClient: async (name) => {
    await wait(800, 1500);
    state.ovpn.clients = state.ovpn.clients.filter((c) => c.name !== name);
    return { state: state.ovpn };
  },
  setOvpnPublicHost: async (host) => {
    await wait(400, 900);
    state.ovpn.public_host = host.trim();
    return { ...state.ovpn };
  },
  tailscale: () => get(state.ts),
  setTailscale: async (enabled) => {
    state.ts.running = enabled;
    state.ts.state = enabled ? "connected" : "stopped";
    return write(state.ts);
  },
  guestwifi: () => get(state.guest),
  setGuestwifi: async (cfg) => {
    state.guest.active = cfg.enabled;
    if (cfg.ssid) state.guest.ssid = cfg.ssid;
    return write(state.guest);
  },
  iotwifi: () => get(state.iot),
  setIotwifi: async (cfg) => {
    state.iot.active = cfg.enabled;
    if (cfg.ssid) state.iot.ssid = cfg.ssid;
    return write(state.iot);
  },
  nlbwmon: () => get(state.nlbwmon),
  setNlbwmon: async (cfg) => write({ ...state.nlbwmon, ...cfg, running: cfg.enabled ?? state.nlbwmon.running }),
  firewall: () => get(state.firewall),
  addFirewallRule: async (rule) => {
    state.firewall.rules.push({ section: `rule_${state.firewall.rules.length}`, ...rule });
    return write(state.firewall);
  },
  deleteFirewallRule: async (section) => {
    state.firewall.rules = state.firewall.rules.filter((r) => r.section !== section);
    return write(state.firewall);
  },
  dpi: () => get(D.demoDpi),
  netifyd: () => get(state.netifyd),
  setNetifyd: async (enabled) => {
    state.netifyd.enabled = enabled;
    state.netifyd.running = enabled;
    return write(state.netifyd);
  },
  dpiApps: () => get({ apps: state.netifyd.apps }),
  dpiTimeline: () => get(D.demoNetifydTimeline),

  // red local
  lan: () => get(state.lan),
  setLan: async (opts) => { Object.assign(state.lan, opts); return write(state.lan); },
  setDhcp: async (cfg) => { state.lan.dhcp = { ...cfg }; return write(state.lan); },
  setReservation: async (mac, ip, name, reserved) => {
    state.lan.reservations = reserved
      ? [...state.lan.reservations.filter((r) => r.mac !== mac), { mac, ip, name }]
      : state.lan.reservations.filter((r) => r.mac !== mac);
    const c = state.clients.find((x) => x.mac === mac);
    if (c) c.reserved = reserved;
    return write(state.lan);
  },
  clearReservations: async () => {
    state.lan.reservations = [];
    state.clients.forEach((c) => { c.reserved = false; });
    return write(state.lan);
  },
  dns: () => get(state.dns),
  setDns: async (opts) => { Object.assign(state.dns, opts); return write(state.dns); },
  setDnsHost: async (ip, hostname, remove) => {
    state.dns.hosts = remove
      ? state.dns.hosts.filter((h) => h.ip !== ip || h.hostname !== hostname)
      : [...state.dns.hosts, { ip, hostname }];
    return write(state.dns);
  },
  vlans: () => get(state.vlans),
  setVlan: async () => write(state.vlans),
  deleteVlan: async () => write(state.vlans),
  lag: () => get(state.lag),
  setLag: async (cfg) => {
    await wait(800, 1500);
    const existing = state.lag.lags.find((l) => l.name === cfg.name);
    if (existing) {
      existing.mode = cfg.mode;
      state.lag.free_ports = [...existing.slaves, ...state.lag.free_ports].filter(
        (p) => !cfg.slaves.includes(p),
      ).sort();
      existing.slaves = [...cfg.slaves];
      existing.up = true;
    } else {
      state.lag.lags.push({ name: cfg.name, device: "bond-" + cfg.name, mode: cfg.mode, slaves: [...cfg.slaves], up: true });
      state.lag.free_ports = state.lag.free_ports.filter((p) => !cfg.slaves.includes(p));
    }
    return { status: "applied" as const, rolled_back: false, state: state.lag };
  },
  deleteLag: async (name) => {
    await wait(800, 1500);
    const lag = state.lag.lags.find((l) => l.name === name);
    state.lag.lags = state.lag.lags.filter((l) => l.name !== name);
    if (lag) state.lag.free_ports = [...state.lag.free_ports, ...lag.slaves].sort();
    return { status: "applied" as const, rolled_back: false, state: state.lag };
  },

  // puertos
  ethports: () => get({ ports: D.demoEthPorts }),
  portforward: () => get(state.fwd),
  addFwdRule: async (src_dport, dest_ip, dest_port, proto) => {
    state.fwd.rules.push({ section: `fwd_${state.fwd.rules.length}`, name: "", src_dport, dest_ip, dest_port, proto });
    return write(state.fwd);
  },
  deleteFwdRule: async (section) => {
    state.fwd.rules = state.fwd.rules.filter((r) => r.section !== section);
    return write(state.fwd);
  },
  bouncePort: async (iface) => { await wait(800, 1500); return { iface, ok: true }; },
  blockPort: async (iface, _blocked) => { await wait(800, 1500); return { iface, ok: true }; },
  switchPorts: () => get(D.demoSwitch),
  setSwitchPort: async () => write(D.demoSwitch),
  portStats: () => get(D.demoPortStats),
  switchModes: () => get({ modes: D.demoSwitchModes }),
  applySwitchMode: async () => { await wait(800, 1500); return { status: "ok" }; },
  poe: () => get(D.demoPoe),
  setPoESchedule: async () => { await wait(800, 1500); return { status: "ok", state: D.demoPoe }; },
  poeWatchdogs: () => get({ watchdogs: D.demoPoeWatchdogs }),
  setPoEWatchdog: async (cfg) => {
    await wait(800, 1500);
    const rest = D.demoPoeWatchdogs.filter((w) => w.config.port !== cfg.port);
    if (cfg.enabled) {
      rest.push({ config: { ...cfg }, failures: 0, last_check: "", last_cycle: "", cooling: false });
    }
    return { status: "applied" as const, watchdogs: rest };
  },
  portTemplates: () => get({ templates: state.portTemplates }),
  savePortTemplate: async (tpl) => {
    await wait(800, 1500);
    state.portTemplates = [...state.portTemplates.filter((p) => p.name !== tpl.name), { ...tpl }];
    return { status: "ok" };
  },
  deletePortTemplate: async (name) => {
    await wait(800, 1500);
    state.portTemplates = state.portTemplates.filter((p) => p.name !== name);
    return { status: "ok" };
  },
  applyPortTemplate: async () => { await wait(800, 1500); return { status: "ok" }; },
  roleProfiles: () => get({ roles: D.demoRoleProfiles }),
  applyRoleProfile: async () => { await wait(800, 1500); return { status: "ok" }; },
  usteer: () => get({ aps: D.demoUsteer }),
  offload: () => get(state.offload),
  setOffload: async (enabled) => { state.offload.software = enabled; return write(state.offload); },
  igmp: () => get(state.igmp),
  setIgmp: async (enabled) => { await wait(800, 1500); state.igmp.enabled = enabled; return { ...state.igmp }; },
  loops: () => get(D.demoLoops),
  cableTest: () => get(D.demoCableTest),
  stormControl: () => get(D.demoStorm),
  setStormControl: async () => { await wait(800, 1500); return { status: "ok" }; },
  macAcl: () => get(D.demoMacAcl),
  runBufferbloatTest: async () => {
    await wait(2500, 4000);
    return { timestamp: new Date().toISOString(), baseline_ms: 8, loaded_ms: 14, delta_ms: 6, grade: "A", samples_loaded: [12, 14, 11, 13] };
  },
  bufferbloatHistory: () => get({ entries: [
    { ts: Date.now() / 1000 - 86400, baseline_ms: 8, loaded_ms: 90, delta_ms: 82, grade: "C", timestamp: new Date(Date.now() * 1000 - 86400 * 1000).toISOString(), samples_loaded: [12, 14, 11, 13] },
    { ts: Date.now() / 1000 - 43200, baseline_ms: 8, loaded_ms: 30, delta_ms: 22, grade: "B", timestamp: new Date(Date.now() * 1000 - 43200 * 1000).toISOString(), samples_loaded: [12, 14, 11, 13] },
    { ts: Date.now() / 1000 - 600, baseline_ms: 8, loaded_ms: 14, delta_ms: 6, grade: "A", timestamp: new Date(Date.now() * 1000 - 600 * 1000).toISOString(), samples_loaded: [12, 14, 11, 13] },
  ] }),
  pushConfigGet: () => get({ server_url: "https://netpulse.example.com", router_id: "rt1", token: "" }),
  pushConfigSet: async () => { await wait(600, 1200); return { status: "saved" }; },
  pushSnapshot: async () => { await wait(1200, 2000); return { ok: true, snapshot_id: "20260829-210000" }; },
  setMacAcl: async () => { await wait(800, 1500); return { status: "ok" }; },
  templates: () => get({ templates: D.demoTemplates }),
  applyTemplate: async () => { await wait(800, 1500); return { status: "ok" }; },

  // sistema / herramientas
  updateCheck: () => get(D.demoUpdate),
  startUpdate: async () => { await wait(800, 1500); return { started: true, reboot_pending: true }; },
  packages: () => get({ upgradable: state.packages }),
  optionalPackages: () => get({ packages: state.optionalPkgs }),
  wizardPackages: async (ids) => {
    await wait(800, 1500);
    for (const p of state.optionalPkgs) if (ids.includes(p.id)) p.installed = true;
    return { job: { phase: "done", total: ids.length, done: ids.length, installed: ids } };
  },
  installJob: async () => ({ job: { phase: "done", total: 0, done: 0, installed: [] } }),
  removePackages: async (ids) => {
    await wait(800, 1500);
    const removed: string[] = [];
    for (const p of state.optionalPkgs) {
      if (ids.includes(p.id) && p.installed) {
        p.installed = false;
        removed.push(p.id);
      }
    }
    return { status: "ok", removed, packages: state.optionalPkgs };
  },
  upgradePackage: async (name) => {
    await wait(800, 1500);
    state.packages = state.packages.filter((p) => p.name !== name);
    return { upgradable: state.packages };
  },
  selfUpdateCheck: () => get(D.demoSelfUpdate),
  selfUpdateApply: async () => { await wait(800, 1500); return { status: "ok", restarting: false } },
  selfUpdateStatus: () => get<T.SelfUpdateStatus>({ phase: "idle", progress: 0 }),
  snapshots: () => get({ snapshots: state.snapshots }),
  createSnapshot: async () => {
    await wait(800, 1500);
    const snap: T.ConfigSnapshot = { id: `demo-${Date.now()}`, timestamp: Math.floor(Date.now() / 1000), configs: 14 };
    state.snapshots = [snap, ...state.snapshots];
    state.drift.has_baseline = true;
    state.drift.snapshot_id = snap.id;
    state.drift.snapshot_ts = snap.timestamp;
    state.drift.changes = 0;
    state.drift.configs = [];
    return snap;
  },
  deleteSnapshot: async (id) => {
    await wait(400, 800);
    state.snapshots = state.snapshots.filter((s) => s.id !== id);
  },
  snapshotDiff: () => get({ diffs: [] }),
  rollbackSnapshot: async () => { await wait(800, 1500); return { status: "ok" }; },
  drift: () => get(state.drift),
  access: () => get(state.access),
  setLuciAccess: async (luci) => { state.access.luci = { ...luci }; return write(state.access); },
  setSshAccess: async (ssh) => { state.access.ssh = { ...ssh }; return write(state.access); },
  setPanelSessionTtl: async (minutes) => {
    await wait(400, 800);
    state.access.panel.session_ttl = `${minutes}m`;
    return { status: "ok", panel: state.access.panel };
  },
  remoteAccess: () => get(state.remote),
  setRemoteAccess: async (opts) => { Object.assign(state.remote, opts); return write(state.remote); },
  httpsState: () => get({ has_cert: state.hasCert }),
  enableHttps: async () => { await wait(800, 1500); state.hasCert = true; return { status: "ok" }; },
  setPassword: async () => { await wait(800, 1500); },
  telegramGet: () => get(state.telegram),
  telegramSet: async (botToken, chatId, enabled) => {
    await wait(800, 1500);
    state.telegram = { botToken, chatId, enabled };
    return { ok: true, botName: "NetGripCasaBot", chatName: "Familia García" };
  },
  telegramTest: async () => { await wait(400, 900); return { ok: true } },

  // netpulse (agente embebido): estado sano y push reciente en la demo
  netpulse: () => get({
    ...state.netpulse,
    phase: "connected",
    discovery: { foundServer: "https://netpulse.example.com", lastDiscoveryAt: new Date(Date.now() - 60_000).toISOString(), lastEnrollNote: "" },
    status: { running: true, pushOk: true, lastPush: new Date(Date.now() - 15_000).toISOString(), lastError: "" },
    standaloneReplacedAt: null,
  }),
  setNetPulse: async (cfg) => {
    await wait(800, 1500);
    state.netpulse = {
      enabled: cfg.enabled,
      configured: cfg.enabled ? true : state.netpulse.configured,
      server: cfg.server || state.netpulse.server,
      slug: cfg.slug || state.netpulse.slug,
    };    return {
      ...state.netpulse,
      phase: cfg.enabled ? "connected" : "searching",
      discovery: { foundServer: cfg.server || state.netpulse.server, lastDiscoveryAt: new Date().toISOString(), lastEnrollNote: "" },
      status: { running: cfg.enabled, pushOk: cfg.enabled, lastPush: cfg.enabled ? new Date().toISOString() : null, lastError: "" },
      standaloneReplacedAt: null,
    };
  },
  restartAgent: async () => { await wait(200, 500); return { ok: true }; },
  nftqos: () => get({ ...state.nftqos }),
  setNftqos: async (limit) => {
    await wait(800, 1500);
    const key = limit.mac!;
    if ((limit.download ?? 0) <= 0 && (limit.upload ?? 0) <= 0) {
      delete state.nftqos.limits[key];
    } else {
      state.nftqos.limits[key] = { ...state.nftqos.limits[key], ...limit } as T.NftQoSLimit;
    }
    return { ...state.nftqos };
  },
  removeNftqos: async (mac) => {
    await wait(800, 1500);
    delete state.nftqos.limits[mac];
    return { ...state.nftqos };
  },

  // wifi
  wifi: () => get({ interfaces: state.wifi }),
  wifiKey: async () => { await wait(80, 200); return { key: "demo-passkey-1234" }; },
  setWifi: async (edit) => {
    const w = state.wifi.find((x) => x.section === edit.section);
    if (w) {
      if (edit.ssid !== undefined) w.ssid = edit.ssid;
      if (edit.encryption !== undefined) w.encryption = edit.encryption;
      if (edit.hidden !== undefined) w.hidden = edit.hidden;
      if (edit.disabled !== undefined) w.disabled = edit.disabled;
      if (edit.key !== undefined) w.has_key = edit.key.length > 0;
    }
    return write(w ?? state.wifi[0]);
  },

  // almacenamiento
  storage: () => get(state.storage),
  setStorageService: async (name, action) => {
    await wait(800, 1500);
    const s = state.storage.services.find((x) => x.name === name);
    if (s) { s.enabled = s.running = action === "enable"; }
    return { status: "ok" };
  },

  // flota
  fleet: () => get({ nodes: state.fleet }),
  addFleetNode: async (node) => {
    await wait(800, 1500);
    state.fleet.push({ id: node.id, name: node.name, address: node.address, reachable: true, current_version: "0.1.2", latest_version: "0.1.2", update_available: false });
    return { status: "ok" };
  },
  deleteFleetNode: async (id) => {
    await wait(400, 800);
    state.fleet = state.fleet.filter((n) => n.id !== id);
    return { status: "ok" };
  },
  checkFleetNode: async (id) => {
    await wait(400, 900);
    const n = state.fleet.find((x) => x.id === id);
    if (!n) throw new Error("nodo no encontrado");
    return n;
  },
  checkAllFleet: async () => { await wait(600, 1200); return { nodes: state.fleet }; },
  discoveredFleet: () => get({ peers: state.discoveredFleet }),
  adoptFleetPeer: async (peer) => {
    await wait(800, 1500);
    state.fleet.push({ id: peer.id, name: peer.name, address: peer.address, reachable: true, current_version: "0.1.2", latest_version: "0.1.2", update_available: false });
    state.discoveredFleet = state.discoveredFleet.filter((p) => p.id !== peer.id);
    return { status: "ok" };
  },
  updateFleetNode: async (id) => {
    await wait(800, 1500);
    const n = state.fleet.find((x) => x.id === id);
    if (n) { n.current_version = n.latest_version; n.update_available = false; }
    return { status: "ok" };
  },
  fleetDiscoveryConfig: () => get({ enabled: true }),
  setFleetDiscoveryConfig: async (enabled: boolean) => {
    await wait(200, 400);
    return { enabled };
  },
};
