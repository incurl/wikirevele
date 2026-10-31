# Audit — 2026-10-28

A full review of the application, carried out against the running code rather than by inspection. Every finding below was
reproduced with a test or a live request; the reproductions are described so they can be re-run.

**Scope**: every Go package under `internal/`, the root `main.go`, the React frontend under `frontend/src`, the build
tooling, the CI and the documentation.

**Verdict**: the application is sound for a single-user desktop application. The **critical** issue is confined to server mode,
which is currently an opt-in build. The **high** issue is a data-loss bug in `RetryJob` that affects every deployment. The
**medium** issues are robustness gaps that surface under load or after a crash.

---

## Status

Seven of the eight findings were fixed on 2026-10-28, in the order below. **Finding 1 (the server-mode authentication) is
still open** — it needs a decision about whether the REST API should be mounted in server mode at all, which is a design
question rather than a bug.

| # | Severity | Finding | Status |
| --- | --- | --- | --- |
| 1 | **Critical** | Server mode exposes every job's prompt and params over HTTP with no authentication | **Open** |
| 2 | **High** | `RetryJob` steals the original job's artifacts | **Fixed** |
| 3 | **High** | A crash leaves jobs stuck in `running` forever | **Fixed** |
| 4 | **Medium** | No job timeout | **Fixed** |
| 5 | **Medium** | No graceful shutdown | **Fixed** |
| 6 | **Medium** | `ListJobs` is an N+1 query | **Fixed** |
| 7 | **Low** | A failed settings save is silent | **Fixed** |
| 8 | **Low** | The key file's mode is not re-asserted | **Fixed** |

The findings below are kept as they were written, so that the reasoning behind each one stays visible. Where a fix changed
the shape of a problem, the fix is described at the end of the section.

---

## Summary

| # | Severity | Area | Finding |
| --- | --- | --- | --- |
| 1 | **Critical** | Security | Server mode exposes every job's prompt and params over HTTP with no authentication |
| 2 | **High** | Correctness | `RetryJob` steals the original job's artifacts; deleting the retry then deletes the original's files |
| 3 | **High** | Robustness | A crash leaves jobs stuck in `running` forever: never executed, never deletable |
| 4 | **Medium** | Robustness | No job timeout: two hung runners halt the whole application |
| 5 | **Medium** | Robustness | No graceful shutdown: in-flight jobs are killed mid-write |
| 6 | **Medium** | Performance | `ListJobs` is an N+1 query, polled every 5 seconds |
| 7 | **Low** | UX | A failed settings save produces an unhandled rejection and no message |
| 8 | **Low** | Robustness | The key file's mode is not re-asserted if it already exists with looser permissions |

---

## 1. Critical — server mode has no authentication

**Where**: `internal/pocketbase/migrations/1730000000_init.go` (the access rules), `main.go` (the mount).

The `jobs` and `artifacts` collections carry public read rules:

```go
jobs.ListRule = types.Pointer("")
jobs.ViewRule = types.Pointer("")
```

In the desktop app this is harmless: the asset server is in-process and no TCP port is bound. In **server mode** the same
handler is served over HTTP, and the collection rules make every record readable by anyone who can reach the port.

**Reproduced.** Starting the server and requesting the collection with no credentials returns the records in full:

```console
$ curl -s 'http://localhost:18099/api/collections/jobs/records' | jq '.items[0]'
{
  "id": "njfvgpwlpil929r",
  "title": "secret job",
  "prompt": "my password is hunter2 and my key is sk-live-ABCDEF",
  "params": { "token": "sk-live-ABCDEF" },
  "status": "pending"
}
```

The write gate holds — `POST` returns 403 — and `app_settings` returns 403, and the admin UI is not mounted. So the secrets
themselves are safe. **But every prompt and every parameter is readable**, and the prompts are exactly where a user pastes
their API keys, tokens and private content.

**Amplifier**: `build/docker/Dockerfile.server` sets `WAILS_SERVER_HOST=0.0.0.0`, so the shipped container image exposes this
to the whole network by default.

There are no CORS headers, so a *browser* on another origin cannot read the response. That is not a mitigation: any local
process can read it, any host on the network can read it when the container is deployed, and DNS rebinding defeats the origin
check in several common setups.

**Fix.** Either close the read rules and have the frontend read through the Wails bindings only (which is the design the
frontend already follows), or require a bearer token in server mode. The first is simpler and matches the architecture: the
frontend never uses the REST API today, so the public rules are not needed.

Recommended: make the read rules **superuser-only** (`types.Pointer("")` → unset) and drop the REST mount in server mode
unless the operator explicitly opts in.

> **Still open.** The fix needs a decision about the shape of server mode, so it was left for a separate change.

## 2. High — `RetryJob` steals the original job's artifacts

**Where**: `internal/bindings/service.go:142`.

```go
request := jobs.Request{
    Title:            job.Title,
    Prompt:           job.Prompt,
    Params:           job.Params,
    InputArtifactIDs: job.ArtifactIDs,   // <-- the OUTPUTS of the original job
}
```

`job.ArtifactIDs` holds every artifact the original job produced. Passing them as `InputArtifactIDs` makes `linkArtifacts`
**re-assign** each artifact's `job` relation to the new job.

**Reproduced**, in three steps:

```console
before retry: original artifact_ids = [55wbq1gj4zx3f2u]
after retry:  original artifact_ids = []
CONFIRMED: the original job lost its artifacts
CONFIRMED: deleting the retry also deleted the original's artifact
```

The consequences compound:

1. The original job silently loses its outputs.
2. Deleting the retry job deletes the artifacts — **which are now owned by the retry** — destroying the original's files.
3. The original job's `artifact_ids` is empty, so the UI shows it as having produced nothing.

This is **data loss**, reachable from a single button in the UI.

**Fix.** The retry must not re-link the original's artifacts. Either drop `InputArtifactIDs` from the retry entirely, or
collect only the artifacts whose `kind` is `input` and copy them (rather than move them) to the new job.

> **Fixed**, in two layers, because either one alone was enough to lose the data:
> - The bindings re-use only the **inputs** of the job they repeat (`job.InputArtifactIDs`).
> - The manager **copies** an artifact that already belongs to another job, instead of moving it. An unowned artifact is
>   claimed, as before.
>
> A job now tracks its inputs and its outputs separately, so the distinction survives in the record. The regression test is
> `internal/bindings/service_test.go::TestRetryJobKeepsTheOriginalOutputs`.

## 3. High — a crash leaves jobs stuck in `running` forever

**Where**: `internal/jobs/manager.go:355`.

The manager holds no list of persisted jobs at startup, and `run` is only ever called from `Submit`. A job that was `pending`
or `running` when the process died is therefore never resumed — and never fails.

**Reproduced**: a record forced to `running` is listed as such, no runner is invoked, and `DeleteJob` refuses to remove it:

```console
CONFIRMED: the orphaned job is listed as "running" and nothing will ever run it
the orphaned job cannot be deleted: job "w56qv12c30gia71" is still running and cannot be deleted
```

The user is stuck: the job shows as running forever, and the delete button refuses.

**Fix.** On startup, reconcile the persisted jobs: mark every `pending` or `running` record as `failed` with a "interrupted
by a restart" message, or re-enqueue it. The first is simpler and safer.

> **Fixed.** `Manager.Reconcile()` marks the interrupted jobs as failed, with the explanation *"The job was interrupted by a
> restart of the application."*, and it runs at startup in `main.go`. The reconciliation is idempotent. The regression test is
> `internal/jobs/reconcile_test.go`.

## 4. Medium — no job timeout

**Where**: `internal/jobs/manager.go:378`.

The job's context is cancelled only by an explicit user action. A runner that blocks forever holds its slot.

**Reproduced**: with the default concurrency of 2, two hung runners mean a third job never starts:

```console
CONFIRMED: the third job is stuck in the queue behind two hung jobs
```

The application is now completely unable to do work, and the user has no way to clear it except cancelling the two hung jobs —
which only works if the runner honours the context.

**Fix.** Add a per-job timeout, configurable, applied with `context.WithTimeout` around the runner call. The runner then
receives a context that expires on its own.

> **Fixed.** `WithTimeout` sets the limit and `DefaultTimeout` (30 minutes) applies it out of the box. The runner receives a
> context that expires on its own, and the job is marked as failed with the limit named. The regression test is
> `internal/jobs/lifecycle_test.go::TestJobTimeoutStopsAHungRunner`.

## 5. Medium — no graceful shutdown

**Where**: `internal/jobs/manager.go` — the public API has no `Shutdown` method.

In-flight jobs run against `context.Background()`, so nothing cancels them when the window closes. The process dies mid-write,
which is the same situation as finding 3, except it happens on every normal quit.

**Fix.** Add `Manager.Shutdown(ctx)`: cancel the running jobs, wait for the goroutines to finish (a `sync.WaitGroup`), and
call it from Wails' `OnShutdown`.

> **Fixed.** `Manager.Shutdown(ctx)` and `ShutdownWithTimeout(timeout)` exist. The Wails `OnShutdown` hook calls it with
> `jobs.ShutdownTimeout`. The manager's root context is the parent of every job context, so the shutdown stops them all.
> The regression test is `internal/jobs/lifecycle_test.go::TestShutdownCancelsTheRunningJobs`.

## 6. Medium — `ListJobs` is an N+1 query, polled every 5 seconds

**Where**: `internal/jobs/manager.go:315` and `frontend/src/app/App.tsx:56`.

`ListJobs` fetches the records and then calls `toJob` on each, and `toJob` runs a second query to collect the artifacts. The
frontend polls this every 5 seconds.

**Measured**, with the artifact query in place:

| Jobs in the table | `ListJobs(200)` |
| --- | --- |
| 10 | 1.2 ms |
| 50 | 4.8 ms |
| 100 | 12.9 ms |
| 200 | 14.0 ms |

It scales linearly, and 201 queries run for a 200-job page. At the default scale this is invisible, but it is the wrong shape:
the cost grows with the table, not with the page.

**Fix.** Fetch the artifacts for the whole page in one query (`job IN (...)`) and index them by job id. One extra query per
page instead of one per job.

> **Fixed.** `Manager.toJobs` converts a whole page at once and fetches the artifacts with a single `dbx.In` query, through
> `app.RecordQuery` so that the records are scanned with their collection context. Measured on the same machine:
>
> | Jobs in the table | Before | After |
> | --- | --- | --- |
> | 50 | 4.8 ms | 0.9 ms |
> | 100 | 12.9 ms | 2.0 ms |
> | 200 | 14.0 ms | 2.8 ms |
>
> The regression test is `internal/jobs/listjobs_test.go`.

## 7. Low — a failed settings save is silent

**Where**: `frontend/src/app/App.tsx:232`.

```tsx
void api.saveCredentials(apiKey, baseUrl).then(() => { ... });
```

There is no `.catch()`. If the save fails, the promise rejects unhandled: no toast, no error, and the dialog stays open with
no explanation. The same applies to `saveUI`.

**Fix.** Use the `useMutation` pattern the other actions already use, which has an `onError` that notifies.

> **Fixed.** The credentials and the interface preferences are saved through mutations, so a failure produces a toast instead
> of an unhandled rejection. There is no bare promise chain left in the frontend.

## 8. Low — the key file's mode is not re-asserted

**Where**: `internal/pocketbase/crypto.go:46`.

`os.WriteFile(keyPath, key, 0o600)` sets the mode **when it creates the file**. If the file already exists with looser
permissions, the mode is left alone. The data directory is created with `0o700`, which mitigates this in practice, but the key
file's own mode should not depend on that.

**Fix.** `os.Chmod(keyPath, 0o600)` after writing, or stat and correct the mode.

> **Fixed.** An existing key file has its mode re-asserted when the cipher is created. The regression test is
> `internal/pocketbase/crypto_test.go::TestCipherSecuresAnExistingKey`.

---

## What is sound

These were checked and hold up:

| Area | Finding |
| --- | --- |
| **Encryption** | AES-256-GCM with a fresh random nonce per encryption, from `crypto/rand`. The nonce is prepended to the ciphertext, so it is never reused. Correct. |
| **Secret exposure** | The API key is never returned to the frontend. `app_settings` has no access rule, so the HTTP API cannot reach it. The admin UI is not mounted. The debug console shows a fingerprint, not the value. |
| **Filename handling** | PocketBase's `normalizeName` strips invalid characters and path separators, so a hostile filename cannot escape the storage directory. The artifact URL is built from the sanitised stored name. |
| **Concurrency** | 50 concurrent submits under `-race`: no race, no lost jobs. `WithConcurrency` was measured and honours its limit exactly. Attaching the emitter while jobs run is race-free (the emitter is called outside the lock). |
| **Write gate** | `POST` to `jobs` returns 403. The `create`/`update`/`delete` rules are closed, so every write goes through the Go services. |
| **Cancellation** | A cancelled job keeps the `cancelled` status rather than being reported as failed. Correct. |
| **Deletion** | A running job cannot be deleted, so it cannot write to the database after removal. Its artifacts are deleted with it, so no orphan files are left. |
| **Event bridge** | The emitter is invoked outside the console's lock, so a slow Wails emit cannot block the log. |
| **Frontend cleanup** | Both Wails event subscriptions are unsubscribed on unmount. |
| **Data dir** | Created with `0o700`; the key file with `0o600` on creation. |

---

## Test coverage

| Package | Coverage | Notes |
| --- | --- | --- |
| `internal/console` | 83.3% | Well covered. |
| `internal/pocketbase` | 69.4% | The schema and the encryption are covered. |
| `internal/jobs` | 67.7% | The lifecycle is covered; the manager options are not. |
| `internal/runner` | 63.0% | Both runners are covered. |
| `internal/bindings` | 40.7% | The retry path is covered now, which is what finding 2 needed. |
| `internal/pocketbase/migrations` | 0.0% | Covered indirectly by the schema test. |

**`internal/bindings` having no tests was the root cause of finding 2.** The `RetryJob` logic is a binding-layer concern, and
the binding layer was untested. A test on `AppService.RetryJob` would have caught it immediately, and there is one now.

The numbers above are a snapshot of the day of the audit. Re-run `go test -cover ./internal/...` for the current ones.

---

## Audit — 2026-11-01

The first audit found eight issues. This one covers the ETL pipeline, which did not exist when that audit was written, and
the parts of the frontend that were rewritten around it.

Three findings, and two of them were real bugs that the test suite was not catching.

| # | Severity | Area | Finding | Status |
| --- | --- | --- | --- | --- |
| 9 | **High** | ETL pipeline | The events of a finished run are lost: the counters of the run are zero | **Fixed** |
| 10 | **High** | ETL pipeline | A run never ends if a child of the worker holds the pipe of the events open | **Fixed** |
| 11 | **Medium** | ETL pipeline | The progress of a run is read without the lock, and the race detector finds it | **Fixed** |

### 9. High — the events of a finished run are lost

The supervisor read the events of the worker on the pipe of `os/exec`, and it waited for the worker before it read them.
`Cmd.Wait` closes that pipe, so the reader got `file already closed` and read nothing at all.

The run still finished, and it still reported `succeeded`, but every counter was zero: the number of items, the number of
lines and the number of properties. The interface showed a run that had done nothing, and the history recorded it.

It was intermittent, and the reason was the scheduler: the reader goroutine sometimes reached the pipe before the wait
closed it. The race detector made it worse, because it slows the process down enough for the reader to win more often.

The fix was to own the pipe. The launcher now creates the pipe with `os.Pipe` and hands the read end to the supervisor.
The wait does not touch it, the reader gets every line, and the supervisor closes it when the run is over.

The order matters and is not obvious: the pipe cannot be closed before the reader is done, because the lines that are
still in it would be lost, and the counters would be wrong again. The pipe is closed only after the reader has finished,
or after a timeout when something holds it open.

### 10. High — a run never ends if a child holds the pipe open

Fixing finding 9 in the other direction exposed this one. If the supervisor waits for the end of the pipe before it waits
for the worker, and a child of the worker holds the pipe open, the run never ends.

The worker is the ETL of this binary, and it does not fork, so this was not reachable in the application. It was reachable
in the tests, where the fake worker is a shell: `sh -c "sleep 30"` forks, and the child inherits the standard output and
holds the pipe open for thirty seconds after the worker is gone. A cancelled run waited for that child.

The order is now: wait for the worker, then wait for the reader, with a timeout. The reader ends when the pipe reaches its
end, which happens when the worker is gone and nothing else holds it open. When something does hold it, the pipe is closed
after `EventDrainTimeout` and the reader stops with it, so a grandchild cannot keep a run alive.

### 11. Medium — the progress of a run is read without the lock

`Start` returned `r.progress` and published it without holding the lock, while the reader of the events was already
writing it. The race detector found it once the two earlier failures were out of the way.

Every access to `r.progress` is now under the lock.

### What was done

The three findings were fixed together, because the first fix was not right on its own: reordering the wait alone traded a
lost run for a run that never ends. The pipe is owned by the supervisor, the reader is bounded by the close, and the
progress is read under the lock.

Three tests pin the behaviour, in `internal/etlpipeline/regression_test.go`. They were checked against the old code:
the first fails with the counters at zero, the second waits for the grandchild and times out. They are not tests that
pass against the bug they describe.

### What was found and not changed

- **`etlpipeline.Service` has no test of its own.** The supervisor has a good one, and the service is covered indirectly
  through `internal/bindings`. The service is the layer that writes the history and calls the importer, and it is the layer
  with the most to lose.
- **The runs are never removed from the supervisor.** `s.runs` grows by one entry per run, and the map is only read
  afterwards. A long session with many runs holds every one of them.

### What was found and fixed on the way

- **The property of a claim filter was not escaped.** The value is quoted and escaped, the property was not: it was put
  into the filter expression as it is. The filter parser refuses every injection that was tried — `||`, `or`, `&`, a
  quoted string and a path — so it was not reachable as an injection, but a property that the parser refuses was an error
  the user saw as a failed page rather than as a rejected filter. A property is now checked against the shape of the
  names of the properties of Wikidata, and it is refused before it reaches the expression.
- **The stylesheet had three `1px` borders.** The zoom scales the root font size, and a `px` border does not scale with
  it. The rest of the stylesheet is in `rem`, and the three borders are too now.

---

## What was done

The findings were fixed in the order 2, 3, 4, 5, 6, 7, 8. Finding 1 was left open, because it needs a decision about the
shape of server mode.

**Finding 2** was fixed in two layers, because either one alone was enough to lose the data. The bindings re-use only the
inputs of the job they repeat, and the manager copies an artifact that already belongs to another job instead of moving it.
A job now tracks its inputs and its outputs separately.

**Findings 4 and 5** were done together: a `Manager` that owns a root context, a `sync.WaitGroup` and a timeout covers both,
and gives the manager the lifecycle it lacked.

**Finding 6** needed a raw `dbx.In` query, because the PocketBase search filter has no `IN` operator. It goes through
`app.RecordQuery`, not `app.DB()`, so the records are scanned with their collection context.

### The remaining work

**Finding 1** — the server-mode authentication — is the one thing left. It is a design decision rather than a bug:

- *Option A*: close the read rules on `jobs` and `artifacts`. The frontend never uses the REST API today (it goes through
  the Wails bindings), so this costs nothing and the server mode stops exposing the data. The file route needs the same
  treatment, which means the frontend would fetch the files through a bound method instead of by URL.
- *Option B*: keep the rules and require a bearer token in server mode. This keeps the REST API usable, which matters if an
  application wants a separate web client.

Option A is the simpler one and it matches the architecture. Option B is the one to pick if the REST API is part of the
product.
