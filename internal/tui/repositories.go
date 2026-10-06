// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// repositoriesState is the Repositories pane: the worktrees and favorites as
// last read, each favorite with what is there now, and the row the cursor is
// on, where you work first.
type repositoriesState struct {
	worktrees    []gitrepo.Worktree
	worktreesErr error
	favorites    []favoritePlace
	err          error
	// read reports the favorites read at least once, as they are not at
	// startup.
	read     bool
	selected int
	scroll   int
}

// favoritePlace is a favorite directory and what is there now: the place, or
// why it could not be read; and whether it is where you work.
type favoritePlace struct {
	dir   string
	place seams.Place
	err   error
	here  bool
}

// repositoryRow is one row of the pane: a directory, what is there, and
// whether it is a favorite and where you work; worktree is set on a row for
// one of the repository's other worktrees.
type repositoryRow struct {
	dir      string
	place    seams.Place
	err      error
	favorite bool
	here     bool
	worktree *gitrepo.Worktree
}

// errWorktreeGone is a worktree git still lists whose directory is gone.
var errWorktreeGone = errors.New("the worktree's directory is gone")

// repositoriesRead is the worktrees as git answered, and the favorites as the
// store and the disk did.
type repositoriesRead struct {
	worktrees    []gitrepo.Worktree
	worktreesErr error
	favorites    []favoritePlace
	err          error
}

var _ applier = repositoriesRead{}

// apply keeps the favorites read, the cursor kept on a row there still is.
func (msg repositoriesRead) apply(m Model) (Model, tea.Cmd) {
	m.repositories.favorites, m.repositories.err, m.repositories.read = msg.favorites, msg.err, true
	m.repositories.worktrees, m.repositories.worktreesErr = msg.worktrees, msg.worktreesErr
	m.repositories.selected = min(m.repositories.selected, len(m.repositoryRows())-1)

	return m, nil
}

// loadRepositories reads the worktrees, then the favorites and each one's
// place, off the update loop: both are on disk, read again each time.
func (m Model) loadRepositories() tea.Cmd {
	worktrees, favorites := m.deps.Repositories.Worktrees, m.favoritePlaces()

	return func() tea.Msg {
		read := repositoriesRead{}
		if worktrees != nil {
			read.worktrees, read.worktreesErr = worktrees()
		}

		read.favorites, read.err = favorites()

		return read
	}
}

// favoritePlaces reads the favorites, then each one's place.
func (m Model) favoritePlaces() func() ([]favoritePlace, error) {
	read, look, here := m.deps.Store.Favorites, m.deps.Repositories.Look, m.deps.Repositories.Here.Dir

	return func() ([]favoritePlace, error) {
		if read == nil {
			return nil, nil
		}

		dirs, err := read()
		favorites := make([]favoritePlace, 0, len(dirs))

		for _, dir := range dirs {
			favorite := favoritePlace{dir: dir, here: dir == here || workdirs.Same(dir, here)}
			if look != nil {
				favorite.place, favorite.err = look(dir)
			}

			favorites = append(favorites, favorite)
		}

		return favorites, err
	}
}

// refreshRepositories reads the favorites again.
func (m Model) refreshRepositories() (Model, tea.Cmd) {
	return m, m.loadRepositories()
}

// repositoryRows are where you work, then the repository's other worktrees
// as git lists them, then every other favorite, as kept.
func (m Model) repositoryRows() []repositoryRow {
	here := repositoryRow{dir: m.deps.Repositories.Here.Dir, place: m.deps.Repositories.Here, here: true}
	rows := append([]repositoryRow{here}, m.worktreeRows()...)

	for _, favorite := range m.repositories.favorites {
		if m.isOtherWorktree(favorite.dir) {
			// Listed, starred, among the worktrees.
			continue
		}

		if favorite.here {
			// Kept by the name it was marked under, which may be another
			// way to reach where you work; that is the name to forget.
			rows[0].favorite, rows[0].dir = true, favorite.dir

			continue
		}

		rows = append(rows, repositoryRow{dir: favorite.dir, place: favorite.place, err: favorite.err, favorite: true})
	}

	return rows
}

// worktreeRows are the repository's worktrees but the one where you work, each
// a favorite when one is kept by its directory.
func (m Model) worktreeRows() []repositoryRow {
	var rows []repositoryRow

	for at, worktree := range m.repositories.worktrees {
		if worktree.Dir == m.deps.Repositories.Here.Root {
			continue
		}

		row := repositoryRow{dir: worktree.Dir, worktree: &m.repositories.worktrees[at], favorite: m.isFavorite(worktree.Dir)}
		if worktree.Missing {
			row.err = errWorktreeGone
		}

		rows = append(rows, row)
	}

	return rows
}

// isOtherWorktree reports dir one of the worktrees listed beside where you
// work.
func (m Model) isOtherWorktree(dir string) bool {
	return dir != m.deps.Repositories.Here.Root &&
		slices.ContainsFunc(m.repositories.worktrees, func(worktree gitrepo.Worktree) bool { return worktree.Dir == dir })
}

// isFavorite reports a favorite kept by exactly dir.
func (m Model) isFavorite(dir string) bool {
	return slices.ContainsFunc(m.repositories.favorites, func(favorite favoritePlace) bool { return favorite.dir == dir })
}

// selectedRepository is the row the cursor is on.
func (m Model) selectedRepository() repositoryRow {
	rows := m.repositoryRows()

	return rows[min(max(0, m.repositories.selected), len(rows)-1)]
}

// shownDir is a directory written from your home, with anything in its name
// that could drive the terminal neutralized: it is read from a file on disk.
func (m Model) shownDir(dir string) string {
	return sanitize.Line(workdirs.Shown(dir, m.deps.Repositories.Home))
}

// repositoriesRail is where you work, and how many favorites there are once
// read.
func (m Model) repositoriesRail(rows int) string {
	lines := []string{m.shownDir(m.deps.Repositories.Here.Dir)}
	if rows > 1 {
		lines = append(lines, m.styles.label.Render(m.favoritesCount()))
	}

	return strings.Join(lines, "\n")
}

// favoritesCount says how many favorites there are, or that they are read
// when the pane is opened.
func (m Model) favoritesCount() string {
	switch count := len(m.repositoryRows()) - 1 - len(m.worktreeRows()); {
	case !m.repositories.read:
		return "favorites, read when opened"
	case count == 1:
		return "1 other favorite"
	default:
		return strconv.Itoa(count) + " other favorites"
	}
}

// repositoriesDetail is where you work, in full, then the other worktrees and
// the favorites, the cursor's row marked.
func (m Model) repositoriesDetail(width int) string {
	rows, worktrees := m.repositoryRows(), len(m.worktreeRows())
	line := func(index int) string { return m.repositoryLine(rows[index], index == m.repositories.selected, width) }

	lines := append(m.workingIn(m.deps.Repositories.Here), "", line(0))

	if worktrees > 0 || m.repositories.worktreesErr != nil {
		lines = append(lines, "", m.styles.strong.Render("Worktrees"))
		if m.repositories.worktreesErr != nil {
			lines = append(lines, m.failureSummary(m.repositories.worktreesErr))
		}

		for index := 1; index <= worktrees; index++ {
			lines = append(lines, line(index))
		}
	}

	lines = append(lines, "", m.styles.strong.Render("Favorites"))

	switch {
	case m.repositories.err != nil:
		lines = append(lines, m.failureSummary(m.repositories.err))
	case !m.repositories.read:
		lines = append(lines, "reading"+m.marks.ellipsis)
	case len(rows) == 1+worktrees:
		lines = append(lines, "No favorites yet; "+m.keys.favoriteDir.Help().Key+" marks the directory under the cursor.")
	}

	for index := 1 + worktrees; index < len(rows); index++ {
		lines = append(lines, line(index))
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// workingIn is a place in full: the directory, the repository it is in and
// the path within, origin, and the configuration that applies.
func (m Model) workingIn(place seams.Place) []string {
	field := func(name, value string) string { return m.styles.label.Render(padded(name, labelWidth)) + value }
	lines := []string{m.styles.strong.Render("Working in"), field("directory", m.shownDir(place.Dir))}

	if place.Root == "" {
		lines = append(lines, field("repository", "not in a repository"))
	} else {
		lines = append(lines, field("repository", m.shownDir(place.Root)), field("within", within(place)))
	}

	return append(lines, field("origin", origin(place)), field("configuration", m.shownFiles(place.Config)))
}

// labelWidth is how wide the Working in block's names are drawn, the longest
// and a space.
const labelWidth = len("configuration ")

// within is the path from a place's repository root to its directory.
func within(place seams.Place) string {
	path, err := filepath.Rel(place.Root, place.Dir)
	if err != nil || path == "." {
		return "the repository's root"
	}

	return sanitize.Line(filepath.ToSlash(path))
}

// origin is a place's origin as a forge host and path.
func origin(place seams.Place) string {
	if place.Remote.Host == "" {
		return "none"
	}

	return sanitize.Line(place.Remote.Host + "/" + place.Remote.Path)
}

// shownFiles is the configuration files that apply, each written from your
// home, the repository's over your home's.
func (m Model) shownFiles(files config.Files) string {
	if files == (config.Files{}) {
		return "none, so the defaults apply"
	}

	return config.Files{Repo: m.shownDir(files.Repo), Home: m.shownDir(files.Home)}.String()
}

// repositoryLine is one row: the cursor, whether it is a favorite, the
// directory, and what is there.
func (m Model) repositoryLine(row repositoryRow, selected bool, width int) string {
	mark := strings.Repeat(" ", len([]rune(m.marks.favorite)))
	if row.favorite {
		mark = m.marks.favorite
	}

	lead := m.marks.marker(selected) + mark + " "
	dir := cutMiddle(m.shownDir(row.dir), width-ansi.StringWidth(lead), m.marks.ellipsis)

	return lead + dir + m.marks.separator + m.repositoryState(row)
}

// cutMiddle shortens a path wider than width in its middle, keeping its root
// and its last element — "~/src/…/feature-x" — so the row it heads stays one
// line; the full path is in the Working in block. A last element too wide on
// its own keeps its end.
func cutMiddle(path string, width int, ellipsis string) string {
	if ansi.StringWidth(path) <= width {
		return path
	}

	parts := strings.Split(path, "/")
	leading, last := parts[:len(parts)-1], parts[len(parts)-1]

	for kept := len(leading) - 1; kept >= 1; kept-- {
		cut := strings.Join(leading[:kept], "/") + "/" + ellipsis + "/" + last
		if ansi.StringWidth(cut) <= width {
			return cut
		}
	}

	room := max(0, width-ansi.StringWidth(ellipsis))

	return ellipsis + ansi.TruncateLeft(last, max(0, ansi.StringWidth(last)-room), "")
}

// repositoryState is what is at a row's directory now.
func (m Model) repositoryState(row repositoryRow) string {
	switch {
	case row.worktree != nil:
		return worktreeState(*row.worktree)
	case row.here:
		return "where you work"
	case row.err != nil:
		return "not there"
	case row.place.Root == "":
		return "not a repository"
	case row.place.Remote.Host == "":
		return "repository"
	default:
		return "repository " + origin(row.place)
	}
}

// worktreeState is what a worktree has checked out, and whether git keeps it
// or finds it gone; its branch is anyone's to name, so neutralized.
func worktreeState(worktree gitrepo.Worktree) string {
	state := "worktree on " + sanitize.Line(worktree.Branch)

	switch {
	case worktree.Missing:
		return "worktree gone"
	case worktree.Detached:
		state = "worktree at " + sanitize.Line(worktree.ShortHead())
	}

	if worktree.Locked {
		state += ", locked"
	}

	return state
}

// repositoriesKeys is what the pane offers.
func (m Model) repositoriesKeys() []key.Binding {
	return []key.Binding{
		m.keys.up, m.keys.down, relabel(m.keys.confirm, verbSwitch), m.keys.favoriteDir, m.keys.goToDir, m.keys.refresh,
	}
}

// handleRepositoriesKey moves the cursor, or marks or forgets a favorite.
func (m Model) handleRepositoriesKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.up):
		m.repositories.selected = max(0, m.repositories.selected-1)
	case key.Matches(msg, m.keys.down):
		m.repositories.selected = min(len(m.repositoryRows())-1, m.repositories.selected+1)
	case key.Matches(msg, m.keys.favoriteDir):
		return m.toggleFavorite(m.selectedRepository())
	case key.Matches(msg, m.keys.confirm):
		return m.switchToSelected()
	case key.Matches(msg, m.keys.goToDir):
		return m.openDirPrompt()
	case key.Matches(msg, m.keys.refresh):
		return m.refreshRepositories()
	}

	return m, nil
}

// favoriteToggled is a favorite marked or forgotten, or why it was not.
type favoriteToggled struct {
	dir   string
	added bool
	err   error
}

var _ applier = favoriteToggled{}

// apply says what changed and reads the favorites again.
func (msg favoriteToggled) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return m.noticedFailure(msg.err), nil
	}

	words := " removed " + m.shownDir(msg.dir) + " from favorites"
	if msg.added {
		words = " added " + m.shownDir(msg.dir) + " to favorites"
	}

	return m.noticed(m.marks.done + words), m.loadRepositories()
}

// toggleFavorite forgets a row that is a favorite, and marks one that is not.
func (m Model) toggleFavorite(row repositoryRow) (Model, tea.Cmd) {
	change, verb := m.deps.Store.Favor, "add "+m.shownDir(row.dir)+" to favorites"
	if row.favorite {
		change, verb = m.deps.Store.Unfavor, "remove "+m.shownDir(row.dir)+" from favorites"
	}

	switch {
	case m.dryRun:
		return m.noticed("dry run: would " + verb), nil
	case change == nil:
		return m.noticed("favorites are not kept: the store is turned off"), nil
	}

	return m, func() tea.Msg { return favoriteToggled{dir: row.dir, added: !row.favorite, err: change(row.dir)} }
}
