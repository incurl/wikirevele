# The Wikidata dataset pipeline

The dataset is produced in two stages, in one binary:

```
latest-all.json.bz2
        │
        │  the ETL worker (the "etl" subcommand)
        ▼
paris-wikidata-subset.ndjson      the items of the first target
shanghai-wikidata-subset.ndjson   the items of the second target
wikidata-properties.ndjson        the property dictionary
        │
        │  the desktop application (internal/wikidata)
        ▼
PocketBase                         wikidata_items
                                  wikidata_properties
        │
        │  the panel (frontend/src/features/wikidata)
        ▼
the cards, the filters and the graph
```

The worker is the `etl` subcommand of the same executable as the
application. The two modes never run together: with the subcommand the
program is the worker, and without it the program is the desktop
application.

The boundary between the two stages is a **process** boundary, not a
package one. The application never reads a dump and never trims
anything: the trimming happens once, in the worker, and the application
imports what the worker wrote.

| Rule | Where it lives |
| --- | --- |
| Only the worker reads a dump | `internal/etl` |
| Only the worker trims the data | `internal/etl/trim.go` |
| Only the application imports the result | `internal/wikidata/importer.go` |
| The application never decompresses anything | there is no bzip2 import in the application |
| The worker never decompresses anything | it reads the standard input, never a bz2 file |
| The worker never opens the database | it writes the files and nothing else |
| Both sides share one shape | `internal/wikidata/schema` |

## Stage 1: the ETL worker

The manual of the worker is [`etl-manual.md`](etl-manual.md): how to
run it, what it writes, and what to do when it misbehaves. This section
is the design of the two stages.

```bash
PATH="$HOME/go/bin:$PATH" wails3 task build:etl

# The dump comes from the standard input, already decompressed.
bzcat latest-all.json.bz2 | ./bin/wikirevele etl --target-qid Q90 --langs en,fr
```

The decompressor is a separate process on purpose: the Go program never
reads a bz2 file, so the decompression and the parsing run at their own
pace, and the program only ever holds one entity in memory.

The command writes two files in the current directory:

| File | Content |
| --- | --- |
| `paris-wikidata-subset.ndjson` | One item per line: the entities linked to Paris. |
| `wikidata-properties.ndjson` | One property per line: the whole property dictionary. |

Both are NDJSON: one JSON value per line, so they can be imported a line
at a time.

The command makes no network request and reads no configuration file.

### What it keeps

An item is kept for a target when **one of its mainsnak statements**
satisfies every rule:

- the property is on the allowlist:
  `P131, P915, P1448, P1071, P1552, P291, P159, P551, P1037, P19, P20`,
- `snaktype` is `value`,
- `datatype` is `wikibase-item`,
- the value is the target.

The qualifiers of the statement are **not** read: a statement that
points at the target with a qualifier that contradicts it is still a
statement that points at the target.

An item that satisfies the rule for two targets is kept for both, and
it is written to both files.

Every property of the dump is kept: the property dictionary is the
catalogue the panel reads the names of the properties from. It is the
same for every target, so it is written once per run.

The entities that are neither items nor properties — the lexemes and
the media information records — are skipped.

### What it trims

The trimming happens **here and nowhere else**. The application never
removes a field.

| Entity | Kept | Removed |
| --- | --- | --- |
| Item | `id`, `type`, labels (`en`, `fr`), descriptions (`en`), aliases (`en`), the whole claims tree | sitelinks, references, every other language, the metadata |
| Property | `id`, `type`, labels (`en`), descriptions (`en`), the whole claims tree | sitelinks, references, every other language, the metadata |

The claims are kept whole, qualifiers included: a statement that
carries a qualifier says something the plain value does not.

### How it runs

The dump is more than a hundred gigabytes, so the command holds **one
entity in memory at a time**. The scanner reads a line, the line is
filtered and trimmed, the result is written, and the line is dropped.

The items and the properties are read in **one pass** over the dump.

A line that is not valid JSON is counted and skipped: the run carries
on.

The progress goes to the standard error, one line every hundred
thousand entities:

```
scanned=100000 items=812 properties=1244 skipped=37 malformed=0
```

## Stage 2: the import

The application imports the files into PocketBase. It never reads a
dump.

A run over several targets wrote one file of items per target, and
every one of them is imported: the items of the targets are merged into
the same collection, and the identifier of an item is its key, so an
item that belongs to two targets is imported once.

The collections are created by the migration in
`internal/pocketbase/migrations/1732000000_add_wikidata_import.go`:

| Collection | Key | Fields |
| --- | --- | --- |
| `wikidata_items` | `qid`, unique | `qid`, `type`, `labels`, `descriptions`, `aliases`, `claims`, `imported_at` |
| `wikidata_properties` | `pid`, unique | `pid`, `type`, `labels`, `descriptions`, `aliases`, `claims`, `imported_at` |

The import:

- reads the file **a line at a time**, so the memory it holds does not
  depend on the size of the file,
- **upserts** on the unique key, so importing the same file twice
  updates the records instead of duplicating them,
- writes the records in **batches**, so a file of a hundred thousand
  records is not a hundred thousand statements,
- **skips** the lines that are not valid records, and counts them,
- runs in the **background**, so the window stays responsive, and
  reports its progress as a Wails event.

The panel drives the import: it has a path field, the buttons for the
items, for the properties and for a whole directory, a progress bar and
a stop button.

## The panel

The panel works on the records of the database. It filters them, draws
the cards and the graph, and never touches a file.

The places of an item are read from its own claims (`P915` for the
filming locations, `P840` for the narrative ones), so that the cards
and the graph agree with what is in the database.

## Testing it without a dump

The pipeline is tested with a small file that has the shape of a dump:

```bash
# The ETL worker over a sample dump. The dump comes from the
# standard input, so it is piped in.
cat sample.json | ./bin/wikirevele etl --out-items items.ndjson --out-props props.ndjson

# The import into an isolated database.
WIKIREVELE_DATA_DIR=/tmp/vk ./bin/wikirevele-server
```

The application imports the files from the data directory, which is the
directory the panel fills in by default.
