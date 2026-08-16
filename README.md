# Something

Something is a local-first desktop dashboard built with Go and Wails. It brings
tasks, favorite-channel updates, weather, currency rates, file browsing, and
desktop customization into one application. Persistent application data stays
in a local SQLite database; live information is fetched from public providers
when it is needed.

The desktop application also exposes the same core features through a
loopback-only REST API, a command-line client, and a Model Context Protocol
(MCP) server.

## Features

- **Dashboard and calendar** — see today's tasks, this week's tasks, calendar
  activity, and new posts from favorite channels.
- **Task management** — create, edit, complete, search, and filter tasks by due
  date, priority, difficulty, tags, and subtasks. Subtasks are available for
  hard tasks.
- **Notes** — capture plain-text notes, search them, pin important notes, and
  archive older notes. Revision-aware autosave protects newer CLI or MCP
  changes from being silently overwritten by a stale desktop editor.
- **Bookmarks** — maintain a local read-later collection of HTTP and HTTPS
  links with descriptions, tags, search, and read or unread state.
- **Telegram and YouTube** — browse public channel updates, save favorite
  channels, organize them into source-specific categories, and scan for new
  content.
- **Weather** — use browser-provided coordinates, search by city, and display a
  cached forecast while fresh data is loaded.
- **Currency rates** — retrieve current exchange rates and save favorite
  currency pairs.
- **File explorer** — browse standard locations or a folder selected for the
  current session, search directory contents, and open files. On Windows,
  files can also be moved to the Recycle Bin.
- **Wallpapers** — choose a bundled wallpaper or import a JPEG, PNG, or WebP
  image up to 20 MB.
- **Multiple interfaces** — use the desktop UI, REST API, CLI, or MCP tools
  against the same running backend and local data.

Live weather, currency, Telegram, and YouTube features require an internet
connection. Tasks, notes, bookmarks, and preference data remain available
locally.

## Technology

| Area | Implementation |
| --- | --- |
| Desktop | Wails v2 with a Go backend |
| Frontend | Framework-free HTML, CSS, and JavaScript |
| Storage | SQLite through the pure-Go `modernc.org/sqlite` driver |
| REST API | Go `net/http` on loopback |
| CLI | Cobra and Viper |
| MCP | Official Go MCP SDK over Streamable HTTP |
| Live providers | Frankfurter, Open-Meteo, Telegram public pages, and YouTube public pages/RSS |

The frontend in `frontend/dist` is embedded directly into the executable.
There is no Node.js dependency, package installation, bundler, or separate
frontend build step.

## Getting started

### Prerequisites

- Go 1.25 or newer
- Wails CLI v2.12.0
- The native platform dependencies required by Wails for your operating system

Install the matching Wails CLI and check the local environment:

```text
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
wails doctor
```

### Run in development

From the repository root:

```text
wails dev
```

This starts the desktop application and, after the backend is ready, binds the
REST API to `127.0.0.1:8080`. The MCP server remains off until it is enabled in
Settings, when it binds to `127.0.0.1:8081`.

### Build the desktop application

```text
wails build
```

The configured executable name is `currency-wails`; Wails writes production
artifacts under `build/bin`.

## Command-line interface

The CLI is a client for the running desktop application's REST API, so launch
the desktop application before using it. During development, commands can be
run without creating a separate CLI binary:

```text
go run ./cli/something health
go run ./cli/something tasks list --date 2026-08-01 --priority high
go run ./cli/something tasks create --title "Prepare release" --due-date 2026-08-01 --difficulty hard --subtask "Write notes"
go run ./cli/something notes create --title "Release notes" --body "Document the new endpoints"
go run ./cli/something bookmarks create --url "https://go.dev/doc/" --title "Go documentation" --tag reference
go run ./cli/something currencies rate --base USD --target EUR --output json
```

The top-level command groups are:

| Command | Purpose |
| --- | --- |
| `health` | Check whether the desktop backend is ready |
| `news` | Read stored favorite-channel news or start an update scan |
| `posts` | Read Telegram or YouTube channel posts |
| `favorites` | Manage Telegram and YouTube favorites and assignments |
| `favorite-categories` | List, create, and rename source-specific categories |
| `tasks` | List, create, update, toggle, and delete tasks and subtasks |
| `notes` | List, create, edit, pin, archive, restore, and delete notes |
| `bookmarks` | Search and manage tagged read-later bookmarks |
| `weather` | Read city or saved-location weather and refresh the cache |
| `currencies` | Read rates and manage favorite currency pairs |

Use `go run ./cli/something <command> --help` for the complete flags and
subcommands. Values required by a mutation are prompted for in an interactive
terminal when their flags are omitted.

### CLI configuration

| Flag | Environment variable | Default |
| --- | --- | --- |
| `--api-url` | `SOMETHING_API_URL` | `http://127.0.0.1:8080` |
| `--output`, `-o` | `SOMETHING_OUTPUT` | `table` |

Output can be `table` for people or `json` for scripts.

Task search checks titles, descriptions, tags, and subtasks. The list command
can combine `--search`, `--date`, `--priority`, `--difficulty`, and repeatable
`--tag` filters. Repeated tags use AND semantics, and the special difficulty
value `unset` selects tasks without a difficulty:

```text
go run ./cli/something tasks list --search release --difficulty hard --tag backend --tag urgent
```

To build a standalone CLI binary:

```text
# Windows
go build -o build/bin/something.exe ./cli/something

# macOS or Linux
go build -o build/bin/something ./cli/something
```

## REST API

The REST API is available only while the desktop application is running.
Its base URL is:

```text
http://127.0.0.1:8080/api/v1
```

Resource groups mirror the desktop features:

| Resource | Main endpoints |
| --- | --- |
| Health | `GET /health` |
| News | `/news`, `/news/refresh`, `/news/state`, `/news/windows` |
| Posts | `/posts/telegram/{channel}`, `/posts/youtube/{channel}`, `/posts/favorites/{source}` |
| Favorites | `/favorites/{source}`, `/favorites/{source}/{channel}`, `/favorite-categories` |
| Tasks | `/tasks`, `/tasks/today`, `/tasks/{id}` |
| Notes | `/notes`, `/notes/{id}`, note pin and archive state routes |
| Bookmarks | `/bookmarks`, `/bookmarks/tags`, `/bookmarks/{id}` |
| Weather | `/weather`, `/weather/stored`, `/weather/stored/refresh` |
| Currencies | `/currencies`, `/currencies/rate`, `/currencies/favorites` |

For example:

```text
curl http://127.0.0.1:8080/api/v1/health
curl "http://127.0.0.1:8080/api/v1/tasks?q=release&priority=high&tag=backend&tag=urgent"
```

Successful responses are JSON. Errors use a JSON object with a stable error
code and a human-readable message. Provider-backed post lists return at most
20 items per request.

Telegram post routes support the non-negative `before` cursor. YouTube's
public RSS feed exposes only recent entries, so YouTube pagination is not
supported; a REST request containing `before` returns
`youtube_pagination_unsupported`. The CLI and MCP interfaces therefore expose
pagination only for Telegram.

## MCP server

Something includes a stateless Streamable HTTP MCP server that can be enabled
from Settings while the desktop app is running:

```text
http://127.0.0.1:8081/mcp
```

Configure an MCP client with that URL while Something is running. The tools
cover backend health, news, posts, favorites and categories, tasks, notes,
bookmarks, weather, and currencies. Read and mutation tools operate on the
same data shown in the desktop UI. Bookmark tools store links but never fetch
arbitrary bookmark URLs or open a browser window.

The Settings view shows the MCP listener state and can start or stop it without
stopping the REST API. MCP starts off on every app launch; enabling it applies
only to the current app session. An MCP port conflict is reported in Settings
but does not prevent the rest of the application from starting.

### Optional MCP authentication

Set `SOMETHING_MCP_TOKEN` in the environment that launches the desktop app to
require bearer-token authentication. MCP clients must then send:

```text
Authorization: Bearer <token>
```

If the variable is unset or blank, no bearer token is required. The REST API
does not use this token.

## Data storage

Something creates its application data beneath the operating system's user
configuration directory:

```text
<user-config-directory>/currency-wails/
├── database.db
├── user-wallpapers/
└── icons/
```

The legacy `currency-wails` data-directory name is retained so existing local
databases and imported assets remain available after the Go module rename.

The SQLite database stores tasks and metadata, notes, tagged read-later
bookmarks, favorites and categories, currency pairs, weather location/cache
data, news scan state, saved desktop application paths, and application
preferences. Imported wallpaper files are copied into the application-owned
`user-wallpapers` directory, and custom application icons are copied into the
sibling `icons` directory.

On upgrade, if the application-data database does not yet exist, Something
checks for a legacy `database.db` beside the installed executable. A valid
legacy database is copied, migrated, and verified without overwriting an
existing destination. The original file is retained.

Existing tasks are migrated without invented metadata: difficulty remains
unset and tags/subtasks remain empty until edited. Subtasks are accepted only
when a task's difficulty is `hard`.

## Security and operational behavior

- REST and MCP listeners are forced to IPv4 loopback addresses. They are not
  exposed to the local network.
- The REST listener is acquired first; MCP is configured with its actual URL
  and does not start if the REST API is unavailable.
- The REST API has no application-level authentication. Any process running as
  the local user can call it while Something is open.
- MCP accepts request bodies up to 1 MiB and can be protected with
  `SOMETHING_MCP_TOKEN`.
- The file explorer exposes only registered roots and rejects path traversal
  and symlink escapes. User-selected roots last for the current app session.
- Opening files and moving them to the Recycle Bin are Windows-only actions;
  directory browsing remains available on other Wails-supported platforms.
- External provider requests are HTTPS-only, response-size bounded, and
  cancellation-aware. Telegram, YouTube, and handle lookups use bounded
  five-minute in-memory caches.
- Request deadlines are layered so outer interfaces outlive the operations
  they call: providers 25s, REST 55s, REST writes 60s, CLI 65s, MCP 70s, and
  MCP writes 75s.

## Architecture

All interfaces share one backend service and SQLite connection:

```text
Desktop UI ── Wails bindings ──┐
                              │
CLI ───────── REST API ────────┼── Backend service ── SQLite / live providers
                              │
MCP ── in-process CLI ── REST ┘
```

Only the narrow `backend.App` facade from `backend/app` is bound to the Wails
frontend. The unbound `service.Service` in `backend/service` owns domain
operations and lifecycle state. Dedicated packages own persistence, native
file browsing, and application launching. The REST and MCP controllers shut
down before the shared database closes.

## Project layout

```text
main.go                  Wails startup and listener orchestration
api/                     Loopback REST API, handlers, and responses
backend/app/             Wails facade and native picker bridges
backend/service/         Domain operations, providers, caches, and shared lifecycle
backend/storage/         SQLite setup, paths, and schema migrations
backend/fileexplorer/    Sandboxed filesystem browsing and OS shell integration
backend/launcher/        OS-specific desktop application launching
cli/internal/apiclient/  Typed client for the desktop REST API
cli/something/           Cobra command-line application
frontend/dist/           Embedded framework-free frontend and bundled assets
frontend/wailsjs/        Generated Wails bindings (ignored by Git)
internal/policy/         Shared timeout policy
mcp-server/              MCP listener, schemas, and read/write tools
build/                   Wails platform metadata and generated build artifacts
wails.json               Wails project configuration
```

When changing the UI, edit `frontend/dist` directly. Wails regenerates
`frontend/wailsjs` as needed, and generated bindings are intentionally ignored
by Git.
