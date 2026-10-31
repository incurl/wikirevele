---
name: new-application
description: >
  Start a new desktop application from the WikiRevele template: copy the
  repository, rename every identifier, write the first runner, and verify
  that the whole pipeline works end to end before any feature work starts.
  Use when the user asks to start a new project from this template, to
  scaffold a new application, or to set up a new product on this stack.
  Do not use for adding a feature to an existing application.
metadata:
  owner: "incurl"
  version: "1.0.0"
  last-reviewed: "2026-10-28"
---

# New application

Take the template to a working new application. The goal of this skill is a running app that submits a job and gets a result
back — nothing more. Features come after that.

## Ground Rules

- **Copy the repository; do not re-create it.** The build tooling, the CI and the tests are the parts that take the longest to
  get right.
- **The queue is generic; the runner is yours.** The domain belongs in `internal/runner/`, never in `internal/jobs/`.
- **Do not add a dependency before the first job works.** The template ships with three runtime dependencies on purpose.
- **Keep the checks green at every step.** A broken baseline makes every later failure ambiguous.

## 1. Copy and rename

```bash
cd /home/peter/GolandProjects
git clone https://github.com/incurl/wikirevele.git <name>
cd <name>
git remote set-url origin https://github.com/incurl/<name>.git
```

Rename the identifiers. The blunt version, followed by a careful review:

```bash
grep -rl 'wikirevele\|WikiRevele' --exclude-dir=.git --exclude-dir=node_modules \
  --exclude-dir=bindings --exclude-dir=dist . \
  | xargs sed -i 's/wikirevele/<name>/g; s/WikiRevele/<Name>/g'
```

Then review the diff. The bundle identifier becomes `com.incurl.<name>`, which is usually right, but a real product may want a
different domain. Check these by hand:

| What | Where |
| --- | --- |
| The module path | `go.mod` |
| The bundle identifier | `build/config.yml`, `build/darwin/Info.plist`, `build/ios/Info.plist` |
| The product name | `build/config.yml`, `internal/bindings/service.go` (`AppName`) |
| The description | `build/config.yml`, `internal/bindings/service.go`, `frontend/index.html` |
| The app name in the UI | `frontend/src/app/App.tsx` |

Regenerate the derived assets, because the config file says they come from it:

```bash
PATH="$HOME/go/bin:$PATH" wails3 task common:generate:icons
PATH="$HOME/go/bin:$PATH" wails3 task linux:generate:dotdesktop
```

## 2. Establish a green baseline

Before writing a line of application code, prove the template works:

```bash
gofmt -l internal/ main.go main_test.go tools/     # must print nothing
go vet ./internal/... .
go test -race ./internal/... .
go run ./tools/e2e-check
cd frontend && npm install && npm run typecheck && npm test && npm run build && cd ..
python3 tools/license-audit.py
PATH="$HOME/go/bin:$PATH" wails3 build
```

Every one of these must pass. If something fails, fix it **before** writing the application: a broken baseline makes every
later failure ambiguous.

Then run the binary and click through it once:

```bash
./bin/<name>
```

Submit a job, watch it move through the queue, cancel it, retry it, delete it. The queue is the heart of the application.

## 3. Write the first runner

Create `internal/runner/<yourname>.go`. Start with the simplest thing that can work — a stub that returns a fixed result — and
grow it into the real backend call.

**`internal/runner/example.go` is a compiled, tested reference.** Copy it rather than writing from scratch, so the API cannot
drift under you:

```bash
cp internal/runner/example.go internal/runner/myapp.go
```

Then edit it. The three things it demonstrates are the three things every runner must do:

```go
func (r *MyRunner) Run(ctx context.Context, job jobs.Job) (*jobs.Result, error) {
    // 1. Honour the context, so that cancelling the job stops the work.
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // 2. Read the credentials in Go. The frontend never sees them.
    apiKey, found, err := r.settings.GetAPIKey()
    if err != nil {
        return nil, err
    }
    if !found {
        return nil, fmt.Errorf("no API key is configured. Add one in Settings.")
    }

    // 3. Return a *jobs.Error when you want to attach a numeric code
    //    the UI can branch on.
    if err := r.callTheAPI(ctx, apiKey, job.Prompt, job.Params); err != nil {
        return nil, &jobs.Error{Code: 42, Message: "the API refused the request", Err: err}
    }

    return &jobs.Result{Payload: map[string]any{"answer": "ok"}}, nil
}
```

Register it in `main.go`, replacing the echo runner:

```go
jobManager := jobs.NewManager(
    pbService.App(),
    jobs.WithLog(consoleLog),
    jobs.WithTimeout(5 * time.Minute),   // if the default 30 minutes is too long
    jobs.WithRunnerFunc(func() jobs.Runner {
        return runner.NewMyRunner()
    }),
)
```

**Honour `ctx`.** It is the only way a cancelled job stops. A runner that ignores it keeps a slot in the bounded queue, and
every later job waits behind it. The context also carries the job's time limit, so a runner that blocks forever is stopped
even if the user does nothing.

The emitter is attached separately with `jobManager.SetEmitter(bridge)` once the Wails application exists.

**The manager already handles the rest.** A job interrupted by a crash is reconciled at startup, and the Wails `OnShutdown`
hook stops the running jobs when the window closes. Neither needs anything from the runner.

## 4. Make the parameters real

The job params are a `map[string]any`. The frontend sends them; the runner receives them. Give them a shape and validate it on
both sides.

The frontend side — a panel in `frontend/src/features/`:

```tsx
const [model, setModel] = useState('default');
// ...
submitMutation.mutate({ prompt, params: { model } });
```

The backend side — validate in the runner, not in the service:

```go
model, _ := job.Params["model"].(string)
if model == "" {
    model = "default"
}
```

**Validate in the runner.** The service is a thin adapter; the runner is where the domain rules live.

## 5. Add the settings

If the application needs a credential, it is already wired:

- `internal/pocketbase/settings.go` — `SetAPIKey`/`GetAPIKey` and `SetBaseURL`/`GetBaseURL` are there.
- The settings dialog already has the fields.
- The secret is encrypted and write-only from the UI's point of view.

Rename the setting keys if they are not generic enough:

```go
const (
    SettingAPIKey = "app.api_key"
    SettingBaseURL = "app.base_url"
)
```

The frontend only ever learns whether a key is present (`has_api_key`). Never return the key itself.

## 6. Update the end-to-end check

`tools/e2e-check/main.go` is the fastest end-to-end signal. It currently exercises the stub runner. Point it at your runner
and assert what your application actually produces:

```go
job, err := manager.Submit(jobs.Request{
    Title:  "a test job",
    Prompt: "a red bicycle",
    Params: map[string]any{"model": "default"},
})
// ...
if received.Prompt != "a red bicycle" {
    fmt.Printf("   FAIL: the runner received %q\n", received.Prompt)
    os.Exit(1)
}
```

## 7. Decide what the scaffold keeps

The scaffold ships panels that show the plumbing. As the application grows, some of them stop earning their place:

| Panel | Keep it when |
| --- | --- |
| Submit | Always — it is the entry point. |
| Queue | Always — the job lifecycle is the core. |
| Console | While you are debugging the backend. It is genuinely useful in production too. |
| Settings | While the application has settings. |

Delete a panel by removing its directory and its use in `frontend/src/app/App.tsx`. Do not leave dead imports: the TypeScript
build has `noUnusedLocals`, so it will fail rather than let them slide.

## 8. Verify and commit

```bash
gofmt -l internal/ main.go main_test.go tools/
go vet ./internal/... .
go test -race ./internal/... .
go run ./tools/e2e-check
cd frontend && npm run typecheck && npm test && npm run build && cd ..
python3 tools/license-audit.py
PATH="$HOME/go/bin:$PATH" wails3 build
```

Commit the rename and the first runner as two commits, so the rename stays reviewable:

```bash
git add -A && git commit -m "chore: rename the template to <name>"
git add internal/runner/ main.go && git commit -m "feat: the <name> runner"
```

## Reference

- [The porting checklist](../../../docs/porting.md) — the same procedure, in more detail.
- [The architecture note](../../../docs/architecture.md) — why the queue is generic.
- `internal/runner/echo.go` — the reference runner.
- The `add-backend-method` and `add-migration` skills, for the follow-up work.
