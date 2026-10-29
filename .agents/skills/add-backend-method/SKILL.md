---
name: add-backend-method
description: >
  Add a backend method to the Wails service and call it from the React frontend,
  end to end: write the Go method on AppService, regenerate the TypeScript
  bindings, add a typed wrapper, use it in a component, and test both sides.
  Use when the frontend needs new backend behaviour, when a component has to
  reach the database or a remote API, or when a bound method's signature
  changes. Do not use for pure frontend work that needs no backend call.
metadata:
  owner: "incurl"
  version: "1.0.0"
  last-reviewed: "2026-10-28"
---

# Add a backend method

The path from a Go method to a React call. Four files, in this order. Skipping a step breaks the build in a way that is easy to
misread.

## Ground Rules

- **The frontend never talks to PocketBase or a remote API directly.** If the frontend needs data, add a bound method.
- **Never hand-edit `frontend/bindings/`.** It is generated and gitignored.
- **Never import a generated file from a component.** Use the wrapper in `frontend/src/bindings/index.ts`.
- **A bound method must not return a secret.** The API key is readable only in Go.
- Keep the domain logic in `internal/runner/` or a new package, not in `internal/bindings/service.go`. The service is a thin
  adapter over the packages below it.

## 1. Write the Go method

Add it to `internal/bindings/service.go`, on `AppService`. The service is a thin adapter: it validates the input, calls the
package that owns the behaviour, and returns the result.

```go
// CountArtifacts returns how many artifacts the application holds.
func (s *AppService) CountArtifacts() (int, error) {
    artifacts, err := s.jobs.ListArtifacts("", 0)
    if err != nil {
        return 0, err
    }
    return len(artifacts), nil
}
```

Rules for the signature:

| Rule | Why |
| --- | --- |
| Return `(T, error)`, never a bare `T` | The generator maps the error onto a rejected promise. |
| Use JSON-tagged structs for anything but a primitive | The generator emits a TS interface from the struct. |
| Use `map[string]any` for open-ended data | It maps to `Record<string, unknown>`. |
| Take and return plain values | A `*core.Record` has no useful generated type. Convert it first. |
| Never return a secret | `GetAPIKey` reads it; nothing returns it. |

If the behaviour belongs to the queue, add it to `internal/jobs/manager.go` and call that from the service. Do not put the
logic in the service.

## 2. Regenerate the bindings

```bash
PATH="$HOME/go/bin:$PATH" wails3 task common:generate:bindings
```

The task is namespaced `common:` — see `AGENTS.md`. The output goes to `frontend/bindings/`, which is gitignored.

Check the output line: `Processed: N Packages, 1 Service, M Methods, ...`. The method count must have gone up by one. If it did
not, the method is not on `AppService` or is not exported.

## 3. Add the typed wrapper

Open `frontend/src/bindings/index.ts` and add a wrapper. The generated method is `CamelCase`; the wrapper is `camelCase` and
returns a typed promise.

```ts
/**
 * Counts the stored artifacts.
 */
export function countArtifacts(): Promise<number> {
  return AppServiceBindings.CountArtifacts() as unknown as Promise<number>;
}
```

Three things to get right:

- **`as unknown as` is required** when the generated type and the wrapper type differ. The generated bindings are JS with JSDoc
  types, so the compiler needs the cast.
- **Normalise the nullables here, not in the components.** The generator types a missing array as `string[] | null`. The backend
  always sends an array, so narrow it once at the boundary:
  ```ts
  export type Job = JobsModels.Job & {
    artifact_ids: string[];
    input_artifact_ids: string[];
  };
  ```
- **A job has two artifact lists.** `artifact_ids` are the outputs, `input_artifact_ids` are the inputs. A retry re-uses the
  inputs and leaves the outputs with the job that produced them.
- **Export the model types** the components need, from the same file.

## 4. Use it in a component

Import from `../../bindings`, never from `../../bindings/github.com/...`.

```tsx
const countQuery = useQuery({
  queryKey: ['artifact-count'],
  queryFn: () => api.countArtifacts(),
});
```

If the call mutates something, use a mutation and invalidate the affected queries:

```tsx
const deleteMutation = useMutation({
  mutationFn: (id: string) => api.deleteArtifact(id),
  onSuccess: () => {
    void queryClient.invalidateQueries({ queryKey: ['artifacts'] });
  },
  onError: (error: Error) => notify(error.message, 'error'),
});
```

## 5. Test both sides

**Go** — a test next to the code that owns the behaviour, not next to the service. The service is a thin adapter; test the
package.

```go
func TestCountArtifacts(t *testing.T) {
    m := newTestManager(t)
    // ... store an artifact, then assert the count.
}
```

**Frontend** — test the pure logic, not the component's rendering. Put the helper next to the component, export it, and test it
in a `*.test.ts` beside it:

```ts
import { describe, expect, it } from 'vitest';
import { formatDetail } from './ConsolePanel';
```

## 6. Verify

```bash
gofmt -l internal/ main.go main_test.go tools/     # must print nothing
go vet ./internal/... .
go test -race ./internal/... .
cd frontend && npm run typecheck && npm test && npm run build
```

`npm run typecheck` is the important one: it catches a wrapper whose cast does not match the generated signature, which is the
most common failure in this flow.

## Common mistakes

| Symptom | Cause | Fix |
| --- | --- | --- |
| `Property 'X' does not exist on type` | The wrapper was written before the bindings were regenerated | Regenerate, then write the wrapper |
| The generated method is missing | The Go method is unexported, or not on `AppService` | Export it and check it is on the service |
| `Cannot invoke an object which is possibly 'undefined'` | The generated call is nullable | Use `as unknown as` in the wrapper |
| The array is `undefined` in a component | The nullable was not normalised in the wrapper | Narrow it in `frontend/src/bindings/index.ts` |
| The build works but the app cannot reach the method | The method was added after the last build | Rebuild; the bindings are regenerated, but the binary is not |

## Reference

- [The binding layer](../../../docs/architecture.md#the-frontend-talks-to-go-not-to-the-database) — why the wrapper exists.
- `internal/bindings/service.go` — the service and its existing methods.
- `frontend/src/bindings/index.ts` — the wrapper module and the model exports.
