# Pala's Groupie Tracker

<p align="center">
  <strong>Go web app</strong> to browse artists, compare multiple data sources, and save favorites.
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white">
  <img alt="Tailwind CSS" src="https://img.shields.io/badge/Tailwind_CSS-3.4.18-38B2AC?logo=tailwindcss&logoColor=white">
  <img alt="PostgreSQL" src="https://img.shields.io/badge/PostgreSQL-required_for_auth-4169E1?logo=postgresql&logoColor=white">
  <img alt="License" src="https://img.shields.io/badge/License-ISC-111827">
</p>

<p align="center">
  <a href="#project-goal">Goal</a> •
  <a href="#run-locally">Run Locally</a> •
  <a href="#main-routes">Routes</a> •
  <a href="#features">Features</a> •
  <a href="#architecture">Architecture</a>
</p>

## Project Goal

This repository is a "Groupie Tracker" school project implemented as a small Go web app. It provides one UI to:

- browse a list of artists
- open an artist detail page with useful context (top tracks, latest releases, Wikipedia summary)
- switch data source on the fly to compare results

Supported sources via `?source=`:
`groupie` (default), `spotify`, `deezer`, `apple`.

## Run Locally

Prerequisites: Go, Node.js/npm. (PostgreSQL is required for accounts and favorites.)

1. Install frontend tooling (Tailwind CLI):
```bash
npm install
```

2. Compile Tailwind in watch mode (terminal 2):
```bash
npm run dev:css
```

3. Run the server:
```bash
go run ./cmd/server
```

Open `http://localhost:8080`.

### Configuration (.env)

The server attempts to load `.env` (optional). Common variables:

```bash
PORT=8080
BASE_PATH=/groupie-tracker
DATABASE_URL=postgres://...
LASTFM_API_KEY=...
SPOTIFY_CLIENT_ID=...
SPOTIFY_CLIENT_SECRET=...
```

Notes:
- `BASE_PATH` (or `X-Forwarded-Prefix`) is for hosting under a sub-path behind a reverse proxy.
- Without `DATABASE_URL`, auth and favorites are disabled.
- Last.fm is best-effort: without `LASTFM_API_KEY`, listener counts may be `0`.

## Main Routes

Routes are registered in `cmd/server/main.go`:

- `GET /`: home page (featured artists and source switcher).
- `GET /artists`: artists list (search/sort; filters in `groupie` mode).
- `GET /artists/ajax`: HTML partial used for live search/filtering updates.
- `GET /artists/suggest`: search suggestions (in `groupie` mode).
- `GET /artists/{id}`: artist detail page (behavior depends on source).
- `GET /favorites`: favorites page (requires login and DB).
- `POST /favorites/toggle`: add/remove a favorite (requires login and DB).
- `GET|POST /login`: login.
- `GET|POST /register`: create account.
- `POST /logout`: logout.
- `GET /static/*`: static assets (CSS, JS, vendor libraries).

## Features

- Multi-source browsing: Groupie, Spotify, Deezer, Apple (iTunes).
- Detail pages: tracks, latest releases, Wikipedia summary.
- Groupie mode: concert map (Leaflet) and geocoded locations.
- Live search and filters (year, first album date, members, location) in Groupie mode.
- Accounts, secure sessions, and persisted favorites (PostgreSQL).

## Architecture

```text
.
├── cmd/
│   └── server/
│       └── main.go                # HTTP mux + routes + static + boot
├── internal/
│   ├── api/                       # API clients: spotify/deezer/apple/lastfm/wiki
│   ├── geo/                       # Geocoding / location parsing (Groupie)
│   ├── handlers/                  # HTTP handlers (pages + actions)
│   └── store/                     # PostgreSQL: users, sessions, favorites
├── web/
│   ├── templates/                 # Go HTML templates (*.gohtml)
│   └── static/
│       ├── css/                   # tailwind.css (input) + style.css (output)
│       ├── js/                    # frontend scripts (filters, modals, map, theme)
│       └── vendor/leaflet/        # Leaflet local (css/js + images + LICENSE)
├── go.mod
└── package.json                   # Tailwind scripts
```

## Useful Commands

- Production/minified CSS: `npm run build:css`
- Go tests: `go test ./...`
