# Phase 4: device-free handler test for GET /api/clients

## Objective

A handler test for `GET /api/clients` (behind requireAuth, inside the
probe cache) that wires fakes for every device read in the ListClients
path and asserts the assembled clients payload, plus the failure
contract where applicable.

## Scope

In: `internal/server/clients_test.go` (or equivalent) with fakes for
Backend.WirelessStatus, uciExec, lease/arp/FDB/parental seams and
ServiceEnabled; cold-cache invalidation for the `clients|<ip>` key.

Out: coverage of client meta/block/reserve mutations (separate handlers,
existing tests).

## Acceptance

- `go test ./...` green on a host with no ubus/uci.
- The test asserts at least: one wireless client with lease identity
  (name/IP), the requester flagged self, wired client from the FDB when
  mode is router.
- After merge, spot check on rt-lab that the panel clients table is
  unchanged.

## Dependencies

Phases 1, 2 and 3.
