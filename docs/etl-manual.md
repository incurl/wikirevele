# The ETL worker: manual

The `etl` subcommand turns the Wikidata dump into the two clean
datasets the desktop application imports.

It is the **same binary** as the desktop application. The subcommand
decides which of the two modes runs, and the two never run together:

```bash
wikirevele          # the desktop application
wikirevele etl ...  # the extraction worker
```

The worker is **dual use**. The application spawns it as a child
process, and a user runs it by hand from a terminal. The code is the
same in the two cases, and so is the behaviour.

This manual is the operator's guide: how to run it, what it does, and
what to do when it misbehaves. For the design of the two-stage
pipeline, read [`wikidata.md`](wikidata.md).

---

## Contents

- [What it does](#what-it-does)
- [Building it](#building-it)
- [Running it](#running-it)
- [The options](#the-options)
- [The events](#the-events)
- [The output](#the-output)
- [What is kept](#what-is-kept)
- [What is trimmed](#what-is-trimmed)
- [The progress and the summary](#the-progress-and-the-summary)
- [Performance](#performance)
- [Importing into the application](#importing-into-the-application)
- [Running it over the full dump](#running-it-over-the-full-dump)
- [Troubleshooting](#troubleshooting)
- [Testing the command](#testing-the-command)

---

## What it does

It reads the Wikidata dump **one entity at a time** and writes two
NDJSON files:

```
latest-all.json.bz2
        │
        │  bzcat (another process)
        ▼
   stdin ──► wikirevele etl ──► paris-wikidata-subset.ndjson
                                 wikidata-properties.ndjson
```

It reads both shapes the dump is published in:

| Shape | What it looks like |
| --- | --- |
| **JSON Lines** | One entity per line. This is what `latest-all.json.bz2` holds. |
| **JSON array** | `[` on its own line, then one entity per line separated by commas, then `]`. This is what some mirrors publish. |

The shape is detected from the first line, and the framing is removed,
so that both give the same result. You do not have to convert one into
the other.

Three things happen in one pass over the dump:

1. **Select** — keep the items that are linked to Paris.
2. **Trim** — remove the fields and the languages nobody asked for.
3. **Write** — one JSON object per line, so that the application can
   import the files a line at a time.

The items and the properties are read in the **same pass**. The dump is
not walked twice.

---

## Building it

```bash
cd /home/peter/GolandProjects/wikirevele
PATH="$HOME/go/bin:$PATH" wails3 task build:etl
```

The binary is `bin/wikirevele`, and it holds both modes.

The equivalent, without the task:

```bash
go build -o bin/wikirevele .
```

There is **one module and one binary**. The worker is not a separate
program: it is the `etl` subcommand of the same executable, and the
application spawns that executable with the subcommand.

---

## The decompressor is not shipped

The worker reads a **decompressed** stream from the standard input. It
never opens a `.bz2` or a `.zst` file, and it holds no decompressor.

The host has to provide one:

| Tool | Reads |
| --- | --- |
| `bzcat` | `.bz2` |
| `zstdcat` | `.zst` |
| `gzip -dc` | `.gz` |
| `xz -dc` | `.xz` |

The application checks that the decompressor is there **before** it
starts a run, and says so plainly when it is missing.

---

## Running it

The dump comes from the **standard input**, and it is **already
decompressed**:

```bash
bzcat latest-all.json.bz2 | ./bin/wikirevele etl
```

That is the whole command. The output files are written to the
**current directory**:

| File | Content |
| --- | --- |
| `paris-wikidata-subset.ndjson` | The items linked to Paris |
| `wikidata-properties.ndjson` | The property dictionary |

The command takes **no positional argument**. Passing a file is
refused:

```
$ ./bin/wikirevele etl latest-all.json.bz2
wikirevele etl: the command takes no argument, the dump is read from the standard input: "latest-all.json.bz2"
```

### Why the decompressor is a separate process

`bzcat` does the decompression, not the Go program. Three reasons:

- The Go program never holds more than one entity, and never holds a
  compressed block either. Its memory does not depend on the dump.
- The decompression and the parsing run at their own pace, so a slow
  disk does not stall the parser and the other way round.
- The Go program has **no compression code at all**. The CLI module has
  no dependencies beyond the standard library.

### Other ways to pipe the dump in

Anything that writes JSON Lines to stdout works:

```bash
# The usual one.
bzcat latest-all.json.bz2 | ./bin/wikirevele etl

# A faster decompressor, if you have one.
lbzcat -cd latest-all.json.bz2 | ./bin/wikirevele etl
pbzip2 -cd latest-all.json.bz2 | ./bin/wikirevele etl

# An already decompressed file.
cat latest-all.json | ./bin/wikirevele etl

# A file you cut down first.
head -n 100000 latest-all.json | ./bin/wikirevele etl
```

> **A compressed file piped straight in is refused.** The command does
> not read a bz2 file, so `cat latest-all.json.bz2 | ./bin/wikirevele etl`
> stops at once with:
>
> ```
> wikirevele etl: the input is compressed: pipe it through a decompressor, for example "bzcat latest-all.json.bz2 | wikirevele etl"
> ```
>
> The magic bytes of bzip2, gzip, zstd, xz and zip are recognised, and
> the run stops before it writes anything.

### Stopping a run

`Ctrl-C` stops the run. The output that was already written is
flushed, so a partial run still leaves usable files.

---

## The options

| Option | Default | What it does |
| --- | --- | --- |
| `--target-qid` | `Q90` | The entity the items are linked to |
| `--langs` | `en,fr` | The languages that are kept |
| `--out-items` | `paris-wikidata-subset.ndjson` | The file the items are written to |
| `--out-props` | `wikidata-properties.ndjson` | The file the properties are written to |
| `--recursive-p131` | off | Keep everything inside the target |
| `--overwrite` | off | Allow replacing an output file that exists |
| `--events` | off | Write the machine readable events to the standard output |
| `--quiet` | off | Report nothing at all |

The options may come in any order, and the command takes no positional
argument: the dump is always the standard input.

```bash
bzcat latest-all.json.bz2 | ./bin/wikirevele etl \
  --out-items out/items.ndjson \
  --out-props out/props.ndjson
```

### `--target-qid`

The entity whose neighbourhood is kept. The default is Paris.

```bash
# Everything linked to Lyon.
bzcat latest-all.json.bz2 | ./bin/wikirevele etl --target-qid Q456
```

The identifier is checked before the run starts: a target that is not
one matches nothing, and a run that matches nothing looks like a
broken dump.

The property allowlist does **not** change with the target: the same
eleven properties are read, and the target decides which entity they
have to point at. A target that nothing points at gives an empty item
file and a full property dictionary.

### `--langs`

The languages of the labels, descriptions and aliases that are kept.
The codes are separated by commas.

```bash
bzcat latest-all.json.bz2 | ./bin/wikirevele etl --langs en,zh,fr
```

The list is cleaned before the run: the codes are trimmed, the empty
ones are dropped and the duplicates are removed.

### `--recursive-p131`

Keeps everything **inside** the target, following the administrative
hierarchy: every entity that is located in the target, and every entity
that is located in one of those.

```bash
zstdcat latest-all.json.zst | ./bin/wikirevele etl \
  --target-qid Q148 --recursive-p131 --langs en,zh
```

The hierarchy is walked **after** the pass over the dump, so the walk
needs no second pass. An entity that is inside the target is kept even
when it does not point at the target itself: a district that is
located in a city that is located in the target is inside the target.

### `--overwrite`

Without it, the command **refuses** to replace an output file that
already exists. A run that silently replaced a dataset would be a
surprise.

```bash
bzcat latest-all.json.bz2 | ./bin/wikirevele etl --overwrite
```

### `--events`

Writes the machine readable events to the standard output. The desktop
application turns them on; a terminal user does not need them.

See [the events](#the-events).

### `--quiet`

Silences the progress lines **and** the final summary. The files are
still written.

```bash
bzcat latest-all.json.bz2 | ./bin/wikirevele etl --quiet
```

---

## The events

With `--events`, the worker writes one JSON object per line on the
standard output. The human readable logs stay on the standard error, so
the two channels never mix.

| Event | What it carries |
| --- | --- |
| `progress` | The lines read, the items kept, the properties written, the speed |
| `done` | The files that were written, and the counters of the run |
| `error` | What went wrong |

```json
{"type":"progress","processedLines":123456,"matchedItems":820,"collectedProperties":4100,"speedMBs":17.8}
```

```json
{"type":"done","itemFile":"items.ndjson","propFile":"props.ndjson","scanned":123456,"items":820,"properties":4100,"skipped":0,"malformed":0}
```

```json
{"type":"error","message":"the dump is not readable"}
```

Without `--events`, the standard output stays empty. A terminal user
reads the human readable summary on the standard error instead:

```bash
bzcat latest-all.json.bz2 | ./bin/wikirevele etl
```

```
done scanned=8062 items=51 properties=0 skipped=8010 malformed=1
```

---

## The output

Both files are **NDJSON**: one JSON value per line, no array wrapper.
A line can be read on its own, which is what lets the application
import a file of a hundred thousand records without holding it in
memory.

### An item

```json
{
  "id": "Q90",
  "type": "item",
  "labels": {
    "en": {"language": "en", "value": "Paris"},
    "fr": {"language": "fr", "value": "Paris"}
  },
  "descriptions": {
    "en": {"language": "en", "value": "Paris in Paris"}
  },
  "claims": {
    "P131": [
      {
        "mainsnak": {
          "snaktype": "value",
          "property": "P131",
          "datatype": "wikibase-item",
          "datavalue": {
            "value": {"entity-type": "item", "id": "Q90", "numeric-id": 90},
            "type": "wikibase-entityid"
          }
        },
        "type": "statement",
        "rank": "normal"
      }
    ]
  }
}
```

### A property

```json
{
  "id": "P131",
  "type": "property",
  "labels": {
    "en": {"language": "en", "value": "located in the administrative territorial entity"}
  },
  "descriptions": {
    "en": {"language": "en", "value": "The property located in the administrative territorial entity"}
  },
  "claims": {}
}
```

The `claims` tree is kept **exactly as the dump wrote it**. The
statements carry their qualifiers, their rank and their references, so
the application reads the same structure the dump has.

---

## What is kept

### The items

An item is kept when **one of its `mainsnak` statements** satisfies
every one of these rules:

| Rule | Why |
| --- | --- |
| The property is on the allowlist | Only eleven properties can link something to a place |
| `snaktype` is `value` | `novalue` and `somevalue` say that a link exists or does not, not which entity |
| `datatype` is `wikibase-item` | The value must point at another entity |
| The value is the target (`Q90`) | That is what "linked to Paris" means |

The allowlist:

| Property | Meaning |
| --- | --- |
| `P131` | located in the administrative territorial entity |
| `P915` | filming location |
| `P1448` | official name |
| `P1071` | location of creation |
| `P1552` | has quality |
| `P291` | place of publication |
| `P159` | headquarters location |
| `P551` | residence |
| `P1037` | director/manager of |
| `P19` | place of birth |
| `P20` | place of death |

**The qualifiers of the statement are not read.** A statement that
points at Paris with a qualifier that contradicts it is still a
statement that points at Paris.

### The properties

**Every** property of the dump is kept. The property dictionary is the
catalogue the panel reads the names of the properties from, so it holds
all of them — not only the eleven of the allowlist.

### What is skipped

The dump mixes several kinds of entity in one stream. The ones that are
neither items nor properties are skipped and counted:

- `lexeme` (`L…`)
- `mediainfo` (`M…`)
- anything else that is not an item or a property

---

## What is trimmed

The trimming happens **here and nowhere else**. The application never
removes a field: it stores and shows what the command wrote.

| | Kept | Removed |
| --- | --- | --- |
| **Item** | `id`, `type`, labels (`en`, `fr`), descriptions (`en`), aliases (`en`), the whole claims tree | sitelinks, references, every other language, the metadata |
| **Property** | `id`, `type`, labels (`en`), descriptions (`en`), the whole claims tree | sitelinks, references, every other language, the metadata |

Two consequences worth knowing:

- **A property has no `aliases` field at all** when it has none. The
  field is absent from the JSON, not `null`.
- **The claims are never touched.** A statement that carries a
  qualifier, a rank and a reference keeps all three, because the
  qualifier says something the plain value does not.

The dump holds a label in more than three hundred languages. Keeping
the two that matter is what makes the output small enough to import in
seconds.

---

## The progress and the summary

The progress goes to the **standard error**, so that the standard
output stays free for the data. It never interferes with a pipe.

Every hundred thousand entities:

```
scanned=100000 items=812 properties=1244 skipped=37 malformed=0
scanned=200000 items=1690 properties=2481 skipped=74 malformed=1
```

At the end:

```
done scanned=5000000 items=42118 properties=12904 skipped=1840 malformed=12
```

| Field | Meaning |
| --- | --- |
| `scanned` | The lines read from the input |
| `items` | The items written |
| `properties` | The properties written |
| `skipped` | The lines that were not kept |
| `malformed` | The lines that are not JSON objects, or not valid JSON |

The counters reconcile:

```
scanned = items + properties + skipped + malformed
```

`skipped` holds everything that was read and not written: the
entities that are neither items nor properties, the items that do not
point at the target, and the lines that were passed over without being
decoded because they mention neither the target nor a property. That
last group is most of a dump, so a large `skipped` is normal.

`malformed` counts the **lines**, not the entities. It should be zero
or near zero. A large number means the input was not JSON Lines — most
often a compressed file that was piped in without `bzcat`.

`skipped` is normal: it counts the lexemes and the media information
records, which are the great majority of a dump.

---

## Performance

Measured on a 400 MB slice of the dump, on an 8-core machine:

| What | Time | Throughput |
| --- | --- | --- |
| `bzcat` alone, to `/dev/null` | ~10 s | ~40 MB/s |
| The command alone, from a decompressed file | ~0.4 s | ~980 MB/s |
| `bzcat` piped into the command | ~10 s | — |

The ETL is **not** what makes the run slow: it is hidden behind the
decompressor. Reading the same slice takes ten seconds of `bzcat` and
four hundred milliseconds of the command, so the command adds nothing
to the wall clock.

### Why it is that fast

An item is only kept when one of its statements points at the target.
The identifiers are written verbatim in the dump, so a byte search
answers the question without decoding anything:

1. A line that is not a JSON object is damaged, and it is reported.
2. A line that mentions neither the target nor a property declaration
   cannot contribute, and it is passed over. That is about 85% of a
   dump.
3. Only what is left is decoded.

Decoding every line instead costs about ten microseconds per line, and
almost all of it is thrown away.

### Making the run faster

The lever is the decompressor, not the command. `bzip2` is
single-threaded, and on a multi-core machine a parallel decompressor
is several times faster:

```bash
# If you have one of these, it uses every core.
pbzip2 -dc latest-all.json.bz2 | ./bin/wikirevele etl
lbzcat latest-all.json.bz2 | ./bin/wikirevele etl
```

A faster CPU will not help much: the run is bound by the
decompression, which is a serial block-sorting transform.

## Importing into the application

The application imports the two files into PocketBase. It never reads a
dump.

### Where the files go

The application looks for them in its **data directory**:

- `pb_data/` next to the executable, by default
- or wherever `WIKIREVELE_DATA_DIR` points

The two filenames must be exactly:

```
paris-wikidata-subset.ndjson
wikidata-properties.ndjson
```

```bash
cp paris-wikidata-subset.ndjson wikidata-properties.ndjson bin/pb_data/
```

### From the panel

Open the Wikidata panel and use the **Import the dataset** bar at the
top:

| Button | What it does |
| --- | --- |
| **Items** | Imports the items file |
| **Properties** | Imports the property file |
| **Directory** | Imports both files of a directory |

The path field is pre-filled with the data directory.

The import runs in the background, so the window stays responsive, and
the progress bar follows it. **Stop** cancels it.

### From the command line

```bash
WIKIREVELE_DATA_DIR=/tmp/vk ./bin/wikirevele-server
```

then import from the panel, or call the binding directly.

### The import is an upsert

The records are keyed on their identifier — `qid` for the items, `pid`
for the properties — and the key is **unique**. Importing the same
file twice **updates** the records instead of duplicating them.

So a re-run after a fresh ETL is safe, and so is an import into a
database that already holds the data.

A line that is not a valid record is counted and skipped, and the
import carries on. A missing file is not an error: the panel shows
what it has.

---

## Running it over the full dump

The full dump is large. A few practical notes.

### Get the dump

```bash
# The latest full dump, about 100 GB compressed.
wget https://dumps.wikimedia.org/wikidatawiki/entities/latest-all.json.bz2
```

### Let it run

```bash
bzcat latest-all.json.bz2 | ./bin/wikirevele etl
```

Expect it to take a while. The progress lines every hundred thousand
entities are how you tell it is alive.

To keep it running after you close the terminal:

```bash
nohup sh -c 'bzcat latest-all.json.bz2 | ./bin/wikirevele etl' > etl.log 2>&1 &
```

To watch it:

```bash
tail -f etl.log
```

### What to expect

The output is a fraction of the input. The items file holds the
neighbourhood of Paris — tens of thousands of records — and the
property file holds every property of the dump, about thirteen
thousand of them.

### Disk

The output is small compared to the dump. The dump itself is what
needs the disk.

---

## Troubleshooting

### The input is compressed

```
wikirevele etl: the input is compressed: pipe it through a decompressor, for example "bzcat latest-all.json.bz2 | wikirevele etl"
```

The dump was piped in without a decompressor:

```bash
# Wrong.
cat latest-all.json.bz2 | ./bin/wikirevele etl

# Right.
bzcat latest-all.json.bz2 | ./bin/wikirevele etl
```

The command stops at once and writes nothing, so there is no empty
dataset to import.

### Every line is malformed

```
done scanned=50 items=0 properties=0 skipped=0 malformed=50
```

The input is neither JSON Lines nor a JSON array, and it is not
compressed either. The usual cause is a dump that was cut in the wrong
place, or a file that is not a dump at all.

A single malformed line at the end of a cut-down dump is normal: the
last line of a slice is cut in the middle of an entity.

The first ten damaged lines are reported, and then the reports stop and
the count carries the rest:

```
level=WARN msg="the line was skipped" reason="the line is not valid JSON" line=1
...
level=WARN msg="the damaged lines are counted and not reported any more" reason="the line is not valid JSON" reported=10
done scanned=50 items=0 properties=0 skipped=0 malformed=50
```

The `malformed` count tells you whether the problem is a handful of bad
records or the whole file.

### The item file is empty

The target is not linked to by anything on the allowlist.

Check the target is an item and not a property (`Q` and not `P`), and
check that the links to it use one of the eleven properties.

```bash
# Is the target in the dump at all?
bzcat latest-all.json.bz2 | grep -c '"Q90"'
```

### The command says it takes no argument

It does not read a file. Pipe the dump in.

### `flag provided but not defined`

An option is misspelled. Run `./bin/wikirevele etl -h` for the list.

### The output files are not where I expected them

The paths are relative to the **current directory**, and there is no
option for a directory. Either `cd` where you want them, or give the
full paths:

```bash
bzcat latest-all.json.bz2 | ./bin/wikirevele etl \
  --out-items /tmp/out/items.ndjson --out-props /tmp/out/props.ndjson
```

### The application shows nothing after the import

The files are not in the data directory, or the names are wrong. The
names must be exactly `paris-wikidata-subset.ndjson` and
`wikidata-properties.ndjson`.

### The import is slow

It writes the records one at a time, in batches, in a transaction. A
hundred thousand records take a while on a slow disk. The progress bar
shows it moving.

### The run was interrupted

The output that was already written is flushed and usable. Run it
again: the import is an upsert, so the records are updated and not
duplicated.

---

## Testing the command

```bash
cd /home/peter/GolandProjects/wikirevele
PATH="$HOME/go/bin:$PATH" wails3 task test:etl
```

The tests cover the parts that are easy to get wrong:

| Test | What it holds |
| --- | --- |
| `TestLinksToParisMatchesTheTarget` | A statement that points at the target matches |
| `TestLinksToParisIgnoresAnotherTarget` | A statement that points elsewhere does not |
| `TestLinksToParisNeedsTheAllowlist` | A property off the allowlist does not match |
| `TestLinksToParisNeedsTheItemDatatype` | A string value does not match |
| `TestLinksToParisNeedsAValue` | `novalue` does not match |
| `TestLinksToParisAcceptsEveryPropertyOfTheAllowlist` | All eleven properties work |
| `TestLinksToParisIgnoresTheQualifiers` | The qualifiers are not read |
| `TestLinksToParisSurvivesADamagedClaim` | A broken claim does not stop the pass |
| `TestTrimEntityKeepsTheLanguagesItIsAskedFor` | The other languages are dropped |
| `TestTrimEntityKeepsTheClaimsWhole` | The claims are not changed |
| `TestTrimEntityOmitsTheEmptyAliases` | The empty field is absent, not null |
| `TestPipelineKeepsTheItemsThatPointAtParis` | The selection is right |
| `TestPipelineKeepsEveryProperty` | The dictionary is complete |
| `TestPipelineSkipsTheOtherEntityKinds` | The lexemes are skipped |
| `TestPipelineSkipsADamagedLine` | A broken line does not stop the run |
| `TestPipelineHandlesTheEntitiesInOnePass` | The items and the properties are read together |
| `TestPipelineWritesOneEntityPerLine` | The output is NDJSON |
| `TestPipelineStopsWhenTheContextIsCancelled` | `Ctrl-C` stops the run |
| `TestParseArgumentsReadsTheOptions` | The options are read |
| `TestParseArgumentsRefusesAnArgument` | A file argument is refused |
| `TestStripArrayIfNeededReadsAnArray` | The bracket of an array is removed |
| `TestStripArrayIfNeededLeavesJsonLinesAlone` | The JSON Lines form is not touched |
| `TestStripArrayIfNeededKeepsTheLongFirstEntity` | The first entity of a huge dump is not cut |
| `TestStripArrayIfNeededAcceptsAnEmptyStream` | An empty stream is not an array |
| `TestTrimEntityComma` | The comma of an array element is removed, a comma in a string is not |
| `TestIsArrayClose` | The closing bracket is recognised |
| `TestThePipelineReadsAnArray` | An array gives the same result as JSON Lines |
| `TestDetectCompressionRefusesBzip2` | A bzip2 stream is refused |
| `TestDetectCompressionRefusesTheOtherFormats` | gzip, zstd, xz and zip are refused |
| `TestDetectCompressionAcceptsAJsonStream` | A JSON stream is not consumed by the detection |
| `TestThePipelineRefusesACompressedStream` | The run stops before it writes anything |
| `TestLooksLikeObject` | A damaged line is recognised |
| `TestTheFastPathSkipsALineThatCannotMatch` | A line that cannot match is not decoded |
| `TestTheFastPathKeepsAnItemThatMentionsTheTarget` | The matching item is decoded and kept |
| `TestTheFastPathKeepsAProperty` | A property is kept without mentioning the target |
| `TestTheFastPathStillReportsDamage` | A damaged line is reported, not hidden by the filter |
| `TestTheCountersReconcile` | `scanned` equals the sum of the other counters |

To try it on a small file:

```bash
bzcat latest-all.json.bz2 | head -n 100000 | ./bin/wikirevele etl \
  --out-items /tmp/small-items.ndjson --out-props /tmp/small-props.ndjson
```

---

## Where the code is

| File | What is in it |
| --- | --- |
| `internal/etl/command.go` | The options, the standard input, the wiring |
| `internal/etl/filter.go` | The allowlist and the selection rule |
| `internal/etl/trim.go` | The languages that are kept |
| `internal/etl/pipeline.go` | The streaming pass, the progress, the statistics |
| `internal/etl/entity.go` | The trimmed record |
| `internal/etl/dump.go` | The part of a dump line that is read |
| `internal/etl/writer.go` | The NDJSON writer |
| `internal/etl/compressed.go` | The detection of a compressed stream |
| `internal/etl/array.go` | The framing of a dump written as a JSON array |
| `internal/etl/bfs.go` | The administrative hierarchy and the walk over it |
| `internal/etl/events.go` | The machine readable events |
| `internal/etl/validate.go` | The checks of the identifiers and the languages |
| `internal/etlpipeline/command.go` | The command the application builds and shows |
| `internal/etlpipeline/supervisor.go` | The child processes and their cancellation |
| `internal/etlpipeline/realbinary_test.go` | The whole path, with the real binary as the worker |
| `internal/etlhistory/service.go` | The record of the runs |
| `internal/wikidata/schema/` | The shape of the records, shared by both sides |

The application side is in `internal/wikidata/`, and it is described in
[`wikidata.md`](wikidata.md).

---

## The two workflows

The same worker serves two workflows, and they differ only in who
starts it.

### The desktop application

The application builds the command, shows it, and runs it:

```
The user fills the parameters
        ↓
The application validates them and builds the command
        ↓
The command is shown, and it can be copied
        ↓
The user clicks Start
        ↓
The application spawns the pipeline: [bzcat] | [wikirevele etl ...]
        ↓
The worker writes the two files
        ↓
The worker sends the done event
        ↓
The application records the run in the history
        ↓
The application imports the two files into the database
```

The command that is shown is the command that runs. There is only one
of them, so a copy always does what the button did.

A run that is cancelled stops the **whole** process tree: the
decompressor dies with the worker, and nothing is left running in the
background.

The worker is **never** a test binary. A test binary run with the
`etl` subcommand would run the tests again, and each of them would
spawn another one, and the tests would fork without end. The launcher
refuses to spawn one, and the tests that need a worker use the
launcher seam instead.

### The terminal

The user copies the command and runs it:

```
bzcat latest-all.json.bz2 | ./bin/wikirevele etl --target-qid Q90
```

The result is the same, and the two files can be imported from the
application afterwards.

A terminal run does **not** write to the history: the history is a
record of what the application did, and the application is not there.

