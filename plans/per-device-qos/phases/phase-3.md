# Phase 3 — Validation and release

## Tasks
- [x] Full build: frontend in `app/` + `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build`.
- [x] Deploy to rt3.
- [ ] Set a limit on a real client, verify rule with `nft list ruleset` and check it matches. (Blocked: rt3 is in AP mode; needs gateway-mode router.)
- [ ] Remove the limit, verify table is gone. (Blocked: rt3 is in AP mode.)
- [x] Playwright check: detail modal shows limit section in demo mode (`applicable:true`) and not-applicable message in rt3 (`applicable:false`), 0 JS runtime errors.
- [ ] Create branch `feat/138-per-device-qos`, push, open PR referencing #138.
- [ ] Merge PR, release v0.54.0, update `memory/netgrip.md`.

## Notes
- Fixed a React hook-order bug in `DetailClientModal` during validation: the early return was placed before `useEffect`, causing a white screen on opening. Moved the early return after all hooks.
- rt3 is currently a dumb AP, so `ProbeNftQoS` correctly returns `applicable:false`. The nft rule path has been unit-tested; end-to-end enforcement needs a gateway-mode device.

## Gate
- Playwright passes and release is published.
