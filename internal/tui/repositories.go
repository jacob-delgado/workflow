// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// repositoriesState is the Repositories pane: the favorites as last read,
// each with what is there now, and the row the cursor is on, where you work
// first.
type repositoriesState struct {
	favorites []favoritePlace
	err       error
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
// whether it is a favorite and where you work.
type repositoryRow struct {
	dir      string
	place    seams.Place
	err      error
	favorite bool
	here     bool
}

// repositoriesRead is the favorites as the store and the disk answered.
type repositoriesRead struct {
	favorites []favoritePlace
	err       error
}

var _ applier = repositoriesRead{}

// apply keeps the favorites read, the cursor kept on a row there still is.
func (msg repositoriesRead) apply(m Model) (Model, tea.Cmd) {
	m.repositories.favorites, m.repositories.err, m.repositories.read = msg.favorites, msg.err, true
	m.repositories.selected = min(m.repositories.selected, len(m.repositoryRows())-1)

	return m, nil
}

// loadRepositories reads the favorites, then each one's place, off the update
// loop: a favorite is a directory on disk, read again each time.
func (m Model) loadRepositories() tea.Cmd {
	read, look, here := m.deps.Store.Favorites, m.deps.Repositories.Look, m.deps.Repositories.Here.Dir

	return func() tea.Msg {
		if read == nil {
			return repositoriesRead{}
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

		return repositoriesRead{favorites: favorites, err: err}
	}
}

// refreshRepositories reads the favorites again.
func (m Model) refreshRepositories() (Model, tea.Cmd) {
	return m, m.loadRepositories()
}

// repositoryRows are where you work, then every other favorite, as kept.
func (m Model) repositoryRows() []repositoryRow {
	here := repositoryRow{dir: m.deps.Repositories.Here.Dir, place: m.deps.Repositories.Here, here: true}
	rows := []repositoryRow{here}

	for _, favorite := range m.repositories.favorites {
		if favorite.here {
			rows[0].favorite = true

			continue
		}

		rows = append(rows, repositoryRow{dir: favorite.dir, place: favorite.place, err: favorite.err, favorite: true})
	}

	return rows
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
	switch count := len(m.repositoryRows()) - 1; {
	case !m.repositories.read:
		return "favorites, read when opened"
	case count == 1:
		return "1 other favorite"
	default:
		return strconv.Itoa(count) + " other favorites"
	}
}

// repositoriesDetail is where you work, in full, then the favorites, the
// cursor's marked.
func (m Model) repositoriesDetail(width int) string {
	lines := append(m.workingIn(m.deps.Repositories.Here), "", m.styles.strong.Render("Favorites"))
	if m.repositories.err != nil {
		lines = append(lines, m.failureSummary(m.repositories.err))
	}

	for index, row := range m.repositoryRows() {
		lines = append(lines, m.repositoryLine(row, index == m.repositories.selected))
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
func (m Model) repositoryLine(row repositoryRow, selected bool) string {
	mark := strings.Repeat(" ", len([]rune(m.marks.favorite)))
	if row.favorite {
		mark = m.marks.favorite
	}

	return m.marks.marker(selected) + mark + " " + m.shownDir(row.dir) + m.marks.separator + m.repositoryState(row)
}

// repositoryState is what is at a row's directory now.
func (m Model) repositoryState(row repositoryRow) string {
	switch {
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

// repositoriesKeys is what the pane offers.
func (m Model) repositoriesKeys() []key.Binding {
	return []key.Binding{m.keys.up, m.keys.down, m.keys.favoriteDir, m.keys.refresh}
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
