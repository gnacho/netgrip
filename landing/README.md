# NetGrip landing page

Static marketing site served at https://netgrip.cloudless.club from the web
server (`/opt/netgrip-web/public`). No build step: plain HTML + CSS + JS.

## Structure

- `index.html` - the whole page (single-file sections)
- `styles.css` - theme (light/dark via `data-theme`), canvas-based look
- `app.js` - ES/EN i18n dictionary (`data-i18n` attributes in the HTML),
  theme toggle, mobile nav, star count, reveal-on-scroll
- `announcements.json` - in-app announcement channel; the NetGrip panel
  fetches this file and shows matching entries. Schema:
  `id`, `urgency` ("info" | "warn"), `title`/`body`/`urlLabel` as
  `{ "es": ..., "en": ... }`, `url`, `starts`, `expires` (YYYY-MM-DD)
- `assets/` - screenshots (`.webp` served, `.png` sources)
- `og.png`, `robots.txt`, `sitemap.xml`

## Editing rules

- Both `es` and `en` strings always, in the same commit.
- No em dashes; plain hyphens.
- Voice: direct, honest, no hype (see the existing copy).
- Never put real IPs, SSIDs, hostnames or credentials in this folder.

## Deploy

The public site is served by Caddy in **CT 2111** (nodo2, `164.132.81.106`)
from `/opt/netgrip-web/public`, files owned by `1000:1000`. The older nginx
path on webs2 is still on disk but no longer answers for this domain.

1. Package only what changed, for example
   `tar czf /tmp/landing.tgz index.html app.js styles.css`.
2. `scp -P 22222` it to `nodo2.domatix.cloud`, then `pct push 2111` it into
   the container.
3. Inside the container: back up the files being replaced into
   `/root/landing-backup-<stamp>/`, extract over `/opt/netgrip-web/public`
   and `chown 1000:1000` the replaced files.
4. Bump the `?v=N` cache buster on `styles.css` and `app.js` in
   `index.html` whenever either changes, or browsers keep the old copy for
   an hour.
5. Verify from outside: the new `?v=N` is served, the title and the new
   section are in `curl https://netgrip.cloudless.club/`, and
   `announcements.json` still parses.
