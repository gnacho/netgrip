# Phase 1: ubus reads via the seam

## Objective

`modules.ListClients` and `modules.AvailableBands` read wireless radios
and associates through `ubus.DefaultBackend.WirelessStatus()` instead of
calling `ubus.GetWirelessStatus()` directly, so a test can fake the
wireless read like the handler seam tests do.

## Scope

In: the `ubus.GetWirelessStatus()` call sites in `internal/modules`
(`clients.go` ListClients, and any other module read on the clients
path, e.g. AvailableBands/blockedBands-adjacent reads).

Out: handler tests (phase 4), other ubus reads unrelated to clients.

## Prerequisites

- `Backend.WirelessStatus` exists on main (PR #476).

## Deliverables

- Modules call `ubus.DefaultBackend.WirelessStatus()`.
- No other change; production behavior identical.

## Acceptance

- `go build`, `go vet`, full test suite green.
- On rt-lab the clients table still lists wireless clients (spot check
  after merge via the MCP wireless tool or the panel).

## Dependencies

None. Unlocks phase 4 together with phases 2-3.
