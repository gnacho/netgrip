# Plan — Per-device bandwidth limiting with nftables (#138)

## Goal
Add per-device upload/download bandwidth limits (Mbps) as a toggle in the client detail modal. Since `nft-qos` is not available in OpenWrt 25.12 feeds, implement a self-owned `inet netgrip_qos` nftables table using native `limit rate` rules.

## Out of scope
- Traffic shaping/queuing (HTB/fq_codel); we only do rate policing (drop excess).
- Time-of-day or scheduled rules.
- Per-app/port limits.
- Persistence across firewall reloads in the MVP (a future improvement can add a fw4 include).

## Backend design

### Storage
- JSON file `/etc/netgrip/qos_limits.json` with an array of limits:
  ```json
  [{"mac":"aa:bb:...","ip":"192.168.1.100","download":10,"upload":5}]
  ```
- Before every write the old file is backed up for rollback.

### nftables rules
- Table `inet netgrip_qos`.
- Chains:
  - `upload` hook prerouting priority -150: drop packets from limited IPs over upload rate.
  - `download` hook postrouting priority -150: drop packets to limited IPs over download rate.
- Rules are generated per active IP/MAC pair:
  ```nftables
  ip saddr 192.168.1.100 limit rate over 625 kbytes/second drop
  ip daddr 192.168.1.100 limit rate over 1250 kbytes/second drop
  ```
- If no limits are active the table is deleted.

### Snapshot/rollback
- Before applying, capture `nft list ruleset`.
- Apply new ruleset with `nft -f /etc/netgrip/qos_limits.nft`.
- Healthcheck: `nft list table inet netgrip_qos` succeeds (or, when empty, table is absent).
- On failure: restore previous ruleset via `nft -f -`.

### API
- `GET /api/nftqos` → `{ applicable: bool, limits: Record<mac, NftQoSLimit> }`.
  - `applicable` is false on dumb AP/switch (no WAN interface).
- `POST /api/nftqos` → body `{ mac, ip, download?, upload? }`. Missing or zero rate disables that direction.
- `DELETE /api/nftqos?mac=...` → remove limit for that MAC.

### Coexistence
- Independent of SQM (`sqm-scripts` operates on WAN qdisc; ours on prerouting/postrouting).
- Independent of client block/allow rules.

## Frontend design

### Client detail modal
- New section "Limitar ancho de banda" / "Bandwidth limit".
- Toggle enables/disables the limit for this device.
- When enabled, two numeric inputs: download (↓) and upload (↑) in Mbps.
- "Save" button calls `POST /api/nftqos`.
- Help tip when not reserved: "Para que el límite sobreviva a cambios de IP, reserva la IP del dispositivo."
- "Not applicable" state when router is in AP/switch mode.
- "Software Flow Offloading may bypass this limit" warning (if we can detect it; otherwise static help text).

### Types and API
- Add `NftQoSLimit`, `NftQoSProbe` types.
- Add `api.nftqos()` and `api.setNftQoS(...)`.
- Add demo fixtures.

### i18n
- New keys under `clients.*` or `qos.*` for limit toggle, labels, warnings and help.

## Implementation phases

1. **Backend module**  
   - Create `internal/modules/nftqos.go`.  
   - Load/save JSON, generate nft rules, snapshot/rollback apply.  
   - Add `GET /api/nftqos`, `POST /api/nftqos`, `DELETE /api/nftqos` handlers in `server.go`.  
   - Add unit tests for rule generation and JSON persistence.

2. **Frontend client detail**  
   - Add types, API methods and demo fixtures.  
   - Add limit section in `DetailClientModal`/`Clients.tsx`.  
   - Add i18n keys (ES/EN).  
   - Poll/refresh limits after save.

3. **Validation and release**  
   - `go test ./...`, `npm run build` in `app/`.  
   - Deploy to rt3, set a limit on a test device and verify with `nft list ruleset`.  
   - Playwright check: detail modal renders limit section, 0 JS runtime errors.  
   - Branch, PR, merge, release v0.54.0, update memory.

## Files to touch
- `internal/modules/nftqos.go` (new)
- `internal/modules/nftqos_test.go` (new)
- `internal/server/server.go`
- `app/src/types.ts`
- `app/src/api.ts`
- `app/src/demo/data.ts`
- `app/src/demo/index.ts`
- `app/src/pages/Clients.tsx`
- `app/src/locales/es.ts`
- `app/src/locales/en.ts`
- `memory/netgrip.md`

## Success criteria
- Setting a 5 Mbps upload limit for a client generates a matching nftables rule.
- Disabling the limit removes the rule.
- Apply rollback works if `nft` fails (e.g., invalid IP).
- UI shows limits and saves them without errors.
