---
type: planning
entity: plan
plan: netifyd-dpi
status: draft
created: 2026-09-02
updated: 2026-09-02
---

# Plan: netifyd-dpi

## Problem / Context

NetGrip currently ships a DPI-lite card in Overview and a Traffic (DPI) page that parses `/proc/net/nf_conntrack` and classifies traffic by destination port/protocol into 12 broad categories. It cannot distinguish applications (for example YouTube vs Google Docs on port 443) and is shown inline.

Issue #136 asks for a dedicated DPI page powered by `netifyd` (libndpi), which is available in the official OpenWrt 25.12.5 feeds and was verified running on the target hardware (aarch64, rt3).

## Target Outcome

A new dedicated DPI page that, when `netifyd` is installed and enabled, shows a live application-level breakdown of traffic (top applications by bytes/flows, with per-app local/other byte counters) parsed from the netifyd UNIX socket. When netifyd is not installed, the UI offers to install it and keeps DPI-lite as fallback.

## Guiding Decisions & Constraints

- Follow the existing NetGrip pattern: package toggle with `snapshot/healthcheck/rollback` (like mDNS, nlbwmon, etc.).
- Keep the DPI-lite parser as fallback when netifyd is absent or disabled.
- netifyd keeps no history, so timeline charts remain out of scope for this issue (#137 covers that).
- The persistent socket client runs inside the NetGrip process; no extra daemon.
- Skip activation on devices with <128 MB RAM or single-core CPUs (issue requirement).
- **No numeric badge on the DPI/Traffic menu item**: keep it badge-free like Tools, Storage or Fleet; only Clients, Services and System keep their existing counters.

### Scope-Bounding Assumptions

- netifyd 4.4.7-r2 from OpenWrt feeds is the target version; the socket protocol is newline-delimited JSON preceded by `{"length": N}`.
- The router used for validation (rt3) has netifyd already installed and running.

## Requirements

### Functional

- Detect whether `netifyd` is installed, enabled and running.
- Allow the user to enable/disable `netifyd` from the UI with rollback on failure.
- Maintain a persistent UNIX socket client while netifyd is enabled.
- Parse `agent_hello`, `agent_status`, `protocols`, `applications`, `flow`, `flow_stats` and `flow_purge` messages.
- Join flow events by digest and keep a live, bounded per-application byte/flow table in RAM.
- Expose the live table via `GET /api/dpi/apps`.
- Upgrade the existing `/api/dpi` response to include both legacy categories and netifyd app data when available.
- Render a dedicated DPI page with: toggle/install hint, top applications list, fallback to DPI-lite.
- Update demo data so the page works in `VITE_DEMO=1` builds.

### Non-Functional

- CPU/RAM guard: refuse to enable netifyd on devices with <128 MB RAM or single-core CPUs.
- Bounded memory: evict stale flows/apps from the live table with a maximum size.
- Graceful degradation: if the socket fails, fall back to DPI-lite and surface the error in the UI.

## Scope

### In Scope

- Go module `internal/modules/netifyd.go`: probe, set, socket client, app table.
- API endpoints: `GET /api/netifyd`, `POST /api/netifyd`, `GET /api/dpi/apps`.
- Update existing `GET /api/dpi` to include netifyd state and apps.
- Frontend page `app/src/pages/Dpi.tsx` upgrade with toggle and app list.
- Demo fixtures and i18n keys (ES/EN).
- Validation on rt3 and release.

### Out of Scope

- Timeline/ring buffer for historical data (issue #137).
- Per-host or per-category charts beyond the existing DPI-lite bars.
- netifyd configuration beyond enable/disable (package defaults are enough for the first iteration).

## Definition of Done

- [ ] Backend builds, unit tests pass and cover socket message parsing + app aggregation.
- [ ] Frontend builds with 0 TypeScript/lint errors.
- [ ] `go test ./...` and `npm run build` pass locally.
- [ ] rt3 validation shows real applications from netifyd with 0 JS errors.
- [ ] PR opened with `Closes #136`.
- [ ] Release created and demo updated.

## Testing Strategy

- Go unit tests for message parsing and app aggregation with recorded netifyd JSON fixtures.
- Frontend manual/Playwright validation on demo and rt3.
- End-to-end validation on rt3 with `netifyd` enabled and generating traffic.

## Phases

| Phase | Title | Contribution | Why Separate | Detail | Status |
|-------|-------|--------------|--------------|--------|--------|
| 1 | Backend netifyd module and API | Persistent socket client, live app table, endpoints | Backend must be stable before UI consumes it | [Phase 1](phases/phase-1.md) | pending |
| 2 | Frontend DPI page upgrade | Toggle, app list, fallback to DPI-lite | UI depends on backend API shape | [Phase 2](phases/phase-2.md) | pending |
| 3 | Integration, validation and release | Demo data, rt3 dogfood, release | Requires real router and CI | [Phase 3](phases/phase-3.md) | pending |

## Risks & Open Questions

| Risk/Question | Impact | Mitigation/Answer |
|---------------|--------|-------------------|
| netifyd socket protocol changes between versions | Parsing breaks | Target version pinned to 4.4.7-r2 from feeds; tests with fixtures |
| High CPU/RAM on low-end routers | Performance regression | Hard guard on <128 MB RAM / single-core; bounded app table |
| Concurrent access to live table from API and socket goroutine | Race conditions | Mutex-protected table, copy-on-read for API |

## Changelog

### 2026-09-02

- Plan created
