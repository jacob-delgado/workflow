// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

// localDataAbout is what Local data holds, said before the files are listed.
const localDataAbout = "workflow keeps a cache a session makes again, and the people and group " +
	"associations you decided, and your favorites, in two files on this machine."

// localData is the Local data overlay: where workflow keeps what it learns
// between sessions, each database file with its size and what it holds, and
// the two removals workflow db-clean makes, each behind a last look, since
// neither can be undone.
type localData struct {
	opened int
	// listing is the store's files, read once as the overlay opens.
	listing latest.Read[storeFiles]
}

// storeFiles is where the store is, written from your home, and the files
// in it.
type storeFiles struct {
	dir   string
	files []store.DataFile
}

var _ opened = localData{}

// canSeeLocalData reports that the store's files can be listed.
func canSeeLocalData(deps Deps) bool {
	return deps.Settings.LocalData != nil
}

// openLocalData opens Local data and starts reading the store's files.
func (m Model) openLocalData() (Model, tea.Cmd) {
	m, opened := m.nextRound()
	m.overlay = localData{opened: opened}

	return m, readLocalData(m.session, m.deps, opened)
}

// readLocalData lists the store's files for the overlay opened as opened.
func readLocalData(ctx context.Context, deps Deps, opened int) tea.Cmd {
	list := deps.Settings.LocalData

	return func() tea.Msg {
		dir, files, err := list(ctx)

		return localDataRead{opened: opened, dir: dir, files: files, err: err}
	}
}

// localDataRead is the store's files, or why they could not be listed.
type localDataRead struct {
	opened int
	dir    string
	files  []store.DataFile
	err    error
}

var _ applier = localDataRead{}

// apply fills in the overlay that asked, and nothing once it has closed.
func (msg localDataRead) apply(m Model) (Model, tea.Cmd) {
	open, isOpen := answerFor[localData](m, msg.opened)
	if !isOpen {
		return m, nil
	}

	// Local data reads its files once, as it opens, before it could begin a
	// round: the listing is round 0's.
	open.listing, _ = open.listing.Answered(0, storeFiles{dir: shownDir(m.deps, msg.dir), files: msg.files}, msg.err)

	return m.withAnswered(open), nil
}

// view lists where the store is and each file in it, or why it could not,
// as the listing stands.
func (d localData) view(kit renderKit, width, _ int) (string, string) {
	failed := func() []string { return []string{kit.failureBlock(d.listing.Err(), width)} }
	lines := append([]string{wrap(localDataAbout, width), ""}, drawRead(d.listing, drawnRead[[]string]{
		reading:    func() []string { return []string{"reading the local data" + kit.marks.ellipsis} },
		shown:      func() []string { return d.listed(width) },
		failed:     failed,
		failedOver: failed,
	})...)

	return "Local data", strings.Join(lines, "\n")
}

// listed is where the store is and each file in it, or that it holds none.
func (d localData) listed(width int) []string {
	listing, _ := d.listing.Value()
	if len(listing.files) == 0 {
		return []string{"Kept in " + listing.dir, "", "No local data: there is nothing to remove."}
	}

	return append([]string{"Kept in " + listing.dir, ""}, d.fileLines(width)...)
}

// files is the store's files, as listed; none until they are.
func (d localData) files() []store.DataFile {
	listing, _ := d.listing.Value()

	return listing.files
}

// fileLines is a line per file — its name, kind and size — with what it holds
// below it.
func (d localData) fileLines(width int) []string {
	const linesPerFile = 2

	lines := make([]string, 0, linesPerFile*len(d.files()))

	for _, file := range d.files() {
		lines = append(lines,
			fmt.Sprintf("%-12s %-6s %10s", file.Name, file.Kind, store.HumanBytes(file.Bytes)),
			wrap("  "+holdings(file.Holds), width))
	}

	return lines
}

// holdings says what a file holds, or that it could not be read as a
// database.
func holdings(holds []store.Held) string {
	if len(holds) == 0 {
		return "not readable as a database"
	}

	parts := make([]string, 0, len(holds))
	for _, held := range holds {
		parts = append(parts, held.What+": "+strconv.Itoa(held.Count))
	}

	return strings.Join(parts, ", ")
}

// footer offers each removal there is something for, another read after a
// failed one, and closing.
func (d localData) footer(keys keyMap) []key.Binding {
	var offered []key.Binding

	if d.cacheNames() != "" {
		offered = append(offered, keys.removeCache)
	}

	if len(d.files()) > 0 {
		offered = append(offered, keys.removeAll)
	}

	if d.listing.Stage() == latest.StageFailed {
		offered = append(offered, relabel(keys.refresh, "try again"))
	}

	return append(offered, relabel(keys.closeOverlay, escClose))
}

// handleKey answers a key while Local data has the keyboard.
func (d localData) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.refresh) && d.listing.Stage() == latest.StageFailed:
		return m.openLocalData()
	case key.Matches(msg, m.keys.removeCache) && d.cacheNames() != "":
		return d.askToRemove(m, store.CleanCache), nil
	case key.Matches(msg, m.keys.removeAll) && len(d.files()) > 0:
		return d.askToRemove(m, store.CleanAll), nil
	default:
		return m, nil
	}
}

// which names Local data.
func (localData) which() overlayKind { return overlayLocalData }

// openedAs is the round Local data opened in.
func (d localData) openedAs() int { return d.opened }

// cacheNames names the cache's file, or is empty when there is none.
func (d localData) cacheNames() string {
	return d.reaches(store.CleanCache)
}

// reaches names the files a removal of scope reaches, joined by "and".
func (d localData) reaches(scope store.CleanScope) string {
	var names []string

	for _, file := range d.files() {
		if file.Kind == store.DataCache || scope == store.CleanAll {
			names = append(names, file.Name)
		}
	}

	return strings.Join(names, " and ")
}

// askToRemove holds a removal of scope for a last look, saying what goes with
// it and that it cannot be undone.
func (d localData) askToRemove(m Model, scope store.CleanScope) Model {
	names := d.reaches(scope)
	look := lastLook{
		title: "Remove local data", verb: "remove", doing: "removing",
		body: "Remove " + names + "?\n\n" + scope.Consequence() + " It cannot be undone.",
	}
	look.proceed = func(m Model) (Model, tea.Cmd) {
		if m.dryRun {
			return m.closeOverlay().noticed("dry run: would remove " + names), nil
		}

		remove := m.deps.Settings.RemoveLocalData

		return m.sendingLook(func(round int) tea.Msg {
			return localDataRemoved{round: round, names: names, err: remove(scope)}
		})
	}
	m.overlay = look

	return m
}

// localDataRemoved is a removal done, or why it was refused, for the look
// that sent it in round.
type localDataRemoved struct {
	round int
	names string
	err   error
}

var _ applier = localDataRemoved{}

// apply keeps the last look open with the refusal, or closes it saying what
// was removed, and reads again what was announced, which the store's claims
// went with, and the favorites, which kept.db's removal takes with it.
func (msg localDataRemoved) apply(m Model) (Model, tea.Cmd) {
	m = m.answerLook(msg.round, msg.err)
	if msg.err != nil {
		return m, nil
	}

	m, favorites := m.noticed(m.marks.done + " removed " + msg.names).readingRepositoriesAgain()
	m, announces := m.readingAnnounces()

	return m, tea.Batch(announces, favorites)
}
