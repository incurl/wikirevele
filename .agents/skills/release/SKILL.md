---
name: release
description: >
  Cut a release of a Valkyrie application: pick the version, verify the
  candidate commit, build the binaries and the packages, publish the GitHub
  release, and verify what was published. Use when the user asks to ship, cut
  a release, tag a version, or publish the desktop application. Do not use for
  a local build, a dev run, or a container image that is not part of a release.
metadata:
  owner: "incurl"
  version: "1.0.0"
  last-reviewed: "2026-10-28"
---

# Release

A release is a tag, plus the artifacts that the platform build produces. This skill covers choosing the version, gating the
candidate, building, tagging, and verifying what was published.

## Ground Rules

- **Never release from an uncommitted working tree.** Tag a commit that is on `origin/main`. Do not stash, commit, or rebase to
  make a release possible.
- **Pushing the tag is the irreversible step.** It publishes to GitHub and creates the release. Show the plan and wait for an
  explicit go, unless the user's request already named the version and said to proceed.
- **Never delete or move a pushed tag**, and never delete a published release, without the user's explicit approval for that
  specific tag. The default recovery is the next version.
- **Run the checks locally first.** A release that fails in CI costs a version number. See "Gate the Candidate" below.
- The Wails CLI is not on the default PATH of non-interactive shells. Prefix every command with
  `PATH="$HOME/go/bin:$PATH"`.

## 1. Establish the state

```bash
git fetch origin --tags --prune
git rev-parse origin/main
git log --oneline origin/main -20
gh release list --limit 10
```

Record: the candidate SHA (default `origin/main`), the current version in `build/config.yml`, the last released tag, and whether
the working tree differs from the candidate. **Mention** a dirty tree; do not touch it.

## 2. Choose the version

The version lives in `build/config.yml`:

```yaml
info:
  version: "0.1.0"
```

It is also embedded in the macOS and Windows metadata (`build/darwin/Info.plist`, `build/windows/info.json`) and in
`internal/bindings/service.go` (`AppVersion`, which reaches the binary through the about-box description). All of them must
agree.

Use **semantic versioning**:

| Change | Version |
| --- | --- |
| A breaking change to the API, the schema or the CLI behaviour | `MAJOR` |
| A new feature, a new field, a new panel | `MINOR` |
| A bug fix, a dependency bump, a documentation change | `PATCH` |

A **schema change** (a new migration) is at least a `MINOR`: an existing database has to be migrated in the field.

Bump the version in every place it appears:

```bash
grep -rn '0\.1\.0' build/config.yml build/darwin/Info.plist build/windows/info.json internal/bindings/service.go
```

## 3. Gate the candidate

All of these must pass **before** you propose the tag:

1. The version does not already exist as a tag or a release.
2. The candidate SHA is on `origin/main`.
3. The working tree is clean, or the difference is explained and accepted.
4. CI is green on the candidate:
   ```bash
   gh run list --commit <sha> --json name,status,conclusion
   ```
   The workflow is `.github/workflows/ci.yml`, with the jobs `backend`, `frontend` and `licences`. Require each to be
   `completed/success`. Wait for the in-progress ones.
5. The local checks pass:
   ```bash
   gofmt -l internal/ main.go main_test.go tools/     # must print nothing
   go vet ./internal/... .
   go test -race ./internal/... .
   go run ./tools/e2e-check
   cd frontend && npm run typecheck && npm test && npm run build
   cd .. && python3 tools/license-audit.py
   ```

A **dry run** is worth it when the release includes a new platform or a packaging change:

```bash
PATH="$HOME/go/bin:$PATH" wails3 task package
```

This produces the packages without publishing anything. Inspect them before you tag.

## 4. Build the release artifacts

```bash
PATH="$HOME/go/bin:$PATH" wails3 task package
```

The task dispatches to the platform by the `GOOS` variable, so a cross-platform package is:

```bash
PATH="$HOME/go/bin:$PATH" wails3 task package GOOS=windows
PATH="$HOME/go/bin:$PATH" wails3 task package GOOS=darwin
```

**Linux** needs a C compiler and the GTK/WebKit development packages. Without them the task falls back to the Docker
cross-compilation image:

```bash
PATH="$HOME/go/bin:$PATH" wails3 task setup:docker   # once
```

**The headless server binary and the container image** are separate:

```bash
PATH="$HOME/go/bin:$PATH" wails3 task build:server
PATH="$HOME/go/bin:$PATH" wails3 task build:docker
```

Check the artifacts:

```bash
ls -la bin/
file bin/valkyrie
```

A Linux release should also carry the packages: `bin/*.deb`, `bin/*.rpm`, `bin/*.AppImage`.

## 5. Commit the version bump

The version bump is a commit on `main`, before the tag:

```bash
git add build/config.yml build/darwin/Info.plist build/windows/info.json internal/bindings/service.go
git commit -m "chore(release): bump the version to X.Y.Z"
git push origin main
```

Wait for CI to go green on that commit, then tag it.

## 6. Propose, then tag

Show the user: the version, the candidate SHA and subject, the last released tag, the commit count, the gate results, the
artifacts that will be published, and the notes. After an explicit go:

```bash
git tag -a X.Y.Z -m "X.Y.Z" <sha>
git push origin X.Y.Z
```

Use an annotated tag. If the push fails, delete the local tag and start again — a tag that was never pushed is safe to remove.

## 7. Publish the release

The tag alone does not create a GitHub release unless a workflow does it. If the repository has no release workflow, create the
release from the CLI:

```bash
gh release create X.Y.Z \
  --title "X.Y.Z" \
  --notes-file /tmp/notes.md \
  bin/valkyrie bin/*.deb bin/*.rpm bin/*.AppImage
```

If a release workflow exists, let it run and watch it:

```bash
gh run list --workflow release.yml --limit 5
gh run watch <id>
```

On failure, read the log with `gh run view <id> --log-failed` and classify it:

- **Infrastructure** (runner loss, network timeout, a registry 5xx): rerun with `gh run rerun <id> --failed`. At most twice.
- **Defect** (a compile error, a failed test, a bad packaging step): stop. Do not rerun. Report the step and the fix. The
  version stays burned; the next release takes the next number.

## 8. Verify the published release

**Do not report success until each check passes:**

1. The release exists and the assets are the ones you built:
   ```bash
   gh release view X.Y.Z --json tagName,assets
   ```
2. The binary runs and reports the right version:
   ```bash
   ./bin/valkyrie --version
   ```
3. The tagged commit is the one you intended:
   ```bash
   git rev-list -n 1 X.Y.Z
   git rev-parse <sha>
   ```
4. The tag points at a commit that is on `origin/main`:
   ```bash
   git merge-base --is-ancestor X.Y.Z origin/main && echo "on main"
   ```

## 9. Report

Give the version, the tag, the SHA, the URL of the release, the list of assets, the verification results, anything that failed
and was retried, and anything that was not done.

## Notes for a template

This repository is a **template**, not a shipped product. It has no release workflow, because the application that is built
from it brings its own. When you port the template, add a `.github/workflows/release.yml` that builds the platforms your users
need, and use this skill for the process around it.
