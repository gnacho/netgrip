---
type: planning
entity: phase
plan: netifyd-dpi
phase: 1
status: pending
created: 2026-09-02
updated: 2026-09-02
---

# Phase 1: Backend netifyd module and API

> Part of [netifyd-dpi](../plan.md)

## Objective

Add a Go module that talks to the netifyd UNIX socket, maintains a live per-application traffic table, and exposes the data through new API endpoints. The backend must be testable without a running router.

## Scope

### Includes

- `internal/modules/netifyd.go`: probe, enable/disable with rollback, persistent socket client, app table.
- `internal/modules/netifyd_socket.go`: low-level socket client and message parsing.
- `internal/server/server.go`: register `GET /api/netifyd`, `POST /api/netifyd`, `GET /api/dpi/apps`, update `GET /api/dpi`.
- Unit tests with recorded netifyd JSON fixtures.
- Add `netifyd` to the optional-packages catalog so the wizard and Tools page can install it.

### Excludes

- Frontend changes (Phase 2).
- Timeline/history (issue #137).
- Per-host breakdown beyond existing client data.

## Prerequisites

- `main` is up to date.
- Feasibility of socket protocol already verified on rt3 (see issue #136 comment).

## Deliverables

- [ ] `internal/modules/netifyd.go` with `ProbeNetifyd`, `SetNetifyd`.
- [ ] `internal/modules/netifyd_socket.go` with `StartNetifydClient`, `StopNetifydClient`, `AppTable()`.
- [ ] Bounded in-memory table keyed by application name with bytes/packets/flows.
- [ ] Server handlers wired and returning JSON.
- [ ] Unit tests with fixtures.
- [ ] `go test ./...` green.

## Acceptance Criteria

- [ ] `ProbeNetifyd` reports installed/enabled/running without a socket connection.
- [ ] `SetNetifyd(true)` installs the package, starts the service and starts the socket client.
- [ ] `SetNetifyd(false)` stops the service and the socket client.
- [ ] The socket client parses `agent_hello`, `applications`, `flow`, `flow_stats`, `flow_purge` messages correctly.
- [ ] API `GET /api/dpi/apps` returns aggregated apps sorted by bytes.
- [ ] Tests cover parsing, aggregation and table eviction.

## Dependencies on Other Phases

| Phase | Relationship | Notes |
|-------|-------------|-------|
| 2 | blocked-by | Backend API must be stable before UI consumes it |

## Notes

- The socket protocol uses `{"length": N}` framing followed by a JSON object per message.
- Keep the module idempotent: starting an already running client is a no-op.
