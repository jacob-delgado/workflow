// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// errCannotLook says this surface was given no way to read a directory.
var errCannotLook = errors.New("cannot look at directories here")

// rowKind is what a row of the picker offers: switching to the directory the
// field points into, a worktree or a favorite to switch to, or a directory
// to go into.
type rowKind int

const (
	rowHere rowKind = iota
	rowWorktree
	rowFavorite
	rowChild
)

// dirRow is one row of the picker: what it offers, the directory, and for a
// directory listed, which of its letters what was typed matched.
type dirRow struct {
	kind  rowKind
	dir   string
	match workdirs.Match
}

// rowsShownMost is the most rows the picker draws; past them it says how
// many more match.
const rowsShownMost = 200

// pickerShortcuts are the repository's other worktrees, then the favorites,
// as the Repositories pane lists them, leaving out where you work.
func (m Model) pickerShortcuts() []dirRow {
	rows := m.repositories.rows(m.deps.Repositories.Here)
	shortcuts := make([]dirRow, 0, len(rows))

	for _, row := range rows[1:] {
		kind := rowFavorite
		if row.worktree != nil {
			kind = rowWorktree
		}

		shortcuts = append(shortcuts, dirRow{kind: kind, dir: row.dir})
	}

	return shortcuts
}

// split is what the field holds cut at its last separator: the path to a
// directory, and the start of a name in it. A ~ alone is your home.
func (p dirPicker) split() (string, string) {
	typed := p.input.Value()
	if typed == "~" {
		typed = "~/"
	}

	cut := strings.LastIndexAny(typed, "/"+string(filepath.Separator)) + 1

	return typed[:cut], typed[cut:]
}

// nameTyped is the start of a name the field holds after its last separator.
func (p dirPicker) nameTyped() string {
	_, name := p.split()

	return name
}

// browsing reports the field holding a path rather than a bare name.
func (p dirPicker) browsing() bool {
	head, _ := p.split()

	return head != ""
}

// parent is the directory the field points into: where you work, until a
// path is typed.
func (p dirPicker) parent() string {
	head, _ := p.split()
	if head == "" {
		return p.base
	}

	return workdirs.Resolve(head, p.base, p.home)
}

// listing is what is known of the directory the field points into: its
// listing narrowed by the name typed where the whole one stopped short, and
// whether it has been asked for at all.
func (p dirPicker) listing() (dirRead, bool) {
	whole, found := p.reads[readKey(p.parent(), "")]
	if !found || !whole.listing.Truncated || p.nameTyped() == "" {
		return whole, found
	}

	narrowed, read := p.reads[readKey(p.parent(), p.nameTyped())]
	if !read {
		return whole, true
	}

	return narrowed, true
}

// needs is the listing the rows need and nobody has asked for: the
// directory's, and then, where that stopped short, the one narrowed by the
// name typed.
func (p dirPicker) needs() (string, string, bool) {
	dir := p.parent()

	whole, found := p.reads[readKey(dir, "")]
	if !found {
		return dir, "", true
	}

	_, narrowed := p.reads[readKey(dir, p.nameTyped())]
	if whole.listing.Truncated && p.nameTyped() != "" && !narrowed {
		return dir, p.nameTyped(), true
	}

	return "", "", false
}

// rows are what the picker offers: switching to where the field points once
// it holds a path, or the worktrees and favorites whose names hold what was
// typed while it does not, then the directories in it that do.
func (p dirPicker) rows() []dirRow {
	var rows []dirRow

	if p.browsing() {
		rows = append(rows, dirRow{kind: rowHere, dir: p.parent()})
	} else {
		rows = append(rows, p.shortcutsMatching()...)
	}

	read, found := p.listing()
	if !found || read.err != nil {
		return rows
	}

	for _, match := range workdirs.Matching(read.listing.Entries, p.nameTyped()) {
		rows = append(rows, dirRow{kind: rowChild, dir: filepath.Join(p.parent(), match.Entry.Name), match: match})
	}

	return rows
}

// shortcutsMatching are the worktrees and favorites whose names hold the
// name typed, letter by letter.
func (p dirPicker) shortcutsMatching() []dirRow {
	kept := make([]dirRow, 0, len(p.shortcuts))

	for _, shortcut := range p.shortcuts {
		named := []workdirs.Entry{{Name: filepath.Base(shortcut.dir)}}
		if len(workdirs.Matching(named, p.nameTyped())) > 0 {
			kept = append(kept, shortcut)
		}
	}

	return kept
}

// highlighted is the row the highlight is on, if there are any.
func (p dirPicker) highlighted() (dirRow, bool) {
	rows := p.rows()
	if len(rows) == 0 {
		return dirRow{}, false
	}

	return rows[min(p.cursor, len(rows)-1)], true
}

// dirLooked is a destination checked, and where it leads or why not, for the
// picker opened as opened.
type dirLooked struct {
	dir    string
	here   bool
	err    error
	opened int
}

var _ applier = dirLooked{}

// apply opens the last look at switching to the directory checked, over the
// picker it goes back to, says you already work there, or pins why it cannot
// be gone to, when the picker that asked is still open.
func (msg dirLooked) apply(m Model) (Model, tea.Cmd) {
	picker, open := m.overlay.(dirPicker)
	if !open || picker.opened != msg.opened {
		return m, nil
	}

	picker.looking = false

	switch {
	case msg.err != nil:
		picker.problem = msg.err
		m.overlay = picker

		return m, nil
	case msg.here:
		return m.closeOverlay().noticed("you already work in " + shownDir(m.deps, msg.dir)), nil
	}

	if busy := m.writeInFlight(); busy != "" {
		m.overlay = picker

		return m.noticed("wait for " + busy + " to finish before switching"), nil
	}

	look := m.switchLook(msg.dir)
	look.back, look.leave = picker, escBack
	m.overlay = look

	return m, nil
}

// look checks a destination off the update loop, since it reads the disk.
func (p dirPicker) look(m Model, dir string) (Model, tea.Cmd) {
	ctx, look := m.session, m.deps.Repositories.Look
	if look == nil {
		p.problem = errCannotLook
		m.overlay = p

		return m, nil
	}

	base, opened := p.base, p.opened
	p.looking = true
	m.overlay = p

	return m, func() tea.Msg {
		place, err := look(ctx, dir)
		here := place.Dir == base || workdirs.Same(place.Dir, base)

		return dirLooked{dir: place.Dir, here: here, err: err, opened: opened}
	}
}

// view draws the field, where it points and how much of it matches, the rows
// that fit, and what keeps them from being drawn. Names and paths are as they
// are on disk, so they are neutralized only as they are drawn.
func (p dirPicker) view(kit renderKit, width, rows int) (string, string) {
	p.input.SetWidth(max(1, width-len(p.input.Prompt)-1))
	where := "in " + shownFrom(p.home, p.parent())
	lines := make([]string, 0, max(0, rows))
	lines = append(lines, drawnField(p.input), p.heading(where, width))
	lines = append(lines, p.drawnRows(kit, width, rows-len(lines)-1)...)
	lines = append(lines, p.status(kit, where)...)

	return "Go to a directory", strings.Join(lines, "\n")
}

// heading says where the field points, and how many of the directories
// there match what was typed.
func (p dirPicker) heading(where string, width int) string {
	read, found := p.listing()
	if !found || read.asked || read.err != nil {
		return where
	}

	count := strconv.Itoa(len(workdirs.Matching(read.listing.Entries, p.nameTyped()))) + " of " +
		strconv.Itoa(len(read.listing.Entries))

	return where + strings.Repeat(" ", max(1, width-ansi.StringWidth(where)-ansi.StringWidth(count))) + count
}

// drawnRows are the rows that fit in room, the highlighted one among them,
// and how many more directories match past the most the picker draws, after
// what it offers before them.
func (p dirPicker) drawnRows(kit renderKit, width, room int) []string {
	rows := p.rows()

	offered := slices.IndexFunc(rows, func(row dirRow) bool { return row.kind == rowChild })
	if offered < 0 {
		offered = len(rows)
	}

	// Trade-off TRADE-49: past rowsShownMost directories the rest are
	// counted, not drawn.
	shown := rows[:min(len(rows), offered+rowsShownMost)]
	first, last := window(p.cursor, len(shown), max(1, room))

	lines := make([]string, 0, last-first+1)
	for index := first; index < last; index++ {
		marker := kit.marks.marker(index == p.cursor)
		lines = append(lines, marker+shown[index].drawn(kit, width-ansi.StringWidth(marker), p.home))
	}

	if more := len(rows) - len(shown); more > 0 {
		lines = append(lines, kit.styles.label.Render(strconv.Itoa(more)+" more match; keep typing to narrow"))
	}

	return lines
}

// status is what stands in for, or follows, the directories listed: that
// they are being read, why they could not be, that there are none, or none
// matching, that the listing stopped short, or why a destination was
// refused.
func (p dirPicker) status(kit renderKit, where string) []string {
	read, found := p.listing()

	switch {
	case p.problem != nil:
		return []string{kit.failureLine(p.problem)}
	case !found || read.asked:
		return []string{kit.marks.turning + " reading " + strings.TrimPrefix(where, "in ") + kit.marks.ellipsis}
	case read.err != nil:
		return []string{kit.failureLine(read.err)}
	case len(workdirs.Matching(read.listing.Entries, p.nameTyped())) == 0 && p.nameTyped() != "":
		return []string{kit.styles.label.Render("nothing " + where + " matches " + sanitize.Line(p.nameTyped()))}
	case len(read.listing.Entries) == 0:
		return []string{kit.styles.label.Render("no directories " + where)}
	case read.listing.Truncated:
		return []string{kit.styles.label.Render("first " + strconv.Itoa(workdirs.ListLimit) +
			" shown; keep typing to narrow")}
	default:
		return nil
	}
}

// drawn is a row in width columns: switching here, a worktree or a favorite
// by its path, or a directory by its name, the letters typed in bold and a
// repository marked in git's hue.
func (r dirRow) drawn(kit renderKit, width int, home string) string {
	switch r.kind {
	case rowHere:
		return "↳ switch here"
	case rowWorktree:
		return shownFrom(home, r.dir) + kit.styles.label.Render("  worktree")
	case rowFavorite:
		return kit.marks.favorite + " " + shownFrom(home, r.dir)
	case rowChild:
	}

	name := r.matchedName(kit)
	if !r.match.Entry.Repository {
		return name
	}

	mark := "repository"

	return name + strings.Repeat(" ", max(1, width-ansi.StringWidth(name)-len(mark))) + kit.styles.git.Render(mark)
}

// matchedName is a directory's name, neutralized, with the letters what was
// typed matched in bold; plain where neutralizing changed its letters.
func (r dirRow) matchedName(kit renderKit) string {
	name := []rune(sanitize.Line(r.match.Entry.Name))
	if len(name) != len([]rune(r.match.Entry.Name)) {
		return string(name)
	}

	var drawn strings.Builder

	for place, letter := range name {
		if slices.Contains(r.match.Letters, place) {
			drawn.WriteString(kit.styles.strong.Render(string(letter)))
		} else {
			drawn.WriteRune(letter)
		}
	}

	return drawn.String()
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
