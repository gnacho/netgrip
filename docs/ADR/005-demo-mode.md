# ADR-005: Demo mode is a frontend contract, not a server mode

## Status

accepted

## Context

demo.netgrip.cloudless.club shows the panel without a router. Options: run
the real binary against a fake device, or stub the API at the client edge.

## Decision

Demo mode is a build-time frontend switch: the API client is replaced by an
in-memory implementation (`demoApi`) returning realistic fixtures, and the
Go backend is not involved. Every new endpoint ships its demo stub in the
same change, so the demo can never drift from the API shape.

## Alternatives considered

- **Server-side demo with faked commands:** would test the backend against
  fiction and invite demo-only bugs in the real paths.

## Consequences

The demo cannot demonstrate backend behavior (apply, rollback, healthchecks)
- it demonstrates the product surface. Fixture realism is a maintenance cost
paid per feature. Screenshots and the landing derive from this same build.
