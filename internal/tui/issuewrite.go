// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// issueAction is a one-line write on an issue: what the overlay is titled, what
// the field asks for, how the typed value is sent, and how success and a dry run
// are worded.
type issueAction struct {
	title  string
	prompt string
	write  func(issueKey jira.Key, value string) error
	done   func(issueKey jira.Key, value string) string
	would  func(issueKey jira.Key, value string) string
}

// assignAction sets an issue's assignee.
func assignAction(assign func(jira.Key, string) error) issueAction {
	return issueAction{
		title:  "Assign",
		prompt: "assignee (username)",
		write:  assign,
		done:   func(issueKey jira.Key, value string) string { return "assigned " + string(issueKey) + " to " + value },
		would: func(issueKey jira.Key, value string) string {
			return "would assign " + string(issueKey) + " to " + value
		},
	}
}

// worklogAction logs work against an issue. The note is left empty here; the
// duration is the one thing a quick log needs.
func worklogAction(add func(jira.Key, string, string) (jira.Worklog, error)) issueAction {
	write := func(issueKey jira.Key, value string) error {
		_, err := add(issueKey, value, "")

		return err
	}

	return issueAction{
		title:  "Log work",
		prompt: "time spent (e.g. 2h, 30m)",
		write:  write,
		done:   func(issueKey jira.Key, value string) string { return "logged " + value + " on " + string(issueKey) },
		would:  func(issueKey jira.Key, value string) string { return "would log " + value + " on " + string(issueKey) },
	}
}

// issueWrite is a one-field write being composed on an issue: type a value, then
// confirm to send it. It reports its outcome like the status picker, so a
// refusal is seen rather than lost.
type issueWrite struct {
	marks   glyphs
	styles  styles
	issue   jira.Issue
	action  issueAction
	input   textinput.Model
	send    sendState
	problem error
}

var _ overlay = issueWrite{}

// openAssign opens the assign form on the selected issue.
func (m Model) openAssign() (Model, tea.Cmd) {
	return m.openIssueWrite(m.deps.Jira.Assign != nil, func() issueAction {
		return assignAction(m.deps.Jira.Assign)
	})
}

// openLogWork opens the log-work form on the selected issue.
func (m Model) openLogWork() (Model, tea.Cmd) {
	return m.openIssueWrite(m.deps.Jira.AddWorklog != nil, func() issueAction {
		return worklogAction(m.deps.Jira.AddWorklog)
	})
}

// openIssueWrite opens a write form on the selected issue, when there is one and
// the action's seam is present.
func (m Model) openIssueWrite(available bool, build func() issueAction) (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok || !available {
		return m, nil
	}

	m.overlay = issueWrite{
		marks: m.marks, styles: m.styles, issue: selected, action: build(), input: newInput(""),
	}

	return m, nil
}

// view draws the value being typed and how sending it is going.
func (w issueWrite) view(width, _ int) (string, string) {
	w.input.SetWidth(max(1, width-len(w.input.Prompt)-1))

	outcome := w.outcome()
	lines := make([]string, 0, 4+len(outcome)) //nolint:mnd // the four header lines above the outcome.
	lines = append(lines, string(w.issue.Key)+" "+w.issue.Summary, "", w.action.prompt, w.input.View())
	lines = append(lines, outcome...)

	return w.action.title + " " + string(w.issue.Key), strings.Join(lines, "\n")
}

// outcome says how sending is going, or names a value the form still needs.
func (w issueWrite) outcome() []string {
	switch {
	case w.send.sending:
		return []string{"", "sending" + w.marks.ellipsis}
	case w.send.err != nil:
		return []string{"", failedGlyph(w.styles, w.marks) + " " + w.send.err.Error()}
	case w.problem != nil:
		return []string{"", failedGlyph(w.styles, w.marks) + " " + w.problem.Error()}
	default:
		return nil
	}
}

// footer offers sending or canceling; while sending, only quitting.
func (w issueWrite) footer(keys keyMap) []key.Binding {
	if w.send.sending {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "send"), relabel(keys.closeOverlay, "cancel")}
}

// handleKey answers a key while the form has the keyboard.
func (w issueWrite) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case w.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return w.confirm(m)
	default:
		w.input, _ = w.input.Update(msg)
		w.problem = nil
		m.overlay = w

		return m, nil
	}
}

// confirm sends the typed value, refusing an empty one, and holding it back in a
// dry run.
func (w issueWrite) confirm(m Model) (Model, tea.Cmd) {
	value := strings.TrimSpace(w.input.Value())
	if value == "" {
		// Clear any earlier send failure, so "needs a value" is what shows rather
		// than a stale reason from the last attempt.
		w.send, w.problem = sendState{}, errNeedsValue
		m.overlay = w

		return m, nil
	}

	if m.dryRun {
		return m.closeOverlay().noticed("dry run: " + w.action.would(w.issue.Key, value)), nil
	}

	w.send = starting()
	m.overlay = w
	write, done, issueKey := w.action.write, w.action.done, w.issue.Key

	return m, func() tea.Msg {
		return issueWritten{issueKey: issueKey, note: done(issueKey, value), err: write(issueKey, value)}
	}
}

// issueWritten reports how a one-field write went.
type issueWritten struct {
	issueKey jira.Key
	note     string
	err      error
}

var _ applier = issueWritten{}

// apply keeps the form open with the reason when the write failed, or closes it
// and refreshes the issue when it worked.
func (msg issueWritten) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		if form, open := m.overlay.(issueWrite); open {
			form.send = form.send.failed(msg.err)
			m.overlay = form
		}

		return m, nil
	}

	return m.closeOverlay().noticed(m.marks.done + " " + msg.note), m.reloadDetail(msg.issueKey)
}
