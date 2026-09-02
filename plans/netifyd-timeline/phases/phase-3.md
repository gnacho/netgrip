# Phase 3 — Validation, deploy and release

## Tasks
- [ ] Full build: frontend in `app/` + `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build`.
- [ ] Deploy binary to rt3, enable netifyd, wait for buckets.
- [ ] Verify `/api/dpi/timeline` returns buckets and top apps.
- [ ] Playwright check: chart renders, table renders, totals row present, 0 JS runtime errors.
- [ ] Commit each logical chunk, push branch, open PR referencing #137.
- [ ] Merge PR, release v0.52.0, update `memory/netgrip.md`.

## Gate
- Playwright passes and release is published.
