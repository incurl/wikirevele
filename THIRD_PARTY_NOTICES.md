# Third party notices

WikiRevele is built on the following third party projects. Every licence
below is permissive; **no GPL, AGPL or other copyleft dependency is
present in the dependency tree**. This is verified automatically by
`tools/license-audit.py`, which exits non-zero if a copyleft licence is
introduced.

## The direct dependencies

These are the projects WikiRevele imports directly. The full transitive
tree is listed further down.

| Project | Why it is here | Licence |
| --- | --- | --- |
| [Wails v3](https://github.com/wailsapp/wails) | The desktop shell: the window, the asset server, the service bindings and the event bus | MIT |
| [PocketBase](https://github.com/pocketbase/pocketbase) | The embedded database: the schema, the migrations, the file storage and the REST API | MIT |
| [dbx](https://github.com/pocketbase/dbx) | The database query builder PocketBase is built on | MIT |
| [sonic](https://github.com/bytedance/sonic) | The JSON parser of the Wikidata extractor: it decodes the dump far faster than the standard library | Apache-2.0 |
| [Refine](https://refine.dev) | The admin framework of the Wikidata panel: the resources, the data provider and the views | MIT |
| [React Router](https://reactrouter.com) | The router of the Refine application | MIT |
| [TanStack Table](https://tanstack.com/table) | The table behind the Refine data table | MIT |
| [Tailwind CSS](https://tailwindcss.com) | The styling of the shadcn/ui components | MIT |
| [Radix UI](https://www.radix-ui.com) | The accessible primitives the shadcn/ui components are built on | MIT |
| [lucide](https://lucide.dev) | The icons of the interface | ISC |
| [React](https://github.com/reactjs/react) | The UI | MIT |
| [TanStack Query](https://github.com/TanStack/query) | The data layer: the queries, the mutations and the cache invalidation | MIT |
| [Vite](https://github.com/vitejs/vite) | The frontend build and the dev server | MIT |
| [Vitest](https://github.com/vitest-dev/vitest) | The frontend test runner | MIT |
| [@wailsio/runtime](https://github.com/wailsapp/wails) | The runtime the generated bindings call | MIT |

## The full dependency tree

### Go modules — Apache-2.0 (9)

- `appengine@v1.6.5`
- `base64x@v0.1.6`
- `cobra@v1.10.2`
- `gopkg@v0.1.3`
- `loader@v0.5.3`
- `mousetrap@v1.1.0`
- `pprof@v0.0.0-20260709232956-b9395ee17fa0`
- `sonic@v1.15.5`
- `yaml.v3@v3.0.1`

### Go modules — BSD-3-Clause (35)

- `arch@v0.0.0-20210923205945-b76863e36670`
- `bigfft@v0.0.0-20230129092748-24d4a6f8daec`
- `crypto@v0.57.0`
- `fexpr@v0.6.0`
- `fileutil@v1.4.0`
- `fsnotify@v1.10.1`
- `go-cmp@v0.7.0`
- `go-difflib@v1.0.0`
- `go-internal@v1.9.0`
- `goabi0@v0.2.0`
- `golang-asm@v0.15.1`
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

### Go modules — MIT (23)

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
- `v2@v2.3.0`
- `v3@v3.6.2`
- `v3@v3.0.0`
- `v4@v4.3.0`
- `v5@v5.3.1`
- `xdg@v0.5.3`

### Go modules — MPL-2.0 (2)

- `mysql@v1.4.1`
- `v2@v2.0.7`

### npm packages — Apache-2.0 (8)
- `baseline-browser-mapping`
- `class-variance-authority`
- `detect-libc`
- `didyoumean`
- `expect-type`
- `ts-interface-checker`
- `typescript`
- `xml-name-validator`

### npm packages — BSD-3-Clause (5)
- `entities`
- `qs`
- `source-map-js`
- `tough-cookie`
- `webidl-conversions`

### npm packages — CC0-1.0 (1)
- `caniuse-lite`

### npm packages — ISC (10)
- `anymatch`
- `electron-to-chromium`
- `fastq`
- `glob-parent`
- `lru-cache`
- `lucide-react`
- `picocolors`
- `siginfo`
- `tslib`
- `yaml`

### npm packages — MIT (188)
- `@alloc/quick-lru`
- `@asamuzakjp/css-color`
- `@csstools/color-helpers`
- `@csstools/css-calc`
- `@csstools/css-parser-algorithms`
- `@csstools/css-tokenizer`
- `@floating-ui/core`
- `@floating-ui/dom`
- `@floating-ui/react-dom`
- `@floating-ui/utils`
- `@jridgewell/gen-mapping`
- `@jridgewell/resolve-uri`
- `@jridgewell/sourcemap-codec`
- `@jridgewell/trace-mapping`
- `@nodelib/fs.scandir`
- `@nodelib/fs.stat`
- `@nodelib/fs.walk`
- `@oxc-project/types`
- `@radix-ui/react-popover`
- `@radix-ui/react-use-effect-event`
- `@refinedev/core`
- `@refinedev/devtools-internal`
- `@refinedev/devtools-shared`
- `@refinedev/react-router`
- `@refinedev/react-table`
- `@rolldown/pluginutils`
- `@standard-schema/spec`
- `@tanstack/query-core`
- `@tanstack/react-query`
- `@tanstack/react-table`
- `@tanstack/table-core`
- `@types/chai`
- `@types/deep-eql`
- `@types/estree`
- `@types/node`
- `@types/react`
- `@types/react-dom`
- `@vitejs/plugin-react`
- `@vitest/expect`
- `@vitest/mocker`
- `@vitest/runner`
- `@vitest/utils`
- `agent-base`
- `any-promise`
- `arg`
- `aria-hidden`
- `assertion-error`
- `asynckit`
- `autoprefixer`
- `binary-extensions`
- `braces`
- `browserslist`
- `call-bind-apply-helpers`
- `call-bound`
- `camelcase-css`
- `chai`
- `chokidar`
- `clsx`
- `combined-stream`
- `commander`
- `convert-source-map`
- `cookie`
- `cssstyle`
- `csstype`
- `data-urls`
- `debug`
- `decimal.js`
- `delayed-stream`
- `detect-node-es`
- `dunder-proto`
- `error-stack-parser`
- `es-define-property`
- `es-errors`
- `es-module-lexer`
- `es-object-atoms`
- `es-set-tostringtag`
- `escalade`
- `estree-walker`
- `fast-glob`
- `fdir`
- `fill-range`
- `form-data`
- `fraction.js`
- `function-bind`
- `get-intrinsic`
- `get-nonce`
- `get-proto`
- `gopd`
- `has-symbols`
- `has-tostringtag`
- `hasown`
- `html-encoding-sniffer`
- `http-proxy-agent`
- `https-proxy-agent`
- `iconv-lite`
- `is-binary-path`
- `is-core-module`
- `is-extglob`
- `is-glob`
- `is-number`
- `jiti`
- `jsdom`
- `lilconfig`
- `lines-and-columns`
- `lodash`
- `lodash-es`
- `magic-string`
- `math-intrinsics`
- `merge2`
- `micromatch`
- `mime-db`
- `mime-types`
- `ms`
- `mz`
- `nanoid`
- `node-releases`
- `normalize-path`
- `normalize-range`
- `nwsapi`
- `object-assign`
- `object-hash`
- `object-inspect`
- `obug`
- `papaparse`
- `parse5`
- `path-parse`
- `pathe`
- `picomatch`
- `pirates`
- `pluralize`
- `postcss`
- `postcss-import`
- `postcss-js`
- `postcss-load-config`
- `postcss-nested`
- `postcss-value-parser`
- `queue-microtask`
- `react`
- `react-dom`
- `react-remove-scroll`
- `react-router`
- `react-style-singleton`
- `read-cache`
- `readdirp`
- `resolve`
- `reusify`
- `rolldown`
- `rrweb-cssom`
- `run-parallel`
- `safer-buffer`
- `scheduler`
- `set-cookie-parser`
- `side-channel`
- `side-channel-list`
- `side-channel-map`
- `side-channel-weakmap`
- `stackframe`
- `std-env`
- `sucrase`
- `supports-preserve-symlinks-flag`
- `symbol-tree`
- `tailwind-merge`
- `tailwindcss`
- `tailwindcss-animate`
- `thenify`
- `thenify-all`
- `tinybench`
- `tinyexec`
- `tinyglobby`
- `tinyrainbow`
- `tldts`
- `tldts-core`
- `to-regex-range`
- `tr46`
- `undici-types`
- `update-browserslist-db`
- `use-callback-ref`
- `use-sidecar`
- `util-deprecate`
- `vite`
- `vitest`
- `w3c-xmlserializer`
- `warn-once`
- `whatwg-encoding`
- `whatwg-url`
- `why-is-node-running`
- `ws`
- `xmlchars`

### npm packages — MPL-2.0 (3)
- `lightningcss`
- `lightningcss-linux-x64-gnu`
- `lightningcss-linux-x64-musl`

### npm packages — UNKNOWN (52)
- `@csstools/css-color-parser`
- `@radix-ui/number`
- `@radix-ui/primitive`
- `@radix-ui/react-arrow`
- `@radix-ui/react-collapsible`
- `@radix-ui/react-collection`
- `@radix-ui/react-compose-refs`
- `@radix-ui/react-context`
- `@radix-ui/react-dialog`
- `@radix-ui/react-direction`
- `@radix-ui/react-dismissable-layer`
- `@radix-ui/react-dropdown-menu`
- `@radix-ui/react-focus-guards`
- `@radix-ui/react-focus-scope`
- `@radix-ui/react-id`
- `@radix-ui/react-label`
- `@radix-ui/react-menu`
- `@radix-ui/react-popper`
- `@radix-ui/react-portal`
- `@radix-ui/react-presence`
- `@radix-ui/react-primitive`
- `@radix-ui/react-roving-focus`
- `@radix-ui/react-scroll-area`
- `@radix-ui/react-select`
- `@radix-ui/react-separator`
- `@radix-ui/react-slot`
- `@radix-ui/react-tabs`
- `@radix-ui/react-tooltip`
- `@radix-ui/react-use-callback-ref`
- `@radix-ui/react-use-controllable-state`
- `@radix-ui/react-use-escape-keydown`
- `@radix-ui/react-use-layout-effect`
- `@radix-ui/react-use-previous`
- `@radix-ui/react-use-rect`
- `@radix-ui/react-use-size`
- `@radix-ui/react-visually-hidden`
- `@radix-ui/rect`
- `@rolldown/binding-linux-x64-gnu`
- `@rolldown/binding-linux-x64-musl`
- `@vitest/pretty-format`
- `@vitest/snapshot`
- `@vitest/spy`
- `@wailsio/runtime`
- `cssesc`
- `dlv`
- `is-potential-custom-element-name`
- `postcss-selector-parser`
- `punycode`
- `react-remove-scroll-bar`
- `saxes`
- `stackback`
- `whatwg-mimetype`

## Adding a dependency

Before adding a dependency, check its licence:

```bash
python3 tools/license-audit.py
```

The audit fails when a GPL, AGPL, LGPL, SSPL, EUPL or OSL dependency
enters the tree, because a copyleft licence in a distributed desktop
application is a legal problem for the people who ship it.
