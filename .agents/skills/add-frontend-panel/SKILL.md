---
name: add-frontend-panel
description: >
  Add a panel to the React frontend: the component, its data fetching, its
  pure helpers, the styles, and the tests. Use when the application needs a
  new piece of UI, a new form, or a new view of the jobs, the artifacts or
  the settings. Do not use for a change that only touches the backend, and do
  not use for a change that only restyles an existing panel.
metadata:
  owner: "incurl"
  version: "1.0.0"
  last-reviewed: "2026-10-28"
---

# Add a frontend panel

A panel is one directory under `frontend/src/features/`, holding a component, its pure helpers and its tests. This skill
covers the pattern the existing panels follow.

## Ground Rules

- **One panel per concern.** `frontend/src/features/` holds `submit/`, `queue/`, `console/` and `settings/`. Add yours next to
  them.
- **Import the backend through `frontend/src/bindings/index.ts`,** never through a generated file.
- **Keep the pure logic out of the component.** Export it from the same file, test it in a `*.test.ts` beside it.
- **Express every size in `rem`.** The zoom factor scales the root font size; a `px` size does not scale.
- **Reuse the existing CSS classes** before adding a new one. The stylesheet defines a small vocabulary: `panel`,
  `panel-header`, `panel-body`, `field`, `field-label`, `field-hint`, `btn`, `btn-sm`, `btn-primary`, `btn-active`, `text-dim`.

## 1. The directory

```
frontend/src/features/<name>/
  ├── <Name>Panel.tsx      the component and its pure helpers
  └── <Name>Panel.test.ts  the helpers
```

The component name matches the directory: `JobQueue.tsx` in `queue/`, `ConsolePanel.tsx` in `console/`.

## 2. The component

Three parts: the props, the data, and the markup.

```tsx
/**
 * The artifact panel.
 *
 * Lists the stored artifacts and lets the user remove one.
 */

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import * as api from '../../bindings';
import type { Artifact } from '../../bindings';

interface ArtifactPanelProps {
  /** The job whose artifacts are shown. Empty means all of them. */
  jobId: string;
  /** Called when an artifact was removed. */
  onRemoved: (message: string) => void;
}

/**
 * Formats the size of an artifact for display.
 */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} kB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/**
 * The artifact panel.
 */
export function ArtifactPanel({ jobId, onRemoved }: ArtifactPanelProps) {
  const queryClient = useQueryClient();

  const artifactsQuery = useQuery({
    queryKey: ['artifacts', jobId],
    queryFn: () => api.listArtifacts(jobId, 50),
  });

  const deleteMutation = useMutation({
    mutationFn: (artifactId: string) => api.deleteArtifact(artifactId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['artifacts'] });
      onRemoved('The artifact was removed.');
    },
    onError: (error: Error) => {
      onRemoved(error.message);
    },
  });

  const artifacts = artifactsQuery.data ?? [];

  return (
    <section className="panel">
      <div className="panel-header">
        <span>Artifacts</span>
        <span className="text-dim">{artifacts.length}</span>
      </div>
      <div className="panel-body">
        {artifacts.length === 0 ? (
          <span className="text-dim">No artifacts yet.</span>
        ) : (
          artifacts.map((artifact) => (
            <div className="param-row" key={artifact.id}>
              <span>{artifact.name}</span>
              <span className="text-dim">{formatBytes(artifact.bytes)}</span>
              <button
                type="button"
                className="btn btn-sm"
                onClick={() => deleteMutation.mutate(artifact.id)}
              >
                ×
              </button>
            </div>
          ))
        )}
      </div>
    </section>
  );
}
```

### The data rules

- **A query key must include everything the query depends on.** `['artifacts', jobId]`, not `['artifacts']`. Otherwise the
  cache serves one job's artifacts for another.
- **A mutation invalidates the queries it affects.** Without the invalidation, the list keeps showing what was deleted.
- **The refetch interval is the safety net, not the mechanism.** The Wails event pushes the updates. Do not shorten the
  interval to work around a stale panel; check that the event is wired up in `frontend/src/app/App.tsx`.
- **Render from `query.data ?? []`,** never from `query.data.length`, so the first render does not crash.

## 3. Wire it into the shell

`frontend/src/app/App.tsx` owns the layout and the shared state. Add the panel to the rail, the canvas or the sidebar:

```tsx
import { ArtifactPanel } from '../features/artifacts/ArtifactPanel';
// ...
<aside className="app-rail">
  <SubmitPanel ... />
  <ArtifactPanel
    jobId={selectedJobId}
    onRemoved={(message) => notify(message)}
  />
</aside>
```

The shell already provides the query client, the toast helper and the event subscriptions. A panel should not create its own
`QueryClientProvider` or its own event subscription.

## 4. The styles

Reuse the existing classes. When you need a new one, add it to `frontend/src/styles.css` **in `rem`**:

```css
/* ---------- Artifacts ---------- */

.artifact-row {
  display: grid;
  grid-template-columns: 1fr auto auto;
  gap: 0.4615rem;
  align-items: center;
  font-size: 0.8462rem;
}
```

Do not hard-code a colour. The stylesheet defines the palette as custom properties: `--bg`, `--bg-raised`, `--bg-input`,
`--bg-hover`, `--border`, `--border-strong`, `--text`, `--text-muted`, `--text-dim`, `--accent`, `--accent-hover`, `--success`,
`--warning`, `--danger`, `--radius`, `--font`, `--mono`. Use them, so that a theme change stays in one place.

## 5. The tests

Test the **pure helpers**, not the rendering. The rendering is the framework's job; the helpers are yours.

```ts
import { describe, expect, it } from 'vitest';

import { formatBytes } from './ArtifactPanel';

describe('formatBytes', () => {
  it('reports the raw bytes below a kilobyte', () => {
    expect(formatBytes(512)).toBe('512 B');
  });

  it('reports the kilobytes', () => {
    expect(formatBytes(2048)).toBe('2.0 kB');
  });

  it('reports the megabytes', () => {
    expect(formatBytes(3 * 1024 * 1024)).toBe('3.0 MB');
  });
});
```

The test file is `*.test.ts` next to the component. It is picked up by `vitest.config.ts` and excluded from the production
`tsconfig.json`, but included in `tsconfig.test.json` — so `npm run typecheck` checks it and `npm run build` does not.

## 6. Verify

```bash
cd frontend
npm run typecheck   # the important one: it checks the component and the test
npm test
npm run build
```

`noUnusedLocals` and `noUnusedParameters` are on, so an unused import fails the build. That is deliberate: a panel that is
wired in but does not use its props is a bug.

## Common mistakes

| Symptom | Cause | Fix |
| --- | --- | --- |
| `Property 'X' does not exist` | A generated file was imported directly | Import from `../../bindings` |
| The list is stale after a delete | The mutation does not invalidate the query | Add `invalidateQueries` in `onSuccess` |
| One job's artifacts show for another | The query key omits the job id | Use `['artifacts', jobId]` |
| The build fails on an unused import | A prop or import was left behind | Remove it; the build is telling the truth |
| The panel does not scale with the zoom | A size is in `px` | Use `rem` |
| The component crashes on the first render | `query.data` was dereferenced | Render from `query.data ?? []` |

## Reference

- `frontend/src/features/console/ConsolePanel.tsx` — the reference for helpers, tests and the webview gotchas.
- `frontend/src/features/queue/JobQueue.tsx` — the reference for a list with per-row actions.
- `frontend/src/styles.css` — the class vocabulary and the palette.
- [The architecture note](../../../docs/architecture.md#the-zoom-scales-the-root-font-size) — why `rem` is required.
