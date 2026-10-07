# Phase 2: uci exec seam

## Objective

The uci reads on the clients path (`uciShowCached`, `uciGet`,
`uciSectionExists`) execute through one overridable package-level
function, so tests can answer uci queries without a router. This also
gives the rest of the codebase a single adoption point for the uci seam.

## Scope

In: introduce the exec var (e.g. `uciExec = func(args ...string) ([]byte,
error)` wrapping `exec.Command("uci", args...).Output()`), route
`uciShowCached`, `uciGet`, `uciSectionExists` through it, and a
`SetUciExecForTest`-style helper.

Out: migrating `exec.Command("uci", ...)` call sites outside the three
helpers (they keep working; adoption is opportunistic).

## Acceptance

- Full suite green; behavior on a device unchanged.
- A unit test shows a faked `uciExec` answering `uci show dhcp` feeds
  `dhcpReservations()` without forking uci.

## Dependencies

None.
