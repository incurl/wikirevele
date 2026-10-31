# Troubleshooting

The failures that cost real time, and what actually fixes them. Each entry states the symptom, the cause and the fix.

---

## The build

### `wails3: command not found`

The CLI is not on the default PATH of non-interactive shells. It is installed in `~/go/bin`.

```bash
PATH="$HOME/go/bin:$PATH" wails3 build
```

The same applies to `golangci-lint` and any other `go install`ed tool. An interactive shell usually has it; a script, a CI
runner and an agent shell often does not.

### `flag provided but not defined: -clean`

`wails3 build` has no `-clean` flag. The `-clean` flag belongs to `wails3 generate bindings`:

```bash
PATH="$HOME/go/bin:$PATH" wails3 generate bindings -clean=true -ts -i
```

### `go build ./...` fails with `function main is undeclared in the main package`

Expected, and not a real failure. `build/ios/` is a separate `main` package behind the `//go:build ios` tag, and it has no
`main` function of its own on other platforms.

Build the parts that matter instead:

```bash
go build ./internal/... .
```

### `pkg-config` cannot find gtk4 or webkitgtk-6.0

The Linux build needs the GTK and WebKit development packages. Install them, or let the Linux task fall back to the Docker
cross-compilation image:

```bash
# Debian / Ubuntu
sudo apt install libgtk-4-dev libwebkitgtk-6.0-dev gcc pkg-config

# Or build with Docker
PATH="$HOME/go/bin:$PATH" wails3 task setup:docker
PATH="$HOME/go/bin:$PATH" wails3 task linux:build
```

### `go: go.mod requires go >= 1.26.0` and the local toolchain is older

`go.mod` pins `toolchain go1.26.10`. With `GOTOOLCHAIN=auto` (the default) Go downloads it automatically. If the download is
blocked, set it explicitly for the command:

```bash
GOTOOLCHAIN=go1.26.10 go build ./...
```

---

## The Task runner

### `task: Task "generate:bindings" does not exist`

The task names are namespaced. The tasks in `build/Taskfile.yml` are included under `common:`, so they are
`common:generate:bindings`, not `generate:bindings`.

```bash
PATH="$HOME/go/bin:$PATH" wails3 task common:generate:bindings
```

List what exists:

```bash
PATH="$HOME/go/bin:$PATH" wails3 task --list-all
```

### `wails3 task --summary <name>` ran the task instead of describing it

`--summary` is not a description flag for this CLI; the task runs. To inspect without executing, use `--dry`:

```bash
PATH="$HOME/go/bin:$PATH" wails3 task --dry common:generate:bindings
```

### The `task` binary is not installed

It is not needed. `wails3 task` runs the Taskfiles itself.

---

## The frontend

### `npm ERR! Cannot read properties of null (reading 'edgesOut')`

A bug in npm 9.2.0 (the system npm on some Debian images) when resolving the vitest 4 peer tree. It is not a problem with the
dependency set.

```bash
npm install --legacy-peer-deps
```

Or use a newer npm. The lockfile is committed, so a working `npm ci` reproduces the tree.

### `Could not resolve "./bindings/<pkg>"` during a parallel build

Two builds regenerate `frontend/bindings/` at once: one deletes it (`-clean`) while the other is reading it. The Taskfiles use
`run: once` on the generation tasks to prevent this. If you hit it, make sure you did not run `wails3 build` and
`wails3 task build` at the same time.

### The TypeScript build fails on the generated bindings

The bindings are gitignored, so a fresh clone has none until they are generated. Generate them before building the frontend:

```bash
PATH="$HOME/go/bin:$PATH" wails3 task common:generate:bindings
```

`npm run build` runs `tsc` first, so a missing binding is a type error, not a bundler error.

### `tsc` reports errors only in the test files

`npm run build` uses `tsconfig.json`, which **excludes** `*.test.ts`. `npm run typecheck` uses `tsconfig.test.json`, which
includes them. Run `npm run typecheck` to check both.

---

## The webview

### The copy button does nothing

`navigator.clipboard` needs a secure context and a user gesture, which a webview does not always provide. `writeClipboard` in
`frontend/src/features/console/ConsolePanel.tsx` already falls back to `document.execCommand('copy')`.

If you write a new copy path, use `writeClipboard` rather than calling `navigator.clipboard` directly.

If the fallback is used, the hidden textarea must **not** be `display: none` — that makes it unselectable and the copy fails
silently. Position it off-screen with `opacity: 0` instead.

### The debug console does not appear

The console is hidden by default. It is shown with **Ctrl+Shift+I** or with the **Console** button in the header.

If the shortcut does nothing, another element has the focus and swallowed the key event: the shortcut is matched on
`window`, so it only works when the event reaches the page. The Command key is accepted as well, for macOS.

### Clicking a log row collapses the panel while the user is selecting text

A click that ends a text drag fires the row's click handler. The console rows check `hasTextSelection()` before toggling the
detail view. Apply the same guard to any new selectable row.

### The native select dropdown is blank

The interface is dark, but the native popup is drawn by the engine and does not inherit the colours. Without
`color-scheme: dark` on `:root`, WebKitGTK renders the popup with a light background while the option text stays light, so the
selected item appears to be missing.

`color-scheme: dark` and the `select option` rule in `frontend/src/styles.css` both exist for this.

### The interface does not scale with the zoom

Every size must be in `rem`. The zoom scales the root font size, so a `px` size stays fixed while the rest of the layout
moves.

---

## The backend

### `pocketbase: service has not been bootstrapped`

`Bootstrap()` was not called, or `Handler()` was called on a service that was never bootstrapped. The bootstrap order in
`main.go` is: `NewService` → `Bootstrap` → `NewCipher` → `NewSettingsStore` → everything else.

### A job stays `pending` and never runs

Two causes:

1. **No runner is registered.** The manager reports `no runner is registered. Register one before submitting jobs.` in the
   debug console and the job fails.
2. **The queue is full.** The manager bounds the concurrency (2 by default). A job waits for a slot. If the runner blocks
   forever, every later job waits too. Check the runner honours `ctx`.

### A cancelled job shows as `failed`

The runner returned an error *without* the context being cancelled — or it ignored the cancellation and failed for another
reason. A job cancelled while the context is cancelled keeps the `cancelled` status. If the runner swallows `ctx.Err()` and
returns its own error, the job is reported as failed. Return `ctx.Err()` from the runner when the context is done.

### `The request requires valid record authorization token` (HTTP 401)

PocketBase's API is answering, and it requires authentication for that route. The application reads the collections through the
Go backend, not over HTTP, so this only appears if you call `/api/collections/...` directly from the browser. It is expected
behaviour, not a misconfiguration.

### The stored file is not served

Check that the URL goes through `/api/files/<collection>/<recordId>/<filename>` and that the request is not caught by the
static handler. `combineHandlers` routes `/api/*` to PocketBase and everything else to the frontend; `main_test.go` pins that
routing.

### The API key is not found after a restart

The key file lives in the data directory, next to the database. If `WIKIREVELE_DATA_DIR` changes between runs, the app reads a
different database and a different key file. Check the data directory first.

---

## The audits

### `tools/license-audit.py` reports `UNKNOWN` for a module

The licence was not detected from the module's own licence file. The tool reads `LICENSE`, `LICENCE` and `COPYING` (and their
prefixed variants) in the module's directory. A module that vendors its licence elsewhere shows as `UNKNOWN`. Check that one
module by hand.

### `tools/license-audit.py` fails on a copyleft dependency

That is the tool doing its job. A GPL or AGPL dependency in a distributed desktop application is a legal problem for whoever
ships it. Replace it, or obtain a commercial licence.

---

## Reporting

If you hit something not listed here, add it. The format is: the symptom as it appears, the cause, and the fix. Put it in this
file and, if it is a decision rather than a fact, in `docs/architecture.md`.
