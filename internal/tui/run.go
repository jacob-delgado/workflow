// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// starter starts a program whose output a run streams.
type starter func() (proc.Output, error)

// commandRun is a program running with its output streamed: a commit, a push,
// a hook. When it fails, every place a tool pointed at can be opened in the
// editor at its line.
type commandRun struct {
	marks  glyphs
	styles styles
	title  string
	id     int
	start  starter
	// succeeded is what happens once the program exits cleanly; nil keeps the
	// output on screen until it is closed.
	succeeded func(m Model) (Model, tea.Cmd)
	// stop kills this run's own process group, set once its program has started.
	// Nil before then, and for a run started by a fake that supplies none.
	stop func()

	lines []string
	// jobList is the parsed run state, folded in as each line arrives rather than
	// re-read from every line on every frame; inSummary carries the scan across
	// lines. See hooks.NextJob.
	jobList   []hooks.Job
	inSummary bool
	done      bool
	// stopped records that the user ended the run, so its killed exit is shown as
	// stopped rather than as a failure, and what would come next is not run.
	stopped    bool
	err        error
	failures   pickList[hooks.Location]
	showOutput bool
}

var (
	_ overlay   = commandRun{}
	_ clickable = commandRun{}
)

// startRun opens a run and starts its program.
func (m Model) startRun(title string, start starter, succeeded func(Model) (Model, tea.Cmd)) (Model, tea.Cmd) {
	m.runs++
	m.overlay = commandRun{
		marks: m.marks, styles: m.styles, title: title, id: m.runs, start: start, succeeded: succeeded,
	}

	return m, launch(m.runs, start)
}

// launch is the command that starts a run's program.
func launch(runID int, start starter) tea.Cmd {
	return func() tea.Msg {
		output, err := start()

		return runStarted{id: runID, output: output, err: err}
	}
}

// runStarted is a run's program started, or not.
type runStarted struct {
	id     int
	output proc.Output
	err    error
}

// apply begins reading the output, or reports the program not starting. It
// keeps the run's Stop handle, so a stop key can end this run alone.
func (msg runStarted) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return runFinished{id: msg.id, err: msg.err}.apply(m)
	}

	if run, open := m.overlay.(commandRun); open && run.id == msg.id {
		run.stop = msg.output.Stop
		m.overlay = run
	}

	return m, nextLine(msg.id, msg.output)
}

// nextLine is the command that waits for a run's next line, or its end.
func nextLine(runID int, output proc.Output) tea.Cmd {
	return func() tea.Msg {
		line, open := <-output.Lines
		if !open {
			return runFinished{id: runID, err: output.Wait()}
		}

		return runLine{id: runID, line: line, output: output}
	}
}

// runLine is one line of a run's output.
type runLine struct {
	id     int
	line   string
	output proc.Output
}

// apply shows the line and waits for the next. The line is neutralized: tools
// color their output, and a hook runs whatever a repository says to.
func (msg runLine) apply(m Model) (Model, tea.Cmd) {
	run, open := m.overlay.(commandRun)
	if open && run.id == msg.id {
		line := sanitize.Text(msg.line)
		run.jobList, run.inSummary = hooks.NextJob(run.jobList, run.inSummary, line)
		run.lines = capLines(append(run.lines, line))
		m.overlay = run
	}

	// Reading continues whatever is open, so the program is never left writing
	// to a pipe nobody empties.
	return m, nextLine(msg.id, msg.output)
}

// runFinished is a run's program exited.
type runFinished struct {
	id  int
	err error
}

// apply records how the run ended, finds where the tools pointed if it failed,
// and hands a clean exit to whatever comes next.
func (msg runFinished) apply(m Model) (Model, tea.Cmd) {
	run, open := m.overlay.(commandRun)
	if !open || run.id != msg.id {
		return m, nil
	}

	// A run the user stopped is already shown as stopped; its killed exit
	// arriving afterward is that stop taking effect, not a failure to report.
	if run.stopped {
		return m, nil
	}

	run.done, run.err = true, msg.err
	if msg.err != nil {
		run.failures = pickList[hooks.Location]{items: m.deps.resolvedFailures(hooks.Failures(run.lines, runtime.GOOS))}
	}

	m.overlay = run

	if msg.err == nil && run.succeeded != nil {
		return run.succeeded(m)
	}

	return m, nil
}

// rowsIn is how many display rows a wrapped string takes.
func rowsIn(wrapped string) int {
	return strings.Count(wrapped, "\n") + 1
}

// view shows the jobs as lefthook reports them, then either the places to jump
// to or the tail of the output, and how the run stands.
func (r commandRun) view(width, rows int) (string, string) {
	lines := r.header(width)

	if len(r.failures.items) > 0 && !r.showOutput {
		lines = append(lines, r.failures.rows(r.marks, rows-len(lines), placeRow)...)
	} else {
		lines = append(lines, tail(r.lines, rows-len(lines))...)
	}

	return r.title, strings.Join(lines, "\n")
}

// header is the rows above the list: the state, a wrapped jobs line when there
// is one, and a blank. The view draws it and the click measures it, so the two
// cannot drift apart the way a hardcoded row count would.
func (r commandRun) header(width int) []string {
	lines := []string{r.state()}

	if jobs := r.jobs(); jobs != "" {
		lines = append(lines, wrap(jobs, width))
	}

	return append(lines, "")
}

// state says whether the run is going, passed, or failed — and, when it failed,
// what step failed rather than the exit code the step happened to end with.
func (r commandRun) state() string {
	switch {
	case !r.done:
		return r.marks.inFlight + " running" + r.marks.ellipsis
	case r.stopped:
		return r.marks.notStarted + " stopped"
	case r.err != nil:
		return r.failureHeadline()
	default:
		return r.marks.done + " done"
	}
}

// failureHeadline names the step that failed in words. It stands in only for a
// failure status — git's "exit status 1", which says nothing on its own; an
// error that already carries its cause — a message that could not be written,
// say — is told the way every failure is.
func (r commandRun) failureHeadline() string {
	sentences := map[string]string{
		"git commit":         "the commit was refused",
		"git commit --amend": "the amend was refused",
		"git commit --fixup": "the fixup was refused",
		"pre-commit":         "the pre-commit hook failed",
		"git push":           "the push was refused",
		"git rebase":         "the rebase stopped — resolve the conflict in your shell, then continue",
	}

	sentence, known := sentences[r.title]
	if known && errors.Is(r.err, proc.ErrExitStatus) {
		return failedGlyph(r.styles, r.marks) + " " + sentence
	}

	return failureLine(r.styles, r.marks, r.err)
}

// jobs is lefthook's jobs, each with its glyph, when the output is lefthook's.
func (r commandRun) jobs() string {
	parsed := r.jobList
	parts := make([]string, 0, len(parsed))

	for _, job := range parsed {
		glyph := map[hooks.JobState]string{
			hooks.JobRunning: r.marks.inFlight, hooks.JobPassed: r.marks.done,
			hooks.JobFailed: failedGlyph(r.styles, r.marks), hooks.JobSkipped: r.marks.notStarted,
		}[job.State]
		parts = append(parts, glyph+" "+job.Name)
	}

	return strings.Join(parts, r.marks.separator)
}

// placeRow names a place to jump to by its file, line and what the tool said.
func placeRow(place hooks.Location) string {
	return place.File + ":" + strconv.Itoa(place.Line) + " " + place.Message
}

// tail is the last rows lines of output.
func tail(lines []string, rows int) []string {
	return lines[max(0, len(lines)-max(0, rows)):]
}

// maxRunLines bounds the output kept for the tail. A hook that prints tens of
// thousands of lines would otherwise grow the model without limit; the jobs and
// the places to jump to are folded in as the lines arrive, so what the kept
// lines are still for is the tail on screen.
const maxRunLines = 5000

// capLines keeps at most the last maxRunLines, cloning when it trims so the
// dropped head's backing array is not held alive.
func capLines(lines []string) []string {
	if len(lines) <= maxRunLines {
		return lines
	}

	return slices.Clone(lines[len(lines)-maxRunLines:])
}

// footer offers what works on the run as it stands.
func (r commandRun) footer(keys keyMap) []key.Binding {
	switch {
	case !r.done:
		return []key.Binding{keys.stopRun, keys.interrupt}
	case len(r.failures.items) > 0 && !r.showOutput:
		return []key.Binding{
			keys.up, keys.down, relabel(keys.confirm, "open in editor"),
			relabel(keys.fullOutput, "full output"), keys.retry, keys.closeOverlay,
		}
	case len(r.failures.items) > 0:
		return []key.Binding{relabel(keys.fullOutput, "places"), keys.retry, keys.closeOverlay}
	default:
		return []key.Binding{keys.retry, keys.closeOverlay}
	}
}

// handleKey answers a key while a run is shown. While it runs, the only key is
// stop, which kills it; once it is done, its outcome decides what happens next.
func (r commandRun) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case !r.done:
		return r.stopRun(m, msg)
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), tea.Batch(m.loadChanges(), m.loadBranch())
	case key.Matches(msg, m.keys.retry):
		return m.startRun(r.title, r.start, r.succeeded)
	case key.Matches(msg, m.keys.confirm):
		return r.openFailure(m)
	case key.Matches(msg, m.keys.fullOutput):
		r.showOutput = !r.showOutput
	case key.Matches(msg, m.keys.down):
		r.failures = r.failures.moved(1)
	case key.Matches(msg, m.keys.up):
		r.failures = r.failures.moved(-1)
	}

	m.overlay = r

	return m, nil
}

// stopRun kills a running program when the stop key is pressed, and ends the
// run at once so the screen says so rather than waiting on the killed exit to
// arrive. Any other key while it runs is ignored — the run is not abandoned.
func (r commandRun) stopRun(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !key.Matches(msg, m.keys.stopRun) || r.stop == nil {
		return m, nil
	}

	r.stop()
	r.stopped, r.done = true, true
	m.overlay = r

	return m, nil
}

// click selects the place on a clicked line.
func (r commandRun) click(m Model, line int) (Model, tea.Cmd) {
	if !r.done {
		return m, nil
	}

	// The offset is the header the view drew, in display rows — the jobs line
	// wraps to as many rows as it took, so a click below it skips every one.
	offset := rowsIn(strings.Join(r.header(m.detailWidth()), "\n"))
	r.failures = r.failures.clicked(line-offset, m.detailRows()-offset)
	m.overlay = r

	return m, nil
}

// openFailure opens the selected place in the editor, at its line.
func (r commandRun) openFailure(m Model) (Model, tea.Cmd) {
	failure, found := r.failures.chosen()
	if !found || m.deps.Editor.Open == nil {
		return m, nil
	}

	return m, m.deps.Editor.Open(failure.File, failure.Line, func(err error) tea.Msg {
		return editorClosed{err: err}
	})
}

// editorClosed is the editor handing the terminal back.
type editorClosed struct {
	err error
}

// apply reports an editor that could not be opened.
func (msg editorClosed) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return m.noticedFailure(msg.err), nil
	}

	return m, nil
}

// startPush pushes the branch, then does what comes next once it is pushed —
// by default, says so and reads the branch again.
func (m Model) startPush(succeeded func(Model) (Model, tea.Cmd)) (Model, tea.Cmd) {
	name := m.branch.branch.Name

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would push " + name), nil
	}

	if succeeded == nil {
		succeeded = func(done Model) (Model, tea.Cmd) {
			done = done.closeOverlay().noticed(done.marks.done + " pushed " + name)

			return done, done.loadBranch()
		}
	}

	push := m.deps.Git.Push

	return m.startRun("git push", func() (proc.Output, error) { return push(name) }, succeeded)
}

// startRebase replays the branch onto its base and streams the result. A clean
// rebase says so and reads the branch again; a conflict leaves git's output on
// screen and the repository mid-rebase, which is the shell's to finish.
func (m Model) startRebase() (Model, tea.Cmd) {
	base := m.branch.branch.Base

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would rebase onto " + base), nil
	}

	rebase := m.deps.Git.Rebase
	succeeded := func(done Model) (Model, tea.Cmd) {
		done = done.closeOverlay().noticed(done.marks.done + " rebased onto " + base)

		return done, done.loadBranch()
	}

	return m.startRun("git rebase", func() (proc.Output, error) { return rebase(base) }, succeeded)
}
