---
type: planning
entity: phase
plan: "advanced-section"
phase: 2
status: pending
created: "2026-10-06"
updated: "2026-10-06"
---

# Phase 2: SNMP configuration (snmpd)

> Part of [advanced-section](../plan.md)

## Objective

Add an SNMP card to the Advanced section that manages the OpenWrt `snmpd` (net-snmp) package: install/remove, enable, and configure communities, location, contact and listen interface, with live daemon state.

## Scope

### Includes

- Probe: snmpd installed/running/enabled, current UCI config snapshot, package offer (optional-packages pattern as banIP/AdGuard).
- Apply via executor: install/remove when needed, write `/etc/config/snmpd` options, restart snmpd only when its config changed.
- UniFi-style card in the Advanced section: toggle, ro/rw communities, location/contact, listen interface.
- i18n ES/EN, demo stub, unit tests for the UCI mapping.

### Excludes

- snmpd-mini, SNMPv3 users, traps (future work per plan).
- SNMP client/monitoring features (NetPulse already polls; nothing to add here).

## Prerequisites

- [ ] Phase 1 merged (the section and the gate exist).

## Deliverables

- [ ] `internal/modules/snmp.go` (or equivalent) with probe + apply.
- [ ] API endpoints under the advanced gate.
- [ ] `SnmpCard` in the Advanced section.
- [ ] Tests + live verification.

## Acceptance Criteria

- [ ] On a router without snmpd: card offers install; after install the daemon runs with the panel-provided config.
- [ ] Community/location/contact changes apply, survive snmpd restart, and roll back on healthcheck failure.
- [ ] Endpoint answers 404 while advanced mode is off.
- [ ] Full gates green; PR merged.

## Dependencies on Other Phases

| Phase | Relationship | Notes |
|-------|-------------|-------|
| 1 | blocks | Needs the section and the gate |

## Notes

- Verify the UCI -> snmpd.conf generation path on the test device before promising traps in a follow-up; some net-snmp UCI options are silently ignored depending on the build.
