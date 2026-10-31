# WikiRevele vocabulary

The terms below are shared between the Go backend, the React frontend and the documentation. The same terms appear in code as
collection names (`jobs`, `artifacts`, `app_settings`) and as JSON field names.

Use the **Avoid** synonyms as a signal that the term is not what you want.

## Language

**Job**:
A unit of work in the queue. It carries a title, a prompt, arbitrary parameters and an arbitrary result. It says nothing about
what the work actually is.
_Avoid_: Task, Request, Generation, Operation

**Runner**:
The implementation that executes a job. It is the extension point of the scaffold: an application replaces the shipped echo
runner with its own. A runner receives a `jobs.Job` and a `context.Context`, and returns a `jobs.Result` or an error.
_Avoid_: Worker, Executor, Handler, Processor

**Echo runner**:
The reference runner in `internal/runner/echo.go`. It waits briefly, honours the context, and returns the prompt back. It exists
so the scaffold runs with no external service and so the queue can be exercised end to end.
_Avoid_: Mock runner, Test runner, Default runner

**Reconcile**:
The startup pass that resolves the jobs a crash left behind, marking them failed or re-enqueueing them. The scaffold does not
do this yet: a job left in `running` is never resumed and cannot be deleted.
_Avoid_: Recover, Replay, Resume

**Retry**:
The action that runs a finished job again with the same parameters. It creates a **new** job; it must not take the original's
outputs with it.
_Avoid_: Repeat, Rerun, Restart

**Job status**:
The lifecycle state of a job: `pending`, `running`, `completed`, `failed` or `cancelled`. It is a fixed enumeration, not a free
string.
_Avoid_: State, Phase, Stage

**Artifact**:
A stored file, either one the user supplied (`input`) or one the job produced (`output`). It carries the bytes, the detected
media type and the dimensions when the content is an image.
_Avoid_: Asset, File, Upload, Attachment, Resource

**Artifact kind**:
Whether an artifact is an `input` or an `output`. It is stored in the `artifacts.kind` select field, so adding a kind means
changing the migration.
_Avoid_: Direction, Role, Type, Category

**Job title**:
The short label of a job, shown in the queue. It defaults to the first line of the prompt, truncated.
_Avoid_: Name, Label, Summary, Subject

**Job prompt**:
The free-form instruction of a job. The scaffold does not interpret it; the runner does.
_Avoid_: Query, Message, Input, Description

**Job params**:
The arbitrary `map[string]any` of parameters of a job. The scaffold stores them verbatim and hands them to the runner.
_Avoid_: Options, Settings, Config, Arguments, Payload

**Job result**:
The arbitrary `map[string]any` a successful job produced, stored verbatim and returned to the frontend.
_Avoid_: Output, Response, Payload, Data

**Settings**:
The key/value preferences in the `app_settings` collection. A setting is either plain or encrypted.
_Avoid_: Preferences, Config, Configuration, Options

**Secret**:
A setting stored encrypted, such as the API key. It is written and read only by the Go backend, and the frontend learns only
whether one is present.
_Avoid_: Credential, Token, API key (when used for the stored value)

**Fingerprint**:
A short, non-reversible digest of a secret, shown in the debug console so that two keys can be told apart without revealing
either.
_Avoid_: Hash, Mask, Redacted key

**Debug console**:
The in-memory log of the application traffic, shown in a panel at the bottom of the window. It is capped at 500 entries and is
not persisted: it is a debugging aid, not an audit trail. It is hidden until it is asked for, with Ctrl+Shift+I or with the
button in the header.
_Avoid_: Log viewer, Audit log, DevTools

**Console entry**:
One record of the debug console: a level, a source, a message and an optional verbatim detail.
_Avoid_: Log line, Record

**Binding**:
A TypeScript wrapper around the Wails-generated bindings, exported from `frontend/src/bindings/index.ts`. It is the only place
the frontend is allowed to import generated code from.
_Avoid_: Generated binding (that is the file under `frontend/bindings/`), API client, Service

**Generated binding**:
The machine-written TypeScript under `frontend/bindings/`, produced by `wails3 generate bindings`. It is gitignored and must
never be edited by hand.
_Avoid_: Wrapper, Client, Types

**Wails service**:
A Go type bound to the frontend, exposed through `application.NewService(...)` in `main.go`. There is one: `AppService`.
_Avoid_: Controller, Handler, API

**Event**:
A Wails event that pushes a backend change to the frontend. Two exist: `job:updated` and `console:entry`. Both are registered
in `main.go` with `application.RegisterEvent`, which is what makes the generator emit a typed TS API for them.
_Avoid_: Message, Signal, Subscription

**Data directory**:
The directory holding the database, the stored files and the encryption key. It is `pb_data` next to the executable, and
`WIKIREVELE_DATA_DIR` relocates it.
_Avoid_: Database directory, Storage path, App data

**Migration**:
A Go function registered with PocketBase's migration list, which creates or evolves the schema. The file name is the identity
and the ordering key.
_Avoid_: Schema file, Upgrade, Patch

**Server mode**:
The headless build of the same binary, compiled with `-tags server`. It serves the frontend and the PocketBase API over HTTP
with no GUI dependencies.
_Avoid_: Web mode, Headless mode, Docker mode
