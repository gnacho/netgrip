# Phase 2 — Frontend client detail limit

## Tasks
- [x] Add TypeScript types `NftQoSLimit`, `NftQoSProbe` in `app/src/types.ts`.
- [x] Add API methods `api.nftqos()` and `api.setNftQoS(...)` in `app/src/api.ts`.
- [x] Add demo fixtures in `app/src/demo/data.ts` and `app/src/demo/index.ts`.
- [x] Add bandwidth limit section in `DetailClientModal` (`app/src/pages/Clients.tsx`):
  - Toggle enable/disable.
  - Two numeric inputs (download/upload) in Mbps.
  - Save button with busy state.
  - Warning when device has no DHCP reservation.
  - Disabled state when not applicable (AP/switch).
- [x] Add i18n keys in `app/src/locales/es.ts` and `app/src/locales/en.ts`.

## Notes
- Fixed React hook-order bug in `DetailClientModal`: the early return (`if (!client) return null;`) was placed before `useEffect`, causing "Rendered more hooks than during the previous render" when the modal opened. Moved the early return after all hooks.

## Gate
- `npm run build` in `app/` passes with no TS errors.
