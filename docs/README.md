# Documentation

| Document | Read it for |
| --- | --- |
| [`architecture.md`](architecture.md) | Why the pieces are shaped the way they are. Read it before changing the structure. |
| [`porting.md`](porting.md) | The checklist for taking the template to a new application. |
| [`etl-manual.md`](etl-manual.md) | The operator's guide to the Wikidata ETL worker: how to run it, what it writes, and what to do when it misbehaves. |
| [`wikidata.md`](wikidata.md) | The dataset pipeline: what the ETL worker trims, what the application imports, and the boundary between them. |
| [`troubleshooting.md`](troubleshooting.md) | The failures that cost time, and what actually fixes them. |
| [`audit.md`](audit.md) | The 2026-10-28 audit: the defects, the reproductions, and what was fixed. |
| [`code-review.md`](code-review.md) | The 2026-10-28 code review: the trade-offs that were deliberately kept, and why. |

The repository-root files carry the rest:

| File | Read it for |
| --- | --- |
| [`../AGENTS.md`](../AGENTS.md) | The working rules, the commands that work here, the change routing and the known rough edges. |
| [`../CONTEXT.md`](../CONTEXT.md) | The vocabulary: what each term means, and what it does not mean. |
| [`../README.md`](../README.md) | What the scaffold contains and how to make it yours. |
| [`../.agents/skills/`](../.agents/skills) | Task-scoped procedures with a `SKILL.md` each. |

## The skills

| Skill | Use it for |
| --- | --- |
| [`new-application`](../.agents/skills/new-application/SKILL.md) | Starting a new project from this template. |
| [`add-backend-method`](../.agents/skills/add-backend-method/SKILL.md) | Adding a Go method the frontend can call. |
| [`add-migration`](../.agents/skills/add-migration/SKILL.md) | Changing the PocketBase schema. |
| [`add-frontend-panel`](../.agents/skills/add-frontend-panel/SKILL.md) | Adding a panel to the React frontend. |
| [`release`](../.agents/skills/release/SKILL.md) | Cutting a release. |

## The one-paragraph version

Valkyrie is a template. The plumbing every desktop application needs is done: an embedded PocketBase mounted on the Wails
asset server so files are served from the UI's own origin, a generic job queue with persistence, bounded concurrency,
cancellation and live event push, an encrypted secret store, a debug console, and the build tooling for every platform. The
application supplies one thing: a `jobs.Runner` that does the work. Everything else keeps working.
