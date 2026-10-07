# Plan: clients module behind the Backend seam (#462)

## Context

#462 migrates NetGrip's device reads behind `ubus.Backend` incrementally,
one module per change. The seam already covers `SystemInfo`, `WanStatus`,
`WirelessStatus` and `SystemBoard`, each with device-free handler tests.
The remaining candidate, `modules.ListClients` (backing `GET /api/clients`),
is the big one: it touches many device reads in a single call, so it gets
this mini-plan instead of a one-method step.

Device reads in the `ListClients` path (audited 2026-10-07):

| Read | Where | Mechanism |
| --- | --- | --- |
| Wireless radios + associates | `clients.go` | `ubus.GetWirelessStatus()` |
| DHCP leases | `clients.go` leasesForClients | file `/tmp/dhcp.leases` or SSH to gateway |
| Neighbor/ARP table | `clients.go` localArp | file `/proc/net/arp` (+ gateway SSH in AP mode) |
| DHCP reservations | `clients.go` dhcpReservations | `uci show dhcp` (via uciShowCached) |
| MAC filter / bands | `clients.go` blockedBands | `uci show wireless` (via uciShowCached) |
| Mode probe (AP?) | `mode.go` ProbeMode | `uci get netgrip.main.mode` |
| Parental rules | `parental.go` | files + `routerLocalNow` (exec `date`) |
| Bridge FDB (wired) | `netinfo.go` bridgeFdb | sysfs `/sys/class/net/*/brif` + `brctl` exec |
| Firewall enabled | executor | `/etc/init.d/firewall enabled` exec |

## Target outcome

`GET /api/clients` (and the `ListClients` logic it delegates to) is
exercised by a device-free handler test: every device read above goes
through a fake on the auth path, proving the wiring without ubus, uci,
ssh or sysfs on the host.

## Binding decisions

- Follow the established seam pattern: package-level `DefaultBackend`
  variable in the ubus package; small package vars with defaults for
  file/exec paths (same idea as `auth.secretPath`).
- One phase = one PR, merged independently; every phase ends green.
- No behavior change: reads return the same data on a real device; the
  handler contracts (200 payload shape, cache keys) are untouched.
- `CachedRead` caches errors and entries outlive tests: tests that need
  a cold cache must `modules.InvalidateKey` (lesson from PR #476).

## Scope

In: the seams above as needed by `ListClients`/`handleClients`, plus the
device-free handler test.
Out: migrating other modules' uci reads to the new seam (they can adopt
it later), changing `ListClients` behavior, caching changes.

## Definition of done

- `go test ./...` green on a host with no ubus/uci/router.
- `GET /api/clients` handler test passes with fakes only.
- On a real device (rt-lab) the clients table renders the same clients
  as before the migration.

## Phases

| # | Phase | PR scope |
| --- | --- | --- |
| 1 | ubus reads via the seam | `ListClients` + `AvailableBands` use `DefaultBackend.WirelessStatus()` |
| 2 | uci exec seam | `uciShowCached`/`uciGet`/`uciSectionExists` run through an overridable exec var |
| 3 | file/exec path seams | leases, arp, brif, parental ledger paths + gateway SSH + `ServiceEnabled` fakeable |
| 4 | device-free handler test | `GET /api/clients` green without a device |

## Changelog

- 2026-10-07: plan created (audit of ListClients device reads).
