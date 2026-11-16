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

// runKind is a run's title, and what it says failed on a bare failure status.
type runKind struct{ title, refusal string }

// runStage is where a run's program stands: asked to start, running, asked
// to stop, or ended one of three ways.
type runStage int

const (
	// runStarting has its program asked to start, with no way to stop it yet.
	runStarting runStage = iota
	// runRunning streams the program's output, and stop can end it.
	runRunning
	// runStopping was asked to stop, and waits for the exit that says how.
	runStopping
	// runPassed exited cleanly, a run stopped too late among them: it
	// finished before the stop reached it.
	runPassed
	// runFailed exited with a failure, or its program did not start.
	runFailed
	// runStopped ended with the failure its stop caused, which is not
	// reported as one, and what would come next is not run.
	runStopped
)

// runNext is what follows a run's end.
type runNext int

const (
	// runRests keeps the run on screen as it ended.
	runRests runNext = iota
	// runFindsPlaces looks for the places a failed run's tools pointed at.
	runFindsPlaces
	// runSucceeds hands a clean exit to what the run was started for.
	runSucceeds
)

// commandRun is a program running with its output streamed: a commit, a push,
// a hook. When it fails, every place a tool pointed at can be opened in the
// editor at its line.
type commandRun struct {
	kind runKind
	// id is the round the run drew as it started, which its start, each line,
	// its exit and its places carry back.
	id    int
	start starter
	// succeeded is what happens once the program exits cleanly; nil keeps the
	// output on screen until it is closed.
	succeeded func(m Model) (Model, tea.Cmd)
	stage     runStage
	// stop kills this run's own process group, kept once its program has
	// started.
	stop func()

	lines []string
	// jobList is the parsed run state, folded in as each line arrives rather than
	// re-read from every line on every frame; inSummary carries the scan across
	// lines. See hooks.NextJob.
	jobList   []hooks.Job
	inSummary bool
	// placeScan is where the tools pointed, folded in the same way, so a place
	// printed before the tail the run keeps is still offered.
	placeScan  hooks.FailureScan
	err        error
	failures   pickList[hooks.Location]
	showOutput bool
}

var (
	_ opened    = commandRun{}
	_ clickable = commandRun{}
	_ steppable = commandRun{}
)

// startRun opens a run and starts its program.
func (m Model) startRun(kind runKind, start starter, succeeded func(Model) (Model, tea.Cmd)) (Model, tea.Cmd) {
	m, round := m.nextRound()
	m.overlay = commandRun{
		kind: kind, id: round, start: start, succeeded: succeeded,
		placeScan: hooks.NewFailureScan(runtime.GOOS),
	}

	return m, launch(round, start)
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

var _ applier = runStarted{}

// apply begins reading the output, or reports the program not starting. It
// keeps the run's Stop handle, so a stop key can end this run alone.
func (msg runStarted) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return runFinished{id: msg.id, err: msg.err}.apply(m)
	}

	if run, open := answerFor[commandRun](m, msg.id); open {
		m = m.withAnswered(run.started(msg.output.Stop))
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

var _ applier = runLine{}

// apply shows the line and waits for the next. The line is neutralized: tools
// color their output, and a hook runs whatever a repository says to.
func (msg runLine) apply(m Model) (Model, tea.Cmd) {
	if run, open := answerFor[commandRun](m, msg.id); open {
		m = m.withAnswered(run.read(sanitize.Text(msg.line)))
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

var _ applier = runFinished{}

// apply records how the run ended, and hands a clean exit to whatever comes
// next. A failed run's places are resolved by a command rather than here: one
// a tool printed relative to its package can take a walk of the whole
// checkout, which would freeze the interface on the update loop.
func (msg runFinished) apply(m Model) (Model, tea.Cmd) {
	run, open := answerFor[commandRun](m, msg.id)
	if !open {
		return m, nil
	}

	run, next := run.ended(msg.err)
	m = m.withAnswered(run)

	return map[runNext]func() (Model, tea.Cmd){
		runRests: func() (Model, tea.Cmd) { return m, nil },
		runFindsPlaces: func() (Model, tea.Cmd) {
			deps, found := m.deps, run.placeScan.Places()

			return m, func() tea.Msg { return placesFound{id: msg.id, places: deps.resolvedFailures(found)} }
		},
		runSucceeds: func() (Model, tea.Cmd) { return run.succeeded(m) },
	}[next]()
}

// placesFound is where a failed run's tools pointed, each place resolved to the
// file that opens it.
type placesFound struct {
	id     int
	places []hooks.Location
}

var _ applier = placesFound{}

// apply offers the places to jump to, while their run is still the one shown.
func (msg placesFound) apply(m Model) (Model, tea.Cmd) {
	if run, open := answerFor[commandRun](m, msg.id); open {
		m = m.withAnswered(run.found(msg.places))
	}

	return m, nil
}

// started is the run with its program started, which stop now ends through
// the run's own Stop.
func (r commandRun) started(stop func()) commandRun {
	r.stage, r.stop = runRunning, stop

	return r
}

// stopping is the run asked to stop, waiting for the exit that says how.
func (r commandRun) stopping() commandRun {
	r.stage = runStopping

	return r
}

// found is the failed run offering the places its tools pointed at.
func (r commandRun) found(places []hooks.Location) commandRun {
	r.failures = pickList[hooks.Location]{items: places}

	return r
}

// read is the run with one more line of output, its jobs and places folded
// in as it arrives.
func (r commandRun) read(line string) commandRun {
	r.jobList, r.inSummary = hooks.NextJob(r.jobList, r.inSummary, line)
	r.placeScan = r.placeScan.Next(line)
	r.lines = capLines(append(r.lines, line))

	return r
}

// ended is the run as its exit leaves it, and what follows. A failure
// after a stop is the stop taking effect, not a failure to report; a clean
// exit after one means the program finished before the stop reached it.
func (r commandRun) ended(err error) (commandRun, runNext) {
	r.err = err

	switch {
	case err == nil && r.succeeded != nil:
		r.stage = runPassed

		return r, runSucceeds
	case err == nil:
		r.stage = runPassed

		return r, runRests
	case r.stage == runStopping:
		r.stage = runStopped

		return r, runRests
	default:
		r.stage = runFailed

		return r, runFindsPlaces
	}
}

// going reports the run's program not yet ended.
func (r commandRun) going() bool {
	return map[runStage]bool{
		runStarting: true, runRunning: true, runStopping: true,
		runPassed: false, runFailed: false, runStopped: false,
	}[r.stage]
}

// rowsIn is how many display rows a wrapped string takes.
func rowsIn(wrapped string) int {
	return strings.Count(wrapped, "\n") + 1
}

// view shows the jobs as lefthook reports them, then either the places to jump
// to or the tail of the output, and how the run stands.
func (r commandRun) view(kit renderKit, width, rows int) (string, string) {
	lines := r.header(kit, width)

	if len(r.failures.items) > 0 && !r.showOutput {
		lines = append(lines, r.failures.rows(kit.marks, rows-len(lines), placeRow)...)
	} else {
		lines = append(lines, tail(r.lines, rows-len(lines))...)
	}

	return r.kind.title, strings.Join(lines, "\n")
}

// header is the rows above the list: the state, a wrapped jobs line when there
// is one, and a blank. The view draws it and the click measures it, so the two
// cannot drift apart the way a hardcoded row count would.
func (r commandRun) header(kit renderKit, width int) []string {
	lines := []string{r.state(kit)}

	if jobs := r.jobs(kit); jobs != "" {
		lines = append(lines, wrap(jobs, width))
	}

	return append(lines, "")
}

// state says whether the run is going, passed, or failed — and, when it failed,
// what step failed rather than the exit code the step happened to end with.
func (r commandRun) state(kit renderKit) string {
	running := func() string { return kit.marks.inFlight + " running" + kit.marks.ellipsis }

	return map[runStage]func() string{
		runStarting: running,
		runRunning:  running,
		runStopping: func() string { return kit.marks.inFlight + " stopping" + kit.marks.ellipsis },
		runPassed:   func() string { return kit.marks.done + " done" },
		runFailed:   func() string { return r.failureHeadline(kit) },
		runStopped:  func() string { return kit.marks.notStarted + " stopped" },
	}[r.stage]()
}

// failureHeadline is the run kind's refusal for a bare failure status — git's
// "exit status 1", which says nothing on its own. An error that carries its
// cause, a message that could not be written say, is told as every failure is.
func (r commandRun) failureHeadline(kit renderKit) string {
	if errors.Is(r.err, proc.ErrExitStatus) {
		return kit.failedGlyph() + " " + r.kind.refusal
	}

	return kit.failureLine(r.err)
}

// jobs is lefthook's jobs, each with its glyph, when the output is lefthook's.
// A job lefthook never reported on once the run is over — stopped, or ended
// before lefthook's summary — is going no longer, and nothing said how it
// ended: it takes the mark a stopped run does.
func (r commandRun) jobs(kit renderKit) string {
	unreported := kit.marks.inFlight
	if !r.going() {
		unreported = kit.marks.notStarted
	}

	glyphs := map[hooks.JobState]string{
		hooks.JobRunning: unreported, hooks.JobPassed: kit.marks.done,
		hooks.JobFailed: kit.failedGlyph(), hooks.JobSkipped: kit.marks.notStarted,
	}
	parts := make([]string, 0, len(r.jobList))

	for _, job := range r.jobList {
		parts = append(parts, glyphs[job.State]+" "+job.Name)
	}

	return strings.Join(parts, kit.marks.separator)
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
// the places to jump to are folded in as the lines arrive, the places bounded
// in turn by hooks.MaxPlaces, so what the kept lines are still for is the tail
// on screen.
const maxRunLines = 5000

// capLines keeps at most the last maxRunLines, cloning when it trims so the
// dropped head's backing array is not held alive.
func capLines(lines []string) []string {
	if len(lines) <= maxRunLines {
		return lines
	}

	return slices.Clone(lines[len(lines)-maxRunLines:])
}

// footer offers what works on the run as it stands: stop only once its
// program has started, and until it is stopped.
func (r commandRun) footer(keys keyMap) []key.Binding {
	switch {
	case r.stage == runRunning:
		return []key.Binding{keys.stopRun, keys.interrupt}
	case r.going():
		return []key.Binding{keys.interrupt}
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
	case r.going():
		return r.stopRun(m, msg)
	case key.Matches(msg, m.keys.closeOverlay):
		m, status := m.closeOverlay().readingChanges()
		m, read := m.readingBranch()

		return m, tea.Batch(status, read)
	case key.Matches(msg, m.keys.retry):
		return m.startRun(r.kind, r.start, r.succeeded)
	case key.Matches(msg, m.keys.confirm):
		return r.openFailure(m)
	case key.Matches(msg, m.keys.fullOutput):
		r.showOutput = !r.showOutput
	case key.Matches(msg, m.keys.cursorKeys()...):
		return r.step(m, m.keys.stepOf(msg)), nil
	}

	m.overlay = r

	return m, nil
}

// which names a running command.
func (commandRun) which() overlayKind { return overlayCommandRun }

// openedAs is the round the run drew as it started.
func (r commandRun) openedAs() int { return r.id }

// step moves the choice of place by delta. A run still going has no places yet:
// they are found once it fails.
func (r commandRun) step(m Model, delta int) Model {
	r.failures = r.failures.moved(delta)
	m.overlay = r

	return m
}

// stopRun kills a running program when the stop key is pressed. The run shows
// as stopping until the exit arrives, which decides how it ended, as a run on
// the web does. Any other key while it goes is ignored — the run is not
// abandoned — and so is stop before the program has started or once it is
// asked to stop.
func (r commandRun) stopRun(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if !key.Matches(msg, m.keys.stopRun) || r.stage != runRunning {
		return m, nil
	}

	r.stop()
	m.overlay = r.stopping()

	return m, nil
}

// click selects the place on a clicked line. A run still going has no
// places yet, so a click there selects none.
func (r commandRun) click(m Model, line int) (Model, tea.Cmd) {
	// The offset is the header the view drew, in display rows — the jobs line
	// wraps to as many rows as it took, so a click below it skips every one.
	offset := rowsIn(strings.Join(r.header(m.kit(), m.detailWidth()), "\n"))
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

var _ applier = editorClosed{}

// apply reports an editor that could not be opened.
func (msg editorClosed) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return m.noticedFailure(msg.err), nil
	}

	return m, nil
}
