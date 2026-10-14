# internal/store

The rules for the two on-disk SQLite files, for a change to a table, a key or
the way a file is opened. The root `CLAUDE.md` still applies; this adds what
holds here alone.

## Two files, read in the code

- **`workflow.db`** holds only conveniences a session can see again: the last
  scope, the announcements, the issue cache. Its tables are `createSchema` in
  `store.go`.
- **`kept.db`**, beside it, holds what the user decided and cannot be seen
  again: whom a forge owner is on Slack, a repository's groups, the directories
  marked as favorites. Its tables are `ownersSchema` (`owners.go`),
  `groupsSchema` (`groups.go`) and `favoritesSchema` (`favorites.go`), joined
  by `keptSchema` in `kept.go`.
- **Read the tables there, never a restatement here**, since a prose copy
  drifts. ARCHITECTURE.md's *The store schema* draws `workflow.db`'s tables and
  restates the version rules below, so a change to either changes it too.
- The driver is `modernc.org/sqlite`, opened by path through
  `internal/sqlitefile`: pure Go, so `CGO_ENABLED=0` holds.

## Normalized and STRICT

- **Third Normal Form — no exceptions the schema doesn't document.** A
  repeating group is a **child table**, never a JSON blob or a delimited column
  (a list in one cell fails 1NF, and so 3NF): the cached issue list is a parent
  `issue_cache` row and one `cached_issue` child per issue, not a serialized
  array. Store **no derived value**; compute it at read. The one
  denormalization the schema keeps, `status_category` beside `status`, is
  justified in the `CachedIssue` doc comment (`cache.go`): the mapping is
  Jira's, external and not ours to derive, in a disposable cache. Any other
  denormalization needs the same written justification or it is a bug.
- **Every table is `STRICT`.** A column's declared type is enforced at write
  time, which is the point, so declare real types (`INTEGER`, `TEXT`, `REAL`,
  `BLOB`, `ANY`) and never lean on SQLite's type coercion.
- **Timestamps are RFC3339 UTC `TEXT` in an `_at` column**, written by
  `timestamp(now)` in `store.go`, **never** `DATETIME`, `CURRENT_TIMESTAMP` or
  a bare epoch integer: STRICT has no date type, and a caller-passed
  `now time.Time` keeps the clock a seam. Columns are `snake_case`.

## Keys between tables

- **Referential integrity is on and enforced.** `dsnPragmas` (`connect.go`)
  sets `foreign_keys(1)` on every writing connection, since SQLite enforces
  `ON DELETE` only per connection and only when it is on.
- **Every child names its `FOREIGN KEY`.** Toward the parent that owns it,
  `ON DELETE CASCADE`. Toward the shared `slack_entity`, `ON DELETE RESTRICT`:
  `owner_slack` (`owners.go`) and `repo_group` (`groups.go`) link a Slack user
  or group whose label lives once there, each with an index on `slack_id`, so
  a Slack entity still linked cannot be deleted from under a decision.
  `pruneSlackEntities` (`kept.go`) drops only the ones nothing links to.
- **A repeating group is replaced in one transaction**: upsert the parent,
  delete the children, insert the new ones, so a re-cache never leaves a
  half-updated view.
- **Parameterized queries only.** Every value is a `?` placeholder bound
  through `database/sql`; SQL is never built by string concatenation, however
  internal the value looks. The one exception is a `PRAGMA`, which binds no
  placeholder: it may be built from this package's own constant, never from a
  value read from anywhere.

## Never trusted, never a secret

- **Trust nothing read back from disk.** A file on disk is tamperable and
  outside the process, so sanitize user, forge and tracker text on the way in
  (defense in depth) *and* again on the way out, through `internal/sanitize`,
  at the seam that renders it; and validate a row's shape on read rather than
  assume this package wrote it. A row of the wrong shape is left out, as
  `Favorites`, `RepoGroups` and the owner links do.
- **The store never holds a secret** and is keyed only by credential-free
  identifiers: a forge host and path, or the repository's root path when the
  remote is missing or unparsable; a hash of a base URL; a view's JQL. The
  repository's key is built in `internal/wiring` (`repoKey`,
  `repositories.go`), and so are the proofs that a credential cannot reach the
  files (`internal/wiring/store_test.go`, `kept_test.go`): a change to a key
  ships with that test.

## One schema version each, no migrations

`workflow.db`'s schema has one version: `schemaVersion` in `store.go` is
stamped as `PRAGMA user_version` by the live `open` that makes the file, before
its first table; a file whose version differs and holds tables is deleted with
its `-wal` and `-shm` companions and made again. `kept.db`'s schema also has
one version, `keptSchemaVersion` in `kept.go`, made and stamped in one
`BEGIN IMMEDIATE` transaction that re-reads it; a file at another version is
**never discarded**: it reads as empty and refuses writes with
`ErrKeptSchemaDiffers`, naming `workflow db-clean --all`, which every surface
reports as `unprocessable` (`internal/report/classes.go`). Neither file has
**migrations**, each for its own reason. `workflow.db` holds only what a
session can see again, so a migration for it is YAGNI: change the `CREATE
TABLE`, bump `schemaVersion`, and a file at another version is made anew and
refills. `kept.db` holds what the user decided, so going without one is a kept
trade-off: before changing a kept table, read TRADE-25 in `TECH_DEBT.md`, which
says why there is none, what a change costs (the answers go unread until the
user runs `workflow db-clean --all` and gives them again) and when that
decision reopens. A bump of `schemaVersion` never touches `kept.db`. A
`--dry-run` store's read-only open reads `workflow.db` as it is and a `kept.db`
at another version as empty, never stamps or changes either file, and never
makes one that is missing.

## How a file is opened

- **One short-lived connection per operation**, so there is no handle to close
  and the terminal and the web server share the files; WAL and a five-second
  busy timeout (`dsnPragmas`) keep their writes from colliding.
- `keptPragmas` (`kept.go`) adds `_txlock=immediate`: every `kept.db`
  transaction takes the write lock as it begins, which is where its stamp's
  `BEGIN IMMEDIATE` comes from, so two processes opening a fresh file never
  both make it.
- **A disabled store no-ops every method.** `New(dir, true)` and the zero
  `Store` are the privacy opt-out, so no caller special-cases it. `ReadOnly()`
  is the dry run's: it reads what is on disk, makes nothing, writes nothing.
- The directory is `0700` and each file `0600` (`dirPerm`, `filePerm`), which
  also covers the `-wal` and `-shm` files beside them.
- The open detaches from its asker (`context.WithoutCancel` in `open`), so a
  stamp or a removal fails only when the file changes underneath it
  (TRADE-16), never because the caller left.

## Tests: red, green, refactor

- **Red first, through the exported method.** A new table or column starts as
  a test that writes through the `Store` method and reads it back; a schema
  bump starts as one that plants a file at the old version and expects it
  discarded (`workflow.db`) or read as empty with `ErrKeptSchemaDiffers`
  (`kept.db`), as `version_test.go` and `kept_test.go` do. Watch it fail, then
  write the method or the `CREATE TABLE`.
- **Black-box, in `package store_test`**, each test on
  `store.New(t.TempDir(), false)` and `t.Parallel()`, marked `// Arrange`,
  `// Act`, `// Assert` as the root asks. Stamp every write with `theTime()`
  (`store_test.go`) rather than the wall clock.
- **Fixtures to reuse.** `seedDatabase` and `seedDatabaseAt`
  (`failure_test.go`) leave a file as another program would; `openKeptFile`,
  `execKept` and `keptPragma` (`kept_test.go`) read and tamper with `kept.db`
  directly. The `storeCall` tables in `failure_test.go` and
  `keptfailure_test.go` run every method against each failure, so a new method
  joins them. Each failure is built from content (a file that is not a
  database, a regular file where a directory goes), never from permissions,
  so it fails alike for every user, root included.
- **A `kept.db` change copies the workspace tests** (`workspace_test.go`,
  `sharedid_test.go`): one Slack ID seen from two workspaces, each keeping its
  own link, choice and groups.
- **Condition coverage.** This package holds a floor of its own in
  `scripts/gobco-floors.txt`;
  `scripts/gobco-report.sh --package <import path> <floor>` measures it alone,
  run as a user, not as root.
