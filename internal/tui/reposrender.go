// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

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

// rail is where you work, and how many favorites there are once read.
func (s repositoriesState) rail(view repositoriesView, rows int) string {
	lines := []string{shownFrom(view.home, view.here.Dir)}
	if rows > 1 {
		lines = append(lines, view.kit.styles.label.Render(s.favoritesCount(view)))
	}

	return strings.Join(lines, "\n")
}

// favoritesCount says how many favorites there are, or that they are read
// when the pane is opened. A read's failures are part of what it found, so
// it never stands failed.
func (s repositoriesState) favoritesCount(view repositoriesView) string {
	counted := func() string {
		count := len(s.rows(view.here)) - 1 - len(s.worktreeRows(view.here.Root))
		if count == 1 {
			return "1 other favorite"
		}

		return strconv.Itoa(count) + " other favorites"
	}

	return map[latest.Stage]func() string{
		latest.StageNeverRead: func() string { return "favorites, read when opened" },
		latest.StageReading:   view.kit.reading,
		latest.StageShown:     counted,
		latest.StageFailed:    counted,
	}[s.read.Stage()]()
}

// detail is where you work, in full, then the other worktrees and the
// favorites, the cursor's row marked.
func (s repositoriesState) detail(view repositoriesView, width int) string {
	kit := view.kit
	rows, worktrees := s.rows(view.here), len(s.worktreeRows(view.here.Root))
	line := func(index int) string { return view.line(rows[index], index == s.selected, width) }

	found, _ := s.read.Value()

	lines := append(view.workingIn(view.here), "", line(0))

	if worktrees > 0 || found.worktreesErr != nil {
		lines = append(lines, "", kit.styles.strong.Render("Worktrees"))
		if found.worktreesErr != nil {
			lines = append(lines, kit.failureSummary(found.worktreesErr))
		}

		for index := 1; index <= worktrees; index++ {
			lines = append(lines, line(index))
		}
	}

	lines = append(lines, "", kit.styles.strong.Render("Favorites"))
	lines = append(lines, s.favoritesNote(view, found, len(rows) == 1+worktrees)...)

	for index := 1 + worktrees; index < len(rows); index++ {
		lines = append(lines, line(index))
	}

	return wrap(strings.Join(lines, "\n"), width)
}

// favoritesNote is what the Favorites heading says before the favorites
// listed, as found has them: that they are being read, why they could not
// be, or, with none, how to mark one. A read's failures are part of what it
// found, so it never stands failed.
func (s repositoriesState) favoritesNote(view repositoriesView, found repositoriesFound, none bool) []string {
	reading := func() []string { return []string{view.kit.reading()} }
	answered := func() []string {
		switch {
		case found.favoritesErr != nil:
			return []string{view.kit.failureSummary(found.favoritesErr)}
		case none:
			return []string{"No favorites yet; " + view.keys.favoriteDir.Help().Key + " marks the directory under the cursor."}
		default:
			return nil
		}
	}

	return map[latest.Stage]func() []string{
		latest.StageNeverRead: reading,
		latest.StageReading:   reading,
		latest.StageShown:     answered,
		latest.StageFailed:    answered,
	}[s.read.Stage()]()
}

// workingIn is a place in full: the directory, the repository it is in and
// the path within, origin — and beside it the base, in a fork's clone — and
// the configuration that applies.
func (v repositoriesView) workingIn(place seams.Place) []string {
	field := func(name, value string) string { return v.kit.styles.label.Render(padded(name, labelWidth)) + value }
	lines := []string{v.kit.styles.strong.Render("Working in"), field("directory", shownFrom(v.home, place.Dir))}

	if place.Root == "" {
		lines = append(lines, field("repository", "not in a repository"))
	} else {
		lines = append(lines, field("repository", shownFrom(v.home, place.Root)), field("within", within(place)))
	}

	lines = append(lines, field("origin", origin(place)))
	if place.Base.Host != "" {
		lines = append(lines, field("base", sanitize.Line(place.Base.Host+"/"+place.Base.Path)))
	}

	return append(lines, field("configuration", shownFiles(v.home, place.Config)))
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

	return sanitize.Line(path)
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
