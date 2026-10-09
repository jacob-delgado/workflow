// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// prLabelWidth is the columns a pull request field's marker, label, prompt and
// the cursor after the text take.
const prLabelWidth = 12

// prBodyPreviewLines is how much of a pull request body the composer shows.
const prBodyPreviewLines = 12

// prBodyHelp is what the editor shows below a pull request body, naming it by
// the forge's own noun.
func prBodyHelp(vocab reviewVocab) string {
	return "Write the " + vocab.noun + " description above this line. Markdown renders on both\n" +
		"GitHub and GitLab."
}

// errNoTitle and errNoBase report a change missing what the forge needs.
var (
	errNoTitle = errors.New("a title is required")
	errNoBase  = errors.New("a base branch to merge into is required")
)

// Pull request composer fields, in the order tab moves through them.
const (
	prFieldTitle = iota
	prFieldBase
	prFieldReviewers
	prFieldAssignees
	prFieldLabels
	prFields
)

// prComposer is a pull request about to be opened, started from the branch's
// commits and the repository's template, all of it editable.
type prComposer struct {
	title     textinput.Model
	base      textinput.Model
	reviewers textinput.Model
	assignees textinput.Model
	labels    textinput.Model
	focus     int
	head      string
	templates []forge.Template
	template  int
	// templatesRead reports the repository's templates read, which they are
	// once the composer is open.
	templatesRead bool
	// proposedFrom is what the title and body were proposed from, which a
	// template chosen later, or the issue read later, proposes from again.
	proposedFrom loop.DraftInput
	body         string
	draft        bool
	edited       bool
	// reviewersSettled reports reviewers typed, or filled from the owners,
	// which a read of the owners then leaves alone.
	reviewersSettled bool
	// opened is the count of overlays opened when this one opened, so the
	// owners read for it land in it alone.
	opened int
	vocab  reviewVocab
	send   sendState
}

var _ failable[prComposer] = prComposer{}

var (
	_ editable             = prComposer{}
	_ failable[prComposer] = prComposer{}
	_ pasteable            = prComposer{}
)

// openPullRequestComposer proposes a pull request for the branch, or reopens the
// draft kept for it. An issue the list does not hold is read for the title
// without holding the composer back: it opens on the title proposed without the
// issue, which the issue's answer replaces while it is still untouched. The
// code owners are read for the reviewers the same way, unless a kept draft
// already says who reviews: one closed before the owners answered, its
// reviewers never typed, is read for again. The templates and the remote
// branches are read after it opens, too.
func (m Model) openPullRequestComposer() (Model, tea.Cmd) {
	branch := m.branch.branch
	issueKey, _ := m.branch.issue(m.cfg.Jira.Project)
	issue, listed := m.issues.find(issueKey)
	proposed := m.proposePullRequest(branch, issueKey, issue.Summary)

	m, proposed.opened = m.opening()
	if m.prDraft.branch == branch.Name {
		proposed = proposed.restore(m.prDraft)
	}

	m.overlay = proposed

	var reviewers tea.Cmd
	if !proposed.reviewersSettled {
		reviewers = readReviewers(m.deps, proposed)
	}

	starts := readPullRequestStarts(m.deps, proposed.opened)
	if listed {
		return m, tea.Batch(starts, reviewers)
	}

	return m, tea.Batch(starts, readTitleIssue(m.deps, proposed), reviewers)
}

// proposePullRequest is the composer filled from the branch's commits, the
// issue it names and the repository's first template.
func (m Model) proposePullRequest(branch gitrepo.Branch, issueKey jira.Key, summary string) prComposer {
	proposedFrom := loop.DraftInput{
		Subjects: loop.Subjects(branch.Commits), IssueKey: issueKey, IssueSummary: summary,
		IssueURL: browseURL(m.deps, issueKey), TitleSource: convention.TitleSource(m.cfg.PullRequest.TitleSource),
	}
	title, _ := loop.Draft(proposedFrom)

	composer := prComposer{
		title:     newInput(title),
		base:      newInput(branch.BaseName()),
		reviewers: newInput(""), assignees: newInput(""), labels: newInput(""),
		focus: prFieldTitle, head: branch.Name, proposedFrom: proposedFrom, vocab: m.vocab,
	}
	composer.base.Blur()
	composer.reviewers.Blur()
	composer.assignees.Blur()
	composer.labels.Blur()
	composer.templatesRead = m.deps.Forge.Templates == nil

	return composer.withTemplate(0)
}

// withTemplate starts the body from a template: the repository's, or none.
func (c prComposer) withTemplate(index int) prComposer {
	c.template = index

	from := c.proposedFrom
	if index < len(c.templates) {
		from.Template = c.templates[index].Body
	}

	_, c.body = loop.Draft(from)

	return c
}

// view shows every part of the pull request as it will be opened.
func (c prComposer) view(kit renderKit, width, _ int) (string, string) {
	inner := max(1, width-prLabelWidth)
	c.title.SetWidth(inner)
	c.base.SetWidth(inner)
	c.reviewers.SetWidth(inner)
	c.assignees.SetWidth(inner)
	c.labels.SetWidth(inner)

	field := func(focus int, label string, input textinput.Model) string {
		return kit.marks.marker(c.focus == focus) + fmt.Sprintf("%-9s ", label) + input.View()
	}

	lines := kit.pinnedOutcome(c.send, "opening", width)
	lines = append(lines,
		field(prFieldTitle, "title", c.title),
		field(prFieldBase, "base", c.base),
		field(prFieldReviewers, "reviewers", c.reviewers),
		field(prFieldAssignees, "assignees", c.assignees),
		field(prFieldLabels, "labels", c.labels),
		fmt.Sprintf("  %-9s %s", "head", c.head),
		"  "+c.templateName(kit)+kit.marks.separator+checkbox(c.draft)+"draft",
		"",
	)

	body := strings.Split(wrap(strings.TrimRight(c.body, "\n"), width), "\n")
	lines = append(lines, body[:min(len(body), prBodyPreviewLines)]...)

	return "Open " + c.vocab.noun, strings.Join(lines, "\n")
}

// templateName names the template in use, and how many there are to choose
// from.
func (c prComposer) templateName(kit renderKit) string {
	switch {
	case !c.templatesRead:
		return "template " + kit.marks.reading()
	case len(c.templates) == 0:
		return "no template in this repository"
	}

	return "template " + sanitize.Line(c.templates[c.template].Name) +
		" (" + strconv.Itoa(c.template+1) + " of " + strconv.Itoa(len(c.templates)) + ")"
}

// footer offers every part that can be changed, opening, and leaving.
func (c prComposer) footer(keys keyMap) []key.Binding {
	if c.send.sending {
		return []key.Binding{keys.interrupt}
	}

	bindings := []key.Binding{keys.nextField}
	if c.canNextTemplate() {
		bindings = append(bindings, keys.nextTemplate)
	}

	return append(bindings, keys.toggleDraft, keys.editBody, relabel(keys.confirm, "open"),
		relabel(keys.closeOverlay, escClose))
}

// canNextTemplate reports another template to cycle to that would not overwrite
// an edited body: ctrl+t is offered only when it can do something safe.
func (c prComposer) canNextTemplate() bool {
	return len(c.templates) > 1 && !c.edited
}

// handleKey answers a key while the pull request is composed.
func (c prComposer) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case c.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		m.prDraft = c.snapshot()

		return m.closeOverlay().noticedDraftKept(m.keys.newPullRequest), nil
	case key.Matches(msg, m.keys.confirm):
		return c.open(m)
	case key.Matches(msg, m.keys.editBody):
		m.overlay = c

		return m, c.editBody(m)
	case key.Matches(msg, m.keys.nextTemplate) && c.canNextTemplate():
		c = c.withTemplate(around(c.template, len(c.templates)).next())
	case key.Matches(msg, m.keys.toggleDraft):
		c.draft = !c.draft
	case key.Matches(msg, m.keys.nextField, m.keys.prevField):
		c = onFieldNav(c, around(c.focus, prFields), m.keys, msg)
	default:
		c = c.typed(msg)
	}

	m.overlay = c

	return m, nil
}

// which names the pull request composer.
func (prComposer) which() overlayKind { return overlayPRComposer }

// suggesting is the base while it has focus, the one field that completes
// what is typed.
func (c prComposer) suggesting() (textinput.Model, bool) {
	return c.base, c.focus == prFieldBase
}

// focusOn moves focus to a field, and the text cursor with it.
func (c prComposer) focusOn(field int) prComposer {
	c.focus = field

	c.title.Blur()
	c.base.Blur()
	c.reviewers.Blur()
	c.assignees.Blur()
	c.labels.Blur()

	switch field {
	case prFieldBase:
		c.base.Focus()
	case prFieldReviewers:
		c.reviewers.Focus()
	case prFieldAssignees:
		c.assignees.Focus()
	case prFieldLabels:
		c.labels.Focus()
	default:
		c.title.Focus()
	}

	return c
}

// typed hands a key to the field with focus.
func (c prComposer) typed(msg tea.Msg) prComposer {
	switch c.focus {
	case prFieldBase:
		c.base, _ = c.base.Update(msg)
	case prFieldReviewers:
		before := c.reviewers.Value()
		c.reviewers, _ = c.reviewers.Update(msg)
		c.reviewersSettled = c.reviewersSettled || c.reviewers.Value() != before
	case prFieldAssignees:
		c.assignees, _ = c.assignees.Update(msg)
	case prFieldLabels:
		c.labels, _ = c.labels.Update(msg)
	default:
		c.title, _ = c.title.Update(msg)
	}

	c.send.err = nil

	return c
}

// pasted types a paste into the field with focus, as typing it would.
func (c prComposer) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if c.send.sending {
		return m, nil
	}

	m.overlay = c.typed(paste)

	return m, nil
}

// editBody opens the editor on the body.
func (c prComposer) editBody(m Model) tea.Cmd {
	if m.deps.Editor.Edit == nil {
		return nil
	}

	return m.deps.Editor.Edit(c.body, prBodyHelp(c.vocab), func(text string, err error) tea.Msg {
		return textEdited{text: text, err: err}
	})
}

// applyEdit puts the body back in the composer, or records why the editor
// failed.
func (c prComposer) applyEdit(m Model, text string, err error) (Model, tea.Cmd) {
	if err != nil {
		c.send = c.send.failed(err)
	} else {
		c.body, c.edited = text, true
	}

	m.overlay = c

	return m, nil
}

// failed is the composer kept open with the reason the pull request did not open.
func (c prComposer) failed(err error) prComposer {
	c.send = c.send.failed(err)

	return c
}
