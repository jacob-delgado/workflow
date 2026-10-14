// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package console

import (
	"errors"
	"io"
	"strconv"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

// Watched runs wait with a live line kept at the foot of stderr, a terminal
// a person watches: a turning mark, what the wait is doing, and, once it
// counts, a bar and how many of how many. Whatever is written to stderr while
// it runs, through these notes or a Stream they are said on, prints above the
// line, which is erased once the wait is over; the wait's error is Watched's.
//
// The line is a Bubble Tea program, inline and taking no input, so it sends
// the terminal no query whose answer would reach the shell, and catching no
// signal, which the command's context hears instead. The wait runs as the
// program's first command, so nothing here starts a goroutine of its own,
// and the program ends when the wait does: a wait stopped by ctrl+c still
// says why before the line goes.
func (n Notes) Watched(wait func(Wait) error) error {
	var (
		program *tea.Program
		outcome error
	)

	stream := streamOver(n.stderr)
	model := newLiveModel(func() tea.Msg {
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

// liveFramesPerSecond is how fast the live line's mark turns: a quarter turn
// a frame, a turn a second.
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
// gave it, and the width it is drawn in.
type liveModel struct {
	wait        tea.Cmd
	mark        spinner.Model
	bar         progress.Model
	doing       string
	done, total int
	width       int
}

var _ tea.Model = liveModel{}

// newLiveModel is the live line of wait, before it has anything to say.
func newLiveModel(wait tea.Cmd) liveModel {
	bar := progress.New(progress.WithWidth(liveBarWidth), progress.WithoutPercentage(),
		progress.WithFillCharacters('━', '─'), progress.WithColors(lipgloss.NoColor{}))
	bar.EmptyColor = lipgloss.NoColor{}

	turning := spinner.Spinner{Frames: []string{"◐", "◓", "◑", "◒"}, FPS: time.Second / liveFramesPerSecond}

	return liveModel{wait: wait, mark: spinner.New(spinner.WithSpinner(turning)), bar: bar, width: liveWidth}
}

// Init starts the wait, and the mark turning.
func (m liveModel) Init() tea.Cmd {
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

	return tea.NewView(ansi.Truncate(line, m.width-1, "…"))
}
