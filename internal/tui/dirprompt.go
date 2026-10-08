// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// dirPrompt is the go-to prompt: a path typed from where you work, or from
// your home after a ~, completed by tab from the directories there and gone
// to once it is checked to be one.
type dirPrompt struct {
	base, home string
	input      textinput.Model
	// choices are the directories that fit what was typed, when tab found
	// several; note is what tab found otherwise.
	choices []string
	note    string
	// looking is a typed path being checked; problem is why it could not be
	// gone to.
	looking bool
	problem error
	// opened is the count of overlays opened when this one opened, so a look
	// answering after esc finds no prompt, even one opened since.
	opened int
}

var (
	_ overlay   = dirPrompt{}
	_ pasteable = dirPrompt{}
)

// openDirPrompt opens the go-to prompt, empty, from where you work.
func (m Model) openDirPrompt() (Model, tea.Cmd) {
	input := newInput("")
	input.Placeholder = m.shownDir(m.deps.Repositories.Here.Dir)
	m, opened := m.opening()

	m.overlay = dirPrompt{
		input: input,
		base:  m.deps.Repositories.Here.Dir, home: m.deps.Repositories.Home, opened: opened,
	}

	return m, nil
}

// view draws the path being typed, what tab found, and why a path could not
// be gone to. The path and the names tab found are as they are on disk, so
// they are neutralized only as they are drawn.
func (p dirPrompt) view(kit renderKit, width, _ int) (string, string) {
	p.input.SetWidth(max(1, width-len(p.input.Prompt)-1))

	lines := []string{"Type a path: from where you work, or from your home after ~.", "", drawnField(p.input)}
	if len(p.choices) > 0 {
		shown := make([]string, 0, len(p.choices))
		for _, choice := range p.choices {
			shown = append(shown, sanitize.Line(choice))
		}

		lines = append(lines, "", kit.styles.label.Render(strings.Join(shown, "  ")))
	}

	switch {
	case p.looking:
		lines = append(lines, "", "reading"+kit.marks.ellipsis)
	case p.problem != nil:
		lines = append(lines, "", failureLine(kit.styles, kit.marks, p.problem))
	case p.note != "":
		lines = append(lines, "", p.note)
	}

	return "Go to a directory", wrap(strings.Join(lines, "\n"), width)
}

// handleKey types every key but the three the footer names, so a path's j
// or q is typed rather than moving or quitting.
func (p dirPrompt) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		// Even while a path is looked at: a mount that does not answer is not
		// waited on, and its answer, when it comes, finds the prompt gone, or
		// another opened since, which it leaves alone.
		return m.closeOverlay(), nil
	case p.looking:
		return m, nil
	case key.Matches(msg, m.keys.confirm):
		return p.look(m)
	case key.Matches(msg, m.keys.nextField):
		return p.complete(m)
	default:
		return p.typed(m, msg)
	}
}

// pasted types a paste into the path, as typing it would.
func (p dirPrompt) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if p.looking {
		return m, nil
	}

	return p.typed(m, paste)
}

// typed hands a key or a paste to the path, forgetting what tab and enter
// last found.
func (p dirPrompt) typed(m Model, msg tea.Msg) (Model, tea.Cmd) {
	p.input, _ = p.input.Update(msg)
	p.choices, p.note, p.problem = nil, "", nil
	m.overlay = p

	return m, nil
}

// dirLooked is a typed path checked, and where it leads or why not, for the
// prompt opened as opened.
type dirLooked struct {
	dir    string
	here   bool
	err    error
	opened int
}

var _ applier = dirLooked{}

// apply goes to the directory checked, or says why it cannot, when the prompt
// that asked is still open.
func (msg dirLooked) apply(m Model) (Model, tea.Cmd) {
	prompt, open := m.overlay.(dirPrompt)
	if !open || prompt.opened != msg.opened {
		return m, nil
	}

	switch {
	case msg.err != nil:
		prompt.looking, prompt.problem = false, msg.err
		m.overlay = prompt

		return m, nil
	case msg.here:
		return m.closeOverlay().noticed("you already work in " + m.shownDir(msg.dir)), nil
	default:
		return m.closeOverlay().leaveFor(msg.dir)
	}
}

// look checks the path typed off the update loop, since it reads the disk.
func (p dirPrompt) look(m Model) (Model, tea.Cmd) {
	look := m.deps.Repositories.Look
	if look == nil {
		p.note = "cannot look at directories here"
		m.overlay = p

		return m, nil
	}

	dir := workdirs.Resolve(p.input.Value(), p.base, p.home)
	base, opened := p.base, p.opened
	p.looking = true
	m.overlay = p

	return m, func() tea.Msg {
		place, err := look(dir)
		here := place.Dir == base || workdirs.Same(place.Dir, base)

		return dirLooked{dir: place.Dir, here: here, err: err, opened: opened}
	}
}

// dirCompleted is the directories that fit what was typed when tab was
// pressed, or why there were none to read.
type dirCompleted struct {
	typed, head, partial string
	names                []string
	err                  error
}

var _ applier = dirCompleted{}

// apply completes the path when one directory fits, types what several
// share and names them, or says none does; an answer for a path since
// changed is dropped.
func (msg dirCompleted) apply(m Model) (Model, tea.Cmd) {
	prompt, open := m.overlay.(dirPrompt)
	if !open || prompt.input.Value() != msg.typed {
		return m, nil
	}

	switch len(msg.names) {
	case 0:
		prompt.note = "no directory there starts with " + sanitize.Line(msg.partial)
		if msg.err != nil {
			prompt.note = inFull(msg.err)
		}
	case 1:
		prompt.input.SetValue(msg.head + msg.names[0] + "/")
	default:
		prompt.input.SetValue(msg.head + sharedPrefix(msg.names))
		prompt.choices = msg.names
	}

	prompt.input.CursorEnd()
	m.overlay = prompt

	return m, nil
}

// complete reads the directories where the path typed so far points, off the
// update loop.
func (p dirPrompt) complete(m Model) (Model, tea.Cmd) {
	list := m.deps.Repositories.Subdirectories
	if list == nil {
		return m, nil
	}

	typed := p.input.Value()
	if typed == "~" {
		typed = "~/"
	}

	cut := strings.LastIndexAny(typed, "/"+string(filepath.Separator)) + 1
	head, partial := typed[:cut], typed[cut:]
	parent := workdirs.Resolve(head, p.base, p.home)

	return m, func() tea.Msg {
		listing, err := list(parent, partial)

		names := make([]string, 0, len(listing.Entries))
		for _, entry := range listing.Entries {
			names = append(names, entry.Name)
		}

		return dirCompleted{typed: p.input.Value(), head: head, partial: partial, names: names, err: err}
	}
}

// sharedPrefix is what every name starts with, whole letters only: v1é and
// v1è share v1, not the first byte of their last letters.
func sharedPrefix(names []string) string {
	shared := []rune(names[0])
	for _, name := range names[1:] {
		for !strings.HasPrefix(name, string(shared)) {
			shared = shared[:len(shared)-1]
		}
	}

	return string(shared)
}

// drawnField is a text field as it is drawn, with each character in it that
// has no shape of its own — a direction mark, a zero-width space — shown as
// U+FFFD, as sanitize.Line shows a name. The field's own styling is kept: its
// value holds no other control, since the field drops each one as it is typed
// or set.
func drawnField(input textinput.Model) string {
	return strings.Map(func(character rune) rune {
		if unicode.In(character, unicode.Cf, unicode.Zl, unicode.Zp) {
			return utf8.RuneError
		}

		return character
	}, input.View())
}
