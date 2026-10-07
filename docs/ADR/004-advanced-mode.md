# ADR-004: Advanced mode is an opt-in gate, not a visibility trick

## Status

accepted

## Context

Power-user features (VLAN editing, IGMP snooping, storm control, MAC ACLs,
and later SNMP and switch features) can lock a user out of their network if
misused. Scattering them among everyday settings (or hiding them in drawers)
both overserves beginners and underserves experts.

## Decision

- A UCI flag (`netgrip.main.advanced`) is the single source of truth,
  writable from Settings through the executor, with an explicit risk
  confirmation to enable.
- The "Advanced" nav group and its pages exist only while the flag is on.
- The section's API endpoints answer **404** while the flag is off. Hiding
  the UI is not the security boundary; the endpoints pretend the feature
  does not exist, so a disabled install reveals nothing.
- The probe and toggle endpoints stay reachable while off (Settings needs
  them to turn the mode on), but enabling requires confirmation, like the
  router/AP mode change.

Inspired by UniFi's reader-mode concept (ADR-015 there, deferred) and
MikroTik's "nothing is hidden" honesty, resolved as: gated, named
correctly, and honest about risk.

## Alternatives considered

- **Always visible with per-card warnings:** overserves nobody; beginners
  still see the minefield.
- **CLI-only activation:** safer but unfriendly; the panel can ask for the
  risk acknowledgment itself.

## Consequences

New advanced features must register in the group and gate their endpoints.
The 404 contract is part of the API surface: clients can rely on it.
