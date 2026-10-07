# ADR-001: Single Go binary, packaged for OpenWrt

## Status

accepted

## Context

NetGrip is a panel that runs on routers: 64-256 MB RAM, flash measured in
tens of megabytes, no guarantee of a toolchain, and sysupgrades that wipe
anything not installed as a package. A panel that dies on every firmware
upgrade is worse than no panel.

## Decision

One static Go binary serves the API and the embedded SPA. Distribution is a
first-class OpenWrt package (apk on 25.12+, ipk on 24.10) built in CI for
seven architectures, with an init.d script that translates UCI options into
flags. Self-update swaps the binary and restarts via init.d; the init script
itself only changes with the package (#433: a binary-only update cannot
change init behavior, and the config/process divergence that follows must be
treated as a bug, not accepted as state).

## Alternatives considered

- **LuCI module (Lua/ucode):** native look, but bound to LuCI's release cycle
  and model; NetGrip wants its own product surface.
- **Node/nginx sidecar:** runtime dependency on a router is a support burden.
- **Binary copied by hand:** dies on every sysupgrade; only previews use it,
  never releases.

## Consequences

Releases must exercise the packaging path, not just the binary. Anything that
changes process flags needs a package release, not a binary update. The
binary size (~15 MB) is accepted: flash is not the binding constraint on the
target class, and gzip-served assets keep the wire footprint small.
