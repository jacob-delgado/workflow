// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// runHeader is the rows above a finished run's list of places to jump to.
const runHeader = 2

// starter starts a program whose output a run streams.
type starter func() (proc.Output, error)

// commandRun is a program running with its output streamed: a commit, a push,
// a hook. When it fails, every place a tool pointed at can be opened in the
// editor at its line.
type commandRun struct {
	marks glyphs
	title string
	id    int
	start starter
	// succeeded is what happens once the program exits cleanly; nil keeps the
	// output on screen until it is closed.
	succeeded func(m Model) (Model, tea.Cmd)

	lines    []string
	done     bool
	err      error
	failures []hooks.Location
	selected int
}

var (
	_ overlay   = commandRun{}
	_ clickable = commandRun{}
)

// startRun opens a run and starts its program.
func (m Model) startRun(title string, start starter, succeeded func(Model) (Model, tea.Cmd)) (Model, tea.Cmd) {
	m.runs++
	m.overlay = commandRun{marks: m.marks, title: title, id: m.runs, start: start, succeeded: succeeded}

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

// apply begins reading the output, or reports the program not starting.
func (msg runStarted) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return runFinished{id: msg.id, err: msg.err}.apply(m)
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
		run.lines = append(run.lines, sanitize.Text(msg.line))
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

	run.done, run.err = true, msg.err
	if msg.err != nil {
		run.failures = hooks.Failures(run.lines)
	}

	m.overlay = run

	if msg.err == nil && run.succeeded != nil {
		return run.succeeded(m)
	}

	return m, nil
}

// view shows the jobs as lefthook reports them, then either the places to jump
// to or the tail of the output, and how the run stands.
func (r commandRun) view(width, rows int) (string, string) {
	lines := []string{r.state()}

	if jobs := r.jobs(); jobs != "" {
		lines = append(lines, wrap(jobs, width))
	}

	if len(r.failures) > 0 {
		lines = append(lines, "")
		lines = append(lines, r.failureRows(rows-len(lines))...)
	} else {
		lines = append(lines, "")
		lines = append(lines, tail(r.lines, rows-len(lines))...)
	}

	return r.title, strings.Join(lines, "\n")
}

// state says whether the run is going, passed, or failed and why.
func (r commandRun) state() string {
	switch {
	case !r.done:
		return r.marks.inFlight + " running" + r.marks.ellipsis
	case r.err != nil:
		return r.marks.failed + " " + r.err.Error()
	default:
		return r.marks.done + " done"
	}
}

// jobs is lefthook's jobs, each with its glyph, when the output is lefthook's.
func (r commandRun) jobs() string {
	parsed := hooks.Jobs(r.lines)
	parts := make([]string, 0, len(parsed))

	for _, job := range parsed {
		glyph := map[hooks.JobState]string{
			hooks.JobRunning: r.marks.inFlight, hooks.JobPassed: r.marks.done,
			hooks.JobFailed: r.marks.failed, hooks.JobSkipped: r.marks.notStarted,
		}[job.State]
		parts = append(parts, glyph+" "+job.Name)
	}

	return strings.Join(parts, r.marks.separator)
}

// failureRows lists the places to jump to, scrolled to the selection.
func (r commandRun) failureRows(rows int) []string {
	first, last := window(r.selected, len(r.failures), rows)
	lines := make([]string, 0, last-first)

	for index := first; index < last; index++ {
		failure := r.failures[index]
		lines = append(lines, r.marks.marker(index == r.selected)+failure.File+":"+strconv.Itoa(failure.Line)+" "+
			failure.Message)
	}

	return lines
}

// tail is the last rows lines of output.
func tail(lines []string, rows int) []string {
	return lines[max(0, len(lines)-max(0, rows)):]
}

// footer offers what works on the run as it stands.
func (r commandRun) footer(keys keyMap) []key.Binding {
	switch {
	case !r.done:
		return []key.Binding{keys.interrupt}
	case len(r.failures) > 0:
		return []key.Binding{keys.up, keys.down, relabel(keys.confirm, "open in editor"), keys.retry, keys.closeOverlay}
	default:
		return []key.Binding{keys.retry, keys.closeOverlay}
	}
}

// handleKey answers a key while a run is shown. A running program is not
// abandoned: its outcome decides what happens next.
func (r commandRun) handleKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case !r.done:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), tea.Batch(m.loadChanges(), m.loadBranch())
	case key.Matches(msg, m.keys.retry):
		return m.startRun(r.title, r.start, r.succeeded)
	case key.Matches(msg, m.keys.confirm):
		return r.openFailure(m)
	case key.Matches(msg, m.keys.down):
		r.selected = min(r.selected+1, max(0, len(r.failures)-1))
	case key.Matches(msg, m.keys.up):
		r.selected = max(0, r.selected-1)
	}

	m.overlay = r

	return m, nil
}

// click selects the place on a clicked line.
func (r commandRun) click(m Model, line int) (Model, tea.Cmd) {
	offset := runHeader
	if r.jobs() != "" {
		offset++
	}

	first, last := window(r.selected, len(r.failures), m.detailRows()-offset)
	index := first + line - offset

	if !r.done || line < offset || index >= last {
		return m, nil
	}

	r.selected = index
	m.overlay = r

	return m, nil
}

// openFailure opens the selected place in the editor, at its line.
func (r commandRun) openFailure(m Model) (Model, tea.Cmd) {
	if len(r.failures) == 0 || m.deps.Editor.Open == nil {
		return m, nil
	}

	failure := r.failures[r.selected]

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
		return m.noticed(m.failure(msg.err)), nil
	}

	return m, nil
}

// pushPreview is a branch about to be pushed, shown for a last look — the one
// outward-facing act reachable from a single key, so it gets one like the rest.
type pushPreview struct {
	branch, remote string
}

var _ overlay = pushPreview{}

// view names what will be pushed and where.
func (p pushPreview) view(width, _ int) (string, string) {
	return "Push branch", wrap("push "+p.branch+" to "+p.remote, width)
}

// footer offers pushing or backing out.
func (p pushPreview) footer(keys keyMap) []key.Binding {
	return []key.Binding{relabel(keys.confirm, "push"), keys.closeOverlay}
}

// handleKey answers a key while the push is previewed.
func (p pushPreview) handleKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return m.startPush(nil)
	default:
		return m, nil
	}
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
