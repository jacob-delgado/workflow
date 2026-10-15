// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"maps"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
)

// settingRefusals are why settings were refused, by setting path: what the
// writes refused, a save's and each removal's after it, which only a write
// judges again, and what Settings' own check last held a save back for, which
// each check judges afresh.
type settingRefusals struct {
	written, checked map[string]string
}

// reason is why a setting was refused: a write's word on it before the
// check's, as the web's Settings keeps the server's.
func (r settingRefusals) reason(path string) (string, bool) {
	if reason, written := r.written[path]; written {
		return reason, true
	}

	reason, checked := r.checked[path]

	return reason, checked
}

// without is the refusals with a setting's taken away.
func (r settingRefusals) without(path string) settingRefusals {
	r.written, r.checked = maps.Clone(r.written), maps.Clone(r.checked)
	delete(r.written, path)
	delete(r.checked, path)

	return r
}

// alsoWritten is the refusals with a write's refusal of each setting in named
// beside what the writes before refused, the later write's reason where both
// refused one setting, since it is the latest word on it.
func (r settingRefusals) alsoWritten(named map[string]string) settingRefusals {
	written := make(map[string]string, len(r.written)+len(named))
	maps.Copy(written, r.written)
	maps.Copy(written, named)
	r.written = written

	return r
}

// heldBackFor is what the cursor goes to once Settings' own check holds a save
// back: each setting the check refused that no write had, as the web's
// Settings goes to what was just refused rather than to a refusal already
// said; while the check refused none of its own, what the writes refused,
// still standing.
func (r settingRefusals) heldBackFor() map[string]string {
	judged := maps.Clone(r.checked)
	maps.DeleteFunc(judged, func(path, _ string) bool {
		_, written := r.written[path]

		return written
	})

	if len(judged) == 0 {
		return r.written
	}

	return judged
}

// markedByWrite is the form with each setting a save's refused write names
// marked on its row, in place of every mark before, and the cursor on the
// first of them. It answers what is left to say of the refusal whole, as marks
// does.
func (f settingsForm) markedByWrite(refusal error) (settingsForm, error) {
	written, unmarked := f.marks(refusal)
	f.refusals = settingRefusals{written: written}

	return f.onFirstOf(written), unmarked
}

// markedByCheck is the form with each setting Settings' own check refused
// marked on its row, in place of what the check refused before, and the
// cursor on the first of what heldBackFor names. What the writes refused stays
// marked, since no write was sent to judge it again, as the web's Settings
// keeps what the server refused. It answers what is left to say of the
// refusal whole, as marks does.
func (f settingsForm) markedByCheck(refusal error) (settingsForm, error) {
	checked, unmarked := f.marks(refusal)
	f.refusals.checked = checked

	return f.onFirstOf(f.refusals.heldBackFor()), unmarked
}

// markedByRemoval is the form with each setting a refused removal names
// marked on its row, beside every mark before, with the removal's reason where
// one was marked already, and the cursor on the first of them. A removal
// writes the file as read, without the form's edits, so it judged none of what
// the marks before say, as the web's Settings keeps them through a refused
// removal. It answers what is left to say of the refusal whole, as marks does.
func (f settingsForm) markedByRemoval(refusal error) (settingsForm, error) {
	named, unmarked := f.marks(refusal)
	f.refusals = f.refusals.alsoWritten(named)

	return f.onFirstOf(named), unmarked
}

// marks are why a refusal refused each setting it names, by setting path, as
// the web's Settings marks each beside its field, and what is left to say of
// the refusal whole: nothing once every setting it names is marked, and all
// of it while it names none, or one no row shows.
func (f settingsForm) marks(refusal error) (map[string]string, error) {
	named := config.FieldErrors(refusal)
	marks := make(map[string]string, len(named))

	var unmarked error
	if len(named) == 0 {
		unmarked = refusal
	}

	for _, field := range named {
		if !slices.ContainsFunc(f.fields, func(row setting) bool { return row.path == field.Key }) {
			unmarked = refusal

			continue
		}

		marks[field.Key] = ownLine(field)
	}

	return marks, unmarked
}

// onFirstOf is the form with the cursor on the first row, in the form's
// order, of a setting marks names, so its reason is in sight; where it was
// while marks names none.
func (f settingsForm) onFirstOf(marks map[string]string) settingsForm {
	first := slices.IndexFunc(f.fields, func(row setting) bool {
		_, marked := marks[row.path]

		return marked
	})
	if first >= 0 {
		f.selected = first
	}

	return f
}

// rowLines are a setting's row, and under it why the setting was refused,
// whether by Settings' own check, a save's write or a removal's, while that
// still stands: under the first of a collection's rows alone, since a refusal
// names the collection whole.
func (f settingsForm) rowLines(kit renderKit, index, width int) []string {
	field := f.fields[index]
	row := ansi.Truncate(f.row(kit, field, index == f.selected), width, kit.marks.ellipsis)

	if index > 0 && f.fields[index-1].path == field.path {
		return []string{row}
	}

	return append([]string{row}, f.refusalLines(kit, field, width)...)
}

// refusalLines are why a setting was refused, whether by Settings' own check,
// a save's write or a removal's: the failure mark and the reason in the
// failure style, wrapped under the row's label; none for a setting no refusal
// stands against.
func (f settingsForm) refusalLines(kit renderKit, field setting, width int) []string {
	reason, refused := f.refusals.reason(field.path)
	if !refused {
		return nil
	}

	return failureLines(kit, reason, strings.Repeat(" ", ansi.StringWidth(kit.marks.unselected)), width)
}

// failureLines are text after the failure mark, in the failure style, wrapped
// to width with each line after indent.
func failureLines(kit renderKit, text, indent string, width int) []string {
	lines := strings.Split(wrap(kit.marks.failed+" "+text, width-len(indent)), "\n")

	for index, line := range lines {
		lines[index] = indent + kit.styles.failure.Render(line)
	}

	return lines
}
