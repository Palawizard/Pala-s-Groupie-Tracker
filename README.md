# Pala's Groupie Tracker

Groupie Tracker is a small server-rendered Go web app built for the Groupie Tracker school project.
It lets you browse artists and open a detail page with extra context (top tracks, recent releases, and a short Wikipedia summary).
The UI can switch between multiple data sources so you can compare results:

- `groupie` (default): the original Groupie Trackers dataset
- `spotify`: Spotify Search + artist details (requires API credentials)
- `deezer`: Deezer Search + artist details (no key required)
- `apple`: Apple iTunes Search + artist details (no key required)

In Groupie mode, artist detail pages also include a Leaflet map with concert locations (geocoded via Open-Meteo and OpenStreetMap Nominatim).

## Features

- Artists list + detail pages.
- Source toggle: `groupie`, `spotify`, `deezer`, `apple`.
- Groupie-only filters: creation year, members count, first album date range, concert location search.
- Live updates on the artists page via `/artists/ajax` (HTML partial render) and `/artists/suggest` (JSON suggestions).
- Groupie-only concerts map (Leaflet) with best-effort geocoding + in-memory caching.

## Tech Stack

- Backend: Go (see `go.mod`)
- Templating: Go HTML templates in `web/templates/`
- Styling: Tailwind CSS compiled to `web/static/css/style.css`
- Client-side: small vanilla JS helpers in `web/static/js/` (live filters, source toggle, embed modals)

## Data Sources

- Groupie dataset: public API at `https://groupietrackers.herokuapp.com/api/` (artists + relations).
- Spotify: requires `SPOTIFY_CLIENT_ID` + `SPOTIFY_CLIENT_SECRET` (Client Credentials flow).
- Deezer: public API (no key).
- Apple: iTunes Search API (no key).
- Wikipedia: best-effort summary on detail pages.
- Last.fm: optional listener counts for sorting/enrichment (set `LASTFM_API_KEY`).

## Architecture

The app is a classic server-rendered Go web app, with a small amount of JavaScript for interactivity on the Artists page and the map.

```text
cmd/
  server/              HTTP entrypoint + route wiring
internal/
  api/                 External API clients + caching (Groupie, Spotify, Deezer, Apple, Last.fm, Wikipedia)
  geo/                 Geocoding (Open-Meteo -> Nominatim fallback) + in-memory cache
  handlers/            Pages/endpoints (home, artists, detail, ajax, suggest)
web/
  templates/            Go HTML templates (*.gohtml)
  static/               Static assets served at /static/
    css/                Stylesheets (Tailwind input + compiled output)
    js/                 Vanilla JS helpers (UX, map, modals)
```

Request flow (simplified):
- Browser -> `GET /artists` -> `internal/handlers` -> `internal/api` -> render `web/templates/*`
- `web/static/js/artists.js` updates filters live via `GET /artists/ajax` and loads suggestions via `GET /artists/suggest`
- Groupie artist details embed a Leaflet map fed by JSON produced server-side and optional browser geolocation

## Routes

- `GET /`: home page
- `GET /artists`: artists page (full render)
- `GET /artists/ajax`: artists list partial (used by live filters in Groupie mode)
- `GET /artists/suggest`: JSON suggestions (Groupie mode only, returns `[]` for other sources)
- `GET /artists/{id}`:
  - Groupie: numeric IDs from the dataset
  - Spotify/Deezer/Apple: provider IDs (validated/sanitized server-side)

## Quick Start (Local Development)

Prereqs: Go, Node.js/npm.

1. Install frontend tooling:
   ```powershell
   npm install
   ```
2. Start Tailwind in watch mode (separate terminal):
   ```powershell
   npm run dev:css
   ```
3. Run the server:
   ```powershell
   go run ./cmd/server
   ```
Open `http://localhost:8080`.

## Configuration (.env)

The server tries to load a local `.env` (optional) via `godotenv`. Common variables:

```bash
PORT=8080
BASE_PATH=/groupie-tracker
LASTFM_API_KEY=...
SPOTIFY_CLIENT_ID=...
SPOTIFY_CLIENT_SECRET=...
```

Notes:
- `PORT` defaults to `8080` in local dev.
- `BASE_PATH` (or the `X-Forwarded-Prefix` header) is for hosting under a subpath behind a reverse proxy.
- Last.fm is best-effort. Without `LASTFM_API_KEY`, listener counts fall back to `0`.
- Spotify mode requires `SPOTIFY_CLIENT_ID` + `SPOTIFY_CLIENT_SECRET`. If missing, Spotify requests will fail.

## CSS Builds

- Input: `web/static/css/tailwind.css`
- Output: `web/static/css/style.css`
- Watch (dev): `npm run dev:css`
- Production/minified: `npm run build:css`

## Usage Tips

- Switch source with the header toggle or with `?source=groupie|spotify|deezer|apple`.
- The artists page updates live via `/artists/ajax` as you type or move sliders (Groupie mode).
- Groupie filters: creation year, first album date, members count, and concert location.
- Groupie search: artist/group name and member names (case-insensitive), plus typed suggestions.

## Hosting Under a Subpath (Reverse Proxy)

If deploying behind a reverse proxy at a subpath (for example `https://example.com/groupie-tracker/`), set either:

- `BASE_PATH=/groupie-tracker` (simple option), or
- `X-Forwarded-Prefix: /groupie-tracker` (preferred when your gateway can inject headers).

The templates use this base path when building links and static asset URLs.

## Contributing

- Keep changes focused and include a short description of what you changed and how to test it (commands + URL/route).
- For UI/template/CSS changes, include screenshots and ensure the Tailwind output is up to date:
  - `npm run build:css` (or keep `npm run dev:css` running while developing).
