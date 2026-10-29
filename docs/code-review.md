# Code review — 2026-10-28

A review of the scaffold's design and API. **Every finding in it has been fixed.** This document keeps the two parts that
are still worth reading: the trade-offs that were deliberately kept, and the order the work was done in.

The findings themselves are listed in the git history. What follows is what a reader of the current code would want to
know and would otherwise have to rediscover.

## Deliberate trade-offs I would keep

A review that only lists complaints is not a review. These looked questionable and turned out to be right:

- **Cancellation state in memory, not the database.** It is a desktop app with one process; persisting the cancel map would
  add a failure mode (stale entries after a crash) to solve a problem that does not exist.
- **A real database per test.** Slow — the `jobs` suite takes 7 seconds under `-race` — and worth every second, because it
  is what caught the data loss.
- **Plain CSS with custom properties.** No build step, no runtime, and the zoom scales it because everything is in `rem`.
  A CSS framework here would be weight without benefit.
- **A 30-minute default timeout.** Opinionated, and the right opinion: the failure it prevents (a hung runner stopping the
  queue) is silent, and the failure it causes (a legitimate long job) is loud and documented.
- **No i18n, no UI kit, three runtime dependencies.** The scaffold's value is that it can be read in an afternoon.

---

## Suggested order

| Order | Finding | Why now |
| --- | --- | --- |
| 1 | **Context leak** (2) | A real leak, in the hottest path, that the linter cannot see |
| 2 | **`errors.As`** (3) | Three-line fix for a silently lost error code |
| 3 | **Delete the three accessors** (1) | Deletion; removes a lie from the generated bindings |
| 4 | **`Submit`'s context, `recordToJob`'s error, `AppInfo.Version`** (6, 7, 8) | API honesty; each is a small edit |
| 5 | **Dead code** (4), **the false comment** (5) | Deletion; frees the reader from checking |
| 6 | **Error boundary, toast timer** (10, 11) | The two frontend bugs a user can see |
| 7 | **`App.tsx` split, `ui?.`, `savedZoom`** (12, 13, 14) | Polish; do it with the first real panel |
| 8 | **Image handling** (15), **tests** (16), **docs** (17) | Decide, cover, correct |

Findings 1–8 are each under an hour and together remove the whole of the "unused API" smell. That is where I would start.
