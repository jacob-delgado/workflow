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
		done:   func(issueKey jira.Key, value string) string { return "assigned " + shownKey(issueKey) + " to " + value },
		would: func(issueKey jira.Key, value string) string {
			return "would assign " + shownKey(issueKey) + " to " + value
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
		done:   func(issueKey jira.Key, value string) string { return "logged " + value + " on " + shownKey(issueKey) },
		would: func(issueKey jira.Key, value string) string {
			return "would log " + value + " on " + shownKey(issueKey)
		},
	}
}

// issueWrite is a one-field write being composed on an issue: type a value, then
// confirm to send it. It reports its outcome like the status picker, so a
// refusal is seen rather than lost.
type issueWrite struct {
	issue   jira.Issue
	action  issueAction
	input   textinput.Model
	send    sendState
	problem error
}

var (
	_ failable[issueWrite] = issueWrite{}
	_ pasteable            = issueWrite{}
)

// openAssign opens the assign form on the selected issue. A forge issue's form
// starts with the forge's name for you, when it is known: assigning one to
// yourself is the usual reason to open it.
func (m Model) openAssign() (Model, tea.Cmd) {
	return m.openIssueWrite(func(issue jira.Issue) (issueAction, string) {
		start := ""
		if isForgeKey(issue.Key) {
			start = m.messaging.author
		}

		return assignAction(m.deps.Jira.Assign), start
	})
}

// openLogWork opens the log-work form on the selected issue, which a forge
// issue has no equivalent for.
func (m Model) openLogWork() (Model, tea.Cmd) {
	return m.openIssueWrite(func(jira.Issue) (issueAction, string) { return worklogAction(m.deps.Jira.AddWorklog), "" })
}

// openIssueWrite opens a write form on the selected issue, holding the text
// build starts it with.
func (m Model) openIssueWrite(build func(jira.Issue) (issueAction, string)) (Model, tea.Cmd) {
	selected, _ := m.issues.current()
	action, start := build(selected)
	m.overlay = issueWrite{
		issue: selected, action: action, input: newInput(start),
	}

	return m, nil
}

// view draws the value being typed, with how sending it is going, or the value
// the form still needs, pinned under the title so a long refusal is wrapped and
// seen rather than clipped.
func (w issueWrite) view(kit renderKit, width, _ int) (string, string) {
	w.input.SetWidth(max(1, width-len(w.input.Prompt)-1))

	lines := append(kit.pinnedOutcome(w.send, "sending", width),
		kit.pinnedProblem(w.problem, width)...)
	lines = append(lines, shownKey(w.issue.Key)+" "+w.issue.Summary, "", w.action.prompt, w.input.View())

	return w.action.title + " " + shownKey(w.issue.Key), strings.Join(lines, "\n")
}

// footer offers sending or canceling; while sending, only quitting.
func (w issueWrite) footer(keys keyMap) []key.Binding {
	if w.send.sending {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "send"), relabel(keys.closeOverlay, escCancel)}
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

// which names an issue's one-field form.
func (issueWrite) which() overlayKind { return overlayIssueWrite }

// pasted types a paste into the form's one field, as typing it would.
func (w issueWrite) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if w.send.sending {
		return m, nil
	}

	w.input, _ = w.input.Update(paste)
	w.problem = nil
	m.overlay = w

	return m, nil
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
		return keepOpenWith[issueWrite](m, msg.err), nil
	}

	return m.closeOverlay().noticed(m.marks.done + " " + msg.note).reloadDetail(msg.issueKey)
}

// failed is the form kept open with the reason the write was refused.
func (w issueWrite) failed(err error) issueWrite {
	w.send = w.send.failed(err)

	return w
}
