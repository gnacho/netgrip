---
type: planning
entity: phase
plan: netifyd-dpi
phase: 2
status: pending
created: 2026-09-02
updated: 2026-09-02
---

# Phase 2: Frontend DPI page upgrade

> Part of [netifyd-dpi](../plan.md)

## Objective

Upgrade the existing Traffic/DPI page so it can toggle netifyd, show an install hint when missing, display the live application breakdown, and keep the DPI-lite fallback. No numeric badge must be added to the menu item.

## Scope

### Includes

- Extend `app/src/pages/Dpi.tsx` with netifyd status, toggle and app list.
- Add API methods in `app/src/api.ts` for netifyd and `/api/dpi/apps`.
- Add TypeScript types in `app/src/types.ts`.
- Update i18n keys in `app/src/locales/es.ts` and `en.ts`.
- Update demo data so the page works in demo mode.
- Ensure the menu item for DPI/Traffic does **not** get a numeric badge.

### Excludes

- Timeline charts (issue #137).
- New navigation groups.
- Backend changes (Phase 1).

## Prerequisites

- Phase 1 backend endpoints are stable.

## Deliverables

- [ ] Updated `Dpi.tsx` with install hint, toggle, app list and fallback bars.
- [ ] New API methods and types.
- [ ] Demo fixtures.
- [ ] i18n ES/EN.
- [ ] `npm run build` and `npm run lint` green.

## Acceptance Criteria

- [ ] When netifyd is not installed, the page shows an install hint and keeps DPI-lite bars.
- [ ] When netifyd is installed but disabled, a toggle enables it.
- [ ] When enabled, the page shows top applications sorted by bytes with bar visuals.
- [ ] The existing category/protocol view remains accessible.
- [ ] The DPI/Traffic menu item has **no numeric badge**.
- [ ] Demo mode renders the page without errors.

## Dependencies on Other Phases

| Phase | Relationship | Notes |
|-------|-------------|-------|
| 1 | blocked-by | Backend API must exist |

## Notes

- Keep the page within the existing layout and card system.
- The app list can reuse the `BarRow` component from the current `Dpi.tsx`.
