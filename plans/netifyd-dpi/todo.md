---
type: planning
entity: todo
plan: netifyd-dpi
updated: 2026-09-02
---

# Todo: netifyd-dpi

> Tracking [netifyd-dpi](plan.md)

## Active Phase: 1 - Backend netifyd module and API

### Phase Context

- **Scope**: [Phase 1](phases/phase-1.md)
- **Implementation**: Not authored yet
- **Latest Handover**: None
- **Relevant Docs**: `internal/modules/mdns.go` (toggle pattern), `internal/modules/dpi.go` (DPI-lite), `internal/server/server.go` (routes)

### Pending

- [ ] Create branch `feat/136-netifyd-dpi` from `main`.
- [ ] Add `internal/modules/netifyd_socket.go` with socket client and message parsing.
- [ ] Add `internal/modules/netifyd.go` with probe, set and live app table.
- [ ] Wire routes in `internal/server/server.go`.
- [ ] Add unit tests with fixtures.
- [ ] Add `netifyd` to optional packages catalog.
- [ ] Run `go test ./...`.

### In Progress

- [ ] None

### Completed

- [x] Plan created and approved by user direction (issue #136 selected).

### Blocked

- [ ] None

## Changelog

### 2026-09-02

- Plan created.
- User feedback integrated: DPI/Traffic menu item must not show a numeric badge.
