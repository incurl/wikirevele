# CLAUDE.md

This file is the Claude Code entry point for WikiRevele. It points at the canonical guides rather than repeating them, so that
there is one source of truth.

## Read these, in this order

1. **`AGENTS.md`** — the working rules, the commands that work on this machine, the change routing and the known rough edges.
   Read it first, every session.
2. **`CONTEXT.md`** — the vocabulary. Use the terms the way it defines them, and treat the *Avoid* list as a signal that you
   have picked the wrong word.
3. **`docs/architecture.md`** — why the pieces are shaped the way they are.
4. **`docs/troubleshooting.md`** — when something breaks, check here before guessing.

## The three things that matter most

1. **The dataset is produced in two stages, in one binary.** The ETL worker (`internal/etl/`) trims the dump; the application
   (`internal/wikidata/`) imports the result. The application never reads a dump and never trims anything.
2. **The job queue is generic; the runner is yours.** `internal/jobs` owns the lifecycle only. The domain goes in
   `internal/runner/`. Do not put domain logic into `internal/jobs/`.
3. **The frontend talks to Go, not to the database.** Every call goes through `internal/bindings/service.go` and the typed
   wrappers in `frontend/src/bindings/index.ts`. Never import a generated binding file from a component.

`frontend/bindings/` and `frontend/dist/` are generated and gitignored. Never hand-edit them. Regenerate with
`PATH="$HOME/go/bin:$PATH" wails3 task common:generate:bindings`.

## Before you finish

Run the checks that match what you changed — the routing table is in `AGENTS.md`. When in doubt:

```bash
gofmt -l internal/ main.go main_test.go tools/     # must print nothing
go vet ./internal/... .
go test -race ./internal/... .
cd frontend && npm run typecheck && npm test && npm run build && cd ..
python3 tools/license-audit.py
```

## Skills

`.agents/skills/` holds task-scoped procedures with a `SKILL.md` each. Read the relevant one before the task:

| Skill | Use it for |
| --- | --- |
| `add-backend-method` | Adding a Go method the frontend can call. |
| `add-migration` | Changing the PocketBase schema. |
| `add-frontend-panel` | Adding a panel to the React frontend. |
| `release` | Cutting a release. |
