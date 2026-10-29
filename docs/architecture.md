# Architecture

Why Valkyrie is shaped the way it is. Each section states the decision and the reason, so that a later change does not undo it
by accident.

## The stack

| Layer | Choice | Why |
| --- | --- | --- |
| Shell | Wails v3 | A native window around a webview. The Go process owns the window and the backend; the webview owns the UI. |
| Backend | Go | One binary, one process, no sidecar. |
| Storage | PocketBase, as a library | Gives a real database, migrations, file storage and access rules without running a server. |
| UI | React 19 + Vite + TanStack Query | The usual stack. The scaffold deliberately adds nothing else. |

## PocketBase runs as a library, not as a process

`internal/pocketbase/service.go` calls `pocketbase.NewWithConfig(...)` and `Bootstrap()`, and **never** calls `Start()`. No HTTP
server is bound by PocketBase, and there is no second process to supervise.

Its REST and file API is mounted on the Wails asset server instead:

```go
Assets: application.AssetOptions{
    Handler: combineHandlers(pbHandler, application.AssetFileServerFS(assets)),
},
```

`combineHandlers` routes `/api/*` to PocketBase and everything else to the built frontend. The consequence is the important
one: **a stored file is served from the same origin as the UI**. No CORS handling, no second port, no cookie or token
plumbing. The file route is `/api/files/<collection>/<recordId>/<filename>`, and it is the only way a file reaches the
frontend.

`combineHandlers` is covered by a test in `main_test.go`. That test is the guard: if the routing is broken, the files stop
loading and nothing else fails loudly.

## The job queue is generic on purpose

`internal/jobs` knows about jobs, artifacts and their lifecycle. It knows nothing about images, models, prompts or any other
domain. A job is a title, a prompt, a `map[string]any` of params and an arbitrary result.

The work is delegated to a `jobs.Runner`:

```go
type Runner interface {
    Run(ctx context.Context, job Job) (*Result, error)
}
```

This is the single most important design decision in the template. It means the queue, the persistence, the cancellation, the
events and the history are written once and never touched again, no matter what the application does. An application that
generates images, transcribes audio or scrapes a website changes `internal/runner/` and nothing else.

**Why a `map[string]any` for the params and the result, rather than a typed struct?** A typed struct would have to be
generified into `map[string]any` at the boundary anyway, and the generation would lose the field names. Keeping it loose means
the runner receives exactly what the frontend sent, and the frontend receives exactly what the runner returned. The cost is
that the shape is not checked at compile time; the price of that is paid in the runner, where the application knows its own
domain.

## The queue mechanics

- **Bounded concurrency.** `Manager.queue` is a buffered channel. A job acquires a slot before it runs. This is what stops a
  burst of submissions from flooding a rate-limited backend.
- **Cancellation is a context.** Each running job gets a `context.Context` whose cancel function is registered in
  `Manager.cancels`. `Cancel` calls it and marks the record `cancelled`. A runner that ignores the context will keep running
  until it finishes — the scaffold cannot force it to stop, so a runner **must** honour `ctx`.
- **A cancelled job is not a failed job.** When a runner returns an error after the context was cancelled, the manager checks
  `ctx.Err()` and keeps the `cancelled` status. The user asked for it; it is not an error.
- **Deletion requires rest.** `DeleteJob` refuses while a job is `pending` or `running`, because a running job would write to
  the database after its record was removed. It deletes the job's artifacts in the same call, so no orphan files are left in
  the storage directory.
- **A job has a time limit.** The default is 30 minutes. The runner receives a context that expires on its own, so a runner
  that never returns cannot hold a slot forever. `WithTimeout` changes the limit; zero removes it.
- **The manager owns a lifecycle.** Its root context is the parent of every job context, so `Shutdown` stops them all at once
  and waits for them. The Wails `OnShutdown` hook calls it, so quitting the application no longer kills a job mid-write.
- **The interrupted jobs are reconciled at startup.** A job that was pending or running when the process died is marked as
  failed with an explanation, so it appears in the history and the user can delete it.
- **The events are a push, the polling is a net.** The backend emits `job:updated` on every transition. The frontend also
  polls with a React Query `refetchInterval`. The event is the mechanism; the poll is the safety net for a missed event.

## The secrets are write-only

PocketBase has no generic secret-encryption facility, so `internal/pocketbase/crypto.go` provides one: AES-256-GCM with a key
generated on first run and stored in a `0600` file in the data directory.

Two independent layers protect a secret:

1. The `app_settings` collection has **no** list or view rule, so PocketBase's own API never returns it. The Go backend reads it
   directly through the record API.
2. The value itself is encrypted before it is written.

The frontend therefore learns only whether a key is present (`has_api_key`), never the key. `SaveCredentials` keeps the stored
key when the submitted one is empty, so the UI never has to read back a secret it cannot see.

The debug console shows `console.Fingerprint(secret)` — a short, non-reversible digest — so that two keys can be told apart in a
log line without either being revealed.

## The migrations are the schema

The schema lives in `internal/pocketbase/migrations/`, not in a JSON file and not in the code that uses it. Each migration
registers itself with `core.AppMigrations` in an `init()`, and the blank import in `internal/pocketbase/service.go` makes the
package discoverable.

**The file name is the identity and the ordering key.** PocketBase sorts the migration list by name, so:

- a new migration must be named with a timestamp that sorts **after** the ones already applied;
- the name must be **stable** once applied, because PocketBase records it and will not re-run it under a different name;
- to make a migration idempotent, check whether the field already exists before adding it.

The shipped migrations are `1730000000_init.go`. A new one would be `1732000000_add_notes.go`.

## The access rules are the write gate

The scaffold sets `ListRule` and `ViewRule` to `""` (public read) on `jobs` and `artifacts`, and leaves `app_settings` without
any rule.

Read access is public because the application is a single-user desktop app: the local database is the user's own data. Write
access is **closed**, because every write goes through the Go services, where the invariants live. That is the shape the
frontend expects, and it is why the frontend never needs write access to the API.

`app_settings` gets no rule at all, which is what keeps it out of reach of the HTTP API.

## The queue knows about images, on purpose

`internal/jobs/media.go` decodes image headers, and the `artifacts` collection carries `width` and `height` columns. That is
image-specific knowledge inside a package whose headline claim is that the queue is generic.

It is a deliberate trade-off, and this is the honest statement of it: **images are the common case, and detecting the format
from the bytes is the thing that is easy to get wrong.** Storing a JPEG under a `.png` name produces a file that every tool
misidentifies. The scaffold does that detection once, correctly, rather than leaving it to each application.

The cost is that a queue storing audio, PDFs or JSON still runs `DimensionsOf`, still fails the decode, and still carries two
columns it never fills. The decode failure is ignored, so nothing breaks; the columns stay zero.

If an application is not image-based, the clean fix is to move the dimensions into the `artifacts.meta` JSON field, which
needs a migration. Until then, this is the trade-off, and it is not a bug.

## The listing is batched

`ListJobs` converts a whole page at once. It fetches the artifacts of the page with a single query, rather than letting each
job collect its own: without that, a page of N jobs costs N+1 queries, and the frontend polls the listing every five
seconds.

The query is a raw `dbx.In`, because the PocketBase search filter has no `IN` operator. It goes through `app.RecordQuery`
rather than `app.DB()`, so that the records are scanned with their collection context — a bare `core.Record` panics in
`PostScan` without one.

## The inputs and the outputs are separate

An artifact belongs to one job, and it is either an **input** (something the job consumed) or an **output** (something it
produced). A job tracks the two separately.

This matters because a retry re-uses the inputs of the job it repeats. Moving them would strip the original job of its
outputs and hand them to the retry, and deleting the retry would then delete files the original still refers to. So the
manager **copies** an artifact that already belongs to another job, and only claims one that belongs to no job.

## The frontend talks to Go, not to the database

Every backend call goes through the Wails bindings. The frontend never touches PocketBase directly, and never calls a remote
API directly. The path is:

```
React component
  → frontend/src/bindings/index.ts      (typed wrapper, normalises nullables)
    → frontend/bindings/.../appservice.ts   (generated)
      → Go: internal/bindings/service.go     (AppService)
        → internal/jobs/ or internal/pocketbase/
```

`frontend/src/bindings/index.ts` exists so that no component ever imports a generated file. It also normalises the nullable
arrays the generator produces: the backend always sends an array, but the generated type says `string[] | null`, so the
wrapper narrows it once, at the boundary, rather than in every component.

The generated files themselves are gitignored and are never edited by hand. Changing the Go service and re-running
`wails3 task common:generate:bindings` is the only way they change.

## The zoom scales the root font size

The application runs in a webview, where the user cannot change the browser's base font size. So the interface zoom scales the
**root font size** instead:

```ts
root.style.fontSize = `${(13 * factor) / 100}px`;
```

Every size in `frontend/src/styles.css` is therefore expressed in `rem`, so the entire layout follows. A size in `px` does not
scale, and is a bug waiting to happen. The factor is also written as a custom property (`--zoom`) and a data attribute
(`data-zoom`) for the few things that cannot be expressed in `rem`.

The zoom is persisted in `app_settings` under the `ui` key, and restored as soon as the settings load. It is set from the
**Zoom** tab of the settings dialog, which is tabbed so that the credentials and the interface scale do not share one form.

## The console panel is a plain text view

The debug console deliberately shows plain text, not a rich editor. The point is to read a raw API response verbatim.

It is hidden until it is asked for, with **Ctrl+Shift+I** or with the button in the header. The shortcut is matched in the page
(`frontend/src/ui/shortcuts.ts`), because the webview has no menu bar of its own, and `event.preventDefault()` keeps the
webview from opening its own inspector on the same keys. The header button carries the error count, so a failure is visible
while the console is closed.

Three details are load-bearing, and they all come from the webview rather than from the design:

- **The clipboard needs a fallback.** `navigator.clipboard` requires a secure context and a user gesture, which a webview does
  not always provide. `writeClipboard` falls back to a hidden textarea plus `document.execCommand('copy')`.
- **The fallback must not use `display: none`.** That makes the textarea unselectable and breaks the copy command. It is
  positioned off-screen with `opacity: 0` instead.
- **A drag that ends in a click fires the row handler.** `hasTextSelection()` is checked before toggling the detail view, so
  selecting a message to read it does not collapse the panel.

## Why there is no UI kit

The source project this was generalised from vendored a large component library. It worked, but it made the template heavy,
introduced a licence obligation, and made the "replace the parts you do not need" story much harder.

The scaffold therefore ships a plain stylesheet with a small set of CSS custom properties, and three runtime dependencies. The
layout is a grid; the panels are plain `div`s. Extending it means adding a panel and a class, not learning a component API.

## What the build tooling does

- `build/config.yml` is the Wails project configuration: the product name, the bundle identifier, the version and the dev-mode
  commands. The CLI binary is `wails3` and the config file is `build/config.yml`, never `wails.json`.
- `Taskfile.yml` at the root includes `build/Taskfile.yml` under `common:` and one Taskfile per platform. The root tasks
  (`build`, `package`, `run`, `dev`) dispatch to the platform by the `GOOS` variable.
- `wails3 task` is the runner. The standalone `task` binary is not needed.
- The Linux build needs a C compiler and the GTK/WebKit development packages. When they are missing, the Linux task falls back
  to a Docker cross-compilation image.
- `-tags server` produces the headless binary from the same source, with no GUI dependencies.

## Where to look first

| Question | File |
| --- | --- |
| How does a job run? | `internal/jobs/manager.go`, `internal/runner/` |
| What does the frontend call? | `internal/bindings/service.go`, `frontend/src/bindings/index.ts` |
| Where is the schema? | `internal/pocketbase/migrations/` |
| How are files served? | `internal/pocketbase/service.go` (`Handler`), `main.go` (`combineHandlers`) |
| How are secrets stored? | `internal/pocketbase/crypto.go`, `internal/pocketbase/settings.go` |
| How does the UI get live updates? | `internal/bindings/events.go`, `frontend/src/app/App.tsx` |
