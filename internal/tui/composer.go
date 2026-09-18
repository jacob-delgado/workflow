// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// composerLabelWidth is the columns a composer field's marker, label, prompt and
// the cursor after the text take.
const composerLabelWidth = 13

// bodyPreviewLines is how much of a commit body the composer shows.
const bodyPreviewLines = 3

// bodyHelp is what the editor shows below a commit body being written.
const bodyHelp = "Write the commit body above this line: why the change was made, wrapped at 72.\n" +
	"The subject and the Refs trailer are added for you."

// Composer fields, in the order tab moves through them.
const (
	fieldType = iota
	fieldScope
	fieldSubject
	composerFields
)

// commitDraft is a commit message being written, kept when a commit fails so
// the composer opens on it again.
type commitDraft struct {
	kind, scope, subject, body string
}

// commitComposer assembles a Conventional Commit subject from its parts, so it
// is always well-formed, and measures it against the limit as it is typed.
type commitComposer struct {
	marks    glyphs
	styles   styles
	types    []string
	kind     int
	focus    int
	scope    textinput.Model
	subject  textinput.Model
	body     string
	issueKey string
	staged   int
	send     sendState
}

var _ overlay = commitComposer{}

// openCommitComposer opens the composer on the last draft, if a commit failed,
// or on a fresh one.
func (m Model) openCommitComposer() (Model, tea.Cmd) {
	if m.deps.Git.Commit == nil {
		return m, nil
	}

	if m.changes.staged() == 0 {
		return m.noticed("nothing is staged: space stages the selected file"), nil
	}

	draft := m.draft
	types := convention.CommitTypes()
	issueKey, _ := m.branchIssue()

	composer := commitComposer{
		marks: m.marks, styles: m.styles, types: types, kind: m.startingType(types, draft),
		focus: fieldSubject, scope: newInput(m.startingScope(draft)), subject: newInput(draft.subject), body: draft.body,
		issueKey: issueKey, staged: m.changes.staged(),
	}
	composer.scope.Blur()

	m.overlay = composer

	return m, nil
}

// startingType is the type the composer opens on: a kept draft's type wins, so a
// failed commit reopens as it was; otherwise the branch's own prefix, which
// already says what kind of change this is; otherwise the first type offered.
func (m Model) startingType(types []string, draft commitDraft) int {
	if draft.kind != "" {
		return max(0, slices.Index(types, draft.kind))
	}

	if branchType, ok := convention.BranchType(m.branch.branch.Name); ok {
		return max(0, slices.Index(types, branchType))
	}

	return 0
}

// startingScope is the scope the composer opens on: a kept draft's scope wins,
// so a failed commit reopens as it was; otherwise the configured default, for a
// team that scopes its commits the same way; otherwise blank.
func (m Model) startingScope(draft commitDraft) string {
	if draft.scope != "" {
		return draft.scope
	}

	return m.cfg.Commit.DefaultScope
}

// assembled is the subject as it stands.
func (c commitComposer) assembled() convention.Subject {
	return convention.Subject{
		Type: c.types[c.kind], Scope: c.scope.Value(), Description: c.subject.Value(), Breaking: false,
	}
}

// view shows each part, the subject they make with its length against the
// limit, the start of the body, and the trailer.
func (c commitComposer) view(width, _ int) (string, string) {
	subject := c.assembled()
	length := strconv.Itoa(utf8.RuneCountInString(subject.String())) + "/" + strconv.Itoa(convention.SubjectLimit)

	c.scope.Width, c.subject.Width = max(1, width-composerLabelWidth), max(1, width-composerLabelWidth)

	lines := pinnedOutcome(c.styles, c.marks, c.send, "", width)
	lines = append(lines,
		c.label(fieldType, "type    ")+c.typeChoice(),
		c.label(fieldScope, "scope   ")+c.scope.View(),
	)

	if problem := c.scopeProblem(); problem != "" {
		lines = append(lines, "  "+failedGlyph(c.styles, c.marks)+" "+problem)
	}

	lines = append(lines,
		c.label(fieldSubject, "subject ")+c.subject.View(),
		"",
		"  "+subject.String()+"  "+length,
	)

	problem := subject.Validate()
	if problem != nil && strings.TrimSpace(c.subject.Value()) != "" {
		lines = append(lines, "  "+failedGlyph(c.styles, c.marks)+" "+problem.Error())
	}

	return "Commit", strings.Join(append(append(lines, ""), c.footnotes()...), "\n")
}

// label marks the field with focus.
func (c commitComposer) label(field int, text string) string {
	return c.marks.marker(field == c.focus) + text
}

// scopeProblem reports what is wrong with the scope as it stands, so the reason
// can be drawn under the scope field rather than waiting for enter.
func (c commitComposer) scopeProblem() string {
	value := strings.TrimSpace(c.scope.Value())
	if value == "" {
		return ""
	}

	subject := convention.Subject{Type: c.types[c.kind], Scope: value, Description: c.subject.Value(), Breaking: false}

	err := subject.Validate()
	if err != nil && errors.Is(err, convention.ErrInvalidScope) {
		return err.Error()
	}

	return ""
}

// typeChoice shows the chosen type among its neighbors.
func (c commitComposer) typeChoice() string {
	parts := make([]string, 0, len(c.types))

	for index, kind := range c.types {
		if index == c.kind {
			kind = c.marks.chosenOpen + kind + c.marks.chosenClose
		}

		parts = append(parts, kind)
	}

	return strings.Join(parts, " ")
}

// footnotes are the body, the trailer, what is staged, and any problem.
func (c commitComposer) footnotes() []string {
	lines := []string{}

	if strings.TrimSpace(c.body) == "" {
		lines = append(lines, "no body yet: ctrl+o writes one in your editor")
	} else {
		body := strings.Split(strings.TrimSpace(c.body), "\n")
		lines = append(lines, body[:min(len(body), bodyPreviewLines)]...)
	}

	if c.issueKey != "" {
		lines = append(lines, "", "Refs: "+c.issueKey)
	}

	lines = append(lines, "", plural(c.staged, "file")+" staged")

	return lines
}

// footer offers moving between parts, the body, committing, and leaving.
func (c commitComposer) footer(keys keyMap) []key.Binding {
	return []key.Binding{keys.nextField, keys.cycleLeft, keys.editBody, relabel(keys.confirm, "commit"), keys.closeOverlay}
}

// handleKey answers a key while the commit is composed.
func (c commitComposer) handleKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		m.draft = c.draft()

		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return c.commit(m)
	case key.Matches(msg, m.keys.editBody):
		m.overlay = c

		return m, c.editBody(m)
	case key.Matches(msg, m.keys.nextField):
		c = c.focusOn((c.focus + 1) % composerFields)
	case key.Matches(msg, m.keys.prevField):
		c = c.focusOn((c.focus + composerFields - 1) % composerFields)
	default:
		c = c.typed(m, msg)
	}

	m.overlay = c

	return m, nil
}

// typed hands a key to the part with focus: the type cycles, text is typed.
func (c commitComposer) typed(m Model, msg tea.KeyMsg) commitComposer {
	switch {
	case c.focus == fieldType && key.Matches(msg, m.keys.cycleRight):
		c.kind = (c.kind + 1) % len(c.types)
	case c.focus == fieldType && key.Matches(msg, m.keys.cycleLeft):
		c.kind = (c.kind + len(c.types) - 1) % len(c.types)
	case c.focus == fieldScope:
		c.scope, _ = c.scope.Update(msg)
	case c.focus == fieldSubject:
		c.subject, _ = c.subject.Update(msg)
	}

	c.send.err = nil

	return c
}

// focusOn moves focus to a part, and the text cursor with it.
func (c commitComposer) focusOn(field int) commitComposer {
	c.focus = field

	c.scope.Blur()
	c.subject.Blur()

	switch field {
	case fieldScope:
		c.scope.Focus()
	case fieldSubject:
		c.subject.Focus()
	}

	return c
}

// draft is the composer's contents, to open on again.
func (c commitComposer) draft() commitDraft {
	return commitDraft{kind: c.types[c.kind], scope: c.scope.Value(), subject: c.subject.Value(), body: c.body}
}

// editBody opens the editor on the body.
func (c commitComposer) editBody(m Model) tea.Cmd {
	if m.deps.Editor.Edit == nil {
		return nil
	}

	return m.deps.Editor.Edit(c.body, bodyHelp, func(text string, err error) tea.Msg {
		return commitBodyEdited{text: text, err: err}
	})
}

// commitBodyEdited is a commit body back from the editor.
type commitBodyEdited struct {
	text string
	err  error
}

// apply puts the body in the composer, if it is still open.
func (msg commitBodyEdited) apply(m Model) (Model, tea.Cmd) {
	composer, open := m.overlay.(commitComposer)
	if !open {
		return m, nil
	}

	if msg.err != nil {
		composer.send = composer.send.failed(msg.err)
	} else {
		composer.body = msg.text
	}

	m.overlay = composer

	return m, nil
}

// commit commits the index with the assembled message, through a plain git
// commit, so the repository's hooks run as they would in a terminal.
func (c commitComposer) commit(m Model) (Model, tea.Cmd) {
	subject := c.assembled()

	c.send.err = subject.Validate()
	if c.send.err != nil {
		m.overlay = c

		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would commit " + subject.String()), nil
	}

	m.draft = c.draft()
	message, commit := convention.Message(subject, c.body, c.issueKey), m.deps.Git.Commit

	return m.startRun("git commit", func() (proc.Output, error) { return commit(message) },
		func(done Model) (Model, tea.Cmd) {
			done.draft = commitDraft{}
			done = done.closeOverlay().noticed(done.marks.done + " committed " + subject.String())

			return done, tea.Batch(done.loadChanges(), done.loadBranch())
		})
}
