# WikiRevele — Copilot instructions

A desktop application for exploring a Wikidata dataset, built on Wails v3, an embedded PocketBase and a React frontend.

Read `AGENTS.md` for the working rules, the commands and the change routing. This file is the short version.

## The two concepts

**The dataset is produced in two stages, in one binary.** The ETL worker (`internal/etl/`) reads a dump from the standard input
and writes two trimmed NDJSON files; the application (`internal/wikidata/`) imports them into PocketBase and reads them back for
the panel. The application never reads a dump and never trims anything.

**The job queue is generic; the runner is yours.**

`internal/jobs` models a job as a title, a prompt, a `map[string]any` of params and an arbitrary result. It owns the
lifecycle: persistence, bounded concurrency, cancellation, status transitions, the Wails event push. The work is delegated to a
`jobs.Runner`.

The application replaces `internal/runner/echo.go` with its own runner and changes nothing else. Do not add domain concepts to
`internal/jobs/`.

## Rules

- The frontend never talks to PocketBase or a remote API directly. Every call goes through `internal/bindings/service.go` and
  the typed wrappers in `frontend/src/bindings/index.ts`.
- Never hand-edit `frontend/bindings/` or `frontend/dist/`. They are generated and gitignored. Regenerate the bindings with
  `PATH="$HOME/go/bin:$PATH" wails3 task common:generate:bindings`.
- Never edit an applied migration. The file name in `internal/pocketbase/migrations/` is the identity and the ordering key: a
  new migration needs a later timestamp and must be idempotent.
- `app_settings` has **no** access rule, which keeps the secrets away from the HTTP API. Never add one. The API key is
  write-only from the UI's point of view.
- Sizes in the stylesheet are in `rem`, because the zoom scales the root font size.
- Read the code before editing it. Keep the diff scoped.

## Commands

`wails3` is not on the default PATH of a non-interactive shell; it lives in `~/go/bin`.

```bash
PATH="$HOME/go/bin:$PATH" wails3 build
PATH="$HOME/go/bin:$PATH" wails3 task common:generate:bindings
go test -race ./internal/... .
cd frontend && npm run typecheck && npm test
python3 tools/license-audit.py
```

`go build ./...` fails on `build/ios` by design; build `./internal/... .` instead.

## Where things live

| Path | Holds |
| --- | --- |
| `internal/jobs/` | The generic job queue: lifecycle, cancellation, artifacts. |
| `internal/runner/` | **The extension point.** Replace the echo runner. |
| `internal/pocketbase/` | The embedded database, the migrations, the settings, the encryption. |
| `internal/bindings/` | The Wails service and the event bridge. |
| `internal/console/` | The in-memory debug log. |
| `frontend/src/features/` | One directory per panel. |
| `frontend/src/bindings/index.ts` | The typed wrapper over the generated bindings. |
| `frontend/src/ui/` | Pure helpers, unit tested. |
