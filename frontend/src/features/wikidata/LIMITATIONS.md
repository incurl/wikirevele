# Limitations of the Wikidata panel

What the panel does not do yet, and what has to be true before the
PocketBase sync can happen.

## The NDJSON parsing

**The subset is parsed in the browser, in one go.** The parser in
`ndjson.ts` splits the whole text on newlines and parses every line.
That is fine for the Paris subset, which is a few thousand items, and
it is not fine for a larger dump: a hundred thousand items would be a
few hundred megabytes of JSON in the tab, and the parse would block the
main thread.

The three ways out, in the order they should be tried:

1. **Let the backend parse it.** The Go side already has a streaming
   parser (`internal/wikidata`, which reads the records of the
   database). A bound method that answers "give me the items that match
   these filters" solves the problem entirely, and it is the right
   answer for the full dump.
2. **Parse in a worker.** The parser moves into a Web Worker, so the
   main thread stays free, and the items are handed over in batches.
   This is the answer for a subset that is too big for the IPC but
   still small enough to keep in memory.
3. **Stream the file.** The browser reads the NDJSON with a
   `ReadableStream` and parses it line by line, keeping only what the
   filters need. This is the answer for a subset that does not fit in
   memory at all.

The panel is written so that the choice is one function: the
`WikidataSource` interface. Nothing else has to change.

**The items are held in memory.** `InMemorySource` keeps every item in
a `Map`. The place list and the relationships are kept as well. This is
the memory bound of the panel, and it is why the subset has to be small.

**A malformed line is skipped, not reported.** The parser returns null
for a line it cannot read and moves on. The extractor already reports
the lines it could not parse, so the panel does not have to.

## The caching strategy

**There is none yet.** The provider reads the source once and keeps the
items in a `Map` for the life of the page. Refine caches the *results*
of the queries, through TanStack Query, but the underlying items are
read once and held.

That is right for a panel that browses a fixed subset. It is not right
for a panel that must show fresh data after the extractor has run
again. Three things would have to change:

1. **The source has to be able to say that it changed.** A version
   number, or the modification time of the file, would do. The panel
   would then invalidate the Refine queries when it changes.
2. **The cache has to have a bound.** The items of a large subset
   should be evicted, least recently used first. The `Map` of the
   source becomes an LRU.
3. **The cache has to survive the panel being closed.** The subset
   would be stored in the browser (IndexedDB, or the Cache API), so
   that reopening the panel does not re-read the whole file.

TanStack Query already handles the second and third of these for the
*query results*. The part that is missing is the invalidation signal.

## The PocketBase sync boundary

**The boundary is the `WikidataSource` interface.** Everything above it
—the provider, the view model, the components, the pages—does not know
where the items came from.

Two implementations are planned:

| Source | Reads | Status |
| --- | --- | --- |
| `InMemorySource` | The NDJSON files | **Done** |
| `InMemorySource` over the database | The `wikidata_items` collection | **Done** |

### What is already in place for it

- The **collections exist**: `wikidata_items` and `wikidata_properties`,
  created by the migration in
  `internal/pocketbase/migrations/1732000000_add_wikidata_import.go`.
- The **importer writes them**: the Go side fills both collections in
  batches, in transactions.
- **The bound method answers with the records**: `GetWikidataItems`
  reads them back, and `backend.ts` turns them into a source.
- The **provider is source-agnostic**: it never touches a file or a
  database, only the interface.

### What is missing

- **A bound method that reads the collections.** The frontend must not
  talk to PocketBase directly: the repository's rule is that every call
  goes through `internal/bindings`. The method would answer "give me
  the items that match these filters", and the panel would call it
  through `frontend/src/bindings/index.ts`.
- **The filter is a "contains" on the claim.** The backend asks for
  `claims.<property> ~ "<value>"`, which is the JSON column containing
  the value, and not the value of one statement of that property. An
  item whose `P131` is `Q1` and whose `P915` is `Q90` matches a filter
  on `P131` being `Q90`, and it should not.

  A precise filter would need the values of the claims to be readable
  by the database, which means either a table of the statements or an
  index on them.

- **The sort is on the identifier.** The order by label would need an
  index on the label the panel shows, which is the first of the
  preferred languages that the item has. The identifier is the stable
  order, and it is the one the panel uses.

- **The filters are combined with "and".** Refine can express a group
  combined with "or", and the backend flattens the group: the claims
  are always all required. An "or" across two properties would need the
  bound method to take the tree and not the list.
- **A second pass for the coordinates.** The `P625` values of the
  places are not resolved. The extractor explicitly leaves them out, and
  a separate program is meant to fill them. The panel shows the
  coordinates it finds in the claims, and the mini map is drawn from
  those. When the second pass has run, the panel should read the
  resolved coordinates from the place collection instead.

- **The exploration records.** "User exploration records"—what the
  user opened, what they searched for—are not stored at all. They
  belong in a collection of their own, written through a bound method,
  and they are the input of the "related items" the panel will show
  later.

## The filters

**The filters work on the claims of the item.** A filter on
`claims.P131` asks whether the values of the `P131` claim hold a given
identifier. This is not how a REST API would do it, and it is the
right shape for a dump.

**The `P131` filter under-counts Paris.** A place that is located in
the 1st arrondissement of Paris is not directly located in Paris. The
filter as it stands only finds the items whose `P131` is `Q90` itself.
A filter that walks the administrative hierarchy would need the
hierarchy, which the subset does not carry.

**The labels of the filter values are not known.** The subset has no
labels for the entities it points at, so the filter bar shows the
identifiers. `Q90` is shown as "Paris (Q90)" because the panel names
it, and the others are shown as their identifier alone. When the
labels are cached (with the items), the bar can show them.

**The genre and the occupation filters take an identifier.** There is
no list of the genres or of the occupations in the subset, so the
filter asks for the identifier. A picker would need the list, which
would come from the cached items.

## The graph

**The graph is capped at thirty nodes.** An item of the dump can hold
thousands of claims, and the graph of the card is meant to be read at a
glance. The cap is reported under the graph.

**The layout is computed in the browser.** A force simulation of a few
dozen nodes is fast, but it is done on the main thread, in animation
frames. It is stepped, so the paint is not blocked, but a much larger
graph would need a worker.

**The nodes are the identifiers.** The subset has no labels for the
entities the claims point at, so the graph shows the identifiers. The
label of the item itself is shown, because the subset has it.

## The SPARQL query builder

**There is none, on purpose.** The panel consumes the result of a
query; it does not build one.

When the builder arrives, it will produce a set of rows that are not
records but tuples of variables. That is not a CRUD shape, so it does
not belong in `getList`: it belongs in the `custom` method of the
provider, which is the escape hatch Refine provides for exactly this.

The `WikidataSource` interface would grow a third method for it, and
the panel would show the rows as a table rather than as cards.
