---
type: planning
entity: phase
plan: "advanced-section"
phase: 3
status: pending
created: "2026-10-06"
updated: "2026-10-06"
---

# Phase 3: Advanced switch features (port mirroring, rate limiting)

> Part of [advanced-section](../plan.md)

## Objective

Add DSA port mirroring and per-port rate limiting (ingress/egress) to the Advanced section, capability-gated per port, verified on the maintainer's dedicated test switch.

## Scope

### Includes

- Capability probe per switch port: mirroring support, ingress/egress rate limit support, current mirror config, current rates.
- Mirroring card: pick source port(s) and monitor port; warning when mirroring the management path.
- Rate limit card: per-port ingress/egress values where supported.
- Apply through the executor without restarting the network; healthcheck verifies connectivity is kept.
- Clean "not supported by this device" state on hardware without the capabilities.
- i18n ES/EN, demo stub, unit tests for the capability probe and plan building.

### Excludes

- swconfig-era targets (report unsupported only).
- QoS/SQM features (already exist elsewhere); this is per-port raw rate limiting.
- Any persistent packet capture or traffic inspection UI.

## Prerequisites

- [ ] Phase 1 merged.
- [ ] The dedicated test switch available (the maintainer provides it); its OpenWrt version and DSA support confirmed.

## Deliverables

- [ ] Capability probe module + tests.
- [ ] Mirroring and rate-limit apply paths with executor integration.
- [ ] Cards in the Advanced section (UniFi style, per-port rows).
- [ ] Live verification report on the test switch (issue #443 updated).

## Acceptance Criteria

- [ ] On the test switch: mirror a source port to a monitor port and verify traffic arrives (tcpdump on the monitor port); remove the mirror cleanly.
- [ ] Set a per-port rate and verify it holds (iperf or equivalent); clear it cleanly.
- [ ] On hardware without support: cards show the unsupported state, no errors in the log.
- [ ] A forced healthcheck failure rolls the change back without dropping the session.
- [ ] Full gates green; PR merged.

## Dependencies on Other Phases

| Phase | Relationship | Notes |
|-------|-------------|-------|
| 1 | blocks | Needs the section and the gate |
| 2 | none | Independent; order between 2 and 3 is arbitrary |

## Notes

- Mirroring on DSA is typically done via tc/mirror or driver-specific hooks depending on the target; the implementation plan must settle the mechanism on the real device before coding the apply path.
- Rate limiting likewise: tc police vs driver offload. Decide on the test switch, document the choice in the issue.
