// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"maps"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/tui/layout"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// dirPicker is the directory picker: a path typed from where you work, or
// from your home after a ~, narrowing the directories there as it is typed,
// and the list of them to move through, go into, and switch to.
type dirPicker struct {
	base, home string
	input      textinput.Model
	// inList is the list having the keys rather than the field; cursor is
	// the row highlighted in it.
	inList bool
	cursor int
	// shortcuts are the repository's other worktrees and the favorites,
	// offered before anything typed narrows to a path.
	shortcuts []dirRow
	// reads are the directories read, by dirRead's key; each copy of the
	// picker that adds one adds it to a copy of its own.
	reads map[string]dirRead
	// standIn is the last listing narrowed by a name to answer; its
	// directories stand in for the next one asked of the same directory
	// while that is out.
	standIn dirListed
	// looking is a destination being checked; problem is why it could not
	// be gone to.
	looking bool
	problem error
	// opened is the count of overlays opened when this one opened, so an
	// answer arriving after esc finds no picker, even one opened since.
	opened int
}

// dirRead is a directory's listing, narrowed by a prefix or not, or why it
// could not be read, listing nothing of its own then; asked while the read is
// out. A narrowed listing joined with a whole one that failed keeps its own
// directories beside the whole one's failure.
type dirRead struct {
	listing workdirs.Listing
	err     error
	asked   bool
}

// The room the picker would like: at 80 by 24 it fills all but a margin of
// the body.
const (
	pickerWidth  = 64
	pickerHeight = 18
)

var (
	_ overlay   = dirPicker{}
	_ floating  = dirPicker{}
	_ pasteable = dirPicker{}
)

// openDirPicker opens the picker, empty, from where you work, offering the
// other worktrees and the favorites first.
func (m Model) openDirPicker() (Model, tea.Cmd) {
	input := newInput("")
	input.Prompt = m.marks.prompt
	input.Placeholder = shownDir(m.deps, m.deps.Repositories.Here.Dir)
	m, opened := m.opening()

	picker := dirPicker{
		input: input, base: m.deps.Repositories.Here.Dir, home: m.deps.Repositories.Home, opened: opened,
		shortcuts: m.pickerShortcuts(), reads: map[string]dirRead{},
	}

	return picker.reading(m)
}

// wants is the picker's own size.
func (dirPicker) wants(renderKit, layout.Size) layout.Size {
	return layout.Size{Width: pickerWidth, Height: pickerHeight}
}

// which names the directory picker.
func (dirPicker) which() overlayKind { return overlayDirPicker }

// footer offers what the field or the list answers now.
func (p dirPicker) footer(keys keyMap) []key.Binding {
	switch {
	case p.looking:
		return []key.Binding{relabel(keys.closeOverlay, escCancel)}
	case p.inList:
		return []key.Binding{
			keys.up, keys.down, relabel(keys.cursorLeft, "up a level"), relabel(keys.cursorRight, "into"),
			relabel(keys.switchDir, "switch"), relabel(keys.nextField, "type"), relabel(keys.closeOverlay, escCancel),
		}
	default:
		return []key.Binding{
			relabel(keys.confirm, "open"), relabel(keys.nextField, "list"), relabel(keys.closeOverlay, escCancel),
		}
	}
}

// handleKey answers a key: the field has every key but those the footer
// names, so a path's j or q is typed; the list moves on them instead, and a
// printable key there goes back to the field and is typed.
func (p dirPicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		// Even while a destination is looked at: its answer, when it comes,
		// finds the picker gone, or another opened since, which it leaves be.
		return m.closeOverlay(), nil
	case p.looking:
		return m, nil
	case p.inList:
		return p.listKey(m, msg)
	default:
		return p.fieldKey(m, msg)
	}
}

// fieldKey answers a key while the field has them.
func (p dirPicker) fieldKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.confirm):
		return p.choose(m)
	case key.Matches(msg, m.keys.nextField):
		p.inList = true
		m.overlay = p

		return m, nil
	case msg.Text == "" && key.Matches(msg, m.keys.up, m.keys.down):
		return p.moved(m, m.keys.stepOf(msg)), nil
	default:
		return p.edited(m, msg)
	}
}

// edited answers a key that edits the field: → at its end goes into the
// highlighted directory, backspace there on an empty name goes up a level,
// and anything else is typed.
func (p dirPicker) edited(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case msg.Text == "" && key.Matches(msg, m.keys.cursorRight) && p.atEnd():
		return p.descend(m)
	case msg.Code == tea.KeyBackspace && p.backspaceGoesUp():
		return p.up(m)
	default:
		return p.typed(m, msg)
	}
}

// listKey answers a key while the list has them.
func (p dirPicker) listKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.confirm, m.keys.cursorRight):
		return p.choose(m)
	case key.Matches(msg, m.keys.cursorLeft):
		return p.up(m)
	case key.Matches(msg, m.keys.switchDir):
		return p.switchTo(m)
	case key.Matches(msg, m.keys.nextField):
		p.inList = false
		m.overlay = p

		return m, nil
	case key.Matches(msg, m.keys.cursorKeys()...):
		return p.moved(m, m.keys.stepOf(msg)), nil
	case msg.Text != "":
		p.inList = false

		return p.typed(m, msg)
	default:
		return m, nil
	}
}

// pasted types a paste into the field, from the list too.
func (p dirPicker) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if p.looking {
		return m, nil
	}

	p.inList = false

	return p.typed(m, paste)
}

// typed hands a key or a paste to the field, highlighting the first match of
// what it now holds, and reads where it points.
func (p dirPicker) typed(m Model, msg tea.Msg) (Model, tea.Cmd) {
	box, _ := m.dialogBox()
	width, _ := dialogInside(m.shape(), box)
	p = p.fitted(width)
	p.input, _ = p.input.Update(msg)

	return p.retyped(m)
}

// retyped is the picker once its field has changed: its first row
// highlighted, any refusal forgotten, and where it now points read.
func (p dirPicker) retyped(m Model) (Model, tea.Cmd) {
	p.problem, p.cursor = nil, 0

	return p.reading(m)
}

// moved is the picker with its highlight moved by delta.
func (p dirPicker) moved(m Model, delta int) Model {
	p.cursor = max(0, min(p.cursor+delta, len(p.rows())-1))
	m.overlay = p

	return m
}

// backspaceGoesUp reports backspace going up a level rather than deleting:
// with the cursor at the end of the field and no name after its last
// separator, below the root. An empty field has no separator to go up from,
// and a lone ~ is deleted like any other letter.
func (p dirPicker) backspaceGoesUp() bool {
	parent := p.parent()
	value := p.input.Value()

	return p.atEnd() && p.nameTyped() == "" && value != "" && value != "~" && filepath.Dir(parent) != parent
}

// atEnd reports the field's cursor at the end of what it holds.
func (p dirPicker) atEnd() bool {
	return p.input.Position() == len([]rune(p.input.Value()))
}

// setField is the picker with its field holding text, its cursor at the end.
func (p dirPicker) setField(text string) dirPicker {
	p.input.SetValue(text)
	p.input.CursorEnd()

	return p
}

// choose goes into the highlighted directory, or toward switching to the
// highlighted destination, as s does; toward the path typed, where there is
// no row to highlight.
func (p dirPicker) choose(m Model) (Model, tea.Cmd) {
	row, found := p.highlighted()

	switch {
	case !found:
		return p.look(m, p.typedPath())
	case row.kind == rowChild:
		return p.descend(m)
	default:
		return p.look(m, row.dir)
	}
}

// switchTo goes toward switching to the highlighted row's directory, a
// directory in the list among them.
func (p dirPicker) switchTo(m Model) (Model, tea.Cmd) {
	row, found := p.highlighted()
	if !found {
		return m, nil
	}

	return p.look(m, row.dir)
}

// descend types the highlighted directory's name into the field, and lists
// what is in it.
func (p dirPicker) descend(m Model) (Model, tea.Cmd) {
	row, found := p.highlighted()
	if !found || row.kind != rowChild {
		m.overlay = p

		return m, nil
	}

	head, _ := p.split()
	p = p.setField(head + row.match.Entry.Name + "/")

	return p.retyped(m)
}

// up types the directory above the one the field points into, and lists it.
func (p dirPicker) up(m Model) (Model, tea.Cmd) {
	above := filepath.Dir(p.parent())
	shown := workdirs.Shown(above, p.home)

	if !strings.HasSuffix(shown, "/") {
		shown += "/"
	}

	p = p.setField(shown)

	return p.retyped(m)
}

// dirListed is a directory read for the picker opened as opened, narrowed by
// prefix or not.
type dirListed struct {
	dir, prefix string
	read        dirRead
	opened      int
}

var _ applier = dirListed{}

// apply keeps the listing in the picker that asked, open or under the last
// look that goes back to it, and reads what it needs next: the same
// directory narrowed by what was typed, when the listing stopped short of
// everything in it or the name typed starts with a dot.
func (msg dirListed) apply(m Model) (Model, tea.Cmd) {
	list := m.deps.Repositories.Subdirectories

	switch open := m.overlay.(type) {
	case dirPicker:
		if open.opened == msg.opened {
			var next tea.Cmd

			m.overlay, next = open.kept(msg).asking(list)

			return m, next
		}
	case lastLook:
		if picker, under := open.back.(dirPicker); under && picker.opened == msg.opened {
			var next tea.Cmd

			open.back, next = picker.kept(msg).asking(list)
			m.overlay = open

			return m, next
		}
	}

	return m, nil
}

// kept is the picker with a listing it asked for kept, and one narrowed by
// a name kept to stand in for the next, its highlight on a row there still
// is.
func (p dirPicker) kept(msg dirListed) dirPicker {
	p.reads = maps.Clone(p.reads)
	p.reads[readKey(msg.dir, msg.prefix)] = msg.read

	if msg.prefix != "" {
		p.standIn = msg
	}

	p.cursor = min(p.cursor, max(0, len(p.rows())-1))

	return p
}

// reading asks for the listing the rows need next, off the update loop, if
// one is missing and not already asked for.
func (p dirPicker) reading(m Model) (Model, tea.Cmd) {
	var next tea.Cmd

	m.overlay, next = p.asking(m.deps.Repositories.Subdirectories)

	return m, next
}

// asking is the picker with the listing its rows need next asked for
// through list, and the read that answers it; the picker alone when none is
// missing or there is nothing to list with.
func (p dirPicker) asking(list func(dir, prefix string) (workdirs.Listing, error)) (dirPicker, tea.Cmd) {
	dir, prefix, needed := p.needs()
	if !needed || list == nil {
		return p, nil
	}

	p.reads = maps.Clone(p.reads)
	p.reads[readKey(dir, prefix)] = dirRead{asked: true}
	opened := p.opened

	return p, func() tea.Msg {
		listing, err := list(dir, prefix)

		return dirListed{dir: dir, prefix: prefix, read: readOf(listing, err), opened: opened}
	}
}

// readOf is what reading a listing answered: the listing, or why it could
// not be read, with nothing listed.
func readOf(listing workdirs.Listing, err error) dirRead {
	if err != nil {
		return dirRead{err: err}
	}

	return dirRead{listing: listing}
}

// readKey is where the listing of dir, narrowed by prefix, is kept.
func readKey(dir, prefix string) string {
	return dir + "\x00" + prefix
}
