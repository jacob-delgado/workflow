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
- **Text typed into a form is a `quietInput`** (`quietinput.go`), a Huh field
  of our own whose cursor never blinks (TRADE-46), never `huh.Input`. The
  accessible mode hands the terminal to Huh's prompts (`promptedForm`,
  `huhform.go`, TRADE-47).
- `layout/` places each region per terminal size and `frame/` draws a titled
  box; 80 by 24 is the size a terminal opens at: the rail keeps its 24 columns
  and the detail loses its border (`eighty_test.go`).

## One file per pane, and when code may leave

The package is the interface spelled one file per pane, overlay and composer,
and it answers to a cohesive ceiling in `scripts/package-size-budgets.txt`:
read the number there, and `scripts/check-package-size.sh --list` for the
count. A new pane or overlay is a new file. The package is not split to chase
a number. A pane or an overlay takes and returns `Model`, and so cannot leave
without carrying `Model` across a package boundary Go does not allow. What may
leave is a leaf with a reason to change of its own and no `Model` in it, as
`layout/` and `frame/` did: `quietinput.go`, which changes as `huh.Field`
does, is one. Reaching the ceiling means rewriting the WHY above the entry and
a history row, or such an extraction, never a split for the count.

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
  `WithLifetime` wraps each write in `outlasting` (`deps.go`): it goes on
  past a quit, so git is never stopped midway with its index locked, is
  counted in flight by name, and ends only when the program's lifetime does,
  on SIGINT or SIGTERM (TRADE-38). A new write seam joins `outlasting`.
- **What leaves the machine waits on a last look** (`lastLook`, `overlay.go`):
  a push, a post, a merge, a Taskwarrior sync and the rest UX.md's principle 2
  names. A reversible local toggle acts at once.
- **A failure is never drawn as empty or as rest.** It is said in the failure
  voice through `failure.go` (`noticedFailure`), in the words `errorSentence`
  (`errorwords.go`) finds for it; a service not set up is guidance
  (`noticedGuidance`), not a failure.

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
- **Motion** — the in-flight mark turning, a stage's rule filling, a bar
  springing — runs on frames through `Deps.After` (`m.deps.after`), never
  `tea.Tick`, which is only its nil default, and only while something moves;
  `ui.motion: reduce` (`config.MotionReduce`) draws every state still.

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
- **Test files near the ceiling.** A test file fails past 700 lines, and
  `tasks_test.go`, `setupform_test.go`, `help_test.go`, `world_test.go` and
  `tasksrender_test.go` are at 600 or more (`scripts/check-file-length.sh
  --list` prints each count): a new case for one of them likely starts a
  sibling file named for its behavior.
