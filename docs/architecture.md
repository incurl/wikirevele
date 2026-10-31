# Architecture

Why WikiRevele is shaped the way it is. Each section states the decision and the reason, so that a later change does not undo it
by accident.

WikiRevele is a desktop application for exploring a Wikidata dataset. A dump is trimmed down to the entities linked to a
place, imported into an embedded database, and browsed as a gallery of cards. The document starts at the edges of that
pipeline and works inwards: the shell, the database, the queue, the dataset, the interface.

## The stack

| Layer | Choice | Why |
| --- | --- | --- |
| Shell | Wails v3 | A native window around a webview. The Go process owns the window and the backend; the webview owns the UI. |
| Backend | Go | One binary, one process, no sidecar. |
| Storage | PocketBase, as a library | Gives a real database, migrations, file storage and access rules without running a server. |
| UI | React 19 + Vite + TanStack Query + Refine | The usual stack. Refine answers the queries and the views; the panel owns the rendering. |

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

`internal/jobs` knows about jobs, artifacts and their lifecycle. It knows nothing about the dataset, about the models of a
remote API, or about any other domain. A job is a title, a prompt, a `map[string]any` of params and an arbitrary result.

The work is delegated to a `jobs.Runner`:

```go
type Runner interface {
    Run(ctx context.Context, job Job) (*Result, error)
}
```

This is the single most important design decision in the queue. It means the queue, the persistence, the cancellation, the
events and the history are written once and never touched again, no matter what the runner does. A runner that queries a
remote API, transcribes audio or scrapes a website changes `internal/runner/` and nothing else.

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
  until it finishes — the queue cannot force it to stop, so a runner **must** honour `ctx`.
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

**An applied migration is never edited.** The init migration `1730000000_init.go` still creates the `width` and `height`
columns of `artifacts`, because it was applied before they were removed. The migration that drops them is
`1736000000_drop_artifact_dimensions.go`. This is why the history of the schema is readable in the file names.

The shipped migrations are:

| File | What it creates |
| --- | --- |
| `1730000000_init.go` | `jobs`, `artifacts` and `app_settings` |
| `1732000000_add_wikidata_import.go` | `wikidata_items` and `wikidata_properties` |
| `1734000000_add_etl_job_history.go` | `etl_job_history` |
| `1736000000_drop_artifact_dimensions.go` | Drops the `width` and `height` columns of `artifacts` |
| `1738000000_widen_etl_history_targets.go` | Widens the target and the item file fields of `etl_job_history` |

A new one would be `1738000000_add_notes.go`.

## The access rules are the write gate

The application sets `ListRule` and `ViewRule` to `""` (public read) on `jobs` and `artifacts`, and leaves `app_settings`
without any rule.

Read access is public because the application is a single-user desktop app: the local database is the user's own data. Write
access is **closed**, because every write goes through the Go services, where the invariants live. That is the shape the
frontend expects, and it is why the frontend never needs write access to the API.

`app_settings` gets no rule at all, which is what keeps it out of reach of the HTTP API.

## Several targets, one pass over the dump

The extraction worker can extract **several targets in one run**, and the
dump is read **once** for all of them.

That is the whole point of the design, and it is a constraint of the
data rather than a convenience: the full dump is more than a hundred
gigabytes. A run per target would read it once per target, and a user
who wants Paris, Shanghai and New York would pay for three passes over
a hundred gigabytes to get what one pass gives.

So the pipeline holds a **set of targets**, and every entity of the dump
is checked against every one of them:

```go
for _, target := range p.targets {
    keep := target.Matches(raw)
    ...
    target.Write(trimmed)
}
```

An item that is linked to two targets is written to both files. The
targets are independent: the item belongs to both, and neither file
knows about the other.

**The property dictionary is written once.** It is the catalogue of the
whole dump and it does not depend on the target, so one run over three
targets writes one dictionary and not three. The two outputs are also
independent of each other: `--items-only` and `--properties-only` say
which of the two a run writes.

**The fast path stays a fast path.** The line is skipped when it
mentions none of the targets, and the search is a byte search over the
identifiers. The cost of a target is one more search on the lines that
are skipped, and the lines that are not skipped are read anyway.

**The hierarchies are separate.** In the recursive mode each target
walks its own administrative hierarchy, and each one filters its own
file. The hierarchy of Shanghai and the hierarchy of New York have
nothing in common, and an item that is inside one of them is not inside
the other.

The run of a single target is the degenerate case of several: there is
no separate code path to keep honest.

## The media type is read from the bytes

`internal/jobs/media.go` sniffs the format of a stored file from its content, not from its name, and the `artifacts`
collection carries the detected `mime` and the size.

This is the one piece of knowledge the queue has about the files it holds, and it is deliberately narrow: **the detection is
about the bytes that were stored, not about what they mean.** Storing a JPEG under a `.png` name produces a file that some
tools reject and all of them misidentify, and reading the format from the bytes gets that right once, in one place.

What the queue deliberately does *not* do is interpret the file. It does not decode it, does not measure it, and does not
know what produced it. A file that carries dimensions, or a record count, or anything else specific to its domain, keeps
that in the `artifacts.meta` JSON field, where the domain that put it there can define its own shape.

The pixel dimensions were removed for exactly this reason: they were image-specific columns in a package whose headline
claim is that the queue is generic, and nothing ever read them. The migration `1736000000_drop_artifact_dimensions.go` is
what drops them from an existing database.

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

## The components are ours, not a dependency

The interface is built on **shadcn/ui**, which is not a package: the components are copied into the repository as source, under
`frontend/src/components/ui/`. The same is true of the Refine views, under `frontend/src/components/refine-ui/`.

That is deliberate. A vendored component library that is *ours* can be edited, cannot drift under a version bump, and does not
add a licence obligation. The layout of the application is a grid over plain `div`s and the CSS custom properties of the
design tokens, so extending it means adding a panel and a class, not learning a component API.

Tailwind carries the styling of the shadcn components, and the components read the HSL channels of the same tokens as the
rest of the interface.

## The design system is the Wikimedia Codex one

The palette, the spacing and the type come from the [Wikimedia Codex design system](https://doc.wikimedia.org/codex/), through
the `@wikimedia/codex-design-tokens` package. The tokens are imported as they are, and the names this interface uses are mapped
onto them in `frontend/src/styles.css`.

Two things are worth knowing about the tokens:

- **Their sense is the opposite of what the names suggest.** `base` is the colour of the surface the text stands on, and
  `inverted` is the colour of the text itself. In the dark theme the base is a pale grey and the text is near black.
- **They are the reason the project is GPL-3.0.** The package is GPL-2.0+, so the whole application is under the copyleft licence.
  `tools/license-audit.py` holds the list of the copyleft dependencies that are accepted, and the reason for each.

The shadcn components of the panel use the HSL channels of the same tokens. They were declared in the Tailwind configuration
and never defined, so every component that used them fell back to nothing; they are defined now.

## The window is a navigator, a view and the jobs

The window is three columns, and only the middle one holds data:

| Column | What it is |
| --- | --- |
| Left | The **navigator**: the list of the scopes. It holds no pane of its own |
| Centre | The **view**: the scope the navigator selected, at the full width the rails leave it |
| Right | The **jobs**: the queue, and nothing else |

A scope is a pane the view can show. The list is `frontend/src/app/scopes.ts`, and it is the one
place that names them: the navigator, the view and the accessible names all read it.

| Scope | What it shows |
| --- | --- |
| Workarea | The gallery of the Wikidata cards |
| Extract | The parameters of an extraction, and the command it produces |
| Import | The import of a dataset into the database |
| Submit | The submission of a job to the queue |
| History | The runs of the extraction |

**Nothing is stacked.** A tool used to be a panel folded into a rail beside the others; it is now the
whole view, so a form has the width of the window. There is one scope at a time, and the navigator says
which.

Both rails collapse, and both give their room to the view: the navigator keeps a strip of icons, and the
jobs rail collapses to a strip that still shows how many jobs are moving. The view is the only column
that grows, which is the point: it is where the data is.

A route of the workarea switches the view to it. A link to an item changes the path, and the page it
addresses has to be shown, so `scopeOfPath` maps the path of a route to the scope that owns it.

The selection of the cards is kept out of the components, in `features/wikidata/selection.ts`: a click selects one item, a click
with the command or the control key toggles it among many, and the bar above the grid selects the whole page or clears the
selection.

## The gallery renders, Refine queries

The workarea shows a dataset of a whole city, so the four jobs of a list
are divided between three libraries, and each does one of them:

| Job | Owner |
| --- | --- |
| Filtering, sorting, pagination | Refine's `useTable`, answered by the backend |
| The row model of the cards | TanStack Table |
| What is on screen | TanStack Virtual |

The division is not cosmetic. Refine sends the filters, the sorters and
the page to Go, which queries PocketBase and answers with one page and
a count; TanStack Table is given that page and sorts and pages nothing,
because the server already did; and the virtualiser renders the cards
of the grid that are in the viewport, so a page of a thousand cards
costs the same as a page of twenty.

Two details of that division are easy to undo by accident:

- **The table owns the identity of the rows.** `getRowId` is the QID of
  the item, so a row is the item and not its position in the page. A
  card that moves between two renders keeps its place in the DOM
  instead of being mounted again.
- **The grid is made of the virtualiser's lanes.** A lane is one
  column of the grid and an item is one card, so the library lays the
  cards out and measures them. The estimate is therefore the height of
  **one card** and not the height of a row of cards, and a card that
  opens its detail panel, or that shows its graph, is measured and
  given the room it needs.

The wiring is in two modules next to the panel, `features/wikidata/gallery/table.ts` and
`features/wikidata/gallery/virtual.ts`, and the drawing is in `features/wikidata/components/GalleryGrid.tsx`.
The page itself, `features/wikidata/pages/WikidataItemList.tsx`, holds the query and the selection and draws
nothing: the grid is a plain function of its props, so it can be rendered on its own.

`useTable` is from `@refinedev/core`. `@refinedev/react-table` is in the
dependency list and is **not** used: its `useTable` wraps TanStack
Table, and the table here is the renderer, not the query.

TanStack Query is present because **Refine is built on it**: it is a
peer dependency of `@refinedev/core`, and Refine uses it for the cache
of the queries. The application does not use it for its own data, and
it must not: the data of a resource goes through `useTable`.

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
| How is the dump trimmed? | `internal/etl/` |
| How is a run driven from the interface? | `internal/etlpipeline/`, `internal/etlhistory/` |
| How is the dataset imported? | `internal/wikidata/importer.go` |
| How are the cards rendered? | `frontend/src/features/wikidata/` |
