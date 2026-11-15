// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"errors"
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// repositoriesState is the Repositories pane: the worktrees and favorites as
// the latest read found them, each favorite with what is there now, and the
// row the cursor is on, where you work first. They are not read at startup,
// but when the pane is first looked at.
type repositoriesState struct {
	read     latest.Read[repositoriesFound]
	selected int
	scroll   int
	// changing counts a favorite marked or forgotten and not yet answered:
	// until it is, the rows show the favorites as before it, so another f
	// would make the same change again rather than undo it, and f waits.
	changing inFlight
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

// repositoriesFound is what one read found: the worktrees as git answered,
// and the favorites as the store and the disk did, each beside why it could
// not be read. Either can fail while the other lists, so a read always
// answers, its failures part of what it found.
type repositoriesFound struct {
	worktrees    []gitrepo.Worktree
	worktreesErr error
	favorites    []favoritePlace
	favoritesErr error
}

// repositoriesRead is what the read begun in round found.
type repositoriesRead struct {
	round latest.Round
	found repositoriesFound
}

var _ applier = repositoriesRead{}

// apply keeps what the read found, the cursor kept on a row there still is,
// unless a read was begun after this one: one begun before a favorite
// changed, or a worktree was made, would show them as they were.
func (msg repositoriesRead) apply(m Model) (Model, tea.Cmd) {
	read, taken := m.repositories.read.Answered(msg.round, msg.found, nil)
	if !taken {
		return m, nil
	}

	m.repositories.read = read
	m.repositories.selected = min(m.repositories.selected, len(m.repositories.rows(m.deps.Repositories.Here))-1)

	return m, nil
}

// found is what the latest read found; nothing before one has answered.
func (s repositoriesState) found() repositoriesFound {
	found, _ := s.read.Value()

	return found
}

// loadRepositories reads, as the read begun in round, the worktrees, then
// the favorites and each one's place, off the update loop: both are on disk,
// read again each time.
func (m Model) loadRepositories(round latest.Round) tea.Cmd {
	ctx, worktrees, favorites := m.session, m.deps.Repositories.Worktrees, favoritePlaces(m.session, m.deps)

	return func() tea.Msg {
		read := repositoriesRead{round: round}
		if worktrees != nil {
			read.found.worktrees, read.found.worktreesErr = worktrees(ctx)
		}

		read.found.favorites, read.found.favoritesErr = favorites()

		return read
	}
}

// favoritePlaces reads the favorites, under ctx, then each one's place.
func favoritePlaces(ctx context.Context, deps Deps) func() ([]favoritePlace, error) {
	read, look, here := deps.Store.Favorites, deps.Repositories.Look, deps.Repositories.Here.Dir

	return func() ([]favoritePlace, error) {
		if read == nil {
			return nil, nil
		}

		dirs, err := read(ctx)
		favorites := make([]favoritePlace, 0, len(dirs))

		for _, dir := range dirs {
			favorite := favoritePlace{dir: dir, here: dir == here || workdirs.Same(dir, here)}
			if look != nil {
				favorite.place, favorite.err = look(ctx, dir)
			}

			favorites = append(favorites, favorite)
		}

		return favorites, err
	}
}

// readingRepositories begins a read of the worktrees and the favorites,
// which every read begun before it gives way to, and is the command that
// reads them.
func (m Model) readingRepositories() (Model, tea.Cmd) {
	read, round := m.repositories.read.Begin()
	m.repositories.read = read

	return m, m.loadRepositories(round)
}

// readingRepositoriesAgain begins a read of the worktrees and the favorites
// once something has changed them, unless the pane has never been read: it
// is read when first looked at, and that read finds the change.
func (m Model) readingRepositoriesAgain() (Model, tea.Cmd) {
	if m.repositories.read.Stage() == latest.StageNeverRead {
		return m, nil
	}

	return m.readingRepositories()
}

// rows are where you work, then the repository's other worktrees as git
// lists them, then every other favorite, as kept.
func (s repositoriesState) rows(here seams.Place) []repositoryRow {
	rows := append([]repositoryRow{{dir: here.Dir, place: here, here: true}}, s.worktreeRows(here.Root)...)

	for _, favorite := range s.found().favorites {
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

	worktrees := s.found().worktrees
	for at, worktree := range worktrees {
		if worktree.Dir == hereRoot {
			continue
		}

		row := repositoryRow{dir: worktree.Dir, worktree: &worktrees[at], favorite: s.isFavorite(worktree.Dir)}
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
		slices.ContainsFunc(s.found().worktrees, func(worktree gitrepo.Worktree) bool { return worktree.Dir == dir })
}

// isFavorite reports a favorite kept by exactly dir.
func (s repositoriesState) isFavorite(dir string) bool {
	return slices.ContainsFunc(s.found().favorites, func(favorite favoritePlace) bool { return favorite.dir == dir })
}

// selectedRow is the row the cursor is on.
func (s repositoriesState) selectedRow(here seams.Place) repositoryRow {
	rows := s.rows(here)

	return rows[min(max(0, s.selected), len(rows)-1)]
}

// repositoriesKeys is what the pane offers.
func (m Model) repositoriesKeys() []key.Binding {
	keys := []key.Binding{m.keys.up, m.keys.down, m.keys.switchDir, m.keys.favoriteDir, m.keys.goToDir}
	switch {
	case m.offersSetup():
		keys = append(keys, relabel(m.keys.settings, "set up"))
	case canEditSettings(m.deps):
		keys = append(keys, m.keys.settings)
	}

	if canSeeLocalData(m.deps) {
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

// handleRepositoriesKey moves the cursor, switches to the row it is on, marks
// or forgets a favorite, or opens what the pane offers. Apply switches nothing:
// it confirms the switch's last look, so a double press would ask and then
// switch.
func (m Model) handleRepositoriesKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.up, m.keys.down):
		m.repositories = m.repositories.movedBy(m.keys.stepOf(msg), m.deps.Repositories.Here)

		return m, nil
	case key.Matches(msg, m.keys.favoriteDir):
		return m.toggleFavorite(m.repositories.selectedRow(m.deps.Repositories.Here))
	case key.Matches(msg, m.keys.switchDir):
		return m.switchToSelected()
	case key.Matches(msg, m.keys.refresh):
		return m.readingRepositories()
	default:
		return m.openFromRepositories(msg)
	}
}

// openFromRepositories opens what the Repositories pane offers: the go-to
// prompt, Settings or Local data.
func (m Model) openFromRepositories(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.goToDir):
		return m.openDirPicker()
	case key.Matches(msg, m.keys.settings) && m.offersSetup():
		return m.openSetup()
	case key.Matches(msg, m.keys.settings) && canEditSettings(m.deps):
		return m.openSettings()
	case key.Matches(msg, m.keys.localData) && canSeeLocalData(m.deps):
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
	m.repositories.changing = m.repositories.changing.answered()

	if msg.err != nil {
		return m.noticedFailure(msg.err), nil
	}

	words := " removed " + shownDir(m.deps, msg.dir) + " from favorites"
	if msg.added {
		words = " added " + shownDir(m.deps, msg.dir) + " to favorites"
	}

	return m.noticed(m.marks.done + words).readingRepositories()
}

// favoriteChanging is why f waits: a favorite changed is shown only once
// the change answers.
const favoriteChanging = "a favorite is still being changed; try again once it is"

// favoritesReading is why f waits once a change has answered: the rows show
// the favorites as the read before it found them until the read after it
// answers, so f would make the same change again rather than undo it.
const favoritesReading = "the favorites are being read; try again once they are"

// toggleFavorite forgets a row that is a favorite, and marks one that is not,
// once no other change of the favorites is on its way and no read of them is
// out.
func (m Model) toggleFavorite(row repositoryRow) (Model, tea.Cmd) {
	change, verb := m.deps.Store.Favor, "add "+shownDir(m.deps, row.dir)+" to favorites"
	if row.favorite {
		change, verb = m.deps.Store.Unfavor, "remove "+shownDir(m.deps, row.dir)+" from favorites"
	}

	switch {
	case m.dryRun:
		return m.noticed("dry run: would " + verb), nil
	case change == nil:
		return m.noticed("favorites are not kept: the store is turned off"), nil
	case m.repositories.changing.out():
		return m.noticed(favoriteChanging), nil
	case m.repositories.read.Out():
		return m.noticed(favoritesReading), nil
	}

	ctx := m.session
	m.repositories.changing = m.repositories.changing.sent()

	return m, func() tea.Msg { return favoriteToggled{dir: row.dir, added: !row.favorite, err: change(ctx, row.dir)} }
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
		refresh: Model.readingRepositories,
		loading: func(m Model) bool { return m.repositories.read.Refreshing() },
		scroll:  func(m *Model) *int { return &m.repositories.scroll }, listInDetail: true,
		answers: []string{
			actionSwitchDirectory, "favorite-directory", "go-to-directory", "settings", "local-data", actionRefresh,
		},
	}
}
