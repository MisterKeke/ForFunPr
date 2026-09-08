# Repository Instructions

These instructions apply to the entire repository. Treat the closest existing
implementation and its tests as the source of truth for feature-specific
behavior; update this file when a new cross-cutting convention is introduced.

## Project Shape

Something is a local-first desktop dashboard written in Go with Wails v2. One
`backend/service.Service` instance and its SQLite connection serve four
surfaces:

- Wails desktop bindings through the narrow facade in `backend/app`
- Loopback REST API in `api` at `127.0.0.1:8080/api/v1`
- Cobra CLI in `cli/something`, which calls the REST API
- MCP server in `mcp-server`, whose tools run fixed CLI commands in-process

The dependency direction is intentional:

```text
frontend -> backend/app -> backend/service -> backend/storage/providers
CLI -> cli/internal/apiclient -> REST -> backend/service
MCP -> in-process CLI -> REST -> backend/service
```

Do not bypass these paths by giving CLI or MCP direct database/service access,
starting a second service, or duplicating domain rules in an adapter.

The frontend is framework-free HTML, CSS, and ES modules in `frontend/dist`.
Those files are embedded directly by `main.go`; there is no Node dependency,
bundler, package-install, or frontend build step. SQLite uses the pure-Go
`modernc.org/sqlite` driver. Schema ownership is in `backend/storage`; service
lifecycle and domain logic belong in `backend/service`.

## Before Changing Code

- Inspect the implementation, adjacent tests, `README.md`, `go.mod`, and
  `wails.json` relevant to the change. Do not assume framework or dependency
  versions from memory.
- Check `git status` and preserve unrelated user changes. Keep edits scoped to
  the requested work.
- For a user-visible domain feature, trace the whole contract and decide which
  of Wails, REST, the typed CLI client, Cobra commands, MCP schemas/tools,
  frontend code, tests, and README need corresponding changes.
- Preserve public JSON names and shapes unless the task explicitly calls for a
  contract change. JSON field names are snake_case.

## Backend And Lifecycle

- Put validation, normalization, persistence, and domain behavior in
  `backend/service`. Keep `backend/app` methods as thin Wails/native-operation
  wrappers.
- Add context-aware service methods for REST, CLI-driven, MCP, provider, and
  asynchronous work. Preserve a legacy non-context method as a wrapper when an
  existing Wails caller or public contract still needs it.
- Wails operations should enter through `App.begin()` (or the established
  equivalent), defer the returned completion function, and pass the operation
  context down. REST operations are guarded by the router middleware and must
  use `r.Context()`.
- Respect startup and shutdown ownership in `main.go`: native services start
  only after service startup succeeds; MCP stops before REST; REST stops before
  the shared database closes. Long-running work must honor cancellation and
  must not outlive service shutdown.
- Use the typed domain errors in `backend/service/errors.go` for caller-caused,
  not-found, conflict, and stale-revision cases. Adapters should translate them
  into safe, stable errors instead of leaking SQLite, provider, filesystem, or
  executable details.
- When a REST mutation changes data displayed by the open desktop UI, invoke
  the established `Emit...Changed` service hook after the mutation succeeds.
  Direct Wails calls generally consume their returned value and should not
  emit a duplicate event.

## Storage And Data Integrity

- Add schema changes as new, monotonically numbered migrations in
  `backend/storage/migrations.go`. Never rewrite, renumber, or reorder a
  migration that may already have been applied.
- Keep migrations transactional and safe for older local databases. Make
  schema operations idempotent where SQLite requires it and reuse helpers such
  as `addColumnIfMissing`.
- SQLite is configured as a single connection with foreign keys enabled. Avoid
  patterns that hold rows open while issuing another query on the same
  connection. Keep relation cleanup and multi-table writes in one transaction.
- Use parameterized SQL. Preserve deterministic ordering and return initialized
  empty slices where the existing JSON contract expects `[]` rather than
  `null`.
- Mutations that can race with the UI must preserve or add optimistic
  concurrency using the established `revision` / `expected_revision` compare-
  and-update pattern. A stale write must not partially modify related rows.
- Use `storage.OpenInMemory` for service tests and register cleanup for the
  service/database.
- Do not expose application database paths, executable paths, unrestricted
  filesystem paths, clipboard contents, or screenshot internals through REST,
  CLI, or MCP unless the existing feature contract explicitly permits it.

## REST API

- Register routes centrally in `api/router.go` and keep handlers as adapters
  over context-aware service methods.
- Every non-health handler must remain protected by the operation middleware
  and call `backendReady`. JSON bodies must go through `decodeJSONBody`, which
  enforces the content type, 1 MiB limit, one-object rule, and rejection of
  unknown fields.
- Validate path and query syntax at the HTTP boundary; leave domain validation
  in the service. Map domain errors to stable status/code pairs.
- Use `writeJSON` for JSON successes, `writeError` for the standard error
  envelope, and `writeNoContent` only when the existing contract calls for it.
  Never return raw internal error text to clients.
- Keep the listener loopback-only and retain request deadlines. State-changing
  localhost routes should require JSON where established to avoid simple
  cross-origin form mutations.
- Add or update router/handler tests when changing routes, validation, status
  codes, error codes, response fields, pagination, or readiness behavior.

## CLI And MCP

- Add REST calls to `cli/internal/apiclient`; Cobra commands in
  `cli/something/cmd` should use that client instead of implementing HTTP or
  domain behavior themselves.
- Keep `--output json` stable, complete, and free of explanatory stdout so it
  remains machine-decodable. Human table output should stay compact. Errors and
  diagnostics belong on stderr.
- Programmatic execution uses a fresh command graph through `cmd.ExecuteArgs`.
  Interactive CLI use may prompt where already supported, but MCP paths must
  always supply required flags and must never block for input.
- MCP tools need explicit input schemas in `mcp-server/schemas`, registration
  through `tools.AddTool`, accurate read/write, idempotence, destructive, and
  open-world annotations, and schema-helper validation before execution.
- Construct a fixed CLI argument slice and invoke `tools.Run` or
  `tools.Execute`; never pass model input to a shell or let callers choose an
  arbitrary command. Keep MCP output structs aligned exactly with CLI JSON.
- Actions with external side effects, such as launching saved applications or
  starting setups, must retain explicit confirmation at REST, CLI, and MCP
  boundaries. Launching remains ID-based; never expose or accept arbitrary
  executable paths on those surfaces.
- Update CLI contract tests and MCP schema/registration/tool tests whenever a
  command, flag, JSON shape, schema, annotation, or tool registration changes.

## Frontend And Native Boundaries

- Edit source files in `frontend/dist` directly. `frontend/dist/js/app.js` is
  the module entry point, `frontend/dist/js/api.js` owns Wails/API wrappers,
  and `frontend/dist/style.css` imports feature styles.
- Prefer existing DOM, state, event, naming, and CSS patterns. Do not add a
  framework, Node tooling, inline duplicate API calls, or a new build step
  unless explicitly requested.
- Prefer Wails bindings in frontend wrappers. Preserve an existing browser
  fallback only when it is safe and intentional; new native or privacy-
  sensitive operations should fail with a clear desktop-only message outside
  Wails.
- Clipboard, screenshots/OCR, file browsing, native pickers, app launching,
  running-window control, local asset import, and arbitrary website fetching
  stay within the Wails boundary unless the task explicitly changes the threat
  model.
- Provider HTTP work belongs in the service and must use the existing bounded,
  cancellation-aware HTTP clients. Preserve HTTPS/redirect/response-size rules.
  Website Search must retain SSRF defenses for initial URLs, DNS results,
  redirects, and browser-fallback requests.
- Native features require both Windows and non-Windows implementations or
  stubs using the existing build-tag pattern. Unsupported platforms should
  report a capability cleanly rather than fail to compile or panic.
- Do not hand-edit or commit `frontend/wailsjs`; Wails regenerates it. Do not
  treat all of `build/` as generated: platform metadata there is source, while
  `build/bin` and generated platform output are ignored artifacts.

## Testing And Completion

Write tests at the lowest layer that owns the behavior, plus contract tests for
every affected adapter. Prefer deterministic clocks/providers and in-memory
storage over sleeps, the user's persistent database, or live network calls.

Run focused tests first:

```text
go test ./backend/service
go test ./backend/storage
go test ./api
go test ./cli/...
go test ./mcp-server/...
```

Before handing off a cross-cutting or shared-contract change, match CI as far as
the environment permits:

```text
gofmt -w <edited-go-files>
go vet ./...
go test ./...
```

Run `wails build` when changing `main.go`, Wails bindings, embedded assets,
startup/shutdown behavior, native integration, or release/build configuration.
For frontend-only work with no automated browser suite, perform a targeted
desktop smoke check when possible and state what was or was not verified.

Do not weaken or delete a failing test merely to make the suite pass. Update
tests and `README.md` when an intentional public contract or operational rule
changes.

## Development Commands

```text
wails dev
wails build
go run ./cli/something health
go run ./cli/something <command> --output json
go build -o build/bin/something.exe ./cli/something
```

The desktop app must be running before the CLI can reach the REST API. The MCP
server binds to `127.0.0.1:8081` only after it is enabled in Settings. Go and
Wails versions are pinned by `go.mod`, `wails.json`, and CI; keep those sources
consistent when upgrading tooling.

## Task Domain Invariants

- Todo dates use `YYYY-MM-DD`; empty dates are stored as SQL NULL and returned
  as an empty string. Date-relative views use the service's injected local
  clock/time zone so tests stay deterministic.
- Todo priority is `low`, `medium`, or `high`; empty input defaults to
  `medium`.
- Todo difficulty is empty, `easy`, `medium`, or `hard`; the list filter also
  accepts `unset`.
- Todo tags are trimmed, case-insensitively de-duplicated, limited to 32 tags,
  and each tag is limited to 64 runes.
- Subtasks are valid only for hard tasks. Updating without a `Subtasks` pointer
  preserves existing subtasks; changing away from hard requires clearing them.
- Todo search covers title, description, tags, and subtasks. Repeated tags use
  AND semantics.

## Style

- Follow existing Go naming and error-wrapping patterns; keep comments useful
  and sparse.
- Prefer small adapters and cohesive service methods over parallel
  implementations of the same rule.
- Avoid unrelated refactors, dependency additions, generated metadata churn,
  and changes to the established visual design.
