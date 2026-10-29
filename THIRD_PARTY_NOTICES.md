# Third party notices

Valkyrie is built on the following third party projects. Every licence
below is permissive; **no GPL, AGPL or other copyleft dependency is
present in the dependency tree**. This is verified automatically by
`tools/license-audit.py`, which exits non-zero if a copyleft licence is
introduced.

## The direct dependencies

These are the projects Valkyrie imports directly. The full transitive
tree is listed further down.

| Project | Why it is here | Licence |
| --- | --- | --- |
| [Wails v3](https://github.com/wailsapp/wails) | The desktop shell: the window, the asset server, the service bindings and the event bus | MIT |
| [PocketBase](https://github.com/pocketbase/pocketbase) | The embedded database: the schema, the migrations, the file storage and the REST API | MIT |
| [dbx](https://github.com/pocketbase/dbx) | The database query builder PocketBase is built on | MIT |
| [React](https://github.com/reactjs/react) | The UI | MIT |
| [TanStack Query](https://github.com/TanStack/query) | The data layer: the queries, the mutations and the cache invalidation | MIT |
| [Vite](https://github.com/vitejs/vite) | The frontend build and the dev server | MIT |
| [Vitest](https://github.com/vitest-dev/vitest) | The frontend test runner | MIT |
| [@wailsio/runtime](https://github.com/wailsapp/wails) | The runtime the generated bindings call | MIT |

## The full dependency tree

### Go modules — Apache-2.0 (5)

- `appengine@v1.6.5`
- `cobra@v1.10.2`
- `mousetrap@v1.1.0`
- `pprof@v0.0.0-20260709232956-b9395ee17fa0`
- `yaml.v3@v3.0.1`

### Go modules — BSD-3-Clause (33)

- `bigfft@v0.0.0-20230129092748-24d4a6f8daec`
- `crypto@v0.57.0`
- `fexpr@v0.6.0`
- `fileutil@v1.4.0`
- `fsnotify@v1.10.1`
- `go-cmp@v0.7.0`
- `go-difflib@v1.0.0`
- `go-internal@v1.9.0`
- `goabi0@v0.2.0`
- `image@v0.45.0`
- `json@v0.0.0-20251027170946-4849db3c2f7e`
- `libc@v1.74.1`
- `mathutil@v1.7.1`
- `memory@v1.11.0`
- `mod@v0.41.0`
- `net@v0.60.0`
- `oauth2@v0.36.0`
- `opt@v0.2.0`
- `pflag@v1.0.10`
- `sortutil@v1.2.1`
- `sqlite@v1.55.0`
- `strutil@v1.2.1`
- `sync@v0.23.0`
- `sys@v0.48.0`
- `text@v0.42.0`
- `token@v1.1.0`
- `tools@v0.49.0`
- `uuid@v1.6.0`
- `v2@v2.6.5`
- `v3@v3.1.4`
- `v4@v4.29.0`
- `v4@v4.34.6`
- `v5@v5.2.2`

### Go modules — ISC (2)

- `go-spew@v1.1.1`
- `websocket@v1.8.14`

### Go modules — MIT (22)

- `cast@v1.10.0`
- `color@v1.19.0`
- `dbx@v1.12.0`
- `go-colorable@v0.1.15`
- `go-humanize@v1.0.1`
- `go-isatty@v0.0.23`
- `go-ole@v1.3.0`
- `go-strftime@v1.0.0`
- `govalidator@v0.0.0-20230301143203-a9d515a09cc2`
- `imaging@v1.6.2`
- `is@v1.4.1`
- `mimetype@v1.4.13`
- `pocketbase@v0.39.11`
- `pretty@v0.3.1`
- `quicktest@v1.14.6`
- `testify@v1.11.1`
- `text@v0.2.0`
- `v3@v3.0.0`
- `v3@v3.6.2`
- `v4@v4.3.0`
- `v5@v5.3.1`
- `xdg@v0.5.3`

### Go modules — MPL-2.0 (2)

- `mysql@v1.4.1`
- `v2@v2.0.7`

### npm packages — Apache-2.0 (4)

- `detect-libc`
- `expect-type`
- `typescript`
- `xml-name-validator`

### npm packages — BSD-3-Clause (4)

- `entities`
- `source-map-js`
- `tough-cookie`
- `webidl-conversions`

### npm packages — ISC (3)

- `lru-cache`
- `picocolors`
- `siginfo`

### npm packages — MIT (97)

- `@asamuzakjp/css-color`
- `@csstools/color-helpers`
- `@csstools/css-calc`
- `@csstools/css-color-parser`
- `@csstools/css-parser-algorithms`
- `@csstools/css-tokenizer`
- `@jridgewell/sourcemap-codec`
- `@oxc-project/types`
- `@rolldown/pluginutils`
- `@standard-schema/spec`
- `@tanstack/query-core`
- `@tanstack/react-query`
- `@types/chai`
- `@types/deep-eql`
- `@types/estree`
- `@types/node`
- `@types/react`
- `@types/react-dom`
- `@vitejs/plugin-react`
- `@vitest/expect`
- `@vitest/mocker`
- `@vitest/pretty-format`
- `@vitest/runner`
- `@vitest/snapshot`
- `@vitest/spy`
- `@vitest/utils`
- `agent-base`
- `assertion-error`
- `asynckit`
- `call-bind-apply-helpers`
- `chai`
- `combined-stream`
- `convert-source-map`
- `cssstyle`
- `csstype`
- `data-urls`
- `debug`
- `decimal.js`
- `delayed-stream`
- `dunder-proto`
- `es-define-property`
- `es-errors`
- `es-module-lexer`
- `es-object-atoms`
- `es-set-tostringtag`
- `estree-walker`
- `fdir`
- `form-data`
- `function-bind`
- `get-intrinsic`
- `get-proto`
- `gopd`
- `has-symbols`
- `has-tostringtag`
- `hasown`
- `html-encoding-sniffer`
- `http-proxy-agent`
- `https-proxy-agent`
- `iconv-lite`
- `jsdom`
- `magic-string`
- `math-intrinsics`
- `mime-db`
- `mime-types`
- `ms`
- `nanoid`
- `nwsapi`
- `obug`
- `parse5`
- `pathe`
- `picomatch`
- `postcss`
- `react`
- `react-dom`
- `rolldown`
- `rrweb-cssom`
- `safer-buffer`
- `scheduler`
- `std-env`
- `symbol-tree`
- `tinybench`
- `tinyexec`
- `tinyglobby`
- `tinyrainbow`
- `tldts`
- `tldts-core`
- `tr46`
- `undici-types`
- `vite`
- `vitest`
- `w3c-xmlserializer`
- `whatwg-encoding`
- `whatwg-mimetype`
- `whatwg-url`
- `why-is-node-running`
- `ws`
- `xmlchars`

### npm packages — MPL-2.0 (3)

- `lightningcss`
- `lightningcss-linux-x64-gnu`
- `lightningcss-linux-x64-musl`

### npm packages — UNKNOWN (7)

- `@rolldown/binding-linux-x64-gnu`
- `@rolldown/binding-linux-x64-musl`
- `@wailsio/runtime`
- `is-potential-custom-element-name`
- `punycode`
- `saxes`
- `stackback`

## Adding a dependency

Before adding a dependency, check its licence:

```bash
python3 tools/license-audit.py
```

The audit fails when a GPL, AGPL, LGPL, SSPL, EUPL or OSL dependency
enters the tree, because a copyleft licence in a distributed desktop
application is a legal problem for the people who ship it.
