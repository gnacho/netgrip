# Contributing

NetGrip runs on the router: a Go backend reading `/proc`, `/sys`, ubus and
UCI, and a React frontend. If you have an OpenWrt device and have hit a rough
edge, that is the most valuable contribution there is - the best fixes in this
repo started as "on my board this page is empty".

## Reporting bugs

One issue per bug, with the shape of the problem rather than the diagnosis:

- What the panel showed, what you expected, what the router actually has
  (a stripped `uci show <pkg>` or ubus dump is worth more than a paragraph).
- Board, OpenWrt version and how NetGrip is installed (release binary,
  package, manual build).

## Development

```bash
# Backend (needs Go)
go build ./...
go test ./...

# Frontend (builds into internal/server/dist, embedded into the binary)
cd app
npm ci
npm run build
```

## Ground rules for code

- **Reads are cheap, forks are not.** The panel is polled every few seconds
  on hardware with four cores. Prefer one parse of a whole config over a
  fork per option, and `/proc` or `/sys` over any subprocess.
- **A missing tool is an empty state, not an error.** Routers differ; a
  feature that does not apply must disappear quietly, not fail red.
- **Writes go through the executor** as declarative ops with a snapshot, a
  health check and a rollback. Never shell out a config change directly.
- **Honest fallbacks.** Resolve names from live state (the bridge the LAN
  sits on, the uplink that carries traffic) instead of hardcoding `br-lan`
  or `wan`; fall back to the conventional name only when nothing resolves.
- Parsers are pure functions with fixtures from real router output. Tests
  run without a router.

## Commits and pull requests

- Conventional commits (`feat:`, `fix:`, `refactor:`...), one logical change
  per commit, subject in imperative mood, in English.
- Code, comments and user-facing strings are in English and Spanish alike:
  every new key in `app/src/locales/en.ts` needs its entry in `es.ts`.
- PRs target `main` and merge as squash.
- Reproducible before/after beats a long description: if you measured
  something (CPU, latency, a failing check), include the number.
- No AI co-author trailers. History is attributable to people: CI rejects
  any commit carrying a `Co-Authored-By:` trailer from an AI assistant, and
  merge bodies are written clean.

## Translations

The UI is bilingual English/Spanish through the locale files in
`app/src/locales/`. Add your language by copying either file and wiring it
into the language switcher; keep keys identical across locales.

## License

AGPL-3.0-only. See [LICENSE](LICENSE). Contributions are accepted under the
same terms.
