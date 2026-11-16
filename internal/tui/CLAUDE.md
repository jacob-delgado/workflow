# internal/tui

How the Bubble Tea interface is built and tested, for a change to a pane, an
overlay, a key or a form. The root `CLAUDE.md` still applies; this adds what
holds here alone. UX.md's Principles and Vocabulary are binding here: an
action keeps one name through its flow, and a new word comes from that table.

## Shape

- **`Model` is a value**: `var _ tea.Model = Model{}` (`tui.go`), so every
  method on it has a value receiver, and every handler returns the whole
  model.
- **A pane is a file**, its state drawing itself with its behavior beside it.
  The panes are listed once, in `panes.go`.
- **An overlay is a value too** (`overlay.go`): it takes the keyboard until it
  closes, and `handleKey` returns the whole model, so it can replace itself,
  close itself, or change what the panes show. One draws in the detail pane;
  one that floats is drawn over the screen as a dialog through Lip Gloss
  layers (`dialog.go`). `Update` gives every key to `m.overlay`, the dialog on
  top, and relays every message no case answers to a Huh form inside it (the
  `relaying` overlays, `tui.go`).
- **Text typed into a form is a `quietinput.Input`** (`quietinput/`), a Huh
  field of our own whose cursor never blinks (TRADE-46), never `huh.Input`. The
  accessible mode hands the terminal to Huh's prompts (`promptedForm`,
  `huhform.go`, TRADE-47).
- `layout/` places each region per terminal size and `frame/` draws a titled
  box; 80 by 24 is the size a terminal opens at: the rail keeps its 24 columns
  and the detail loses its border (`eighty_test.go`).

## One file per pane, and when code may leave

The package is the interface spelled one file per pane, overlay and composer,
and it answers to a ratcheted budget in `scripts/package-size-budgets.txt`,
at the count its leaves left it: read the number there, and
`scripts/check-package-size.sh --list` for the count. A new pane or overlay is
a new file, and a bump: raise the number, rewrite the WHY above it, and append
a row to `scripts/package-size-budget-history.md`. The package is not split to
chase a number. A pane or an overlay takes and returns `Model`, and so cannot
leave without carrying `Model` across a package boundary Go does not allow.
What may leave is a leaf with a reason to change of its own and no `Model` in
it, as `layout/` and `frame/` did, as `quietinput/` did when `quietinput.go`,
which changes as `huh.Field` does, left, and as `latest/` was born a leaf: a
pane's read, whose answer counts only if no later read has begun; and
`send/`, the one write an overlay has out, which an answer changes only
while it is out; and `notice/`, the footer's notice row, which decides what
it reports and what waits for it; and `listsearch/`, the search a list is
narrowed by as it is typed and once it is kept, which takes the same keys
wherever a list is searched. Code with no `Model` in it leaves for the
package whose concern it is even when its file stays, as the commit scopes
offered did for `internal/convention` (`ScopeSuggestions`) and the
outlasting plumbing for `internal/lifetime` (`lifetime.Outlasting0` and the
rest).

## Deps: the world outside the terminal

- Every outside call is a function field of `Deps` (`deps.go`), grouped as
  `internal/seams` declares them, so the model holds no credential and a test
  hands it canned answers. `EditorDeps` stays here, not in `seams`, since it
  speaks Bubble Tea's messages (TRADE-11).
- **A load the interface starts by itself is skipped when its function is
  nil**, so a test supplies only what it exercises; an action is offered only
  once the model has what it acts on.
- **Reads end with the session, writes do not.** `WithContext` (`options.go`)
  sets the session a quit or a switch ends, stopping every read in flight.
  `WithLifetime` wraps each write in `outlasting` (`deps.go`), over the
  wrappers in `internal/lifetime/outlasting.go`: it goes on past a quit, so
  git is never stopped midway with its index locked, is counted in flight by
  name, and ends only when the program's lifetime does, on SIGINT or SIGTERM
  (TRADE-38). A new write seam joins `outlasting`.
- **What leaves the machine waits on a last look** (`lastLook`, `lastlook.go`):
  a push, a post, a merge, a Taskwarrior sync and the rest UX.md's principle 2
  names. A reversible local toggle acts at once.
- **A failure is never drawn as empty or as rest.** It is said in the failure
  voice through `failure.go` (`noticedFailure`), in the words `errorSentence`
  (`errorwords.go`) finds for it; a service not set up is guidance
  (`noticedGuidance`), not a failure.

## Reads: only the latest answer counts

A pane reads the world again and again, and a service answers two reads out
at once in whatever order it likes. These rules keep a slow answer from
putting back what a later read replaced.

- **A pane's read is a `latest.Read`** (`latest/`), never a counter, a key
  or a flag of its own: an answer is taken only for the round begun last.
- **Every read begins in a helper that begins nothing without its seam**
  (`readingTasks`, `readingIssues`, `lookingForHooks`, `checkCI` and the
  rest, each beside its pane), so a missing seam never leaves a read out
  or a mark turning. `Init` reads in round 0, which any `Begin` supersedes.
  Nothing is begun in `New` but the Issues list, whose cache stands in from
  the start; Repositories begins its round as it opens on the pane
  (`openedOnRepositories`).
- **A write's answer begins the read after it, or tells its value**
  (`Told`), so every read begun before the write is superseded, with no
  watermark. A change of what is read (another issue, file or review)
  carries the Read through `Cleared`, never a fresh literal: rounds never
  restart.
- **A view switches on the Stage through `drawRead`** (`failure.go`), whose
  map `exhaustive` checks. A view that draws in every stage (Messaging, the
  Slack directory) or never fails (Repositories) says so beside its own.
- **A failed refresh keeps what was shown, and says why beside it.** In a
  pane's detail the failure goes beneath what is held, so each line of a
  list stays the row a click picks; in a rail it goes first, so a rail of
  one row still says it. The Issues detail heads the selected issue with
  the list's failure, as it headed the configuration before, and an overlay
  (People and its groups) heads its list with its reading and failure
  lines, the list taking the rows they leave.
- **A key that writes from what a read found waits on `StageShown`**: over
  a failure, what is held is moved through, sorted and filtered, not acted
  on, and `r` offers to try again.
- **The in-flight mark is `Refreshing()`**: a read out over a value or an
  error held. A first read says it is reading, with no mark.

## Overlays: an answer finds the overlay that asked

An overlay sends a write and reads for itself, and it can close, and
another of its kind open, before an answer comes back. These rules keep an
answer from landing in an overlay that did not ask for it.

- **An overlay draws its round as it opens** (`nextRound`, `overlay.go`),
  from the one counter every overlay and run draws from, and every read and
  write it starts carries that round back (`openedAs`). A run draws one as
  it starts; a last look, which reads nothing, as its act is sent
  (`sendingLook`).
- **An answer finds its overlay through `answerFor`**: on top, or under the
  owner picker or a last look floating over it, and only while the round it
  carries is the one that overlay opened in. `withAnswered` puts it back
  where it was found. An answer for an overlay since closed finds none, even
  when another of its kind has opened since; never match the open overlay
  by its type alone.
- **The one write an overlay has out is a `send.State`** (`send/`), and
  so is the directory picker's check of a destination, though it writes
  nothing. It is handed over as `sent` and taken back as `withSend`
  (`sender[T]`), and its answer goes through `answered[T]`, which pins a
  refusal and reports a success for the applier to follow. `pinnedSend`
  draws the refusal in most overlays. The rest draw it where it reads:
  the directory picker below its rows; the status picker, the branch
  switcher and the task line in their outcome lines through
  `refusalOrProblem`, which wraps a refusal rather than cut it; the
  branch creator's refused fetch beside its offer to branch anyway; the
  tag section among its failure lines; and the settings form at the
  fields it names. What the
  overlay finds wrong before anything is sent, such as a subject the
  convention refuses or an editor that would not open, is its `problem`,
  drawn by `pinnedProblem`, never a refusal in its send.
- **What it says in the footer goes through `notice.Footer`** (`notice/`):
  `noticed` for a result, `noticedFailure` for a failure, and
  `noticedUnrepeated` for what no pane repeats, or `said` for a report built
  as a value first (`footerReport`, `footer.go`), as a step of the held post
  builds one to say with a postscript after it. The footer decides what takes
  the row and what waits for it, a failure under a search among them; the
  row is never set by hand.
- **A late answer changes nothing**: `answered` calls it late and the
  applier returns. Two keys reach one. `esc` on the directory picker
  does, since its check reads a disk that may never answer. `esc` on the
  messaging preview does while an owner link it sent is being saved: the
  link's answer finds no overlay and only frees the links' write
  (`ownerLinked`, `ownerlink.go`). Every other overlay with a write out
  takes only interrupt, which quits. The Summary post alone still says
  in the footer how its post went.

## Keys, drawing and motion

- **Every binding goes through `helpBuilder`** (`keys.go`), so the `?` help
  lists it: `TestHelpListsEveryPlacedBinding` (`help_test.go`) fails on one
  placed around it. `CheckKeys` (`keycheck.go`) validates `ui.keys` and names
  the configuration sentinel each refusal wraps.
- **The web mirrors the keys.** `KeyActions` (`keys.go`), `PaneSections`
  (`panes.go`) and `WebBoundActions` (`keycheck.go`) are what
  `web/src/features/keyboard/keysApi.ts` and `web/src/dev/mockKeys.ts`
  mirror, so a key change can reach `web/`, and `keys_docs_test.go` holds
  `configuration.md` and `usage.md` to them.
- **No theme of its own.** Styles draw in ANSI palette indices (`glyphs.go`),
  so the terminal's own theme, light or dark, decides the shades. **Shape
  carries state, not color**: a glyph's shape tells the state in monochrome
  and in ASCII, and a hue only repeats it.
- **Motion** — the in-flight mark turning (`progress.Turn`'s frames, which
  the command line's live line turns too), a stage's rule filling, a bar
  springing — runs on frames through `Deps.After` (`m.deps.after`), never
  `tea.Tick`, which is only its nil default, and only while something moves;
  `ui.motion: reduce` (`config.MotionReduce`) draws every state still, the
  mark held on the in-flight glyph.

## Tests: red, green, refactor

- **Red first, on the screen.** A change starts as a test that types the keys
  a user would and asserts what the screen then shows, or what the world was
  asked; watch it fail, then make the change. Tests are black-box, in
  `package tui_test`, through `tui.New` and the screen, marked `// Arrange`,
  `// Act`, `// Assert` as the root asks.
- **The world** (`world_test.go`): `newWorld()` is a repository on a feature
  branch for an issue in progress, with a pull request whose CI passed; a
  test changes only the fields it is about. `w.live(t, width, height)` is its
  interface, sized and with its start loaded; `w.deps()` is its `Deps`, which
  record every call, and `w.asked(prefix)` reads them back. `stillConfig()`
  reduces motion, so a screen is the state it settled in; `testNow()` is the
  clock.
- **The harness** (`harness_test.go`): `drain` and `settle` run the model to
  quiescence on a fake clock (`fakeAfter`; waits past `horizon`, one second,
  never fire, and `failsafe`, ten seconds of wall clock, fails a test whose
  fake is blocked); `typing` sends keys, `pasted` hands a Huh field its text
  at once, `click` clicks; `plain`, `footerLine` and `lineShowing` read the
  screen, `requireScreen` and `refuseScreen` assert on it; `newHold` and
  `holding` keep a seam from answering, for a write still in flight.
- **Late answers** (`withhold_test.go`): `withheldKeys` types keys and
  keeps back the answer of each command during which a seam's count grew
  (`w.counting(prefix)`), and `handedOver` delivers them later, as a slow
  answer arrives. It fails a test that kept nothing back, which would
  assert on an answer never delivered.
- **Test files near the ceiling.** A test file fails past 700 lines, and
  `tasks_test.go`, `setupform_test.go`, `help_test.go`, `world_test.go` and
  `tasksrender_test.go` are at 600 or more (`scripts/check-file-length.sh
  --list` prints each count): a new case for one of them likely starts a
  sibling file named for its behavior.
