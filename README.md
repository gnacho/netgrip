# NetGrip

<p align="center">
  <a href="README.md">English</a> |
  <a href="README.es.md">Español</a>
</p>

<p align="center">
  <a href="https://netgrip.cloudless.club"><img alt="Website" src="https://img.shields.io/badge/website-netgrip.cloudless.club-2E6BE6"></a>
  <a href="https://demo.netgrip.cloudless.club"><img alt="Live demo" src="https://img.shields.io/badge/demo-demo.netgrip.cloudless.club-0D9488"></a>
  <a href="https://github.com/gnacho/netgrip/releases"><img alt="Release" src="https://img.shields.io/github/v/release/gnacho/netgrip"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/github/license/gnacho/netgrip"></a>
</p>

<p align="center">
  <strong>Every service on your OpenWrt router, behind a switch your family
  can press.</strong><br>
  NetGrip is a companion panel that runs on the router itself, next to LuCI:
  WireGuard, guest Wi-Fi, QoS, DNS and more, one click each, with a snapshot,
  a health check and an automatic rollback if anything goes wrong.
</p>

<p align="center">
  <a href="https://demo.netgrip.cloudless.club"><strong>Try the live demo</strong></a> ·
  <a href="https://netgrip.cloudless.club"><strong>Visit the website</strong></a>
</p>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/hero-en-dark.png">
    <img alt="NetGrip overview: system and WAN cards, a live traffic chart and the ethernet port panel, in light theme" src="assets/hero-en-light.png" width="800">
  </picture>
</p>

## See it before you install it

- **[demo.netgrip.cloudless.club](https://demo.netgrip.cloudless.club)** runs
  the real panel on sample data: click every switch, open every card, nothing
  is applied anywhere.
- **[netgrip.cloudless.club](https://netgrip.cloudless.club)** lists
  **every single feature**, screen by screen, with the reasoning behind
  each one.

## Why NetGrip?

I kept handing OpenWrt routers to people who only want working Wi-Fi and a
VPN. LuCI is a tool for engineers: every toggle asks for a section name, an
interface and an option. Vendor portals are friendlier, but they are closed
apps that only work on that vendor's firmware. I wanted the machine in
between: the real router, behind buttons a relative can press. NetGrip takes
the services LuCI already exposes through rpcd and puts a switch in front of
each one. Flip it, and NetGrip snapshots the config, applies the change,
waits, checks the service came up, and undoes itself if it did not.

Three rules shape it:

- **It lives on the router.** One static Go binary with the UI embedded
  (about 10 MB on disk and ~15 MB of RAM in use, measured on an ARM64
  router; ~11.4 MB on mipsle), packaged as a real OpenWrt `.apk`/`.ipk`
  that survives sysupgrade. No container, no extra box, no Node on a
  router.
- **Safe by construction.** Every change goes through an allowlisted
  executor with a config snapshot and automatic rollback. The panel cannot
  wander off the beaten path.
- **No new accounts.** Login validates against rpcd, the same session LuCI
  uses; one admin per router, nothing extra to administer. AGPL-3.0, no
  premium anything.

## What you get

**An overview that answers "is everything fine?"** System health, WAN state
and a live traffic chart on one screen, plus an ethernet chassis with
per-port state, device names and unmanaged-switch detection. That is the
screenshot above.

**Clients you can actually manage.** Every station with speed and signal,
one-click reserved IPs, block actions and per-device bandwidth limits.

<p align="center">
  <img alt="Clients page: sortable table with device names, connection type, signal and usage per client" src="assets/screenshot-clients-en.png" width="800">
</p>

**One card per service, each with a real switch.** WireGuard (peers with QR
codes) and OpenVPN (ready `.ovpn` downloads), DNS with rebind protection,
cake-based SQM with a bufferbloat grade, DDNS, guest and IoT Wi-Fi on their
own isolated subnets, port forwarding, and a router/AP mode switch that
moves the WAN role without editing `network` and `firewall` by hand.

<p align="center">
  <img alt="Services page with cards for WireGuard, DDNS, SQM and the visual firewall" src="assets/screenshot-services-en.png" width="800">
</p>

**System care without the ceremony.** Access and session settings, security
cards, a first-run wizard, firmware updates through owut/ASU that rebuild
the image with your packages inside, and the roaming mesh rendered as a
radial graph via usteer.

<p align="center">
  <img alt="System page with access settings, security, Router/AP mode and update cards" src="assets/screenshot-system-en.png" width="800">
</p>

**And more:** advanced traffic analysis by application (netifyd) with a 24 h
timeline, an optional `luci-app-netgrip` entry under LuCI > Services, and an
ES/EN interface that switches in one click. That is still not the whole
list: **[the website shows every feature](https://netgrip.cloudless.club)**,
each one with its screenshots at full size.

**A NetPulse agent built in.** The same binary also reports metrics, WiFi
events and clients to [NetPulse](https://netpulse.cloudless.club), so the
router shows up labeled as NetGrip in its fleet with no extra install. It is
a capability, not a requirement: if you do not use NetPulse, nothing changes.

## Get it on your router

Requirements: a 64-bit ARM router (`aarch64_cortex-a53`, covers MediaTek
filogic and Qualcomm ipq807x) or x86_64, running OpenWrt 24.10 or 25.12. The
panel listens on port 8090 and logs in with your LuCI credentials.

SSH into the router and run:

```sh
wget -qO- https://raw.githubusercontent.com/gnacho/netgrip/main/install.sh | sh
```

The script picks the right package for the router's architecture and package
manager (`apk` on OpenWrt 25.12+, `opkg` on 24.10), installs the latest
release, enables and starts the service, and prints the panel URL. Prefer to
review it first? Read [install.sh](install.sh). To pin a version:
`NETGRIP_VERSION=vX.Y.Z sh install.sh`.

Open `http://<router-ip>:8090` and log in with the same username and
password you use for LuCI.

<details>
<summary><strong>Manual install (packages)</strong></summary>

Download the right package from the
[releases](https://github.com/gnacho/netgrip/releases/latest) page and, on
the router:

```sh
# OpenWrt 25.12 (apk)
apk add netgrip-<version>-r1-arm64.apk

# OpenWrt 24.10 (ipk)
opkg install netgrip_<version>-1_aarch64_cortex-a53.ipk

/etc/init.d/netgrip enable && /etc/init.d/netgrip start
```

The postinst adds the binary, the init script and the rc.d link to
`/etc/sysupgrade.conf`, so the panel comes back after a firmware update. The
optional `luci-app-netgrip` package embeds the panel under LuCI > Services
in the same way.

</details>

<details>
<summary><strong>Manual install (bare binary) and build from source</strong></summary>

A bare binary works but dies on every firmware update; the package is the
recommended path.

```sh
# Busybox dropbear has no scp; pipe the file instead:
cat netgrip-linux-arm64 | ssh root@<router-ip> "cat > /usr/sbin/netgrip && chmod 755 /usr/sbin/netgrip"
/usr/sbin/netgrip -listen 0.0.0.0 -port 8090
```

Building from source requires Go 1.24+ and Node 22+:

```sh
git clone https://github.com/gnacho/netgrip.git
cd netgrip
cd app && npm ci && cd ..
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o netgrip ./cmd/netgrip
```

</details>

<details>
<summary><strong>Power-user reference: flags, service and JSON API</strong></summary>

Flags (defaults are what most routers want):

| Flag | Default | Description |
| --- | --- | --- |
| `-listen` | `0.0.0.0` | Address to bind. |
| `-port` | `8090` | Port to listen on. GL.iNet firmware serves its own web UI on 8080, hence the default. |
| `-rpcd-url` | `http://127.0.0.1/ubus` | rpcd JSON-RPC endpoint for login validation. |

The session timeout changes from the UI's Access card
(`options.main.session_timeout` in UCI). The service runs from procd:

```sh
/etc/init.d/netgrip status
logread -e netgrip -f
/etc/init.d/netgrip restart
```

There is a JSON API behind every card, used by the frontend: reads via
`GET /api/board`, `/api/system`, `/api/wan`, `/api/wifi`, `/api/lan`,
`/api/dns`, `/api/usteer`, `/api/clients`, `/api/netifyd`, `/api/dpi/apps`,
`/api/dpi/timeline` and `/api/nftqos`; writes via the matching `POST`
endpoints (`/api/wireguard`, `/api/openvpn`, `/api/sqm`, `/api/guestwifi`,
`/api/iotwifi`, `/api/portforward`, `/api/netifyd`, `/api/nftqos`) with a
`{ "state": ..., "rolled_back": ..., "status": "applied|rolled_back|failed" }`
shape. Every write requires a session cookie.

</details>

## What's next

Done recently: application-level traffic analysis with a per-app timeline,
per-device bandwidth limits over nftables, and mipsle support for older
hardware. Coming next: a custom packages feed so owut/ASU can keep NetGrip
inside your firmware image ([#63](https://github.com/gnacho/netgrip/issues/63)),
and keeping the public demo in step with the panel. Ideas and reports in the
[issues](https://github.com/gnacho/netgrip/issues) steer what gets built.

## Development

Stack: Go (single static binary) + React 19 + TypeScript + Vite + Tailwind,
embedded with `go:embed`. No external database.

```sh
cd app && npm ci
npm run dev      # frontend dev server (see vite.config.ts for the /api proxy)
go build -o netgrip ./cmd/netgrip
go test ./...
```

The CI builds the frontend, cross-compiles and packages `.apk`/`.ipk` with
the OpenWrt SDK on every release tag.

## License

AGPL-3.0-only. See [LICENSE](LICENSE).

Built by gnacho as a personal self-hosted project. If NetGrip is useful to
you, a star on GitHub is the way to say thanks; issues and PRs are welcome.
