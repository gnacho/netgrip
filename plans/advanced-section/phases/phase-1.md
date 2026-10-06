---
type: planning
entity: phase
plan: "advanced-section"
phase: 1
status: pending
created: "2026-10-06"
updated: "2026-10-06"
---

# Phase 1: Advanced mode + section framework

> Part of [advanced-section](../plan.md)

## Objective

Deliver the opt-in gate and the Advanced nav section, grouping the power-user features that already exist (VLAN, IGMP snooping, storm control, MAC ACL) under it, with zero behavior change for users who keep the flag off.

## Scope

### Includes

- UCI flag `netgrip.main.advanced` probe (read) and write through the executor, with Settings toggle + risk warning.
- Backend gate: a helper that makes every Advanced endpoint answer 404 while the flag is off.
- New "Advanced" nav section (UniFi-style card grouping: Networks/VLAN, Services, Switch placeholder) visible only with the flag on.
- VLAN card moved from Ports to Advanced; IGMP / storm control / MAC ACL moved from the Tools advanced drawer to Advanced. Tools drops its advanced drawer.
- i18n ES/EN, demo stubs where applicable, unit tests for probe + gate.

### Excludes

- SNMP card (phase 2) and switch mirroring/rate limit (phase 3); the section may show a disabled placeholder only if it helps navigation.
- Any functional change to the moved cards themselves.
- Changes to the executor core.

## Prerequisites

- [ ] #441 open and accepted as the tracking issue.

## Deliverables

- [ ] Backend: `ProbeAdvanced()` + `SetAdvanced(enabled)` in a new or existing module, executor-based.
- [ ] Backend: gate middleware/helper applied to the section's endpoints.
- [ ] Frontend: `Advanced.tsx` page with grouped cards; nav entry hidden per flag.
- [ ] Frontend: Settings toggle card with warning.
- [ ] Moved cards integrated without duplication; Tools page cleaned.
- [ ] Tests + smoke on rt-lab.

## Acceptance Criteria

- [ ] Flag off: no nav entry, `/api/advanced/*` answers 404, app otherwise identical.
- [ ] Flag on via UCI: section appears; toggle in Settings can turn it off (executor snapshot/rollback verified with a forced failure if safe).
- [ ] VLAN, IGMP, storm control and MAC ACL all work from the new section on rt-lab.
- [ ] `go vet`, full test suite, frontend build and lint pass.
- [ ] PR merged; rt-lab updated to the merged main.

## Dependencies on Other Phases

| Phase | Relationship | Notes |
|-------|-------------|-------|
| 2, 3 | blocked-by | Both add cards into this section |

## Notes

- Moving cards: prefer moving the components to `components/advanced/` over re-export shims, to avoid two import paths drifting.
- The flag lives in `netgrip.main` so it survives sysupgrade and self-updates like the rest of the panel config.
