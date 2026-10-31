# WikiRevele

A desktop application scaffold: **Wails v3** + **PocketBase** + **Vite** +
**React**, wired together and ready to be built on.

It is a template, not a product. The plumbing — the embedded database, the
job queue, the event bridge, the debug console, the build and packaging
tooling, the test setup — is done, so that a new project starts from a
working application instead of an empty folder.

```
Wails webview
├── React 19 + Vite + TypeScript + TanStack Query
│   ├── app/           the application shell
│   ├── features/      one panel per concern
│   ├── bindings/      typed wrappers over the generated bindings
│   └── ui/            pure helpers, unit tested
│
└──── Wails bindings (primary channel) ───────────────────────┐
        Go: internal/bindings/ → internal/jobs/ → internal/runner/
                                                (your backend) │
                            └→ internal/pocketbase/            │
                                    (embedded, library)       │
                                     └→ migrations, file fields│
        PocketBase's REST + file API mounted at /api/* on the    │
        same origin as the UI.                                   │
```

## What is in the box

| Piece | Where | What it does |
| --- | --- | --- |
| Job queue | `internal/jobs/` | Persistence, scheduling, cancellation, status transitions, events |
| Runner | `internal/runner/` | **The extension point.** The scaffold ships an echo runner; replace it |
| Embedded database | `internal/pocketbase/` | Bootstrap, migrations, settings, encrypted secrets |
| Wails services | `internal/bindings/` | The API the frontend calls |
| Debug console | `internal/console/` | An in-memory log of the application traffic, shown in the UI |
| React shell | `frontend/src/` | A working three-column layout with the queue, the console and the settings |
| Wikidata ETL worker | `internal/etl/` | The `etl` subcommand: reads a dump from the standard input and writes the two trimmed datasets |
| ETL pipeline | `internal/etlpipeline/` | Builds the shell pipeline, runs it as a child process, and cancels it |
| ETL history | `internal/etlhistory/` | The record of the runs, in PocketBase |
| Wikidata import | `internal/wikidata/` | Imports the datasets into PocketBase and reads them back for the panel |

The frontend talks to the backend **only** through the Wails bindings. It
never talks to PocketBase or to a remote API directly.

## Requirements

| Tool | Version | Notes |
| --- | --- | --- |
| Go | 1.26+ | `toolchain go1.26.10` is pinned in `go.mod` |
| Node | 20+ | |
| Wails CLI | v3.0.0 | `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0` |
| Linux build deps | gtk4, webkitgtk-6.0, gcc | `pkg-config` must find them |

> The Wails v3 CLI binary is **`wails3`**, not `wails`, and the project
> configuration is **`build/config.yml`**, not `wails.json`.

## Getting started

```bash
# 1. Install the Wails v3 CLI
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0

# 2. Install the frontend dependencies
cd frontend && npm install && cd ..

# 3. Run the app in development mode
wails3 dev
```

`wails3 dev` starts the Vite dev server, regenerates the bindings and
rebuilds the Go backend on every change.

To build a distributable binary:

```bash
wails3 build      # → bin/wikirevele
```

The [Task](https://taskfile.dev) wrappers are the ones to reach for when
you want to package:

```bash
task build            # a production binary for the host platform
task package          # the platform packages (deb, rpm, AppImage, dmg, nsis)
task run              # build and run
task build:server     # a headless HTTP server binary
task build:docker     # a minimal container image
```

## Making it yours

### 1. Rename it

| What | Where |
| --- | --- |
| The module path | `go.mod` |
| The product name | `build/config.yml`, `build/*/Taskfile.yml`, `internal/bindings/service.go` |
| The bundle identifier | `build/config.yml`, `build/darwin/Info.plist`, `build/ios/Info.plist` |
| The frontend name | `frontend/package.json` |

The `APP_NAME` variable in the root `Taskfile.yml` drives the binary name
and the package names.

### 2. Write your runner

The queue is generic. A job carries a title, a prompt, an arbitrary set of
parameters and an arbitrary result, and the work is delegated to a
`jobs.Runner`:

```go
type Runner interface {
    Run(ctx context.Context, job Job) (*Result, error)
}
```

Replace `internal/runner/echo.go` with your own:

```go
// MyRunner talks to the remote API.
type MyRunner struct {
    settings *pocketbase.SettingsStore
    log      *console.Log
}

func (r *MyRunner) Run(ctx context.Context, job jobs.Job) (*jobs.Result, error) {
    apiKey, found, err := r.settings.GetAPIKey()
    if err != nil {
        return nil, err
    }
    if !found {
        return nil, fmt.Errorf("no API key is configured. Add one in Settings.")
    }

    // The context is cancelled when the user cancels the job: honour
    // it, so that the job stops promptly.
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    result, err := callTheAPI(ctx, apiKey, job.Prompt, job.Params)
    if err != nil {
        // A *Error carries a numeric code, which the UI can branch on.
        return nil, &jobs.Error{Code: 42, Message: "the API refused the request", Err: err}
    }

    return &jobs.Result{
        ArtifactIDs: result.ArtifactIDs,
        Payload:     map[string]any{"answer": result.Answer},
    }, nil
}
```

Then register it in `main.go`:

```go
jobManager := jobs.NewManager(
    pbService.App(),
    jobs.WithLog(consoleLog),
    jobs.WithRunnerFunc(func() jobs.Runner {
        return runner.NewMyRunner(settings, consoleLog)
    }),
)
```

The queue, the persistence, the cancellation, the events and the history
all work the same way, whatever the runner does.

### 3. Extend the schema

The schema lives in `internal/pocketbase/migrations/`. Add a migration
with a **later timestamp** than the ones already applied — the list is
sorted by file name, so the name is the ordering key:

```go
func init() {
    registerMigration("1732000000_add_notes.go", func(app core.App) error {
        collection, err := app.FindCollectionByNameOrId("jobs")
        if err != nil {
            return err
        }
        if collection.Fields.GetByName("notes") != nil {
            return nil // already applied
        }
        collection.Fields.Add(&core.TextField{Name: "notes"})
        return app.Save(collection)
    })
}
```

### 4. Extend the frontend

`frontend/src/features/` holds one panel per concern. Add yours next to
them and wire it into `frontend/src/app/App.tsx`.

The shell has two things worth knowing about:

- The **debug console** is hidden until it is asked for, with
  **Ctrl+Shift+I** or with the **Console** button in the header. The
  button carries the error count, so a failure is visible while the
  console is closed.
- The **settings dialog** is tabbed: the credentials are on the
  *Connection* tab and the interface scale on the *Zoom* tab.

The generated bindings are plain JavaScript with JSDoc types. They are
re-exported behind a strongly typed surface in `frontend/src/bindings/`,
so the components never import a generated file directly.

## Data

Everything lives in `pb_data/` next to the executable:

| Path | Contents |
| --- | --- |
| `pb_data/data.db` | The SQLite database (3 collections) |
| `pb_data/storage/` | The uploaded and generated files, served by PocketBase's native file route |
| `pb_data/.wikirevele_key` | The AES-256-GCM key for the secrets (mode 0600) |

Set `WIKIREVELE_DATA_DIR` to relocate it.

## Collections

| Collection | Purpose |
| --- | --- |
| `jobs` | The job metadata: the prompt, the parameters, the status, the result, the errors |
| `artifacts` | The files a job produced or consumed, with their metadata |
| `app_settings` | The encrypted secrets and the preferences |
| `wikidata_items` | The imported Wikidata items, one record per item, keyed by `qid` |
| `wikidata_properties` | The imported property dictionary, one record per property, keyed by `pid` |

All writes go through the Go services: the `create`, `update` and `delete`
rules are closed, so the frontend can only read. PocketBase's own rules,
migrations, file fields and hooks are used as-is.

## Secrets

The API key is write-only from the UI's point of view. It is encrypted
with AES-256-GCM before it is written to the database, and the key is
generated on first run and kept in a `0600` file inside the data
directory. The `app_settings` collection has no public list or view rule,
so the Go backend is the only reader. The UI only ever learns whether a
key is present.

The debug console shows a fingerprint of the key, never the key itself.

## Server mode

The same binary runs headless, with no GUI dependencies:

```bash
task build:server
WAILS_SERVER_HOST=0.0.0.0 WAILS_SERVER_PORT=8080 ./bin/wikirevele-server
```

It serves the frontend and the PocketBase API over HTTP. A minimal
container image is built with `task build:docker`.

## The Wikidata dataset

The dataset is produced in two stages, in **one binary**.

**Stage 1 — the ETL worker**, the `etl` subcommand of the same
executable. It reads the dump from the standard input in one pass,
keeps the entities linked to the target, trims them to the fields the
application needs, and writes two NDJSON files.

```bash
PATH="$HOME/go/bin:$PATH" wails3 task build:etl

# The dump comes from the standard input, already decompressed.
bzcat latest-all.json.bz2 | ./bin/wikirevele etl --target-qid Q90 --langs en,fr
```

It writes `paris-wikidata-subset.ndjson` and
`wikidata-properties.ndjson`.

**Stage 2 — the import**, in the application. It reads those files a
line at a time and upserts them into the `wikidata_items` and
`wikidata_properties` collections. The panel imports them from the
import panel, and then works on the records of the database.

The application never reads a dump and never trims anything: the
trimming happens once, in the ETL stage.

### The two ways to run it

The worker is **dual use**. The desktop application runs it as a child
process, and a user runs it by hand from a terminal. The code is the
same in the two cases.

**From the application.** The Extract panel collects the parameters,
shows the shell pipeline they produce, and runs it. The command is
shown in a copyable block: copy it and run it in a terminal to do the
same job outside the application. A run is recorded in the history, and
its output is imported into the database when it succeeds.

```bash
bzcat /path/to/latest-all.json.bz2 | /path/to/wikirevele etl \
  --target-qid Q90 --langs en,fr \
  --out-items ./items.ndjson --out-props ./props.ndjson --events
```

**From a terminal.** The same command, by hand:

```bash
bzcat latest-all.json.bz2 | ./bin/wikirevele etl --target-qid Q90 --langs en,fr
```

The decompressor is **not shipped**. The host has to provide `bzcat`,
`zstdcat`, `gzip` or `xz`; the application checks that the one it
needs is there before it starts a run.

[docs/etl-manual.md](docs/etl-manual.md) is the manual of the worker,
and [docs/wikidata.md](docs/wikidata.md) has the details of the two
stages.

## Tests

```bash
go test ./...                    # the queue, the schema, the settings, the console, the import, the ETL
cd frontend && npm test           # the zoom, the queue helpers, the console, the ETL panel
python3 tools/license-audit.py    # no copyleft dependency
go run ./tools/e2e-check          # a live end-to-end run of the backend
```

The end-to-end check runs the whole pipeline without a GUI: it applies the
schema, submits a job, runs the stub runner, stores an artifact and
fetches it back through PocketBase's file route.

## Documentation

| Document | Read it for |
| --- | --- |
| [docs/architecture.md](docs/architecture.md) | Why the pieces are shaped the way they are |
| [docs/porting.md](docs/porting.md) | The checklist for taking this template to a new application |
| [docs/etl-manual.md](docs/etl-manual.md) | The operator's guide to the ETL worker: how to run it, what it writes, and what to do when it misbehaves |
| [docs/wikidata.md](docs/wikidata.md) | The Wikidata dataset pipeline: what the ETL worker trims, what the application imports, and the boundary between them |
| [docs/troubleshooting.md](docs/troubleshooting.md) | The failures that cost time, and what fixes them |
| [docs/audit.md](docs/audit.md) | The audit of the scaffold: the defects and their reproductions |
| [docs/code-review.md](docs/code-review.md) | The trade-offs that were deliberately kept, and why |
| [CONTEXT.md](CONTEXT.md) | The vocabulary: what each term means, and what it does not |
| [AGENTS.md](AGENTS.md) | The working rules for AI coding agents |
| [.agents/skills/](.agents/skills) | Task-scoped procedures: new application, backend method, migration, frontend panel, release |

## Licence

WikiRevele is MIT licensed. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)
for the third party attributions.
