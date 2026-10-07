# ADR-003: Every write goes through the executor

## Status

accepted

## Context

A panel that edits router configuration can lock the operator out of the
device with one bad write (a firewall drop on the management source, a VLAN
that strands the only admin port). Hand-rolled "write then maybe fix" paths
fail silently and half-applied: uci stages changes uncommitted, so a failed
batch can also be picked up later by an unrelated commit.

## Decision

Every mutation is a list of allowlisted executor ops (`uci_set`, `uci_commit`,
`initd`, `pkg_add`, ... — see `internal/executor`): snapshot the affected UCI
package, apply in order, healthcheck, and restore the snapshot on failure.
Module code never shells out for writes outside this list, and the allowlist
is the review point for "what the panel can ever do".

Write paths must be idempotent where uci allows it (`uci delete` of an
absent entry is treated as success), and named UCI sections are preferred
over anonymous ones so re-apply never duplicates.

## Alternatives considered

- **Free-form command execution:** rejected; unauditable and unbounded.
- **ubus uci apply with rollback:** rpcd session-bound and bypassed by root
  processes; storing the root password to get sessions back is a worse hole
  than the one it closes (verified against rpcd on 21.02/25.12).
- **Confirm-or-revert armed timers (GlassOnTin/openwrt-mcp style):** an
  attractive addition for future write-capable MCP tools, noted here so the
  idea has a home.

## Consequences

Healthchecks must exist for a write to be safe, and they must check the real
thing (hostapd serving, not ubus "up": #452). The executor is also the unit
NetPulse delegates to for fleet operations, so its allowlist is a public
contract once documented.
