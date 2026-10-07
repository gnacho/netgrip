# ADR-002: Authentication: one admin, one session, one MCP token

## Status

accepted

## Context

NetGrip targets the single-admin home router. OpenWrt has exactly one login
account (root) and an rpcd session system already bound to it. A multi-user,
ACL-graded model (the way we-are-mono/verso does it: unprivileged shell,
every action re-authorized through rpcd ACLs) solves a problem this product
does not have, at the cost of a much larger attack surface to design and
audit.

A second consumer appeared later: AI assistants speaking MCP. They are not
browsers: they cannot hold a session cookie and they must not be able to
ride one either.

## Decision

- The panel authenticates against rpcd with the router's root password;
  sessions are stateless HMAC tokens in a per-transport cookie (#423).
- The MCP endpoint (`/mcp`) authenticates with a dedicated bearer token read
  from UCI (`netgrip.mcp.*`), never the session cookie, and is gated off by
  default (404 while disabled). MCP tools are read-only; write tools would
  need their own justification ADR.
- No user accounts, no roles, no ACL tiers.

## Alternatives considered

- **rpcd ACL model for every action (verso-style):** correct for fleets with
  restricted operators; over-engineering here, and every privileged path
  would still end in a root companion.
- **Reusing the session cookie for MCP:** would let any cross-site request
  ride an active browser session into the tools.

## Consequences

Whoever holds the token or the password owns the router: that is the threat
model, and it must stay written down. If a real multi-user need ever
appears, this ADR is the thing to reopen first.
