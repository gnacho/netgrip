# Plan — DPI bandwidth timeline per application (#137)

## Goal
Upgrade the DPI page to a Netify-style dashboard: top applications by bandwidth over time (multi-series area chart) plus a searchable, sortable application statistics table with totals and percentages.

## Out of scope
- Persisting history across NetGrip restarts (memory-only aggregation keeps it light).
- nDPI protocol categories; we keep the existing application name returned by netifyd.
- Favicon fetching from the internet; icons are mapped to bundled/generic icons.

## Backend design

### Time buckets
- 5-minute buckets (`bucket = ts / 300 * 300`).
- Keep up to 24 h of buckets (288 entries). Older buckets are evicted.
- Every `flow_stats`/`flow_purge` event updates the current bucket for the application name with `local_bytes`, `other_bytes` and `total_bytes`.
- Data is stored in the existing `netifydAppTable` under a new `buckets map[int64]map[string]*NetifydBucket` field.

### API
- `GET /api/dpi/timeline?range=24h` (range ignored for now; always returns full window)
- Response:
  ```json
  {
    "buckets": [
      { "time": "2026-09-02T16:00:00Z", "apps": { "HTTPS": { "local": 120, "other": 80, "total": 200 } } }
    ],
    "top": [
      { "name": "HTTPS", "local_bytes": 12345, "other_bytes": 6789, "total_bytes": 19134, "percent": 28.3 }
    ],
    "totals": { "download": 12345, "upload": 6789, "total": 19134 }
  }
  ```
- Only the top 10 applications by `total_bytes` in the requested window are returned; the timeline contains one series per top app.

### Memory budget
- 288 buckets × ~100 apps × 3 ints ≈ small (a few MB worst case). Bounded by evicting smallest apps and oldest buckets.

## Frontend design

### Chart section
- Reuse `MultiSeriesChart` from `components/ui/charts.tsx` with one `MultiSeries` per top app, color from a generated palette.
- Legend chips above the chart with app names; hidden series are togglable.
- Hover tooltip shows per-app values at the hovered bucket.

### Table section
- Search input filters by application name.
- Columns: Application (icon + name), Download, Upload, Total bandwidth, each with value + percentage.
- Sortable by all numeric columns and by name.
- Highlighted totals row "Todo el trafico" at the top.

### States
- Skeleton while loading.
- Empty state when netifyd is disabled or there are no flows.
- Error state preserved from existing page.

## Implementation phases

1. **Backend buckets and timeline API**  
   - Extend `netifydAppTable` with buckets and eviction.  
   - Add `NetifydTimeline()`, `NetifydTopApps()` helpers.  
   - Wire `GET /api/dpi/timeline` in `server.go`.  
   - Unit tests for bucket aggregation and eviction.

2. **Frontend timeline and table**  
   - Add types `NetifydTimeline`, `TimelineBucket`, `TimelineApp`, `TimelineTotals`.  
   - Add `api.dpiTimeline()` and demo fixture.  
   - Build `DpiTimeline` chart and `DpiAppsTable` components in `Dpi.tsx` (or split files).  
   - Add i18n keys under `dpi.*`.  
   - Keep existing toggle and DPI-lite fallback.

3. **Validation and release**  
   - `go test ./...`, `npm run build` in `app/`, deploy to rt3.  
   - Playwright check for chart + table, 0 JS runtime errors.  
   - Commit, PR, merge, release v0.52.0, update memory.

## Files to touch
- `internal/modules/netifyd_socket.go`
- `internal/modules/netifyd.go`
- `internal/modules/netifyd_socket_test.go`
- `internal/server/server.go`
- `app/src/types.ts`
- `app/src/api.ts`
- `app/src/demo/data.ts`
- `app/src/demo/index.ts`
- `app/src/pages/Dpi.tsx`
- `app/src/locales/es.ts`
- `app/src/locales/en.ts`
- `memory/netgrip.md`

## Success criteria
- `/api/dpi/timeline` returns buckets and top-10 series after netifyd has run for a few minutes.
- DPI page shows an area chart and a sortable app table with totals row.
- Low-end routers still refuse to enable netifyd; no extra cost if disabled.
- All existing tests pass; new tests added; no JS runtime errors.
