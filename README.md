# Something

Something is a local-first Wails desktop dashboard with a plain HTML/CSS/JS
frontend and a Go backend. It combines:

- tasks with tags, difficulty, hard-task subtasks, a calendar, and dashboard summaries;
- Telegram and YouTube channel favorites and update scanning;
- favorite categories with source-scoped renaming;
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
facade. Database lifecycle and cache maintenance are owned by an unbound
service, while loopback listener lifecycle is owned by dedicated controllers.

The REST API listens on `127.0.0.1:8080` and the MCP server listens on
`127.0.0.1:8081`. Both addresses are restricted to loopback. The REST listener
is bound first and its actual address is injected into MCP; MCP is not started
when that listener cannot be acquired. Set `SOMETHING_MCP_TOKEN` to require a
bearer token for MCP calls.

MCP is started automatically with the desktop app. The MCP Server view shows
its current runtime status and can stop or restart it without stopping the REST
API or other desktop features. This switch applies only to the current app
session, so the next launch attempts to start MCP again. An MCP-only bind
failure is reported by that view and does not make the rest of the app
unavailable.

Timeouts form an explicit outer-to-inner hierarchy: provider requests (25s),
REST operations (55s), REST writes (60s), CLI requests (65s), MCP operations
(70s), and MCP writes (75s). Caller and application cancellation reach remote
provider requests.

## Task search and filters

The desktop task view searches titles, descriptions, tags, and subtasks as you
type. It can combine that search with an exact due date, priority, difficulty,
and one or more exact tags. Multiple tag filters use AND semantics, and the
special difficulty value `unset` selects tasks without a difficulty.

The same filters are available through the REST API:

```text
GET /api/v1/tasks?q=release&date=2026-08-01&priority=high&difficulty=hard&tag=backend&tag=urgent
```

The CLI exposes `--search`, `--date`, `--priority`, `--difficulty`, and
repeatable `--tag` flags on `something tasks list`. The MCP `list_tasks` tool
accepts the equivalent `query`, `date`, `priority`, `difficulty`, and `tags`
inputs.

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

Existing tasks are migrated without invented metadata: difficulty is returned
as an empty string and tags/subtasks as empty arrays until the task is edited.
Subtasks are accepted only for tasks whose difficulty is `hard`.

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
