# Phase 1 — Backend buckets and timeline API

## Tasks
- [ ] Extend `netifydAppTable` with `buckets map[int64]map[string]*NetifydBucket`.
- [ ] Add `NetifydBucket` struct with `Local`, `Other`, `Total` int64, JSON tags lower-case.
- [ ] On each `addStats`, increment the current bucket for the app.
- [ ] Implement bucket eviction: keep only the last 288 buckets (5-minute buckets → 24 h).
- [ ] Add `Timeline() (buckets, top10, totals)` method returning sorted snapshots.
- [ ] Add exported helpers `NetifydTimeline()` and types in `netifyd.go`.
- [ ] Wire `GET /api/dpi/timeline` in `server.go`.
- [ ] Add unit tests covering bucket rounding, aggregation, top-10 selection and eviction.

## Gate
- `go test ./internal/modules/...` passes, including new tests.
