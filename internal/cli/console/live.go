// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"errors"
	"io"
	"strconv"
	"time"

	progressbar "charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/progress"
)

// Wait is what a wait a person may be watching tells its live line — what it
// is doing, and how much of it is done — and where it says what it finds
// while the line is kept: its notes, and what it prints above the line.
type Wait struct {
	// program draws the live line, or is nil when no line is kept.
	program *tea.Program
	notes   Notes
	above   io.Writer
}

// Doing says what the wait is doing, the words of its live line.
func (w Wait) Doing(what string) {
	w.send(liveDoing(what))
}

// Counted says how much of the wait is done: done of total, which the live
// line shows as a bar and in words.
func (w Wait) Counted(done, total int) {
	w.send(liveCount{done: done, total: total})
}

// Notes are the notes the wait says, printed above the live line while it is
// kept.
func (w Wait) Notes() Notes {
	return w.notes
}

// Above is a stream printed above the live line, on stderr, a write at a
// time; with no line kept, it is the notes' stream, each line a note.
func (w Wait) Above() io.Writer {
	return w.above
}

// send tells the live line msg, when one is kept.
func (w Wait) send(msg tea.Msg) {
	if w.program != nil {
		w.program.Send(msg)
	}
}

// Unwatched runs wait with no live line kept, for stderr that no person
// watches — a pipe, a file: what it does goes unsaid, and only what it says
// prints.
func (n Notes) Unwatched(wait func(Wait) error) error {
	return wait(Wait{notes: n, above: n.Writer()})
}

// Look is how a live line is drawn: in plain ASCII, for a terminal or font
// without the Unicode marks — ui.ascii, or status --plain — and still, its mark
// held rather than turning, for anyone motion distracts — ui.motion reduce.
type Look struct {
	ASCII bool
	Still bool
}

// frames is the frames the line's mark turns through: the in-flight glyph
// alone, which does not turn, when the look is still.
func (l Look) frames() []string {
	if l.Still {
		return []string{progress.InFlight.Glyph(l.ASCII)}
	}

	return progress.Turn(l.ASCII)
}

// liveGlyphs are the characters a live line draws its bar with, filled and
// empty, and ends a line cut to the terminal's width with.
type liveGlyphs struct {
	filled, empty rune
	cut           string
}

// glyphs is the characters the look draws a line with: plain ASCII, or the
// Unicode a line is drawn in by default.
func (l Look) glyphs() liveGlyphs {
	if l.ASCII {
		return liveGlyphs{filled: '=', empty: '-', cut: "..."}
	}

	return liveGlyphs{filled: '━', empty: '─', cut: "…"}
}

// Watched runs wait with a live line kept at the foot of stderr, a terminal
// a person watches, drawn as look asks: a mark, turning unless look holds it
// still, what the wait is doing, and, once it counts, a bar and how many of
// how many. Whatever is written to stderr while it runs, through these notes
// or a Stream they are said on, prints above the line, which is erased once
// the wait is over; the wait's error is Watched's.
//
// The line is a Bubble Tea program, inline and taking no input, so it sends
// the terminal no query whose answer would reach the shell, and catching no
// signal, which the command's context hears instead. The wait runs as the
// program's first command, so nothing here starts a goroutine of its own,
// and the program ends when the wait does: a wait stopped by ctrl+c still
// says why before the line goes.
func (n Notes) Watched(look Look, wait func(Wait) error) error {
	var (
		program *tea.Program
		outcome error
	)

	stream := streamOver(n.stderr)
	model := newLiveModel(look, func() tea.Msg {
		outcome = wait(Wait{program: program, notes: n.To(stream), above: stream})

		return liveOver{}
	})

	program = tea.NewProgram(model, tea.WithOutput(stream.drawnBy(&program)), tea.WithInput(nil),
		tea.WithoutSignalHandler(), tea.WithEnvironment(environOf(n.getenv, liveVariables())),
		tea.WithWindowSize(liveWidth, liveHeight))

	defer stream.undrawn()

	// The program fails only when it is killed, as a panic in the wait kills
	// it, which it says on stderr as it restores the terminal.
	_, err := program.Run()

	return errors.Join(outcome, err)
}

// The size a live line is drawn for on a stream that cannot say its own.
const (
	liveWidth  = 80
	liveHeight = 24
)

// liveBarWidth is how wide the live line's bar is.
const liveBarWidth = 24

// liveFramesPerSecond is how fast the live line's mark turns: four frames a
// second, a turn of its frames.
const liveFramesPerSecond = 4

// liveVariables are the variables the live line's program reads: what the
// terminal can draw, and its size where the terminal cannot say it.
func liveVariables() []string {
	return append(colorVariables(), "COLUMNS", "LINES")
}

// liveDoing is what the wait is doing.
type liveDoing string

// liveCount is how much of the wait is done.
type liveCount struct {
	done, total int
}

// liveOver is the wait, over.
type liveOver struct{}

// liveModel is the live line: its mark, the words and count the wait last
// gave it, the width it is drawn in, and what ends it when it is cut to that
// width.
type liveModel struct {
	wait        tea.Cmd
	mark        spinner.Model
	bar         progressbar.Model
	doing       string
	done, total int
	width       int
	cut         string
}

var _ tea.Model = liveModel{}

// newLiveModel is the live line of wait, drawn as look asks, before it has
// anything to say.
func newLiveModel(look Look, wait tea.Cmd) liveModel {
	drawn := look.glyphs()
	bar := progressbar.New(progressbar.WithWidth(liveBarWidth), progressbar.WithoutPercentage(),
		progressbar.WithFillCharacters(drawn.filled, drawn.empty), progressbar.WithColors(lipgloss.NoColor{}))
	bar.EmptyColor = lipgloss.NoColor{}

	turning := spinner.Spinner{Frames: look.frames(), FPS: time.Second / liveFramesPerSecond}

	return liveModel{
		wait: wait, mark: spinner.New(spinner.WithSpinner(turning)), bar: bar, width: liveWidth, cut: drawn.cut,
	}
}

// Init starts the wait, and the mark turning when it has more than one frame
// to turn through.
func (m liveModel) Init() tea.Cmd {
	if len(m.mark.Spinner.Frames) == 1 {
		return m.wait
	}

	return tea.Batch(m.wait, m.mark.Tick)
}

// Update takes what the wait says, turns the mark, and quits once the wait
// is over.
//
//nolint:ireturn // tea.Model's own signature
func (m liveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case liveDoing:
		m.doing = string(msg)
	case liveCount:
		m.done, m.total = msg.done, msg.total
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case liveOver:
		return m, tea.Quit
	case spinner.TickMsg:
		var turned tea.Cmd

		m.mark, turned = m.mark.Update(msg)

		return m, turned
	}

	return m, nil
}

// View is the live line, cut to the terminal's width, and nothing before the
// wait says what it does. The program erases it as it ends.
func (m liveModel) View() tea.View {
	if m.doing == "" {
		return tea.NewView("")
	}

	line := m.mark.View() + " " + m.doing
	if m.total > 0 {
		line += " " + m.bar.ViewAs(float64(m.done)/float64(m.total)) + " " +
			strconv.Itoa(m.done) + " of " + strconv.Itoa(m.total)
	}

	return tea.NewView(ansi.Truncate(line, m.width-1, m.cut))
}
