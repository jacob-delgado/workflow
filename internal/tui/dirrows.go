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
// as the Repositories pane lists them, leaving out where you work and those
// whose directory is gone, as the web's dialog does.
func (m Model) pickerShortcuts() []dirRow {
	rows := m.repositories.rows(m.deps.Repositories.Here)
	shortcuts := make([]dirRow, 0, len(rows))

	for _, row := range rows[1:] {
		if row.err != nil {
			continue
		}

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

// listing is what is known of the directory the field points into, and
// whether any of it has been asked for: the whole listing, or, where the
// rows are drawn from one narrowed by the name typed too, the two joined as
// the web joins them, still being read until both have answered. While the
// narrowed one is out, the last listing of the same directory narrowed by a
// name stands in for it, as the web's last answer does.
func (p dirPicker) listing() (dirRead, bool) {
	prefixes := p.drawnFrom()
	whole, found := p.reads[readKey(p.parent(), prefixes[0])]

	if len(prefixes) == 1 {
		return whole, found
	}

	narrowed, asked := p.reads[readKey(p.parent(), prefixes[1])]
	if !asked || narrowed.asked {
		narrowed = p.standingIn()
	}

	return narrowed.joined(whole), found
}

// standingIn is what stands in for a listing narrowed by a name while it is
// read: the directories the last one of the same directory to answer held.
func (p dirPicker) standingIn() dirRead {
	reading := dirRead{asked: true}
	if p.standIn.dir == p.parent() {
		reading.listing = p.standIn.read.listing
	}

	return reading
}

// joined is a narrowed listing's read with the rest of the whole listing's
// directories after its own, so a directory the whole one matched letter by
// letter stays once the narrowed one answers. It is being read while
// either one is, since the narrowed one may answer first. Where the narrowed
// one failed, it is that failure, listing nothing; where the whole one alone
// failed, it is that failure beside the narrowed one's directories, so a
// hidden one found there can still be gone into.
func (r dirRead) joined(whole dirRead) dirRead {
	r.asked = r.asked || whole.asked
	if r.err != nil {
		return r
	}

	found := make(map[string]bool, len(r.listing.Entries))
	for _, entry := range r.listing.Entries {
		found[entry.Name] = true
	}

	rest := slices.DeleteFunc(slices.Clone(whole.listing.Entries), func(entry workdirs.Entry) bool {
		return found[entry.Name]
	})
	r.listing.Entries = slices.Concat(r.listing.Entries, rest)
	r.err = whole.err

	return r
}

// needs is the prefix of the first listing the rows are drawn from that
// nobody has asked for, in the directory the field points into.
func (p dirPicker) needs() (string, string, bool) {
	dir := p.parent()

	for _, prefix := range p.drawnFrom() {
		if _, asked := p.reads[readKey(dir, prefix)]; !asked {
			return dir, prefix, true
		}
	}

	return "", "", false
}

// drawnFrom are the prefixes of the listings of the directory the field
// points into that its rows are drawn from, in the order they are asked for:
// the whole listing, then the one narrowed by the name typed where that
// holds what the whole one cannot, a hidden directory once the name starts
// with its dot or one past where the whole listing stopped short.
func (p dirPicker) drawnFrom() []string {
	name := p.nameTyped()
	whole := p.reads[readKey(p.parent(), "")]

	if strings.HasPrefix(name, ".") || whole.listing.Truncated && name != "" {
		return []string{"", name}
	}

	return []string{""}
}

// rows are what the picker offers: the worktrees and favorites whose names
// hold what was typed while the field holds no path, then the directories
// where it points that do; and once it holds a path, switching there, ahead
// of them until a name is typed and after them from then on, so the first
// row is always the one Enter takes. Switching there is to the path typed
// whole while no directory listed matches the name it ends in, which is how
// one not listed is reached.
func (p dirPicker) rows() []dirRow {
	children, _ := p.childrenMatching()
	if !p.browsing() {
		return append(p.shortcutsMatching(), children...)
	}

	here := dirRow{kind: rowHere, dir: p.parent()}
	if p.nameTyped() == "" {
		return append([]dirRow{here}, children...)
	}

	if len(children) == 0 {
		here.dir = p.typedPath()
	}

	return append(children, here)
}

// typedPath is the directory the field names, read from where you work, or
// from your home after a ~.
func (p dirPicker) typedPath() string {
	return workdirs.Resolve(p.input.Value(), p.base, p.home)
}

// childrenMatching are the directories listed where the field points whose
// names hold the name typed: at most rowsShownMost of them, and how many more
// match past those. A read that failed lists none of its own; a narrowed one
// joined with a whole listing that failed still lists what it found.
func (p dirPicker) childrenMatching() ([]dirRow, int) {
	read, _ := p.listing()
	matches := workdirs.Matching(read.listing.Entries, p.nameTyped())
	// Trade-off TRADE-49: past rowsShownMost directories the rest are
	// counted, not drawn or reached.
	shown := matches[:min(len(matches), rowsShownMost)]
	children := make([]dirRow, 0, len(shown))

	for _, match := range shown {
		children = append(children, dirRow{kind: rowChild, dir: filepath.Join(p.parent(), match.Entry.Name), match: match})
	}

	return children, len(matches) - len(shown)
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
	p = p.fitted(width)
	where := "in " + shownFrom(p.home, p.parent())
	lines := make([]string, 0, max(0, rows))
	lines = append(lines, drawnField(p.input), p.heading(where, width))
	lines = append(lines, p.drawnRows(kit, width, rows-len(lines)-1)...)
	lines = append(lines, p.status(kit, where)...)

	return "Go to a directory", strings.Join(lines, "\n")
}

// fitted is the picker with its field's line fitted to width columns, less
// the prompt and the cursor at its end, and scrolled to keep the cursor in
// view. The line scrolls as it is typed in, so it is fitted before each key
// as well as before it is drawn, where the screen may since have changed.
func (p dirPicker) fitted(width int) dirPicker {
	p.input.SetWidth(max(1, width-ansi.StringWidth(p.input.Prompt)-1))
	p.input.SetCursor(p.input.Position())

	return p
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

// drawnRows are the rows that fit in room lines, the highlighted one among
// them, and on the last line how many more directories match past the most
// the picker draws.
func (p dirPicker) drawnRows(kit renderKit, width, room int) []string {
	rows := p.rows()

	_, more := p.childrenMatching()
	if more > 0 {
		room--
	}

	first, last := window(p.cursor, len(rows), max(1, room))

	lines := make([]string, 0, last-first+1)
	for index := first; index < last; index++ {
		marker := kit.marks.marker(index == p.cursor)
		lines = append(lines, marker+rows[index].drawn(kit, width-ansi.StringWidth(marker), p.home))
	}

	if more > 0 {
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
		return kit.marks.switchHere + "switch here"
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
