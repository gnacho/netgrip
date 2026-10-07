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

Manual for now. The site is a plain nginx root on the web server:

1. `tar -czf landing.tgz -C landing .`
2. Copy to the server (via the npm2 -> webs2 hop) and extract into
   `/opt/netgrip-web/public`, keeping a backup of the files you replace
   (`index.html`, `app.js`, `announcements.json` change most often).
3. Verify: `curl -s https://netgrip.cloudless.club/ | grep <new-key>` and
   `curl -s https://netgrip.cloudless.club/announcements.json` parses.
