# Phase 2 — Frontend timeline chart and app statistics table

## Tasks
- [ ] Add TypeScript types `NetifydTimeline`, `TimelineBucket`, `TimelineAppPoint`, `TimelineTotals`.
- [ ] Add `api.dpiTimeline()` and mock response in `demo/index.ts` + `demo/data.ts`.
- [ ] Build chart section in `Dpi.tsx` using existing `MultiSeriesChart`.
- [ ] Build sortable/searchable app table with totals row and percentages.
- [ ] Add/extend i18n keys (ES/EN) for table headers, search placeholder and totals row.
- [ ] Keep existing netifyd toggle, install hint and DPI-lite fallback.

## Gate
- `npm run build` in `app/` passes and `tsc --noEmit` has no errors.
