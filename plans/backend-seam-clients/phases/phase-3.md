# Phase 3: file/exec path seams

## Objective

The remaining device touches in the clients path become overridable:
lease file path, ARP source (`/proc/net/arp` or the gateway SSH read),
parental ledger/rules paths, bridge FDB source (sysfs/brctl), and the
`executor.ServiceEnabled` check used to decide blockability.

## Scope

In: package vars with real-device defaults for the paths above plus a
fakeable gateway-SSH read and a fakeable ServiceEnabled on the clients
path only.

Out: generalizing the executor package's exec seam beyond what
`ListClients` needs.

## Acceptance

- Full suite green; on a device the clients table is unchanged.
- Each seam has a unit test proving the override is honored.

## Dependencies

Phases 1-2 (the seams compose in phase 4).
