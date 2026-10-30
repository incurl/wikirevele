# Research notes: Refine + shadcn/ui for the Wikidata panel

Everything below was checked against the published packages and the
official documentation, not from memory. Where the sources disagree with
what one might assume, the source wins and the discrepancy is called out.

## 1. Refine version and the shadcn/ui integration

**Refine v5 is current**: `@refinedev/core@5.0.12` (the `next` tag is
`4.44.10`, so v5 is the stable line).

**There is no `@refinedev/shadcn-ui` package.** The name was checked on
the npm registry and it does not exist. The community package
`@ferdiunal/refine-shadcn` is a *theme* for refine.dev, not an adapter.

The official shadcn/ui support in Refine v5 is a **component registry**:
the components are copied into the project as source, the way shadcn/ui
itself works. They are installed with the shadcn CLI:

```bash
npx shadcn@latest add https://ui.refine.dev/r/views.json
npx shadcn@latest add https://ui.refine.dev/r/data-table.json
npx shadcn@latest add https://ui.refine.dev/r/layout-01.json
```

The registry URLs were read from the Refine documentation and confirmed
to resolve. `https://ui.refine.dev/r/views.json` returns the registry
item with the actual component source, which is how the exact API below
was verified.

**Consequence for this repo**: the components live in
`frontend/src/components/refine-ui/...` and are *ours* to edit. They are
not an npm dependency, so they cannot drift under us, and the
`frontend/bindings/` rule (never hand-edit generated files) does not
apply to them.

### The components that exist

| Registry item | Installed to | Exports |
| --- | --- | --- |
| `views.json` | `src/components/refine-ui/views/list-view.tsx` | `ListView`, `ListViewHeader` |
| | `.../views/show-view.tsx` | `ShowView`, `ShowViewHeader` |
| | `.../views/create-view.tsx` | `CreateView`, `CreateViewHeader` |
| | `.../views/edit-view.tsx` | `EditView`, `EditViewHeader` |
| `data-table.json` | `src/components/refine-ui/data-table/data-table.tsx` | `DataTable`, `DataTableSorter`, `DataTableFilterDropdownText`, `DataTableFilterCombobox`, ... |
| `layout-01.json` | `src/components/refine-ui/layout/layout.tsx` | `Layout` |

### Verified props

`ListView` takes `children` and `className`. `ListViewHeader` takes
`resource`, `title`, `canCreate`, `hideBreadcrumb`, `wrapperClassName`,
`headerClassName`. `ShowView` and `ShowViewHeader` take the same set.

The headers read the resource from the router context
(`useResourceParams`), derive the title from `resource.meta.label` via
`useUserFriendlyName`, and render `CreateButton` / `EditButton` /
`ListButton` / `RefreshButton` according to the resource definition.

**Important**: the `Create` button is shown when `resource.create` is
set. For a read-only Wikidata browse panel the resource must **not**
define `create`, or the header must be given `canCreate={false}`.

## 2. The DataProvider contract (from `@refinedev/core@5.0.12`)

Read out of `dist/contexts/data/types.d.ts` in the published package.

```ts
export type BaseRecord = { id?: BaseKey; [key: string]: any };
export type BaseKey = string | number;

export interface GetListResponse<TData = BaseRecord> {
  data: TData[];
  total: number;
  [key: string]: any;
}
export interface GetOneResponse<TData = BaseRecord> { data: TData; }
export interface GetManyResponse<TData = BaseRecord> { data: TData[]; }
export interface CustomResponse<TData = BaseRecord> { data: TData; }

export interface GetListParams {
  resource: string;
  pagination?: Pagination;
  sorters?: CrudSort[];
  filters?: CrudFilter[];
  meta?: MetaQuery;
  dataProviderName?: string;
}
export interface GetOneParams { resource: string; id: BaseKey; meta?: MetaQuery; }
export interface GetManyParams { resource: string; ids: BaseKey[]; meta?: MetaQuery; }
```

`DataProvider` requires `getList`, `getOne`, `create`, `update`,
`deleteOne`, `getApiUrl`. `getMany`, `getManyReference`, `createMany`,
`updateMany`, `deleteMany` and `custom` are optional.

**The mutations are mandatory.** A read-only browse panel still has to
supply `create`/`update`/`deleteOne`, because the type requires them.
They are implemented as rejected promises: the panel is read-only, and a
mutation that is attempted must fail loudly rather than silently do
nothing.

### Filters

```ts
export type CrudOperators =
  | "eq" | "ne" | "eqs" | "nes" | "lt" | "gt" | "lte" | "gte"
  | "in" | "nin" | "ina" | "nina"
  | "contains" | "ncontains" | "containss" | "ncontainss"
  | "between" | "nbetween" | "null" | "nnull"
  | "startswith" | "nstartswith" | "startswiths" | "nstartswiths"
  | "endswith" | "nendswith" | "endswiths" | "nendswiths"
  | "or" | "and";

export type LogicalFilter = {
  field: string;
  operator: Exclude<CrudOperators, "or" | "and">;
  value: any;
};
export type ConditionalFilter = {
  key?: string;
  operator: Extract<CrudOperators, "or" | "and">;
  value: (LogicalFilter | ConditionalFilter)[];
};
export type CrudFilter = LogicalFilter | ConditionalFilter;
```

`value` is typed `any` in the library. Our own provider narrows it
internally; that is the one place `any` is unavoidable, and it does not
leak into the components.

### Resource definition

`ResourceProps` (from `dist/contexts/resource/types.d.ts`):

```ts
export interface ResourceProps extends IResourceComponents {
  name: string;
  identifier?: string;
  meta?: ResourceMeta;   // label, hide, icon, canDelete, ...
}
export interface IResourceComponents {
  list?: string; create?: string; clone?: string;
  edit?: string; show?: string;
}
```

`meta.label` is what the shadcn headers show as the page title.

## 3. The Wikidata data model

Read from the official Wikibase JSON documentation
(`doc.wikimedia.org/Wikibase/master/php/docs_topics_json.html`), which
is the canonical description of the dump format.

### Top level

```json
{
  "id": "Q60", "type": "item",
  "labels": {}, "descriptions": {}, "aliases": {},
  "claims": {}, "sitelinks": {},
  "lastrevid": 195301613, "modified": "2020-02-10T12:42:02Z"
}
```

`labels`/`descriptions`/`aliases` map a language code to a term object.
`aliases` maps a language code to an **array** of terms.

### Claims

`claims` maps a property id (`P31`) to an **array** of statements.

```json
"P6": [{
  "id": "Q60$5cc8fc79-...",
  "mainsnak": {
    "snaktype": "value",
    "property": "P6",
    "datatype": "wikibase-item",
    "datavalue": {
      "value": { "entity-type": "item", "id": "Q4911497", "numeric-id": 4911497 },
      "type": "wikibase-entityid"
    }
  },
  "qualifiers": {
    "P580": [{
      "snaktype": "value", "property": "P580", "datatype": "time",
      "datavalue": {
        "value": {
          "time": "+00000002014-01-01T00:00:00Z",
          "timezone": 0, "before": 0, "after": 0, "precision": 11,
          "calendarmodel": "http://www.wikidata.org/entity/Q1985727"
        },
        "type": "time"
      }
    }]
  },
  "qualifiers-order": ["P580"],
  "type": "statement",
  "rank": "preferred"
}]
```

Key facts that shape the types:

- The value lives in **`mainsnak`**, not directly on the statement.
- `snaktype` is `value`, `novalue` or `somevalue`. Only `value` carries
  a `datavalue`.
- The **datatype discriminator** is `datavalue.type`, and there is a
  second one, `mainsnak.datatype`, which names the *property* datatype
  (`wikibase-item`, `time`, `quantity`, `globecoordinate`, `url`,
  `external-id`, ...). `datavalue.type` names the *value* kind
  (`wikibase-entityid`, `string`, `monolingualtext`, `time`, `quantity`,
  `globecoordinate`, `external-id`).
- **`rank`** is `preferred`, `normal` or `deprecated`.
- **qualifiers** map a property id to an array of snaks (snaks, not
  statements).
- **references** are an array of *snak groups*: each reference is
  `{ "snaks": { "P248": [ {...snak...} ], "P854": [ {...snak...} ] } }`.
- The `time` string has a **leading `+` or `-`**, a year that may be
  zero-padded to more than four digits (`+00000002014-...`), and the
  precision is a separate field: `0` year, `1` year-month, `2` day,
  `3` hour, `4` minute, ... `11` second.
- `before`/`after` are non-zero when the date is imprecise at the
  Gregorian/Julian boundary.
- A **quantity** is `{ "amount": "+123", "unit": "http://.../unit/1" }`
  — the amount is an arbitrary-precision **decimal string**, and the
  unit is a URL.
- A **globe coordinate** is
  `{ "latitude": 48.85, "longitude": 2.35, "globe": "http://www.wikidata.org/entity/Q2", "precision": 0.0001 }`.
- A **monolingual text** is `{ "text": "...", "language": "fr" }`.
- An **external id** is a plain string with `"type": "external-id"`.
- A **URL** is a plain string with `"type": "string"` and
  `datatype: "url"`.

### The filter used by this project

The Go extractor in `internal/wikidata` already writes the subset that
matters: items with **P915** (filming location) or **P840**
(narrative location), plus **P345** (IMDb). The panel consumes that
subset, so the panel never has to re-derive the filter.

## 4. Custom data providers that are not REST

Refine's provider is a plain object of async functions, so a provider
over an in-memory array or an NDJSON file is written exactly like a
REST one. The parts that differ:

- `getList` receives `filters` as `CrudFilter[]` and must apply them
  itself. A filter on a nested field (`claims.P915`) is expressed with
  the `field` string `"claims.P915"` and the operator `eq`.
- `total` is the count **after** filtering, before pagination.
- `getOne` takes a `BaseKey`; for Wikidata the natural id is the QID,
  so `id` is the QID string and no lookup table is needed.
- `getMany` is optional but cheap to implement for an in-memory
  provider, and `useSelect` depends on it.
- `custom` is the escape hatch for anything that is not CRUD: a SPARQL
  endpoint, a batch lookup, a facet count.

**SPARQL result binding** is not a CRUD shape at all: a row is a
tuple of variables, not a record with an id. The right tool is
`custom`, with the row shape carried in the response rather than in the
record. This is noted for later; no SPARQL builder is built now.

## 5. Filtering by P131 / genre / occupation

All three are the same shape: a claim on the item whose value is an
entity id. So one filter mechanism covers them:

- `claims.P131` eq `Q90` — located in Paris
- `claims.P136` eq `Q...` — genre
- `claims.P106` eq `Q...` — occupation

`P131` is *located in the administrative territorial entity*. For the
Paris use case the practical filter is `claims.P131` containing `Q90`,
but note that many Paris places are only reachable through an
intermediate arrondissement, so a strict `eq` on `Q90` under-counts.
The provider therefore supports a `contains` operator on the claim
value list, which is what the UI uses by default.

The labels of the filter values (`Q90` -> "Paris") are not in the dump
subset, so the filter bar shows the QID with the label when the label
is known, and the QID alone otherwise.

## 6. Entity card layout

Three levels, all from the same data:

1. **Compact preview card** (the grid): QID, English label, French
   fallback, description, a thumbnail when P18 is present, and the
   count of the claims. Enough to pick an item in the grid.
2. **Expandable detail panel**: the same card, opened, with the
   property table.
3. **Inline property table**: one row per property, with the value
   rendered by the datatype renderer, the rank as a badge, and the
   qualifiers and references collapsed under the row.

The rule that keeps this sane: the **card never mutates the dump
object**. Everything is derived into a read-only view model first
(`toEntityView`), and the view model is what the components read.

## 7. Datatype renderers

| Datatype | Value shape | Renderer |
| --- | --- | --- |
| `wikibase-item` | `{entity-type, id, numeric-id}` | QID chip, links to the entity page |
| `wikibase-entityid` (property) | same | property label chip |
| `string` | plain string | text |
| `url` | plain string | external link |
| `external-id` | plain string | external link to the id resolver |
| `monolingualtext` | `{text, language}` | text with the language badge |
| `time` | `{time, precision, ...}` | precision-aware date |
| `quantity` | `{amount, unit}` | amount with the unit |
| `globecoordinate` | `{latitude, longitude, globe, precision}` | coordinates and a mini map |
| `novalue` / `somevalue` | none | an explicit "no value" / "some value" badge |

The globe-coordinate mini map is a **static tile-less preview**: a small
SVG plotting the point on an equirectangular grid with the lat/lon
axes. It deliberately does not fetch tiles, because the panel runs
offline inside a Wails webview and must not make network requests.

## 8. Force graph

`react-force-graph-2d` (1.29.2) is the lightest option that fits: it
is a React wrapper over `d3-force` with a canvas renderer, which is
fast enough for the small local graph (a few dozen nodes) that the card
shows.

It is used through a **thin local wrapper**, not directly, so that:

- the graph is rendered from a derived node/link list, never from the
  raw claims;
- the simulation can be capped at a fixed node count, because a full
  dump item can hold thousands of claims;
- the dependency stays optional: the wrapper falls back to a plain
  list when the graph package is not installed.

For a graph of this size `d3-force` directly would also work and would
avoid a dependency, but `react-force-graph-2d` handles the canvas
lifecycle (resize, device pixel ratio) that is otherwise fiddly.

## 9. Constraints and consequences

- **Strict TypeScript, no `any`.** The library's own `BaseRecord` and
  `LogicalFilter.value` use `any`; both are contained at the provider
  boundary. The types below model the dump with closed unions, so a
  malformed value is a *type error at the parse boundary*, not a crash
  in a component.
- **Shadcn/ui.** The Refine components are installed into
  `src/components/refine-ui/`; the primitives come from the shadcn
  registry into `src/components/ui/`. The existing plain CSS in
  `src/styles.css` is left alone: the panel uses the shadcn classes.
- **No SPARQL builder.** Only the consumption of a bound result set.
- **Paris 40-day exploration.** English label first, French label as the
  fallback, and the card stays compact enough to scan a large grid.
- **Presentation separated from the raw structure.** `types.ts` mirrors
  the dump; `view.ts` derives the view model; the components read only
  the view model.
- **Ready for the PocketBase sync.** The provider is written against a
  `WikidataSource` interface with two implementations: the local NDJSON
  subset and the PocketBase collections. Swapping the source is a
  provider change, not a component change.
