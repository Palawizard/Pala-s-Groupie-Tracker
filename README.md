# Pala's Groupie Tracker

Groupie Tracker is a small Go web app built for the Groupie Tracker school project. It lets you browse artists and open a detail page with useful context (top tracks, recent releases, and a short Wikipedia summary). The UI can switch between multiple data sources so you can compare results:

- `groupie` (default): the original Groupie Trackers dataset
- `spotify`: Spotify Search + artist details (requires API credentials)
- `deezer`: Deezer Search + artist details (no key required)
- `apple`: Apple iTunes Search + artist details (no key required)

In Groupie mode, artist detail pages also include a Leaflet map with concert locations (geocoded via Open-Meteo and OpenStreetMap Nominatim).

## Tech Stack

- Backend: Go (see `go.mod`)
- Templating: Go HTML templates in `web/templates/`
- Styling: Tailwind CSS compiled to `web/static/css/style.css`
- Client-side: small vanilla JS helpers in `web/static/js/` (live filters, source toggle, embed modals)

## Quick Start (Local)

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

The server tries to load a local `.env` (optional). Common variables:

```bash
PORT=8080
BASE_PATH=/groupie-tracker
LASTFM_API_KEY=...
SPOTIFY_CLIENT_ID=...
SPOTIFY_CLIENT_SECRET=...
```

Notes:
- `BASE_PATH` (or the `X-Forwarded-Prefix` header) is for hosting under a subpath behind a reverse proxy.
- Last.fm is best-effort; without `LASTFM_API_KEY`, listener counts may show as 0 and listener-based sorting will be less meaningful.

## CSS Builds

- Watch: `npm run dev:css`
- Production/minified: `npm run build:css`

## Usage Tips

- Switch source with the header toggle or with `?source=groupie|spotify|deezer|apple`.
- The artists page updates live via `/artists/ajax` as you type or move sliders (Groupie mode).
- Groupie filters: creation year, first album date, members count, and concert location.
- Groupie search: artist/group name and member names (case-insensitive), plus typed suggestions.
