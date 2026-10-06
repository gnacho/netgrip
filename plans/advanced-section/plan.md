---
type: planning
entity: plan
plan: "advanced-section"
status: active
created: "2026-10-06"
updated: "2026-10-06"
---

# Plan: advanced-section

## Problem / Context

NetGrip keeps power-user features scattered: VLAN management lives inside the Ports page, IGMP snooping / storm control / MAC ACL hide in a drawer under Tools, and there is no SNMP configuration or advanced switch control (mirroring, rate limiting) at all. These features can lock a user out of their network, so they should not sit at the same visibility level as everyday settings. Inspiration: how UniFi and MikroTik keep an "advanced" tier that is opt-in and honest about risk.

## Target Outcome

An opt-in "Advanced" section in the nav, gated by `netgrip.main.advanced=1`, that groups the existing power-user features and grows with SNMP configuration and advanced switch features (port mirroring, rate limiting). The default UI stays unchanged for non-advanced users. Tested on a dedicated switch running OpenWrt before release.

## Guiding Decisions & Constraints

- Activation: UCI flag `netgrip.main.advanced=1` + a toggle in Settings that writes it through the executor (snapshot + rollback). The frontend hides the nav entry when off; backend endpoints for the section answer 404 while disabled.
- The Advanced section groups what already exists (VLAN, IGMP snooping, storm control, MAC ACL) instead of leaving it scattered.
- SNMP: full `snmpd` (net-snmp) only; no snmpd-mini. Config through UCI `/etc/config/snmpd`.
- Advanced switch features target DSA; unsupported hardware reports a clean "not supported" state instead of errors.
- Visual language: UniFi-style cards (clean domain grouping, large switches, live status); no change to the rest of the app's design system beyond reusing it.
- Every write keeps the existing executor model: snapshot, allowlist, healthcheck, rollback. Mirroring/rate-limit changes must not restart the network.
- Issues: #441 (framework), #442 (SNMP), #443 (switch features).

### Scope-Bounding Assumptions

- The dedicated test switch runs OpenWrt with DSA. If it turns out to be swconfig-era, phase 3 reports unsupported and the issue is updated.
- snmpd traps and SNMPv3 users are deferred until the UCI mapping is proven on the test device.

## Requirements

### Functional

- [ ] Advanced mode flag readable/writable via UCI, toggle in Settings with a risk warning, applied through the executor.
- [ ] New "Advanced" nav section visible only when the flag is on.
- [ ] VLAN, IGMP snooping, storm control and MAC ACL reachable from the Advanced section (moved or linked, no duplicated code paths).
- [ ] All Advanced endpoints return 404 when the flag is off, independent of the UI hiding.
- [ ] SNMP card: install/remove snmpd, configure enabled, ro/rw communities, location, contact, listen interface; live daemon state in the probe.
- [ ] Switch card: port mirroring (source ports + monitor port) and per-port rate limiting (ingress/egress) where the driver supports it; capability probe per port.
- [ ] i18n ES/EN for every new string; demo stubs where the app has demo mode.

### Non-Functional

- [ ] A failed healthcheck after an Advanced change rolls back and says so, like every other module.

## Scope

### In Scope

- Advanced mode flag + Settings toggle + nav section (phase 1).
- Grouping/moving existing VLAN and advanced Tools cards into the section (phase 1).
- SNMP snmpd configuration card (phase 2).
- DSA port mirroring and rate limiting on the test switch (phase 3).

### Out of Scope

- snmpd-mini support, SNMPv3 users, trap configuration (documented as future work).
- Non-DSA (swconfig) switch support; phase 3 reports unsupported there.
- Auto-unlock or discovery of "advanced" capability per device; the flag is global per router.
- Any change to the executor core or to non-advanced pages beyond the moves in phase 1.

## Definition of Done

- [ ] With the flag off, the app behaves exactly as before and Advanced endpoints answer 404.
- [ ] With the flag on, all grouped features work from the new section on a real router (rt-lab) and on the dedicated test switch (phases 2-3).
- [ ] Full test suite green; new module probes have unit tests; UI smoke passes.
- [ ] PRs merged with clean bodies; release cut only on explicit user confirmation.

## Testing Strategy

- Unit tests for the flag probe, the advanced-gate middleware, SNMP probe/apply mapping, and the mirroring/rate-limit capability probe.
- Live verification on rt-lab (phase 1) and on the user's dedicated switch (phases 2-3): apply a change, confirm it lands (`uci show`, `tc`/mirror state), force a rollback case where safe.
- Playwright smoke for the new section (visible/hidden per flag, toggle flow).

## Phases

| Phase | Title | Contribution | Why Separate | Detail | Status |
|-------|-------|--------------|--------------|--------|--------|
| 1 | Advanced mode + section framework | The gate, the nav section, the moved cards: everything else builds on this | Consumable boundary: a complete, testable UI/backend gate deliverable before adding new hardware-facing features | [Phase](phases/phase-1.md) | pending |
| 2 | SNMP configuration (snmpd) | A self-contained card with its own package lifecycle, independent of switch hardware | Different domain (package + UCI mapping) and different test surface; can ship after phase 1 alone | [Phase](phases/phase-2.md) | pending |
| 3 | Advanced switch features (mirroring, rate limit) | Hardware-dependent DSA features that need the dedicated switch to even develop against | Blocked on physical test hardware; the riskiest changes (network-facing), so last | [Phase](phases/phase-3.md) | pending |

## Risks & Open Questions

| Risk/Question | Impact | Mitigation/Answer |
|---------------|--------|-------------------|
| Test switch hardware unknown yet (DSA vs swconfig, mirror/ratelimit support) | Phase 3 scope may shrink to "unsupported" reporting | Capability probe per port; issue #443 updated with findings |
| Mirroring the management port can lock the user out | Real operational risk on the test switch | Warning before apply; mirror config validated against the active management interface; executor rollback still applies |

## Changelog

### 2026-10-06

- Plan created. Decisions confirmed with the maintainer: UCI flag + Settings toggle, grouping existing features, snmpd (no mini), DSA target, UniFi-style cards, 3 phases.
