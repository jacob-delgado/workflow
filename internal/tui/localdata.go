// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/store"
)

// localDataAbout is what Local data holds, said before the files are listed.
const localDataAbout = "workflow keeps a cache a session makes again, and the people and group " +
	"associations you decided, in two files on this machine."

// localData is the Local data overlay: where workflow keeps what it learns
// between sessions, each database file with its size and what it holds, and
// the two removals workflow db-clean makes, each behind a last look, since
// neither can be undone.
type localData struct {
	opened  int
	reading bool
	// dir is where the store is, written from your home.
	dir   string
	files []store.DataFile
	err   error
}

// canSeeLocalData reports that the store's files can be listed.
func (m Model) canSeeLocalData() bool {
	return m.deps.Settings.LocalData != nil
}

// openLocalData opens Local data and starts reading the store's files.
func (m Model) openLocalData() (Model, tea.Cmd) {
	m, opened := m.opening()
	m.overlay = localData{opened: opened, reading: true}

	return m, m.readLocalData(opened)
}

// readLocalData lists the store's files for the overlay opened as opened.
func (m Model) readLocalData(opened int) tea.Cmd {
	list := m.deps.Settings.LocalData

	return func() tea.Msg {
		dir, files, err := list()

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
	open, isOpen := m.overlay.(localData)
	if !isOpen || open.opened != msg.opened {
		return m, nil
	}

	open.reading, open.dir, open.files, open.err = false, m.shownDir(msg.dir), msg.files, msg.err
	m.overlay = open

	return m, nil
}

// view lists where the store is and each file in it, or why it could not.
func (d localData) view(kit renderKit, width, _ int) (string, string) {
	lines := []string{wrap(localDataAbout, width), ""}

	switch {
	case d.reading:
		lines = append(lines, "reading the local data"+kit.marks.ellipsis)
	case d.err != nil:
		lines = append(lines, failureBlock(kit.styles, kit.marks, d.err, width))
	case len(d.files) == 0:
		lines = append(lines, "Kept in "+d.dir, "", "No local data: there is nothing to remove.")
	default:
		lines = append(lines, "Kept in "+d.dir, "")
		lines = append(lines, d.fileLines(width)...)
	}

	return "Local data", strings.Join(lines, "\n")
}

// fileLines is a line per file — its name, kind and size — with what it holds
// below it.
func (d localData) fileLines(width int) []string {
	const linesPerFile = 2

	lines := make([]string, 0, linesPerFile*len(d.files))

	for _, file := range d.files {
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

	if len(d.files) > 0 {
		offered = append(offered, keys.removeAll)
	}

	if d.err != nil {
		offered = append(offered, relabel(keys.refresh, "try again"))
	}

	return append(offered, relabel(keys.closeOverlay, escClose))
}

// handleKey answers a key while Local data has the keyboard.
func (d localData) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.refresh) && d.err != nil:
		return m.openLocalData()
	case key.Matches(msg, m.keys.removeCache) && d.cacheNames() != "":
		return d.askToRemove(m, store.CleanCache), nil
	case key.Matches(msg, m.keys.removeAll) && len(d.files) > 0:
		return d.askToRemove(m, store.CleanAll), nil
	default:
		return m, nil
	}
}

// cacheNames names the cache's file, or is empty when there is none.
func (d localData) cacheNames() string {
	return d.reaches(store.CleanCache)
}

// reaches names the files a removal of scope reaches, joined by "and".
func (d localData) reaches(scope store.CleanScope) string {
	var names []string

	for _, file := range d.files {
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

		return m, func() tea.Msg { return localDataRemoved{names: names, err: remove(scope)} }
	}
	m.overlay = look

	return m
}

// localDataRemoved is a removal done, or why it was refused.
type localDataRemoved struct {
	names string
	err   error
}

var _ applier = localDataRemoved{}

// apply keeps the last look open with the refusal, or closes it saying what
// was removed.
func (msg localDataRemoved) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[lastLook](m, msg.err), nil
	}

	return m.closeOverlay().noticed(m.marks.done + " removed " + msg.names), nil
}
