# The sync with a remote server

WikiRevele can move collections between its own database and a remote
PocketBase server, which the user calls **Ruis**. The two sides are
both PocketBase, and the sync moves two things: the **schema** of a
collection and the **records** in it.

Every sync is started from WikiRevele. Ruis never calls back: it is a
server that answers, and the desktop application is the one that
asks.

## What PocketBase already does

The first version of this feature assumed that PocketBase had no
support for a partial sync, and that the whole thing had to be built
by hand. That was wrong for the version this repository pins
(v0.39.11), and it is worth writing down what the library gives us,
because it is most of the work:

| What | Where | What it does |
| --- | --- | --- |
| The schema of a collection | `GET /api/collections` | Answers with the whole schema as JSON. |
| The schema of a collection | `PUT /api/collections/import` | Applies a schema in one transaction. Superuser only. |
| The records of a collection | `PUT /api/collections/{c}/records` inside `POST /api/batch` | Upserts a record by its identifier. |
| The records of a collection | `GET /api/collections/{c}/records?filter=…` | Answers with the records that match a filter, page by page. |

**Ruis needs no Go extension at all.** A standard PocketBase server is
enough. The one thing that has to be changed on it is a setting: the
batch API is **off by default**, and the sync needs it.

```go
// On the server, once.
app.Settings().Batch.Enabled = true
```

The batch API is what upserts: `PUT` on the route of the records
creates the record when it is not there and updates it when it is.
That is the idempotency the sync needs, and it comes with the
library.

## The problem with `updated`

The plan of the feature was to make the conflict resolution on the
`updated` field of the records, because PocketBase keeps it up to date
on its own.

**That does not work.** The server overwrites `updated` with the time
of the write, and it does it over the REST API as well:

```
POST /api/collections/notes/records
{"title":"first","updated":"2020-01-02 03:04:05.000Z"}

→ {"id":"…","title":"first","updated":"2026-11-01 02:35:02.168Z"}
```

The time that was sent is gone. The same happens to every `autodate`
field, because the setter of an autodate field is a no-op: the value
of the field is owned by the server.

So the newest record is always the one that was written last, and a
push would overwrite a newer local edit with an older one, and a pull
would overwrite a newer remote edit. Last-write-wins on `updated` is
last-write-wins by accident, not by comparison.

## The fields of the sync

The sync owns two fields in every collection it syncs, and it writes
them and nothing else does:

| Field | Type | What it holds |
| --- | --- | --- |
| `source_updated` | text | The time the record was last changed **on the side it came from**. |
| `source_origin` | text | The side it was last written on: `local` or `remote`. |

They are **text** fields and not autodate ones, and that is the whole
trick. A text field keeps the value that is given to it, on both
sides, over the Go API and over the REST API. The value is the text
of the format of the server:

```
2026-11-01 02:35:02.168Z
```

That format sorts the same way the times sort: the year, the month,
the day, the hour, the minute and the second are all in their natural
order in it. The filter of the server is a comparison of strings, and
the comparison has to be the comparison of the times.

The fields are added to a collection when it is synced, and the
addition is idempotent: a collection that already has them is left
alone. A collection of an older database becomes syncable without the
user having to write a migration.

## The protocol

### The cursor

The sync remembers the time of the last run, per collection and per
direction, in the `app_settings` collection. The next run only moves
the records that changed since then, and it moves them with a filter
on `source_updated`.

The cursor is the time of the **run**, and not the time of the newest
record that was moved. A record that was written while the run was
going is newer than the cursor, and it is picked up by the next run.
A cursor that was the time of the newest record would lose it.

### The order

The collections are sorted so that a relation always points at
something that is already there: a collection is synced before the one
that points at it. A relation that makes a circle has no order that
satisfies it, and the collections of the circle are left in the order
they were given.

### The three directions

| Direction | What it does |
| --- | --- |
| **Push** | The local records are sent to the server. |
| **Pull** | The records of the server are taken into the local database. |
| **Reconcile** | Both sides are compared, and the newer record wins. |

A push that carries the schema sends it first: a record cannot be
written into a collection that does not exist.

### The conflict resolution

The rule is the same everywhere: **the record with the newer
`source_updated` wins.**

- A **pull** takes the remote record when it is newer than the local
  one, and leaves the local one alone when it is the newer.
- A **push** does not send a record that the server has newer. The
  push would overwrite the newer record with the older one, and the
  change would be lost.

The second rule is what makes a reconcile a comparison and not two
writes in a row. Without it, the push of a reconcile would overwrite
the newer remote record with the older local one before the pull
could see it.

### The batches

The records are written in batches of forty, and the server accepts
fifty by default. The number is well under the limit so that a run
does not fail on the last batch against a server that was never
configured.

The records are read page by page, a hundred at a time, so that a
collection of a million records does not have to be held in memory.

### The schema

The schema of a collection is the JSON of the collections API, with the
same identifiers on the two sides: the identifier of the collection
and the identifier of each of its fields. That is what makes a
relation point at the same collection on the two sides.

The import is sent with `deleteMissing` **false**. The import of the
collections API deletes every collection that is not in the list when
it is asked to, and a sync of two collections must not delete the
third one.

## The credentials

The sync authenticates as a superuser of the remote server, over
`POST /api/collections/_superusers/auth-with-password`. The token is
held in memory and is fetched when it is missing; a request that is
refused with a 401 is retried once with a new token.

The password is a secret and it is **stored encrypted**, with the same
cipher as the API key of the application. It is never returned to the
frontend: the panel only ever learns whether one is stored.

The address of the server, the identity of the superuser and the set
of the collections live in the `app_settings` collection, next to the
credentials. The collection has no public rule, so the Go backend is
the only reader of them.

## The log of a sync

Every run writes a line to `sync.log`, next to the database in the
data directory. A sync of a large collection has more to say than the
interface can show, and the log is the trace of the run.

The progress of a run is pushed to the frontend as a Wails event,
`sync:progress`, and the panel shows the collection it is on, the
number of collections that are finished and the error of a failure.

## The shape of the code

| File | What it holds |
| --- | --- |
| `internal/sync/types.go` | The directions, the set of the collections and the cursors. |
| `internal/sync/client.go` | The HTTP client: the authentication, the token and the transport. |
| `internal/sync/records_client.go` | The two operations against the server: the batch and the filtered list. |
| `internal/sync/local.go` | The local side: the reading and the writing of the records. |
| `internal/sync/schema.go` | The schema of the collections and the fields of the sync. |
| `internal/sync/service.go` | The run: the order, the directions and the conflicts. |
| `internal/sync/settings.go` | The configuration, in the settings of the application. |
| `internal/sync/log.go` | The log of the runs. |

The service is a thin adapter in `internal/bindings`, and the frontend
never talks to the remote server: it goes through the bindings.

## How to extend it

**Another direction.** Add the value to the `Direction` constants in
`internal/sync/types.go`, the label to `Direction.Label`, and the case
to `runCollection` in `internal/sync/service.go`. The panel picks the
directions up from `DIRECTION_LABELS`.

**Another conflict rule.** The rule is in `selectWinners` in
`internal/sync/service.go` and in the push of `internal/sync/service.go`.
Both halves have to agree: a rule that is only in the pull would let
the push overwrite what the pull refused.

**Another transport.** The client is behind the `Client` type. A
server that is not a PocketBase needs a client of its own, and the
service does not have to change: it asks for the collections, for the
records and for the schema, and nothing else.

**A file on the server.** Some deployments would rather have a file
than a server. The records are a list of JSON documents and the schema
is one JSON document, so the two ends of a file transfer are the
`LocalSchema` and the `SaveRecords` of the local store.
