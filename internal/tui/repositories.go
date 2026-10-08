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
	// startup; loading, a read begun and not yet answered.
	read     bool
	loading  bool
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
	m.repositories.loading = false
	m.repositories.worktrees, m.repositories.worktreesErr = msg.worktrees, msg.worktreesErr
	m.repositories.selected = min(m.repositories.selected, len(m.repositories.rows(m.deps.Repositories.Here))-1)

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
	m.repositories.loading = true

	return m, m.loadRepositories()
}

// repositoriesView is what the Repositories pane draws with beside its own
// state: the glyphs and styles, the keys it names, where you work, and your
// home, which every directory is written from.
type repositoriesView struct {
	kit  renderKit
	keys keyMap
	here seams.Place
	home string
}

// repositoriesView is the Repositories pane's view of the rest of the
// interface.
func (m Model) repositoriesView() repositoriesView {
	return repositoriesView{
		kit: m.kit(), keys: m.keys, here: m.deps.Repositories.Here, home: m.deps.Repositories.Home,
	}
}

// rows are where you work, then the repository's other worktrees as git
// lists them, then every other favorite, as kept.
func (s repositoriesState) rows(here seams.Place) []repositoryRow {
	rows := append([]repositoryRow{{dir: here.Dir, place: here, here: true}}, s.worktreeRows(here.Root)...)

	for _, favorite := range s.favorites {
		if s.isOtherWorktree(favorite.dir, here.Root) {
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

// worktreeRows are the repository's worktrees but the one where you work, at
// hereRoot, each a favorite when one is kept by its directory.
func (s repositoriesState) worktreeRows(hereRoot string) []repositoryRow {
	var rows []repositoryRow

	for at, worktree := range s.worktrees {
		if worktree.Dir == hereRoot {
			continue
		}

		row := repositoryRow{dir: worktree.Dir, worktree: &s.worktrees[at], favorite: s.isFavorite(worktree.Dir)}
		if worktree.Missing {
			row.err = errWorktreeGone
		}

		rows = append(rows, row)
	}

	return rows
}

// isOtherWorktree reports dir one of the worktrees listed beside where you
// work, at hereRoot.
func (s repositoriesState) isOtherWorktree(dir, hereRoot string) bool {
	return dir != hereRoot &&
		slices.ContainsFunc(s.worktrees, func(worktree gitrepo.Worktree) bool { return worktree.Dir == dir })
}

// isFavorite reports a favorite kept by exactly dir.
func (s repositoriesState) isFavorite(dir string) bool {
	return slices.ContainsFunc(s.favorites, func(favorite favoritePlace) bool { return favorite.dir == dir })
}

// selectedRow is the row the cursor is on.
func (s repositoriesState) selectedRow(here seams.Place) repositoryRow {
	rows := s.rows(here)

	return rows[min(max(0, s.selected), len(rows)-1)]
}

// shownDir is a directory written from your home, with anything in its name
// that could drive the terminal neutralized: it is read from a file on disk.
func (m Model) shownDir(dir string) string {
	return shownFrom(m.deps.Repositories.Home, dir)
}

// shownFrom is dir written from home, neutralized.
func shownFrom(home, dir string) string {
	return sanitize.Line(workdirs.Shown(dir, home))
}

// rail is where you work, and how many favorites there are once read.
func (s repositoriesState) rail(view repositoriesView, rows int) string {
	lines := []string{shownFrom(view.home, view.here.Dir)}
	if rows > 1 {
		lines = append(lines, view.kit.styles.label.Render(s.favoritesCount(view)))
	}

	return strings.Join(lines, "\n")
}

// favoritesCount says how many favorites there are, or that they are read
// when the pane is opened.
func (s repositoriesState) favoritesCount(view repositoriesView) string {
	switch count := len(s.rows(view.here)) - 1 - len(s.worktreeRows(view.here.Root)); {
	case !s.read && s.loading:
		return view.kit.marks.reading()
	case !s.read:
		return "favorites, read when opened"
	case count == 1:
		return "1 other favorite"
	default:
		return strconv.Itoa(count) + " other favorites"
	}
}

// detail is where you work, in full, then the other worktrees and the
// favorites, the cursor's row marked.
func (s repositoriesState) detail(view repositoriesView, width int) string {
	kit := view.kit
	rows, worktrees := s.rows(view.here), len(s.worktreeRows(view.here.Root))
	line := func(index int) string { return view.line(rows[index], index == s.selected, width) }

	lines := append(view.workingIn(view.here), "", line(0))

	if worktrees > 0 || s.worktreesErr != nil {
		lines = append(lines, "", kit.styles.strong.Render("Worktrees"))
		if s.worktreesErr != nil {
			lines = append(lines, kit.failureSummary(s.worktreesErr))
		}

		for index := 1; index <= worktrees; index++ {
			lines = append(lines, line(index))
		}
	}

	lines = append(lines, "", kit.styles.strong.Render("Favorites"))

	switch {
	case s.err != nil:
		lines = append(lines, kit.failureSummary(s.err))
	case !s.read:
		lines = append(lines, kit.marks.reading())
	case len(rows) == 1+worktrees:
		lines = append(lines, "No favorites yet; "+view.keys.favoriteDir.Help().Key+" marks the directory under the cursor.")
	}

	for index := 1 + worktrees; index < len(rows); index++ {
		lines = append(lines, line(index))
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// workingIn is a place in full: the directory, the repository it is in and
// the path within, origin, and the configuration that applies.
func (v repositoriesView) workingIn(place seams.Place) []string {
	field := func(name, value string) string { return v.kit.styles.label.Render(padded(name, labelWidth)) + value }
	lines := []string{v.kit.styles.strong.Render("Working in"), field("directory", shownFrom(v.home, place.Dir))}

	if place.Root == "" {
		lines = append(lines, field("repository", "not in a repository"))
	} else {
		lines = append(lines, field("repository", shownFrom(v.home, place.Root)), field("within", within(place)))
	}

	return append(lines, field("origin", origin(place)), field("configuration", shownFiles(v.home, place.Config)))
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

// shownFiles is the configuration files that apply, each written from home,
// the repository's over home's.
func shownFiles(home string, files config.Files) string {
	if files == (config.Files{}) {
		return "none, so the defaults apply"
	}

	return config.Files{Repo: shownFrom(home, files.Repo), Home: shownFrom(home, files.Home)}.String()
}

// line is one row: the cursor, whether it is a favorite, the directory, and
// what is there.
func (v repositoriesView) line(row repositoryRow, selected bool, width int) string {
	marks := v.kit.marks

	mark := strings.Repeat(" ", len([]rune(marks.favorite)))
	if row.favorite {
		mark = marks.favorite
	}

	lead := marks.marker(selected) + mark + " "
	dir := cutMiddle(shownFrom(v.home, row.dir), width-ansi.StringWidth(lead), marks.ellipsis)

	return lead + dir + marks.separator + repositoryState(row)
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
func repositoryState(row repositoryRow) string {
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
	keys := []key.Binding{m.keys.up, m.keys.down, relabel(m.keys.confirm, verbSwitch), m.keys.favoriteDir, m.keys.goToDir}
	switch {
	case m.offersSetup():
		keys = append(keys, relabel(m.keys.settings, "set up"))
	case m.canEditSettings():
		keys = append(keys, m.keys.settings)
	}

	if m.canSeeLocalData() {
		keys = append(keys, m.keys.localData)
	}

	return append(keys, m.keys.refresh)
}

// movedBy moves the cursor delta rows down, or up for a negative delta,
// stopping at either end.
func (s repositoriesState) movedBy(delta int, here seams.Place) repositoriesState {
	s.selected = max(0, min(s.selected+delta, len(s.rows(here))-1))

	return s
}

// handleRepositoriesKey moves the cursor, marks or forgets a favorite, or
// opens what the pane offers.
func (m Model) handleRepositoriesKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.up, m.keys.down):
		m.repositories = m.repositories.movedBy(m.keys.stepOf(msg), m.deps.Repositories.Here)

		return m, nil
	case key.Matches(msg, m.keys.favoriteDir):
		return m.toggleFavorite(m.repositories.selectedRow(m.deps.Repositories.Here))
	case key.Matches(msg, m.keys.confirm):
		return m.switchToSelected()
	case key.Matches(msg, m.keys.refresh):
		return m.refreshRepositories()
	default:
		return m.openFromRepositories(msg)
	}
}

// openFromRepositories opens what the Repositories pane offers: the go-to
// prompt, Settings or Local data.
func (m Model) openFromRepositories(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.goToDir):
		return m.openDirPrompt()
	case key.Matches(msg, m.keys.settings) && m.offersSetup():
		return m.openSetup()
	case key.Matches(msg, m.keys.settings) && m.canEditSettings():
		return m.openSettings()
	case key.Matches(msg, m.keys.localData) && m.canSeeLocalData():
		return m.openLocalData()
	default:
		return m, nil
	}
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

// repositoriesBehavior is the Repositories pane's behavior.
func repositoriesBehavior() behavior {
	return behavior{
		rail:   func(m Model, rows int) string { return m.repositories.rail(m.repositoriesView(), rows) },
		detail: func(m Model, width int) string { return m.repositories.detail(m.repositoriesView(), width) },
		keys:   Model.repositoriesKeys, handle: Model.handleRepositoriesKey, pick: nil, narrow: nil,
		move: func(m Model, delta int) (Model, tea.Cmd) {
			m.repositories = m.repositories.movedBy(delta, m.deps.Repositories.Here)

			return m, nil
		},
		refresh: Model.refreshRepositories,
		loading: func(m Model) bool { return m.repositories.loading },
		scroll:  func(m *Model) *int { return &m.repositories.scroll }, listInDetail: true,
		answers: []string{"favorite-directory", "go-to-directory", "settings", "local-data", actionRefresh},
	}
}
