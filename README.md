# Something

Something is a local-first Wails desktop dashboard with a plain HTML/CSS/JS
frontend and a Go backend. It combines:

- tasks, a calendar, and dashboard summaries;
- Telegram and YouTube channel favorites and update scanning;
- favorite categories;
- weather forecasts;
- currency rates and saved currency pairs;
- a loopback REST API, CLI, and MCP server.

The frontend lives in `frontend/dist` and has no npm framework, bundler, or
frontend build step.

## Desktop development

Prerequisites are Go, Wails v2, and the platform tools required by Wails.

```text
wails dev
```

Production builds normally use `wails build` and are written to `build/bin`.

## Interfaces and security

The Wails binding remains `window.go.backend.App`, but it is a narrow UI
facade. Database lifecycle, startup/shutdown, listener management, and cache
maintenance are owned by an unbound service.

The REST API listens on `127.0.0.1:8080` and the MCP server listens on
`127.0.0.1:8081`. Both addresses are restricted to loopback. The REST listener
is bound first and its actual address is injected into MCP; MCP is not started
when that listener cannot be acquired. Set `SOMETHING_MCP_TOKEN` to require a
bearer token for MCP calls.

Timeouts form an explicit outer-to-inner hierarchy: provider requests (25s),
REST operations (55s), REST writes (60s), CLI requests (65s), MCP operations
(70s), and MCP writes (75s). Caller and application cancellation reach remote
provider requests.

## YouTube pagination

YouTube's public RSS feed reliably provides only its latest entries. Therefore:

- normal YouTube retrieval is supported;
- YouTube CLI and MCP commands do not expose a `before` cursor;
- REST requests that supply `before` to a YouTube posts route receive
  `youtube_pagination_unsupported`;
- Telegram pagination is unchanged.

## Storage and browser fallback

SQLite data is stored in the per-user configuration directory under
`currency-wails/database.db`. A legacy database is considered only beside the
installed executable, never in the process working directory. It is validated,
copied to a temporary file, migrated, checked, and installed without
overwriting an existing destination. The legacy source is retained.

Favorite/category localStorage fallback exists only when the static frontend is
opened outside Wails. A present Wails backend error is reported to the user and
does not silently create a second localStorage copy.

## Project layout

```text
main.go                 Wails and listener orchestration
backend/                domain, SQLite, providers, facade, and caches
api/                    loopback REST API
cli/                    Something CLI and typed API client
mcp-server/             loopback MCP server and tools
frontend/dist/           embedded framework-free application
frontend/wailsjs/        generated Wails bindings and models
```
