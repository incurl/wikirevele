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
The implementation that executes a job. It is the extension point of the queue: the application replaces the shipped echo
runner with its own. A runner receives a `jobs.Job` and a `context.Context`, and returns a `jobs.Result` or an error.
_Avoid_: Worker, Executor, Handler, Processor

**Echo runner**:
The reference runner in `internal/runner/echo.go`. It waits briefly, honours the context, and returns the prompt back. It exists
so that the queue can be exercised end to end without any external service.
_Avoid_: Mock runner, Test runner, Default runner

**Reconcile**:
The startup pass that resolves the jobs a crash left behind, marking them failed or re-enqueueing them.
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
media type and the size.
_Avoid_: Asset, File, Upload, Attachment, Resource

**Artifact kind**:
Whether an artifact is an `input` or an `output`. It is stored in the `artifacts.kind` select field, so adding a kind means
changing the migration.
_Avoid_: Direction, Role, Type, Category

**Job title**:
The short label of a job, shown in the queue. It defaults to the first line of the prompt, truncated.
_Avoid_: Name, Label, Summary, Subject

**Job prompt**:
The free-form instruction of a job. The queue does not interpret it; the runner does.
_Avoid_: Query, Message, Input, Description

**Job params**:
The arbitrary `map[string]any` of parameters of a job. The queue stores them verbatim and hands them to the runner.
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
A Wails event that pushes a backend change to the frontend. Three exist: `job:updated`, `console:entry` and `etl:progress`. All
are registered in `main.go` with `application.RegisterEvent`, which is what makes the generator emit a typed TS API for them.
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

## The dataset

**Dump**:
The Wikidata JSON dump. It is a stream of JSON entities, one per line, and it is more than a hundred gigabytes. It reaches the
ETL worker through the standard input, already decompressed.
_Avoid_: Dataset, Snapshot, Export, Archive

**ETL worker**:
The `etl` subcommand of the binary. It reads a dump, keeps the entities that are linked to the target, trims them, and writes
two NDJSON files. It never opens the database.
_Avoid_: Importer, Loader, Indexer, Parser

**Target**:
The entity a dataset is built around, named by its QID. For the Paris exploration it is `Q90`. It is the value the kept
statements must point at. A run may have **several** targets, and the dump is read once for all of them.
_Avoid_: Subject, Root, Anchor, Seed

**Target set**:
The targets of one run. Every entity of the dump is checked against every one of them, so the run costs one pass over the
dump and not one pass per target.
_Avoid_: Multi-target, Batch, Group

**Property dictionary**:
The catalogue of every property of the dump. It is the same for every target, so a run writes it once and not once per
target.
_Avoid_: Property file, Schema, Dictionary

**Item**:
A Wikidata entity that is not a property: a thing, a place, a person. It is stored as one record of `wikidata_items`, keyed by
its QID.
_Avoid_: Entity, Node, Subject, Object

**Property**:
A Wikidata property: the kind of statement an item can make. It is stored as one record of `wikidata_properties`, keyed by its
PID. The dictionary is small enough to be sent whole to the panel, which needs the names to label the claims.
_Avoid_: Attribute, Field, Relation, Predicate

**QID**:
The identifier of an item, such as `Q90`. It is the key of the `wikidata_items` collection.
_Avoid_: ID, Identifier, Key

**PID**:
The identifier of a property, such as `P131`. It is the key of the `wikidata_properties` collection.
_Avoid_: ID, Identifier, Key

**Claim**:
A statement of an item: a property, a value, and optionally some qualifiers and references. The claims of an item are stored
whole, as the JSON tree of the dataset, so that the panel can read the qualifiers and the references of a statement.
_Avoid_: Property, Statement pair, Fact, Attribute

**Subset**:
The NDJSON file of the items that are linked to the target. The other file is the **property dictionary**.
_Avoid_: Sample, Subset file, Trimmed dump

**Import**:
The operation that upserts the two NDJSON files into PocketBase. It runs in the background and reports its progress as an event.
_Avoid_: Load, Sync, Ingest

**Extraction**:
One run of the ETL worker, with its parameters and its outcome. It is recorded in `etl_job_history`, and the command that
produced it can be copied out and run again in a terminal.
_Avoid_: ETL run, Job, Task

**Items file**:
The NDJSON file of the items of one target. A run over several targets writes one file per target.
_Avoid_: Subset file, Item set, Output

**Workarea**:
The scope of the navigator that shows the gallery of the imported cards. It is the default scope.
_Avoid_: Home, Dashboard, Browser, Gallery
