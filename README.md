# Currency Exchange Rates — Wails App

A desktop currency-rate lookup app: Go backend (Wails v2) + plain HTML/CSS/JS frontend.
No JS framework or bundler required — the frontend is served straight from `frontend/dist`.

## Prerequisites

- Go 1.21+
- Wails v2 CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Platform build tools (see https://wails.io/docs/gettingstarted/installation for your OS)

## Run in dev mode

```bash
wails dev
```

This launches the app with hot reload. Since there's no frontend build step,
editing files in `frontend/dist/` and refreshing is enough to see changes.

## Build a production binary

```bash
wails build
```

The compiled app appears in `build/bin/`.

## Project layout

```
currency-wails/
├── main.go              # Wails bootstrap, window config, asset embedding
├── app.go                # App struct — Go methods bound to the frontend:
│                          #   GetRate(base, target)   -> single rate lookup
│                          #   GetAllRates(base)        -> all rates for a base currency
├── wails.json             # Wails project config (no npm build step configured)
├── go.mod
└── frontend/
    └── dist/
        ├── index.html      # UI shell (tabs: Single Rate / All Rates)
        ├── style.css       # Dark-themed styling
        └── main.js         # Calls window.go.main.App.* (Wails bindings)
```

## How the frontend talks to Go

Wails auto-generates a `window.go.main.App` object in the frontend at runtime,
with one method per bound Go method on `App`. `main.js` calls these directly:

```js
const result = await window.go.main.App.GetRate("USD", "EUR");
const all    = await window.go.main.App.GetAllRates("USD");
```

For convenience, `main.js` also has a fallback: if opened in a plain browser
(no Wails runtime present), it calls the Frankfurter API directly via `fetch`.
This is only for quick UI iteration outside the Wails shell — the real app
always uses the Go backend.

## Note on RUB (Russian Ruble)

Frankfurter sources data exclusively from the European Central Bank, which
does not currently publish RUB rates. Requesting `RUB` as a target currency
will correctly show a "not found" message in the UI rather than crashing —
this isn't a bug, the currency just isn't in the ECB dataset Frankfurter uses.
