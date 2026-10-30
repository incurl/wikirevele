# Porting the template to a new application

The checklist for taking Valkyrie to a new project. Do the steps in order: the later ones assume the earlier ones.

The fastest way to start is to copy the repository and rename it. Copying keeps the build tooling, the CI and the tests, which
are the parts that take the longest to get right.

```bash
git clone https://github.com/incurl/wikirevele.git my-app
cd my-app
git remote set-url origin https://github.com/incurl/my-app.git
```

Then work through the steps below.

---

## 1. Rename it

The name appears in several places, and missing one produces a binary that does not match its window title, or a package
named after the wrong project.

| What | Where |
| --- | --- |
| The module path | `go.mod` — `module github.com/incurl/<name>` |
| The product name and identifier | `build/config.yml` — `productName`, `productIdentifier`, `description`, `comments` |
| The app name and version in Go | `internal/bindings/service.go` — `AppName`, `AppVersion` |
| The binary and package name | `Taskfile.yml` — `APP_NAME` |
| The frontend package name | `frontend/package.json` — `name` |
| The macOS bundle | `build/darwin/Info.plist`, `build/darwin/Info.dev.plist` |
| The iOS bundle | `build/ios/Info.plist`, `build/ios/Info.dev.plist` |
| The Windows metadata | `build/windows/info.json` |
| The MSIX manifests | `build/windows/msix/template.xml`, `build/windows/msix/app_manifest.xml` |
| The NSIS installer | `build/windows/nsis/wails_tools.nsh` |
| The Linux package | `build/linux/nfpm/nfpm.yaml`, `build/linux/valkyrie.desktop` |
| The Xcode project | `build/ios/project.pbxproj` |

A blunt but effective approach for the bulk of it:

```bash
# In the copied repository, before the first build
grep -rl 'valkyrie\|Valkyrie' --exclude-dir=.git --exclude-dir=node_modules \
  --exclude-dir=bindings --exclude-dir=dist . \
  | xargs sed -i 's/valkyrie/<name>/g; s/Valkyrie/<Name>/g'
```

Then review the diff: `com.incurl.valkyrie` becomes `com.incurl.<name>`, which is usually what you want, but the bundle
identifier may need a different domain.

After renaming, regenerate the build assets, because the config file says they are derived from it:

```bash
PATH="$HOME/go/bin:$PATH" wails3 task common:generate:icons
PATH="$HOME/go/bin:$PATH" wails3 task linux:generate:dotdesktop   # if you are on Linux
```

Check that the binary and the window title agree:

```bash
PATH="$HOME/go/bin:$PATH" wails3 build
./bin/<name>
```

---

## 2. Write your runner

This is the actual work of the application, and it is the only part that is genuinely yours.

Create `internal/runner/<yourname>.go`.

**`internal/runner/example.go` is a compiled, tested reference.** Copy it rather than writing from scratch, so the API cannot
drift under you:

```bash
cp internal/runner/example.go internal/runner/myapp.go
```

Then edit it. The three things it demonstrates are the three things every runner must do:

```go
package runner

import (
    "context"
    "fmt"

    "github.com/incurl/wikirevele/internal/console"
    "github.com/incurl/wikirevele/internal/jobs"
    "github.com/incurl/wikirevele/internal/pocketbase"
)

// MyRunner talks to the remote API.
type MyRunner struct {
    settings *pocketbase.SettingsStore
    log      *console.Log
    client   *http.Client
}

func NewMyRunner(settings *pocketbase.SettingsStore, log *console.Log) *MyRunner {
    return &MyRunner{settings: settings, log: log, client: &http.Client{}}
}

func (r *MyRunner) Run(ctx context.Context, job jobs.Job) (*jobs.Result, error) {
    // 1. The secret is read here, in Go. The frontend never sees it.
    apiKey, found, err := r.settings.GetAPIKey()
    if err != nil {
        return nil, err
    }
    if !found {
        return nil, fmt.Errorf("no API key is configured. Add one in Settings.")
    }

    // 2. Honour the context, so that cancelling the job stops the work.
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // 3. A *Error carries a numeric code, which the UI can branch on.
    result, err := r.callTheAPI(ctx, apiKey, job.Prompt, job.Params)
    if err != nil {
        return nil, &jobs.Error{Code: 42, Message: "the API refused the request", Err: err}
    }

    return &jobs.Result{
        ArtifactIDs: result.ArtifactIDs,
        Payload:     map[string]any{"answer": result.Answer},
    }, nil
}
```

Then register it in `main.go`, replacing the echo runner:

```go
jobManager := jobs.NewManager(
    pbService.App(),
    jobs.WithLog(consoleLog),
    jobs.WithRunnerFunc(func() jobs.Runner {
        return runner.NewMyRunner(settings, consoleLog)
    }),
)
```

The event emitter is attached with `jobManager.SetEmitter(bridge)` once the Wails application exists, because it needs the
application. There is no option for it: one way to attach it is one fewer thing to get wrong.

**Keep the runner in `internal/runner/`.** Do not put domain logic in `internal/jobs/`: that package is the generic queue, and
anything domain-specific in it has to be removed again by the next application that uses the template.

### Storing an output file

If the runner produces a file, store it as an artifact rather than inlining the bytes:

```go
artifact, err := r.jobs.StoreArtifact(job.ID, jobs.ArtifactKindOutput, data, "result.png", nil)
if err != nil {
    return nil, err
}
```

The file is then served by PocketBase's own file route at `/api/files/artifacts/<id>/<name>`, from the UI's own origin.

---

## 3. Extend the schema

If your application needs a field the queue does not have, add it with a migration. **Never** edit an applied migration.

```go
// internal/pocketbase/migrations/1732000000_add_notes.go
package migrations

import "github.com/pocketbase/pocketbase/core"

func init() {
    registerMigration("1732000000_add_notes.go", func(app core.App) error {
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
    })
}
```

Rules that matter:

- The file name is the identity **and** the ordering key. It must sort after every applied migration, and it must never
  change once applied.
- Make it idempotent. Check before adding.
- New files go in the `migrations` package; the blank import in `internal/pocketbase/service.go` already covers it.
- If you add a collection, decide its access rules deliberately. Read it public, write it closed — that is the pattern the
  frontend expects. `app_settings` is the exception: it gets no rule at all.

---

## 4. Extend the frontend

The panels live in `frontend/src/features/`, one directory per concern. Add yours next to them.

Two rules:

1. **Call the backend through `frontend/src/bindings/index.ts`,** never through a generated file. Add a wrapper there for each
   new method:

   ```ts
   export function myNewCall(arg: string): Promise<string> {
     return AppServiceBindings.MyNewCall(arg) as unknown as Promise<string>;
   }
   ```

2. **Keep the pure logic out of the component.** Put the formatters and the helpers in the same file as the component, export
   them, and unit test them in a `*.test.ts` next to it. `ConsolePanel.tsx` is the reference.

Wire the panel into `frontend/src/app/App.tsx`, which owns the data fetching and the event subscriptions.

If you need a new backend method, add it to `AppService` in `internal/bindings/service.go` first, then regenerate:

```bash
PATH="$HOME/go/bin:$PATH" wails3 task common:generate:bindings
```

---

## 5. Keep the tooling

Do not delete these; they are the parts that take the longest to rebuild.

| Keep | Why |
| --- | --- |
| `.github/workflows/ci.yml` | Format, vet, race tests, e2e, typecheck, tests and the licence audit on every push. |
| `tools/license-audit.py` | Fails the build on a copyleft dependency. |
| `tools/e2e-check/main.go` | A live run of the whole backend, without a GUI. Update the assertions for your runner. |
| `internal/jobs/manager_test.go` | The queue's behaviour is pinned: cancellation, deletion, failure. Do not weaken these. |
| `internal/pocketbase/settings_test.go` | The encryption is pinned. |
| `frontend/src/**/*.test.ts` | The pure helpers are pinned. |

Update `tools/e2e-check/main.go` so it exercises **your** runner rather than the echo one. It is the fastest end-to-end
signal you have.

---

## 6. Check it

```bash
# Go
gofmt -l internal/ main.go main_test.go tools/   # must print nothing
go vet ./internal/... .
go test -race ./internal/... .
go run ./tools/e2e-check

# Frontend
cd frontend && npm run typecheck && npm test && npm run build

# Audits
python3 tools/license-audit.py

# The real thing
PATH="$HOME/go/bin:$PATH" wails3 build && ./bin/<name>
```

Then run the app and click through it: submit a job, watch it move through the queue, cancel it, retry it, delete it. The
queue is the heart of the application; if it behaves correctly in the window, it behaves correctly.

---

## 7. Ship it

```bash
PATH="$HOME/go/bin:$PATH" wails3 task package        # the platform packages
PATH="$HOME/go/bin:$PATH" wails3 task build:server   # the headless binary
PATH="$HOME/go/bin:$PATH" wails3 task build:docker   # the container image
```

Add a release workflow when you need one, and a signing setup with `wails3 setup signing`.
