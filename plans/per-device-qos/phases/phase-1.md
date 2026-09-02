# Phase 1 — Backend nftables module

## Tasks
- [x] Create `internal/modules/nftqos.go`:
  - Types `NftQoSLimit`, `NftQoSProbe`.
  - JSON load/save for `/etc/netgrip/qos_limits.json` with backup.
  - `ProbeNftQoS()` returning `applicable` and limits map.
  - `SetNftQoSLimit(mac, ip, download, upload int)` and `RemoveNftQoSLimit(mac)`.
  - `applyNftQoSRules()` generates and applies ruleset with snapshot/rollback.
- [x] Add routes in `server.go`:
  - `GET /api/nftqos`
  - `POST /api/nftqos`
  - `DELETE /api/nftqos`
- [x] Create `internal/modules/nftqos_test.go` with tests for rule generation and JSON round-trip.

## Gate
- `go test ./internal/modules/...` passes.
