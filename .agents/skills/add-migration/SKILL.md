---
name: add-migration
description: >
  Change the PocketBase schema safely: add a field, add a collection, or
  evolve an existing one, using an idempotent Go migration with a correct
  timestamped file name, and pick the right access rules for it.
  Use when the application needs a new field, a new collection, a new enum
  value, or a change to an existing rule. Do not use for changing Go code
  that does not touch the schema, and do not use to edit an already applied
  migration.
metadata:
  owner: "incurl"
  version: "1.0.0"
  last-reviewed: "2026-10-28"
---

# Add a migration

The schema lives in `internal/pocketbase/migrations/`. A migration is a Go function registered with PocketBase's migration
list. It runs once, on a database that already exists, and it must be safe to run against a database that has already been
migrated.

## Ground Rules

- **Never edit an applied migration.** PocketBase records the file name and will not re-run it. Change the file name and the
  change never happens on an existing database.
- **The file name is the identity and the ordering key.** It must sort after every applied migration.
- **Make it idempotent.** Check before you add.
- **Never drop a field or a collection without a plan.** The data is the user's. A rename means add-and-migrate, not rename.
- **Decide the access rules deliberately.** Read public, write closed — unless it holds a secret, in which case no rule at all.

## 1. Choose the file name

The list is sorted by file name, so the name is the ordering key.

```
1730000000_init.go        ← the shipped schema
1732000000_add_notes.go   ← a later change
```

The prefix is a Unix timestamp. Any 10-digit value works as long as it sorts after the existing ones. Use the current date, and
bump the number when two migrations land on the same day.

**Check what exists first:**

```bash
ls internal/pocketbase/migrations/
```

## 2. Write the migration

Create `internal/pocketbase/migrations/<timestamp>_<name>.go`:

```go
package migrations

import (
    "github.com/pocketbase/pocketbase/core"
)

// init registers the migration that adds the notes field.
func init() {
    registerMigration("1732000000_add_notes.go", func(app core.App) error {
        return addNotesField(app)
    })
}

// addNotesField adds the "notes" field to the jobs collection.
func addNotesField(app core.App) error {
    collection, err := app.FindCollectionByNameOrId("jobs")
    if err != nil {
        return err
    }

    // A field that is already there means the migration already ran.
    if collection.Fields.GetByName("notes") != nil {
        return nil
    }

    collection.Fields.Add(&core.TextField{Name: "notes", Max: 500})

    return app.Save(collection)
}
```

The `registerMigration` helper wraps `core.AppMigrations.Register` so the body stays readable. It is defined in the `init.go`
file of the package.

**Idempotency is not optional.** A migration can run twice when a developer has an old database and pulls the branch, or when
the migration list is rebuilt. The `GetByName` check is the pattern.

### Adding a collection

```go
func createNotesCollection(app core.App) error {
    // A collection that is already there means the migration already ran.
    if _, err := app.FindCollectionByNameOrId("notes"); err == nil {
        return nil
    }

    notes := core.NewBaseCollection("notes")
    notes.Fields.Add(
        &core.TextField{Name: "body"},
        &core.AutodateField{Name: "created", OnCreate: true},
    )

    // Read is public: the application is a single-user desktop app and
    // the local database is the user's own data.
    notes.ListRule = types.Pointer("")
    notes.ViewRule = types.Pointer("")

    // Write is closed: every write goes through the Go services, where
    // the invariants live.
    return app.Save(notes)
}
```

The collections are created in dependency order, so that a relation field can reference a collection id that already exists:

```go
jobs := core.NewBaseCollection("jobs")
// ... add the jobs fields ...
if err := app.Save(jobs); err != nil {
    return err
}

artifacts := core.NewBaseCollection("artifacts")
artifacts.Fields.Add(
    &core.RelationField{Name: "job", CollectionId: jobs.Id, MaxSelect: 1},
    // ...
)
if err := app.Save(artifacts); err != nil {
    return err
}
```

### Adding an enum value

A select field holds its values in the schema, so a new value is a schema change:

```go
if collection.Fields.GetByName("kind") == nil {
    collection.Fields.Add(&core.SelectField{
        Name:      "kind",
        Values:    []string{"input", "output", "thumbnail"},
        MaxSelect: 1,
    })
}
```

A migration that **removes** a value leaves the existing rows in an invalid state. Add the new value first, migrate the rows in a
second migration, then remove the old value in a third.

## 3. Pick the access rules

The rule is the write gate. The frontend reads through the Go backend, so it never needs write access.

| Collection | List / View | Create / Update / Delete | Why |
| --- | --- | --- | --- |
| `jobs`, `artifacts` | public (`""`) | closed (unset) | Read is the user's own data; writes go through the Go services. |
| `app_settings` | unset | unset | No rule at all: the HTTP API cannot reach it, so the Go backend is the only reader. |

**A collection with no rule is invisible to the HTTP API.** That is the mechanism that keeps the secrets out of reach of the
frontend. Do not add a list or view rule to `app_settings`.

## 4. Wire the Go side

A new field on a record is not automatically on the frontend view. The frontend-facing structs are built by hand in
`internal/jobs/manager.go`:

```go
func (m *Manager) toJob(record *core.Record) (Job, error) {
    job := Job{
        // ...
        Notes: record.GetString("notes"),
    }
    // ...
}
```

And the struct in `internal/jobs/types.go` needs the field with a JSON tag, because the generator emits the TS type from it:

```go
type Job struct {
    // ...
    // Notes is the free-form note attached to the job.
    Notes string `json:"notes"`
}
```

Then regenerate the bindings and add a wrapper — see the `add-backend-method` skill.

## 5. Verify

```bash
gofmt -l internal/
go test ./internal/pocketbase/...
go test -race ./internal/...
go run ./tools/e2e-check
```

**Test the migration against a fresh database and an existing one.** The scaffold's test helper creates a throwaway database
per test, so a migration that only works on a fresh database will pass the tests and fail in the field.

```go
func TestMigrationIsIdempotent(t *testing.T) {
    service := NewService(t.TempDir())
    if err := service.Bootstrap(); err != nil {
        t.Fatalf("bootstrap: %v", err)
    }
    defer service.Close()

    // The schema must exist.
    if _, err := service.Collection("jobs"); err != nil {
        t.Errorf("the jobs collection is missing: %v", err)
    }
}
```

Then check the real thing: run the app against a **copy** of an existing database and confirm the migration applies.

```bash
cp -r ~/.local/share/wikirevele/pb_data /tmp/pb_data_backup
WIKIREVELE_DATA_DIR=/tmp/pb_data_backup ./bin/wikirevele
```

## Common mistakes

| Symptom | Cause | Fix |
| --- | --- | --- |
| The change never appears on an existing database | The migration file name was changed after it was applied | Restore the name, or add a new migration with a later timestamp |
| The migration does not run at all | The file name sorts before an applied one | Use a later timestamp |
| The migration runs twice | It is not idempotent | Check with `GetByName` (or `FindCollectionByNameOrId`) before adding |
| `collection not found` in the migration | A relation field references a collection that is created later | Create the collections in dependency order |
| The frontend cannot read the new field | The field is not on the frontend-facing struct | Add it to the struct in `types.go` and to `toJob` |
| The frontend can read a secret | A rule was added to `app_settings` | Remove the list and view rules |

## Reference

- `internal/pocketbase/migrations/1730000000_init.go` — the shipped schema, and the patterns for each field type.
- [The architecture note](../../../docs/architecture.md#the-migrations-are-the-schema) — why the schema lives here.
- `internal/pocketbase/settings_test.go` — a test that bootstraps a throwaway database.
