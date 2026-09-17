# Technical debt

What this repository owes itself: defects that are waiting for the right
input, shortcuts that will make the next change harder, gates with blind
spots, and docs that have drifted from the code. It is a record, not a plan.
Nothing here is scheduled.

Two readers are in mind: a contributor looking for something worth fixing,
and a later Claude Code session asked to "pick up DEBT-18". Each entry says
what is wrong, where, what it costs, one way to fix it, and how to tell when
it is fixed. [FEATURES.md](FEATURES.md) and [UX.md](UX.md) hold the ideas;
this file holds the debts.

Checked against commit `817d323` on 2026-09-17. Line numbers drift, so every
pointer also names the symbol it means.

## How this was produced

1. **An audit.** Every non-test Go file, every script, every workflow and
   every page of docs was read in full, against the standard the project sets
   for itself in [CLAUDE.md](CLAUDE.md).
2. **An attempt to refute it.** Each finding was handed to a second pass that
   had not written it, with instructions to prove it wrong: read the callers,
   check the primary source, run an experiment. Of 126 claims, 98 held as
   written, 28 were corrected in some detail and none was refuted; the
   corrected versions are what is written here. Experiments ran against
   scratch copies and scratch repositories; nothing in this repository was
   changed.
3. **A third read.** The cited lines of every high and medium entry were read
   again while writing this file.
4. **Measurement.** `task test:cover` and `task cover:branch` were run at this
   commit for the test-suite section.

## How to read an entry

- **Severity** is high (a wrong result a user could act on, or a blocked
  release), medium (a real defect with a narrower trigger, or a shortcut that
  several future changes will trip over) or low (friction and tidiness).
- **Confidence** is *reproduced* (an experiment or a live run showed it),
  *measured* (a tool reported it) or *read* (two independent readings of the
  code agree). Anything that rests on outside behavior nobody checked says so.
- **Done when** is observable, so a test or a command can assert it.

An entry is accidental debt unless it appears under
[Deliberate trade-offs](#deliberate-trade-offs-that-carry-a-cost), which lists
the choices that were made on purpose and written down, and what they cost.

## Security-relevant findings

They are not in this file. [SECURITY.md](SECURITY.md) asks that a security
problem stay private until a fix has shipped, and this audit follows the same
rule for what it found: those findings went to the maintainer directly. The
entries below contain nothing that helps anyone misuse a credential, a
terminal or a release.

## The terminal interface

### DEBT-02 CI polling chains multiply

Severity: medium · Confidence: reproduced

- Evidence: `pullFound.apply` replaces the whole `reviewState`
  (`internal/tui/review.go:59`), which zeroes `polling` while a tick may be
  outstanding. `ciPoll` (`internal/tui/review.go:117`) carries no pull request
  number or generation, and `keepPolling` (`internal/tui/review.go:105`)
  starts a tick whenever `polling` is false. Every `branchLoaded` leads to a
  new `pullFound` (`internal/tui/branch.go:45`).
- Cost: measured with a 50 ms interval over 500 ms: 0, 1, 2 and 3 refreshes
  gave 10, 20, 30 and 40 `CheckStatus` calls. Each `r`, commit or push while
  CI runs adds up to one more chain, until CI finishes. The state also drops
  back to "checking…" on each refresh of the same pull request. Separately,
  `ciChecked.apply` never looks at its error, so a failing check with a post
  waiting is asked again at the full rate for as long as the program runs.
- Remedy: give `ciPoll` the pull request number and a generation, as `runs`
  does for command runs; keep `ci`, `checked` and `polling` when the same pull
  request is found again; back off on errors.
- Done when: a test that refreshes twice during polling counts one check per
  interval.

### DEBT-03 Errors are styled and then wrapped

Severity: medium · Confidence: reproduced

- Evidence: `wrap(m.failure(m.changes.err), width)`
  (`internal/tui/commits.go:102`); the same order at
  `internal/tui/detail.go:253` with the wrap at `:229`, and
  `internal/tui/review.go:180` with the wrap at `:187`. `wrapLine`
  (`internal/tui/render.go:166`) adds no reset, and neither does the frame.
- Cost: the color opens on the first row and closes on the last. Seen on a
  live terminal: outside a repository, red ran from the Commits error across
  three rows, through the pane's right border, the rail's borders and the
  title "2 Branch". It is the first screen of anyone who starts the program
  in the wrong directory. Tests cannot see it, because they render without
  color.
- Remedy: wrap first, then style, so each row closes what it opens. A test
  can force a color profile and assert that every row which opens a color
  closes it.
- Done when: that test exists and passes.

### DEBT-04 An overlay's outcome can be clipped or pushed off screen

Severity: medium · Confidence: reproduced

- Evidence: six overlays append their state line after their body, unwrapped:
  `internal/tui/hookgen.go:92`, `internal/tui/comment.go:77`,
  `internal/tui/prcomposer.go:135`, `internal/tui/slack.go:177`,
  `internal/tui/branch.go:220`, `internal/tui/picker.go:123`. Overlay bodies
  are returned as they are (`detailContent`, `internal/tui/render.go:83`) and
  the frame clips rows past its height. Six of eight overlay views ignore the
  row count they are given (`view(width, _ int)`).
- Cost: a failed lefthook install at 80×24 with two hooks showed no error at
  all; at 120×30 it showed. A long reason is cut with an ellipsis on any
  terminal. `internal/tui/picker.go:183` states the rule this breaks: "a
  refused change must never go unseen".
- Remedy: draw the state first, under the title, as `commandRun.view` does,
  and wrap it to the width.
- Done when: a screen test at 80×24 sees the full failure text in every
  overlay.

### DEBT-05 Scrolling has no memory of where the end is

Severity: medium · Confidence: reproduced

- Evidence: the offset grows without bound (`m.scroll += m.halfPage()`,
  `internal/tui/tui.go:175` and `:202`; `internal/tui/mouse.go:88`) and is
  clamped only when drawn (`scrolled`, `internal/tui/render.go:143`).
  `pickChange` maps a click with the raw value (`internal/tui/commits.go:197`).
  `handleCommitsKey` moves the selection without moving the view
  (`internal/tui/commits.go:176`), and the Commits detail draws every row.
- Cost: after twelve scroll-downs on a long issue, six scroll-ups changed
  nothing. With 40 changed files, thirty `j` presses put the selection off
  screen, where `space` still acts on it. The other lists use `window` and do
  not have this problem.
- Remedy: clamp where the offset is written, with one helper that knows the
  content's height, and window the change list like the others.
- Done when: the selected file is on screen after any number of `j` presses,
  and one scroll-up after over-scrolling moves the view.

### DEBT-06 The pull request composer loses what was typed

Severity: medium · Confidence: reproduced

- Evidence: `withTemplate` overwrites `c.body`
  (`internal/tui/prcomposer.go:102`) and `ctrl+t` calls it even when there is
  one template. When `enter` must push first, the composer survives only
  inside the `succeeded` closure (`internal/tui/prcomposer.go:279`); `esc` on
  a failed run is `closeOverlay` (`internal/tui/run.go:227`).
- Cost: a description written in the editor is wiped by a key labeled "next
  template". A failed push followed by `esc` discards the title, the
  description and the draft flag. The commit composer keeps `m.draft` for
  exactly this case; this one keeps nothing.
- Remedy: re-template only a body that has not been edited; keep a draft, as
  the commit composer does.
- Done when: after a failed push, `esc` and `n` reopen the composer with the
  typed title.

### DEBT-07 Six overlays, one state machine, written six times

Severity: medium · Confidence: read

- Evidence: a `sending bool` beside an error named three ways (`applyErr`,
  `err`, `problem`) in `statusPicker`, `commentPreview`, `branchCreator`,
  `prComposer`, `slackPreview` and `hookgenOffer`. Each repeats a view tail, a
  footer guard, a key guard and a failure applier of the same shape
  (`internal/tui/picker.go:60`, `internal/tui/comment.go:134`,
  `internal/tui/branch.go:296`, `internal/tui/prcomposer.go:315`,
  `internal/tui/slack.go:305`, `internal/tui/hookgen.go:144`). The editor
  round trip is line for line the same at `internal/tui/composer.go:253`,
  `internal/tui/prcomposer.go:239` and `internal/tui/slack.go:299`. The fourth
  copy differs: `commentEdited.apply` closes the overlay on an editor error
  (`internal/tui/comment.go:47`), so a failed re-edit discards a written
  comment while the other three keep their text. Reproduced.
- Cost: six copies is twice the rule of three, DEBT-04 exists because each
  copy draws its own outcome line, and the one copy that diverged has a bug
  the others do not.
- Remedy: a small value type, `sendState{sending bool; err error}`, held as a
  named field, with the view lines, the lock and the failure transition on
  it; one `textEdited` message for the editor round trip.
- Done when: the failure appliers are one function, and the comment preview
  survives an editor failure.

### DEBT-08 `failure` is "the one way", and thirteen places go around it

Severity: medium · Confidence: read

- Evidence: `failure` (`internal/tui/render.go:272`) is documented as "the
  one way the interface says something broke". These draw the glyph and
  `err.Error()` unstyled instead: `internal/tui/picker.go:123` and `:160`,
  `internal/tui/comment.go:77`, `internal/tui/branch.go:220`,
  `internal/tui/prcomposer.go:135`, `internal/tui/slack.go:100` and `:177`,
  `internal/tui/hookgen.go:92`, `internal/tui/run.go:168`,
  `internal/tui/review.go:140`, `internal/tui/composer.go:114` and `:158`,
  `internal/tui/fields.go:101`. `internal/tui/issues.go:135` draws no glyph
  at all. Overlays are handed the glyphs (`marks`) and not the styles, which
  explains the overlay sites; `review.go:140` and `slack.go:84` are `Model`
  methods with `failure` in reach.
- Cost: red is the one color with a rule ("red always means something broke",
  `internal/tui/glyphs.go:76`) and most failures are not red. There is also no
  single place to wrap or clean error text before it is drawn.
- Remedy: pass one small `theme{marks, styles}` into `overlay.view`, with
  `failure(err, width)` on it.
- Done when: no site outside `failure` concatenates `marks.failed` with an
  error.

### DEBT-09 Dry run is twelve `if`s, not a property of the seam

Severity: medium · Confidence: read

- Evidence: `if m.dryRun` at `internal/tui/picker.go:258`,
  `internal/tui/comment.go:110`, `internal/tui/branch.go:274`,
  `internal/tui/commits.go:221`, `:242` and `:276`,
  `internal/tui/composer.go:282`, `internal/tui/prcomposer.go:274`,
  `internal/tui/run.go:296`, `internal/tui/slack.go:218` and `:234`,
  `internal/tui/hookgen.go:125`. `WithDryRun` (`internal/tui/tui.go:90`) sets
  a flag and swaps nothing.
- Cost: a new write that forgets its guard writes for real under `--dry-run`,
  and nothing structural stops it. DEBT-35 makes this worse: no test connects
  the flag to the model at all.
- Remedy: keep the per-action messages, and have `WithDryRun` also replace
  every write function in `Deps` with one that returns a sentinel, so a missed
  guard fails safe.
- Done when: a test that calls every write seam under dry run sees none of
  them reach the fake.

### DEBT-10 Five lists, three clamps, and click math that mirrors the view

Severity: low · Confidence: reproduced

- Evidence: selection is clamped as `move` (`internal/tui/issues.go:91`),
  `max(0, min(…))` (`internal/tui/picker.go:191`), `min(x+1, max(0, n-1))`
  (`internal/tui/run.go:234`, `internal/tui/commits.go:177`) and with no lower
  bound at `internal/tui/fields.go:130`, which yields -1 for a field with no
  options. `window(selected, count, rows int) (int, int)`
  (`internal/tui/issues.go:149`) takes three ints that can be transposed.
  Each click handler recomputes its view's layout from constants
  (`internal/tui/run.go:246`, `internal/tui/picker.go:205`,
  `internal/tui/commits.go:196`, `internal/tui/issues.go:122`);
  `internal/tui/run.go:148` appends a wrapped jobs line as one element and
  `:153` counts elements, not lines.
- Cost: with eight lefthook jobs at width 100 the jobs line wrapped to three
  rows, and a click on `c.go:3` selected `a.go:1`. Any change to a view
  silently breaks its click handler.
- Remedy: one `selection` value with `moved`, `window` and `rowAt`, returned
  by the view so the click handler asks it.
- Done when: the four click handlers contain no layout constants.

### DEBT-11 A notice with a newline breaks the one-row footer

Severity: low · Confidence: reproduced

- Evidence: `stageAll` joins failures with `errors.Join`
  (`internal/tui/commits.go:255`) and `proc.Run` appends a program's stderr to
  its error (`internal/proc/proc.go:41`); `footer` passes the text to
  `ansi.Truncate` (`internal/tui/render.go:211`), which lets a newline through
  when it falls within the width.
- Cost: a staging error with two lines of stderr rendered 31 rows on a 30-row
  terminal. The progress row scrolls off and every mouse target is one row
  out.
- Remedy: `noticed` keeps the first line, or joins lines with the separator.
- Done when: no notice can make `View` taller than the terminal.

### DEBT-12 One model that every message can change

Severity: low · Confidence: read

- Evidence: `Model` has 24 fields (`internal/tui/tui.go:34`) and 100 methods
  in 16 files. Every overlay and applier takes and returns the whole value.
  Feature state sits at the root (`runs`, `draft`). Help is an overlay in
  every way but its type: `helpOpen` is special-cased at
  `internal/tui/tui.go:159` and `:168`, `internal/tui/render.go:87` and
  `:220`. `Update` itself is small and routes through `applier`.
- Cost: DEBT-02 is state that outlived what it described, and the Slack
  pane's state once was too, which is what happens when any message can reach
  any field. This is a
  judgment call, and a rewrite is not the answer.
- Remedy: make help an `overlay`; move `runs` and `draft` beside the code that
  owns them; give each pane's state the methods that change it.
- Done when: `helpOpen` is gone.

### DEBT-13 Small things in the interface

Severity: low · Confidence: read

- The branch's issue is derived at six sites
  (`convention.IssueKey(m.branch.branch.Name)` in `branch.go`, `composer.go`,
  `detail.go`, `prcomposer.go`, `spine.go`, `slack.go`), three of which drop
  the found flag and use `""` as "none". `"origin/"` is trimmed by hand at
  `internal/tui/branch.go:66` and `:90` and `internal/tui/prcomposer.go:78`.
  `m.branch.branch` appears 23 times. Remedy: `Model.branchIssue()` returning
  a typed value, and `BaseName()` on `gitrepo.Branch`.
- The pane set is written down in six places: `internal/tui/panes.go`,
  `key.WithKeys("1","2","3","4","5")` (`internal/tui/keys.go:58`),
  `pane(msg.String()[0] - '1')` (`internal/tui/tui.go:196`), the help groups,
  a second list of names in `internal/tui/spine.go:54`, and the literal
  "(4 Review)" in `internal/tui/slack.go:113`.
- The run overlay calls `hooks.Jobs(r.lines)` on every frame and on every
  click (`internal/tui/run.go:176` and `:247`), and appends output with no cap
  (`internal/tui/run.go:105`). Each output line is a message and each message
  redraws, so a chatty hook is quadratic.
- Vestigial: `Style.ASCII()` (`internal/tui/frame/frame.go:36`) is called
  only by a test; `hookgenState.offered` can never be true when it is read;
  `var _ help.KeyMap = keyMap{}` (`internal/tui/keys.go:11`) asserts an
  interface nothing uses; the comment "FullHelp is every key"
  (`internal/tui/keys.go:101`) is false by eleven bindings; the comment that
  the model "never holds … a credential" (`internal/tui/deps.go:19`) sits
  beside `Model.cfg`, which is the unredacted configuration (it is not shown;
  a test proves that).
- A failed Slack post's error stays in `slackState.err` until a post succeeds,
  so after a branch change the Slack pane shows one pull request's error
  beside another's announcement (`Model.slackState`, `internal/tui/slack.go`).
- "1 files staged" (`internal/tui/composer.go:155`) is pinned by
  `internal/tui/composer_test.go:50`. `Breaking: false` is hardcoded at
  `internal/tui/composer.go:92`.
- Loads carry no sequence number, so the last answer to arrive wins.
  `detailLoaded` checks the key only (`internal/tui/detail.go:49`). Every
  request is bounded at ten seconds, which keeps the window narrow.

## The clients: forge, Jira and Slack

### DEBT-15 GitHub CI listings stop at the first page

Severity: medium · Confidence: read

- Evidence: `githubStatus` asks for `/status` with no `per_page` and
  `/check-runs?per_page=100` with no second page
  (`internal/forge/github.go:87` and `:92`). The decode structs
  (`githubCombined`, `githubRuns`) drop `total_count`. GitHub documents a
  default of 30 and a maximum of 100 for both.
- Cost: a failing status past the thirtieth, or a failing run past the
  hundredth, is never seen, and the tally says passed. Truncation cannot even
  be detected, because the count is discarded.
- Remedy: decode `total_count`; page with a bounded loop; when the count still
  exceeds what was read, report running, not passed.
- Done when: a fixture with 31 statuses, the last failing, yields `CIFailed`.

### DEBT-16 The SSH port of the remote becomes the HTTPS port of the API

Severity: medium · Confidence: reproduced

- Evidence: `ParseRemote` keeps `address.Host`, port included
  (`internal/forge/remote.go:93`), and `APIBase` builds
  `"https://" + r.Host + "/api/v4"` (`internal/forge/remote.go:140`) and
  `"/api/v3"` (`:160`). The same value goes to `gh auth token --hostname`
  (`internal/forge/token.go:254`). `config.Forge` has a kind, a host and a
  token, and no setting for the API's address.
- Cost: `ssh://git@git.example.com:2222/group/repo.git`, a common shape for a
  self-managed GitLab, yields `https://git.example.com:2222/api/v4`, and the
  user has no setting to correct it. For an `https` remote, keeping the port
  is right.
- Remedy: use `Hostname()` for the API when the remote's scheme is `ssh`; add
  a `forge.base_url` override.
- Done when: the remote above resolves to `https://git.example.com/api/v4`.

### DEBT-17 Three HTTP clients copied by hand, already drifting

Severity: medium · Confidence: read

- Evidence: `HTTPClient`, `Doer`, `ErrRedirected`, `ErrUnreachable`,
  `ErrUnexpectedStatus` and `bodyLimit` are each defined three times
  (`internal/jira/jira.go`, `internal/forge/client.go`,
  `internal/slack/slack.go`). Drift so far: Jira and Slack's post path strip
  the transport error's URL (`internal/jira/search.go:132`,
  `internal/slack/post.go:152`) while the forge and Slack's identity check do
  not (`internal/forge/client.go:179`, `internal/slack/slack.go:129`), so the
  same failure reads two ways; a refused redirect says "could not reach" in
  all four although the server answered, and its target survives only where
  the URL was not stripped; hitting the body limit reads as "unexpected end
  of JSON input". Inside Jira, request-exchange-decode-wrap is repeated five
  times (`internal/jira/jira.go:112`, `internal/jira/search.go:99`,
  `internal/jira/detail.go:77` and `:117`, `internal/jira/transitions.go:54`)
  where the forge has `call[T]` (`internal/forge/client.go:113`).
- Cost: the redirect policy is the security-critical part of a client, and it
  exists three times with nothing holding the copies equal. `config.go`
  records the lesson already: "when each did this by hand, one of them forgot
  the mask."
- Remedy: one small internal package, standard library only, holding `Doer`,
  the client constructor, the three transport sentinels, a bounded read that
  says "too large", and one decision about what a transport error shows.
  Status mapping stays per service, where it genuinely differs.
- Done when: `CheckRedirect` is written once.

### DEBT-18 Every Slack refusal is reported as a rejected credential

Severity: medium · Confidence: read

- Evidence: `ErrRejected = errors.New("the credential was not accepted")`
  (`internal/slack/slack.go:41`) is returned for any `ok: false`
  (`internal/slack/post.go:86`) and any 4xx, 429 included
  (`internal/slack/post.go:144`). `internal/slack/post_test.go:133` pins it.
- Cost: "the credential was not accepted: not_in_channel" sends the user to
  rotate a working token.
- Remedy: keep `ErrRejected` for Slack's authentication codes and add a
  sentinel for a refused post.
- Done when: `not_in_channel` does not mention the credential.

### DEBT-19 Smaller items in the clients

Severity: low · Confidence: read

- No client reads `Retry-After` or a rate-limit header. Jira maps 429 to an
  unexpected status, the forge folds it into 403's message
  (`internal/forge/client.go:228`), Slack calls it a rejected credential.
  Polling alone will not reach GitHub's limit (about 360 requests an hour of
  5,000), so this is about saying the right thing when it happens.
- Adding a forge means editing eight places. Three are switches the
  `exhaustive` linter checks (`Kind.String`, `APIBase`, `environmentNames`);
  five are not, and fail quietly: `kindOf` and `ParseKind`
  (`internal/forge/remote.go`), `dialectFor`'s map
  (`internal/forge/pulls.go:46`, a runtime error), `FindTemplates`' map
  (`internal/forge/templates.go:36`, silently nothing) and `cliCommand`
  (`internal/forge/token.go:250`, silently false). Folding these into
  `dialect` would make FEAT-54 one file.
- `forge.Token`'s comment says a token cannot be printed by accident "nested
  inside any struct". It has a `String` method and nothing else, so `%#v`,
  `%d`, `json.Marshal` and `%+v` of a struct holding one in an unexported
  field (`forge.Client` is one) print the value. No production code does
  this; the guard is narrower than it claims. The four secrets in
  `config.Config` are bare strings. None of the five `Stringer` types carries
  the static assertion CLAUDE.md asks for.
- `pipelineState` (`internal/forge/gitlab.go:100`) treats every status it
  does not name as running, which includes `manual`. A pipeline waiting on a
  manual job would then wait forever. GitLab's docs list the status; that the
  blocked state is reported as `manual` was not confirmed.
- `jira.User.Active` is decoded and never read. Stale comments: "Only what
  doctor needs so far" (`internal/jira/jira.go:6`), "both tokens masked"
  (`internal/cli/config_cmd.go:122`) where five values are.

## Configuration, wiring and the command line

### DEBT-20 `doctor` says things about the forge that are not true

Severity: medium · Confidence: reproduced

- Evidence: it lists `glab` as "supplies a GitLab token when none is
  configured" (`internal/cli/doctor.go:69`); no code runs `glab`, a test
  asserts that, and the configuration guide says "There is no `glab` step".
  It says "the hook panes stay hidden" without lefthook
  (`internal/cli/doctor.go:59`); there are no hook panes. `checkForge`'s comment says it "does not call the forge"
  (`internal/cli/doctor.go:160`) and it calls `Whoami`. Offline, `forgeLabel`
  takes no settings (`internal/cli/doctor.go:288`), so it reports "cannot tell
  GitHub Enterprise from self-managed GitLab" when `forge.kind` says which,
  and no Forge line appears under Configuration, so a misspelled `forge.kind`
  is never reported on a github.com or gitlab.com remote.
- Cost: `doctor` is the diagnostic people trust and paste into bug reports.
- Remedy: drop `glab`; load the configuration before the repository section
  and pass it to `forgeLabel`; validate `forge.kind` with `ParseKind`.
- Done when: `doctor` does not mention `glab`, and `forge.kind: githb` is
  reported offline.

### DEBT-21 The forge connection is built twice, and remembered when it fails

Severity: medium · Confidence: read

- Evidence: `connectForge` (`internal/wiring/wiring.go:206`) and `checkForge`
  (`internal/cli/doctor.go:163`) each run parse, configured kind, API base and
  token resolution, and each build the same `forge.Resolver` literal. The
  comment "the same way doctor --online does" is the only link. Partial third
  and fourth copies are `templatesFor` and `forgeLabel`. `forgeDeps` wraps the
  connection in `sync.OnceValues` (`internal/wiring/wiring.go:156`), which
  remembers the error as well as the value, and `Workspace.Remote` is read
  once at start (`internal/cli/cli.go:117`).
- Cost: DEBT-20's offline drift is the two copies disagreeing already. In the
  interface, "no forge token found" persists until restart: `gh auth login`
  in another terminal changes nothing, and `r` returns the cached error. A
  remote added after start is never seen. gobco also reports that the success
  side of all four `connect()` callers is never exercised (DEBT-36).
- Remedy: one exported connect function that both callers use; remember a
  connection only when it succeeded.
- Done when: `r` after signing in finds the pull request.

### DEBT-22 Configuration is checked for emptiness only

Severity: medium · Confidence: reproduced

- Evidence: `Missing` tests `== ""` (`internal/config/config.go:322`). A URL
  is checked on each request (`internal/jira/jira.go:160`), a webhook's scheme
  at post time (`internal/slack/post.go:95`), `forge.kind` at use.
- Cost: a file with `base_url: "jira.example.com"`, an `http://` webhook and
  `forge.kind: "githb"` loads with nothing missing, and offline `doctor` says
  "Everything required is set." The insecure-webhook refusal arrives after the
  user has written the Slack message.
- Remedy: a `Config.Problems()` beside `Missing`, using the same predicates,
  printed by `doctor` and by the interface.
- Done when: `doctor` refuses the file above without `--online`.

### DEBT-23 The file format has no version and rejects what it does not know

Severity: medium · Confidence: reproduced

- Evidence: `Config` has no version field; `LoadFile` calls
  `DisallowUnknownFields` (`internal/config/config.go:219`) and returns
  `Default()` on any error; the README says the format may change before 1.0.
- Cost: the first renamed key fails every existing file with
  `json: unknown field "url"`, and the run carries on with every credential
  gone. A newer file on an older binary fails the same way. The strictness is
  deliberate and good; the missing half is what to do when the shape changes.
- Remedy: decide the story before the first rename. At the least, catch the
  unknown-field error and name the key that replaced it. See FEAT-51.
- Done when: an old key produces a message that names the new one.

### DEBT-24 Nothing can be canceled, and no subprocess has a deadline

Severity: medium · Confidence: reproduced

- Evidence: `root.Execute()` (`internal/cli/cli.go:89`) gives every command
  `context.Background()`; production code contains no `ExecuteContext`,
  `NotifyContext`, `WithTimeout`, `WithCancel` or `WithDeadline`. HTTP has
  `RequestTimeout`; `git`, `gh` and `lefthook` have nothing. `build`
  (`internal/proc/start.go:137`) sets no process group, `Cancel` or
  `WaitDelay`. `Start` documents "Canceling ctx kills the program"
  (`internal/proc/start.go:49`), which cannot happen. A push gets
  `GIT_TERMINAL_PROMPT=0` (`internal/gitrepo/branch.go:215`); a commit gets no
  environment at all (`:222`).
- Cost: a hung `git status` or `gh auth token` leaves its pane on "loading…"
  forever. A streamed run ignores every key but `ctrl+c`, which quits the
  program; a parent built the same way left its child running with a parent
  process of 1, so a push can complete after the user "quit", and the commit
  message's temporary file is never removed. `GIT_TERMINAL_PROMPT` governs
  git's own prompts only: ssh and gpg open the terminal directly while the
  interface owns it (shown for ssh's documented behavior and a child's access
  to `/dev/tty`; gpg was not installed to try).
- Remedy: `signal.NotifyContext` with `ExecuteContext`; a default deadline in
  `proc.Run`; a build-tagged Unix file that starts streamed children in their
  own session, so prompts fail fast and the group can be stopped; a "stop" key
  in the run overlay.
- Done when: a test cancels a streamed run and its grandchild exits.

### DEBT-25 `doctor --online` files every failure under one heading

Severity: low · Confidence: read

- Evidence: every failing check returns `errCredentialRejected`
  (`internal/cli/doctor.go:175`, `:197`, `:209`, `:232`, `:248`), including a
  bad `forge.kind` and an unreachable server. The three clients export 29
  sentinels; outside their own packages, one is ever tested for
  (`internal/cli/doctor.go:228`).
- Cost: with the VPN down, the error reads "a credential was rejected: jira".
- Remedy: a second sentinel chosen with `errors.Is(err, ErrUnreachable)`.
- Done when: an unreachable Jira is reported as unreachable.

## Git, hooks, conventions and processes

### DEBT-26 File names are handed to git as patterns

Severity: medium · Confidence: reproduced

- Evidence: `Stage` runs `git add --all -- <path>`
  (`internal/gitrepo/status.go:116`); `Unstage` runs
  `restore --staged --` (`:130`) or `rm --cached --quiet --` (`:134`). `--`
  ends options; it does not turn off globbing or pathspec magic.
- Cost: staging `[id].tsx` also staged `i.tsx` and `d.tsx` beside it, and
  unstaging it unstaged all three. A file named `:(top)README` staged
  `README`. Bracketed file names are ordinary in web frameworks. The status
  parser preserves such names carefully with `-z` and then hands them back as
  patterns.
- Remedy: `git --literal-pathspecs` on the three commands. It fixed every
  case tried.
- Done when: a test stages `[id].tsx` beside `i.tsx` and only one is staged.

### DEBT-27 The lefthook generator converts scripts it does not understand

Severity: medium · Confidence: reproduced

- Evidence: `plainCommand` (`internal/hooks/generate.go:238`) is a line
  heuristic: a marker list (`$(`, a backtick, `<<`, `$1`, `$2`, `$@`, `$*`,
  `$#`, `${`) and a word list. All of these converted to jobs: `$3` to `$9`,
  `$0`, `$?`; a line ending in `&&`, `||` or a pipe; a multi-line
  `( … )`; `pushd`, `umask`, `alias`, `eval`, `wait`; `<(`. `setOption`
  (`:231`) drops `set -o pipefail`, leaving `make lint | tee log` without it.
  A script with no `set -e` still becomes `piped: true` (`:152`). `bash` and
  `zsh` scripts convert (`:200`), and lefthook runs jobs with `sh -c`. The
  table test (`internal/hooks/generate_test.go:195`) covers none of these.
- Cost: lefthook passes hook arguments as `{1}`, so a converted `$3` silently
  becomes an empty string, which is what `prepare-commit-msg` reads. A hook
  that used to continue past a failing command now stops. The offer is
  presented as safe, and what it changes is what gates a commit.
- Remedy: narrow what converts to `sh` scripts that contain `set -e`, no `$`
  at all and no trailing operator; everything else stays a script, which the
  generator already does well. Add each case above to the table first.
- Done when: every case above is in the table and is kept whole.

### DEBT-28 A bare `#!/usr/bin/env` ends the program at start-up

Severity: medium · Confidence: reproduced

- Evidence: `runner` (`internal/hooks/generate.go:186`) takes `args[0]` after
  `env` with no length check: "slice bounds out of range". It is reached from
  `Init` through `findHooks` and `hooks.Structured`. `env -u FOO bash` and
  `env VAR=1 bash` are misread as the interpreters `FOO` and `VAR=1`.
  (`env -S bash -e` is handled, and a test asserts it.)
- Cost: with lefthook installed and no lefthook configuration, an executable
  hook with that first line makes `workflow` exit 1 on every start in that
  repository. Bubble Tea recovers the panic and restores the terminal, so it
  is a stack trace, not a broken terminal.
- Remedy: the failing test, then a guard and skipping `VAR=` words.
- Done when: that shebang yields `sh` and the interface opens.

### DEBT-29 Hook failure locations that name nothing are still offered

Severity: low · Confidence: reproduced

- Evidence: `Failures` keeps the matched file verbatim
  (`internal/hooks/output.go:167`). `editor.Locate` now refuses a place that is
  not a file when it is opened, so no empty buffer opens and no stray file is
  left, but the place is still listed.
- Cost: `go test ./...` prints package-relative places
  (`foo_test.go:4: got 1, want 2`, indented). Each is offered, and `enter` on
  it answers "no such file". A lefthook job with `root:` has the same
  mismatch. No test has a `go test`-shaped case.
- Remedy: resolve each place when the run finishes, through a seam, since the
  interface does not touch the file system itself; drop what does not exist,
  or search for a unique match below the root.
- Done when: a place that does not resolve is not offered.

### DEBT-30 `origin` is spelled out in eight places, and nothing fetches

Severity: medium · Confidence: read

- Evidence: `internal/gitrepo/gitrepo.go:71`; `internal/gitrepo/branch.go:57`,
  `:110`, `:115` and `:214`; `internal/tui/branch.go:66` and `:90`;
  `internal/tui/prcomposer.go:78`. No code path runs `git fetch`. The base
  falls back through `origin/HEAD`, `origin/main`, `origin/master`, local
  `main`, local `master`, then nothing (`base`,
  `internal/gitrepo/branch.go:109`).
- Cost: a remote under another name means "not pushed yet" forever. With a
  fork, the base is the fork's default branch. When no base is found,
  `Commits` stays empty, and both `canPush` and `canOpenPullRequest` are
  false: the loop stops, with no setting to restart it. "Starts from origin's
  default branch" means "as of the last fetch", which the screen does not say.
  See FEAT-12 and FEAT-15.
- Remedy: carry the remote as one value on a gitrepo type; ask git for
  `remote.pushDefault` before assuming; say how old the base is.
- Done when: the word `origin` appears once in production code.

### DEBT-31 `hooks.Write` can fail halfway and then refuse to retry

Severity: low · Confidence: reproduced

- Evidence: `Write` (`internal/hooks/generate.go:322`) creates `lefthook.yml`
  first and each script after it, exclusively.
- Cost: with a leftover `.lefthook/commit-msg/commit-msg`, the first call
  failed after writing the configuration, and the retry failed with "this
  repository already has a lefthook configuration". The result is a
  configuration that names missing scripts, no `lefthook install`, and an
  offer that never appears again because a configuration now exists.
- Remedy: check every target first; write the configuration last.
- Done when: a failed `Write` leaves nothing behind.

### DEBT-32 Windows is a release target the code has not met

Severity: low · Confidence: read

- Evidence: `windows/amd64` is built (`Taskfile.yml:30`); there are no build
  tags and no `runtime.GOOS` checks. `ExistingHooks` requires an executable
  bit (`internal/hooks/generate.go:82`), and Go reports `0666` or `0444` for
  every file on Windows, so no hook is ever found. The editor falls back to
  `vi` (`internal/editor/editor.go:31`). The location pattern
  (`internal/hooks/output.go:131`) cannot match `C:\path\file.go:12`. CI never
  compiles or tests the Windows build (DEBT-40). None of this was run on
  Windows.
- Remedy: a macOS and a Windows leg in CI first, to learn what else is true;
  then small build-tagged helpers.
- Done when: the test suite runs on Windows in CI.

### DEBT-33 Smaller items in the local packages

Severity: low · Confidence: reproduced

- The issue-key pattern (`internal/convention/convention.go:54`) matches
  `UTF-8` in `fix/UTF-8-decoding`, and `SHA-256`, `CVE-2024` and `ISO-8601`,
  although its comment says such names are why it is strict. `Message` and
  `PullRequestBody` look for an existing reference with `strings.Contains`,
  so `PROJ-1` is "found" inside `PROJ-12` (`convention.go:261`,
  `internal/convention/pullrequest.go:32`). The `Refs:` trailer is appended as
  a new paragraph, after which `git interpret-trailers --parse` sees only it
  and loses `Co-authored-by:`.
- `ReadBranch` asks for `--reverse --max-count=200`
  (`internal/gitrepo/branch.go:89`). git limits before it reverses, so past
  200 commits the list holds the newest 200, the count silently caps, and the
  pull request title comes from a commit that is not the branch's first.
- Dead code: `ReadConfig`, `wireHook`, `names` and `Runner`
  (`internal/hooks/config.go`) are called only by tests, and
  `File.Executable` is set and never read. `task deadcode` exists and its
  note says test-only helpers are invisible to it.
- All eight `regexp.MustCompile` calls sit inside functions, one per script
  line (`setOption`) and one per frame (`IssueKey` by way of `spine`). The
  cost is microseconds. The useful fact is that `gochecknoglobals` exempts
  package-level regexps, so nothing forces this.
- `Output.Wait` blocks forever on a second call
  (`internal/proc/start.go:77`). No caller calls it twice.
- The branch name `@` is refused with "it is empty"
  (`internal/convention/convention.go:158`).
- `(ctx, run Runner, dir string)` repeats on eight gitrepo functions, and
  `Status` and `Stage` silently require `dir` to be the root. A `Repository`
  value returned by `Describe` would carry all three.

### DEBT-34 Domain values travel as bare strings

Severity: low · Confidence: read

- Evidence: five `JiraDeps` functions take `issueKey string`
  (`internal/tui/deps.go:44`), and `Comment func(issueKey, text string)`
  compiles with its arguments swapped. `BranchName(issueType, key, summary
  string)` (`internal/convention/convention.go:70`) has the same shape, and
  matches the issue type against the English word "bug", which a localized or
  renamed type defeats. Jira's status categories (`"new"`,
  `"indeterminate"`, `"done"`) exist only as map keys in
  `internal/tui/glyphs.go:56`. `GitHook.Name` is a string joined into a path
  (`internal/hooks/generate.go:130`) on the strength of a `//nolint` comment.
  `config.Forge.Kind` is a string parsed again at each use.
- Cost: CLAUDE.md names the issue key as its own example of primitive
  obsession. The compiler cannot help with any of these today.
- Remedy: `jira.Key`, `jira.StatusCategory`, a `HookName` built only from the
  known list. The interface inherits the types.
- Done when: `Comment(text, key)` does not compile.

## The test suite

Snapshot at this commit: statement coverage 98.1% against a floor of 95, and
condition coverage 96.4% (1642 of 1704 arms) against a floor of 93. Both
gates print an available ratchet (to 96 and to 94). The weakest packages by
condition coverage are `internal/cli` at 75.7% and `internal/wiring` at 81.6%.
These numbers are a dated reading, not a floor; the floors live in
`Taskfile.yml`.

### DEBT-35 No test runs the root command

Severity: medium · Confidence: measured

- Evidence: gobco reports `internal/cli/cli.go:113` and `:118` as "never
  evaluated". That is the `RunE` which loads the configuration, builds the
  model, and applies `if dryRun { model = model.WithDryRun() }`.
- Cost: the flag that promises "every write held back" has no test connecting
  it to the model. Every dry-run test in `internal/tui` sets the model up
  directly. Swapping the `if` for nothing would pass the suite.
- Remedy: make `tui.Run` a seam the root command is given, and assert the
  model it receives is in dry run.
- Done when: gobco sees both arms of `dryRun`.

### DEBT-36 The forge's success path is never exercised outside its package

Severity: medium · Confidence: measured

- Evidence: `err != nil` after `connect()` is "true but never false" at
  `internal/wiring/wiring.go:161`, `:169`, `:177` and `:186`, and once true
  and never false at `:220`. In `doctor`, the forge and Slack checks
  (`internal/cli/doctor.go:206` and `:222`) are never seen to succeed, because
  `askForge` and `checkSlack` build real clients against real addresses, with
  no seam for a test server. `config init --global` is never run
  (`internal/cli/config_cmd.go:83`).
- Cost: everything between a resolved token and a working `tui.ForgeDeps` is
  untested, which is where DEBT-21's two copies live.
- Remedy: let the API base and the `Doer` be injected where `doctor` and
  wiring build clients.
- Done when: gobco sees the false arm at all five sites.

### DEBT-37 The shared fake world is 27 lines from the file-length gate

Severity: low · Confidence: measured

- Evidence: `internal/tui/world_test.go` is 473 lines of a 500-line limit.
  Of 187 tests in `internal/tui`, 135 build a world and 173 use a helper
  defined in that file. `internal/tui/edges_test.go` (447 lines) holds 21
  tests across composer, review, Slack, commits, mouse, picker and hook
  generation, each of which has a test file of its own.
- Cost: the next `tui.Deps` seam pushes the largest file in the repository
  past the gate in the middle of a feature. `edges_test.go` is the "divergent
  change" smell CLAUDE.md names.
- Remedy: split the world by seam; move each edge test to its concern's file.
- Done when: no test file is within 50 lines of the limit.

### DEBT-38 Smaller items in the tests

Severity: low · Confidence: read

- `patience = 400 * time.Millisecond` (`internal/tui/world_test.go:27`):
  `within` gives up on wall-clock time, and `drain` silently drops a command
  that takes longer. A slow runner under `-race` turns a late fake into a
  confusing failure.
- Four fuzz targets exist and nothing passes `-fuzz`, so only their seeds
  ever run. An advisory `task fuzz` with a short `-fuzztime` would cost
  little.
- Tests pin defects as contracts: "1 files staged"
  (`internal/tui/composer_test.go:50`) and the field form's "esc close" for a
  key that goes back (`internal/tui/fields_test.go:158`).
- Three wiring tests skip when lefthook is absent
  (`internal/wiring/hooks_test.go:29`), which it is in the build container
  (DEBT-41).

## Build, CI, scripts and release

### DEBT-40 Four of five release platforms are first compiled at release

Severity: medium · Confidence: read

- Evidence: every job in `.github/workflows/ci.yml` runs on `ubuntu-latest`,
  and its build step runs `task build` only. `task release:binaries`, the five
  platforms (`Taskfile.yml:29`), is called from `release.yml` alone.
- Cost: a break that only macOS or Windows can see surfaces after the tag
  exists. A Windows binary ships that CI has never compiled, let alone tested
  (DEBT-32).
- Remedy: run `task release:binaries` in the CI build job; add a macOS leg to
  the test job.
- Done when: CI compiles all five targets on every pull request.

### DEBT-41 The build container is never built, and is not the same gate

Severity: medium · Confidence: read

- Evidence: no workflow builds `build/Dockerfile`. `scripts/tool-versions.sh`
  passes 14 of `mise.toml`'s 19 pins; lefthook, shellcheck, hugo-extended,
  cloc and deadcode are left out. shellcheck, jq, nodejs and npm come from apt
  with no version (`build/Dockerfile:45`). typos, hadolint, taplo and zizmor
  are downloaded with `curl` and no checksum, while
  `.devcontainer/postCreate.sh:17` argues for verifying one.
- Cost: `task container:check` is documented as "the same gate", and it skips
  three tests, lints with whichever shellcheck Debian ships, and would not
  notice a dead download URL until someone built it by hand.
- Remedy: build the image and run `task container:check` weekly in CI; pass
  the missing pins; have `tool-versions.sh` fail when a tool is neither passed
  nor skipped on purpose; verify the downloads.
- Done when: a workflow builds the image.

### DEBT-42 mise itself floats in CI

Severity: medium · Confidence: reproduced

- Evidence: none of the eight `jdx/mise-action` uses sets `version`, and the
  action's own description says that means "the latest release". `ci.yml`
  caches the binary, so there it is the latest as of the last cache miss;
  `release.yml` and `pages.yml` set `cache: false`, so there it is the latest
  on every run. A CI log showed `mise 2026.9.10`, one day old. The devcontainer
  pins `2026.9.3`; `mise.toml` asks for at least `2024.1.0`.
- Cost: "A floating `latest` changes what the gate accepts without anyone
  deciding to" is this repository's rule, and the mise binary carries the
  registry that maps a tool's name to where it is downloaded from. The release
  job is provisioned by a mise younger than the seven-day gate allows anything
  else to be.
- Remedy: set `version`, or the action's `minimum_release_age`, in one place,
  and raise `min_version` to match.
- Done when: every workflow names the mise it runs.

### DEBT-43 The gate needs jq and node, and `mise.toml` declares neither

Severity: medium · Confidence: read

- Evidence: `scripts/gobco-report.sh:171` and
  `scripts/coverage-summary.sh:36` call `jq`. The `npm:` tools install without
  node, and mise's documentation says the installed program may still need it
  and that mise will not add it. On the maintainer's machine both come from a
  personal global mise configuration; in CI, from the runner image.
  `CONTRIBUTING.md:32` says `mise install` is the only step that installs
  anything.
- Cost: `task check` on a fresh machine depends on what else is installed.
- Remedy: pin `node` and `jq` in `mise.toml`, or replace the jq arithmetic
  with a few lines of Go.
- Done when: `mise install && task check` passes in a clean container.

### DEBT-44 Two gates can pass having measured nothing

Severity: medium · Confidence: reproduced

- Evidence: `check-file-length.sh` reads its list from
  `done < <(git ls-files '*.go' '*.sh')` (`scripts/check-file-length.sh:65`).
  A failure inside process substitution escapes `set -e`: in a directory that
  is not a repository, beside a 901-line `big.go`, the script printed "every
  tracked Go and shell file is within 500 lines" and exited 0. It also sees
  tracked files only, where the license check and testshape see untracked ones
  too (`scripts/check-license-headers.sh:26`, `Taskfile.yml:117`).
  `gobco-report.sh` selects packages that have tests
  (`scripts/gobco-report.sh:121`), so a new package without tests adds nothing
  to the total that `Taskfile.yml:12` says covers "EVERY package"; the three
  `cmd/` packages are absent today. Its floor defaults to 0
  (`scripts/gobco-report.sh:101`), and the statement gate's to 70
  (`scripts/coverage-gate.sh:13`), if a caller ever drops the argument.
- Cost: a gate that reports success on no input is the quiet shrinkage
  `gobco-report.sh` says it exists to prevent.
- Remedy: read the file list first and fail when git fails or the list is
  empty; include untracked files; list every package and make one without
  tests an error unless it is named with a reason; make both floors required
  arguments.
- Done when: each script exits non-zero when git is unavailable.

### DEBT-45 CI does not scan history for secrets, and two comments say it does

Severity: medium · Confidence: reproduced

- Evidence: `task secrets` runs `gitleaks dir` (`Taskfile.yml:229`), which
  scans files. The scan job's comment says "gitleaks scans history as well as
  the working tree" (`.github/workflows/ci.yml:69`) and pays for
  `fetch-depth: 0`; `lefthook.yml:102` says "CI still sweeps history". In a
  scratch repository, a key committed and then removed gave "no leaks found"
  from `gitleaks dir` and two findings from `gitleaks git`.
- Cost: `main` merges by rebase only, so every commit lands as written. A
  token added in one commit of a pull request and removed in the next is
  invisible to CI; the only guard is the local staged-diff hook. GitHub's own
  secret scanning and push protection are enabled, which narrows this to
  tokens GitHub has no pattern for, and a Jira Data Center token is one.
- Remedy: a `secrets:history` task running `gitleaks git` over the pull
  request's range, called from the scan job; correct both comments.
- Done when: the experiment above fails CI.

### DEBT-46 Smaller items in the tooling

Severity: low · Confidence: reproduced

- `_typos.toml` sets no locale, so `typos` accepts British spellings: the
  British forms of "color" and "organized" pass it without a word. Its header
  and CLAUDE.md both say it enforces American English. With
  `locale = "en-us"` the whole repository still passes today, and the wire
  value `cancelled` is not flagged, so the fix is free.
- `check-commit-message.sh` checks neither the 72-character limit nor the
  trailing period that CLAUDE.md states: a 127-character subject ending in a
  period passed. It reads the raw message file, so under `git commit -v` its
  unanchored `BREAKING-CHANGE:` pattern matched a line of the diff and the
  commit was refused. CI is unaffected.
- `.golangci.yml` has no `exhaustive` settings, so only switches are checked,
  and the interface deliberately uses map literals to avoid a switch arm that
  gobco cannot cover. A new `forge.CIState` or `hooks.JobState` therefore
  draws an empty glyph in silence (`internal/tui/review.go:128` and `:147`,
  `internal/tui/run.go:180`). `check: [switch, map]` exists and would flag
  them.
- Checks differ by where they run. Pre-push runs tests, golangci-lint and the
  file-length check, under a comment that says a green push is very likely a
  green CI; it runs no coverage floor, docs check, script test or whole-tree
  linter. The `shfmt` flags are written out three times. "`task check` is
  what CI runs" is said in four documents and is literally true of
  `release.yml` only; `ci.yml` restates the steps, and today restates them
  all.
- Dependabot covers the root module, Actions and `build/`. It does not cover
  `docs/go.mod` or the devcontainer, whose `base:trixie` and
  `docker-in-docker:2` float. `build/Dockerfile:14` gives `GO_VERSION` a
  default, a version written outside `mise.toml`.
- The coverage comment job asks for `pull-requests: write` with no guard for
  forks (`.github/workflows/ci.yml:103`). GitHub gives fork pull requests a
  read-only token, so it should fail on every outside contribution, outside
  the required checks. No fork pull request exists yet to show it.
- `push-release-tag.sh` pushes the tag before `release.yml` runs `task check`
  (`.github/workflows/release.yml:42`), so a gate that is red at release time
  leaves a tag with no release behind it.
- Two scripts have a test, `scripts/check-commit-message.sh` and
  `scripts/release/push-release-tag.sh`. The coverage gates, the file-length and
  license checks, `tool-versions.sh` and the docs drift check have none, and
  it is their failure paths that never run.
- A scratch `.go` file under the gitignored `tmp/` joins `go vet ./...`,
  `go test ./...` and golangci-lint, as an experiment confirmed, and CLAUDE.md
  sends scratch files there without saying so.

## Docs and configuration drift

### DEBT-47 The docs promise `.workflow.json` is gitignored; nothing makes it so

Severity: medium · Confidence: read

- Evidence: the help text (`internal/cli/cli.go:76`), `README.md:151` and
  `docs/content/docs/configuration.md:167` say the file "is listed in
  `.gitignore`". That is true of this repository's `.gitignore`.
  `runConfigInit` writes the file and checks nothing.
- Cost: in a user's repository the default target puts live tokens one
  `git add -A` from a commit, under a sentence that says otherwise.
- Remedy: inside a work tree, ask `git check-ignore` and warn; reword the
  three texts. See FEAT-45.
- Done when: `config init` in a repository that does not ignore the file says
  so.

### DEBT-48 The documents name a version and a flag that do not exist

Severity: medium · Confidence: reproduced

- Evidence: `README.md:54`, `docs/content/docs/install.md:34` and the bug
  report form use `v0.1.0`. There are no tags; the manifest says `0.0.1`; the
  release pull request proposes `0.0.2`; under the deliberate pre-1.0 bump
  rules `0.1.0` appears only after a breaking change. The bug report form's
  required Version field asks for the output of `workflow --version`, or a
  commit (`.github/ISSUE_TEMPLATE/bug_report.yml:20`), and the binary answers
  "unknown flag". The build stamps no version (`Taskfile.yml:305`).
- Cost: the documented `go install …@v0.1.0` fails, and the first thing the
  bug report form suggests cannot be run.
- Remedy: a placeholder until a tag exists; FEAT-40 for the flag. The bump
  rules are deliberate and stay.
- Done when: every version in the docs is one that can be installed.

### DEBT-49 Smaller drift

Severity: low · Confidence: read

- The usage guide's promise about the bottom row, its "`?` lists every key",
  and its "anywhere" keys do not match the bindings. The detail is in UX.md
  (UX-32, UX-33).
- Four places say gobco cannot read every package (`CONTRIBUTING.md:76`,
  `CLAUDE.md:330`, `.github/workflows/ci.yml:39` and `:164`, the last of which
  is posted on every pull request) while the skip list is empty
  (`scripts/gobco-report.sh:65`). The comment above `BRANCH_COVERAGE_MIN`
  quotes "84 arms" and "83.3%" where the gate measures 1704 and 96.4%.
- "macOS, Linux, and Windows on amd64 and arm64" (`README.md:59` and `:203`,
  `CONTRIBUTING.md:178`, `docs/content/docs/install.md:44`) promises a
  `windows/arm64` build that `Taskfile.yml:30` does not make.
- CLAUDE.md's layout block omits `cmd/docsgen/`, `cmd/testshape/`, `docs/`
  and `.devcontainer/`, and its `task lint` row omits `docs:check`. The tool
  lists in `CONTRIBUTING.md` and `docs/content/docs/contributing.md` disagree
  with each other and with `mise.toml`.
- The usage guide shows the scissors line as `# ---- >8 ----`; the real one
  (`internal/editor/editor.go:29`) is much longer.

## Deliberate trade-offs that carry a cost

These were chosen, and the reason is written down in the code or the docs.
They are listed so the cost is visible, not so they get "fixed".

- **The issue list is one query, capped at 50, with no paging.** The pane
  says "showing N of M", so the cut is not silent. FEAT-02, FEAT-03.
- **Every redirect is refused**, in all three clients, to keep credentials
  from following one. The cost is DEBT-17's unhelpful message.
- **`gh` failures are swallowed** when resolving a token, so `doctor` cannot
  say "`gh` is installed but not signed in to this host".
- **There is no `glab` step**, because it reports its token as prose.
- **Configuration is looked for in the current directory, then home.** From
  a subdirectory of a repository, the repository's own file is skipped for
  the one at home, and `config init` writes to wherever it was run. FEAT-44.
- **Convention rules are constants**: the eleven commit types, the 72 and 48
  character limits, `fix/` and `feat/`, the `Refs:` trailer, the title from
  the oldest commit. A team with other conventions has no setting. FEAT-14.
- **`$EDITOR` is split on spaces and never given to a shell**, so an editor
  whose path contains a space fails, and `GIT_EDITOR` and `core.editor` are
  ignored.
- **Failure locations need a file extension**, which misses `Dockerfile:3`,
  though hadolint is in this repository's own gate. FEAT-25.
- **lefthook's decorative output is scraped**, with no check of the installed
  version. A change degrades to "no job rows", not to a wrong answer.
- **A line over 1 MiB ends output capture** for that run; the exit status is
  still reported.
- **Interface seams may be nil**, which costs 29 nil checks in production
  code for the benefit of partial test fakes. One path has no check
  (`startPush` calls `m.deps.Git.Push`), which today's wiring always sets.
- **Map literals stand in for switches** so that gobco has no uncoverable
  arm. The cost is in DEBT-46.
- **Timing and layout are named constants, not settings**: the ten-second
  request timeout, the twenty-second CI interval, the 90- and 60-column
  breakpoints, five comments shown. FEAT-47, UX-02.
- **Test files are exempt from the complexity linters**, and
  `internal/cli` and `internal/wiring` run their tests serially.
- **gobco runs without `-race`, one package at a time**, so `task check` runs
  the suite twice.
- **The license header's year is a fixed string.** From 2027 a new file still
  says 2026, and a contributor's own copyright line is refused. A fixed
  first-publication year is common practice; it should be written down as the
  policy.
- **Dependabot's cooldown exempts `actions/*` and `github/*`.** CLAUDE.md's
  age-gate rule names only the CVE exception.

## Checked and found clean

Recorded so the next audit can spend its time elsewhere.

- `os/exec` is imported only in `internal/proc`.
- Every response body is closed and read through a limit, and every client is
  built with a timeout.
- Server text is sanitized before it is decoded or shown, at every site in
  the three clients.
- No request or parse error echoes a URL that could hold a credential; the
  webhook URL never enters an error; Jira refuses userinfo in its base URL
  before sending anything.
- Slack announcement text is escaped; issue keys, repository paths and commit
  hashes are path-escaped.
- Configuration: unknown keys are rejected, defaults survive a partial file,
  and "local replaces home, never merged" is implemented as documented.
- Sentinel errors are static and wrapped with `%w` throughout; nothing builds
  an error at the point of failure.
- No file is over 500 lines, there is no package-level mutable state, and
  there are no goroutines outside `tea.Cmd` and `sync.OnceValues`.
- Every `//nolint` directive was read; each one's stated reason holds.
- The porcelain `-z` status parser handles spaces, quotes, newlines and
  renames. Detached heads, unborn branches and worktrees are handled. No
  parsing depends on the locale.
- `ValidateBranchName` agrees with `git check-ref-format` except for the name
  `HEAD`.
- Stale-response guards are correct for issue detail, transitions, pull
  request lookup, CI checks and the three run messages.
- All 40 `uses:` lines in the workflows are pinned to a full commit hash, and
  workflow permissions are least-privilege.
- The Go version agrees across `go.mod`, `mise.toml`, the Dockerfile, the docs
  module and the README badge. `go.mod` has no `replace`, and every direct
  dependency is used.
- The generated command reference is checked for drift in both directions.
- Every shell script sets `set -euo pipefail` and quotes its expansions; none
  disables a shellcheck rule inline.
- Across 48 commits, no subject is over 72 characters and none ends with a
  period.
