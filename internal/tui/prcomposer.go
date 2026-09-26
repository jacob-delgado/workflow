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
)

// prLabelWidth is the columns a pull request field's marker, label, prompt and
// the cursor after the text take.
const prLabelWidth = 12

// prBodyPreviewLines is how much of a pull request body the composer shows.
const prBodyPreviewLines = 12

// prBodyHelp is what the editor shows below a pull request body.
const prBodyHelp = "Write the pull request description above this line. Markdown renders on both\n" +
	"GitHub and GitLab."

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
	marks     glyphs
	styles    styles
	title     textinput.Model
	base      textinput.Model
	reviewers textinput.Model
	assignees textinput.Model
	labels    textinput.Model
	focus     int
	head      string
	templates []forge.Template
	template  int
	// proposedFrom is what the title and body were proposed from, which a
	// template chosen later, or the issue read later, proposes from again.
	proposedFrom loop.DraftInput
	body         string
	draft        bool
	edited       bool
	vocab        reviewVocab
	send         sendState
}

var (
	_ editable             = prComposer{}
	_ failable[prComposer] = prComposer{}
)

// prDraft is a pull request the composer was filled with, kept for the session
// so a push that fails, or an esc, does not throw the work away.
type prDraft struct {
	branch, title, base, body    string
	reviewers, assignees, labels string
	template                     int
	draft, edited                bool
}

// snapshot is the composer's editable state, to reopen on.
func (c prComposer) snapshot() prDraft {
	return prDraft{
		branch: c.head, title: c.title.Value(), base: c.base.Value(), body: c.body,
		reviewers: c.reviewers.Value(), assignees: c.assignees.Value(), labels: c.labels.Value(),
		template: c.template, draft: c.draft, edited: c.edited,
	}
}

// restore fills the composer from a kept draft.
func (c prComposer) restore(draft prDraft) prComposer {
	c.title.SetValue(draft.title)
	c.base.SetValue(draft.base)
	c.reviewers.SetValue(draft.reviewers)
	c.assignees.SetValue(draft.assignees)
	c.labels.SetValue(draft.labels)
	c.body, c.template, c.draft, c.edited = draft.body, draft.template, draft.draft, draft.edited

	return c
}

// openPullRequestComposer proposes a pull request for the branch, or reopens the
// draft kept for it. An issue the list does not hold is read for the title
// without holding the composer back: it opens on the title proposed without the
// issue, which the issue's answer replaces while it is still untouched.
func (m Model) openPullRequestComposer() (Model, tea.Cmd) {
	branch := m.branch.branch
	issueKey, _ := m.branchIssue()
	issue, listed := m.issues.find(issueKey)
	proposed := m.proposePullRequest(branch, issueKey, issue.Summary)

	m.overlay = proposed
	if m.prDraft.branch == branch.Name {
		m.overlay = proposed.restore(m.prDraft)
	}

	if listed {
		return m, nil
	}

	return m, m.readTitleIssue(proposed)
}

// proposePullRequest is the composer filled from the branch's commits, the
// issue it names and the repository's first template.
func (m Model) proposePullRequest(branch gitrepo.Branch, issueKey jira.Key, summary string) prComposer {
	proposedFrom := loop.DraftInput{
		Subjects: loop.Subjects(branch.Commits), IssueKey: issueKey, IssueSummary: summary,
		IssueURL: m.browseURL(issueKey), TitleSource: convention.TitleSource(m.cfg.PullRequest.TitleSource),
	}
	title, _ := loop.Draft(proposedFrom)

	composer := prComposer{
		marks: m.marks, styles: m.styles,
		title:     newInput(title),
		base:      newInput(branch.BaseName()),
		reviewers: newInput(""), assignees: newInput(""), labels: newInput(""),
		focus: prFieldTitle, head: branch.Name, proposedFrom: proposedFrom, vocab: m.vocab,
	}
	composer.base.Blur()
	composer.reviewers.Blur()
	composer.assignees.Blur()
	composer.labels.Blur()
	composer = composer.withBaseSuggestions(m.deps.Git.RemoteBranches)
	composer = composer.withReviewerSuggestions(m.deps.Git.CodeOwners)

	if m.deps.Forge.Templates != nil {
		composer.templates = m.deps.Forge.Templates()
	}

	return composer.withTemplate(0)
}

// readTitleIssue reads the composer's issue for its title, when the title is to
// come from the issue. A failed read leaves the summary empty, so the title
// stays the one proposed, as the command line's does.
func (m Model) readTitleIssue(composer prComposer) tea.Cmd {
	read, from := m.deps.Jira.Issue, composer.proposedFrom
	if from.TitleSource != convention.TitleFromIssue || from.IssueKey == "" || read == nil {
		return nil
	}

	head, proposed := composer.head, composer.title.Value()

	return func() tea.Msg {
		from.IssueSummary = ""

		detail, err := read(from.IssueKey)
		if err == nil {
			from.IssueSummary = detail.Issue.Summary
		}

		fromIssue, _ := loop.Draft(from)

		return titleIssueRead{head: head, proposed: proposed, fromIssue: fromIssue}
	}
}

// titleIssueRead is the title the branch's issue gives the pull request, read
// after its composer opened on the title proposed without it.
type titleIssueRead struct {
	head, proposed, fromIssue string
}

var _ applier = titleIssueRead{}

// apply puts the issue's title in the composer, unless the composer has closed,
// or is for another branch, or its title is no longer the one proposed: typed
// over, or already being sent.
func (read titleIssueRead) apply(m Model) (Model, tea.Cmd) {
	composer, open := m.overlay.(prComposer)
	if !open || composer.head != read.head || composer.send.sending || composer.title.Value() != read.proposed {
		return m, nil
	}

	composer.title.SetValue(read.fromIssue)
	m.overlay = composer

	return m, nil
}

// withBaseSuggestions offers the remote branches as completions for the base
// field, when the repository can list them. A failure to list is no reason to
// refuse the composer, so the field is simply left without completions.
func (c prComposer) withBaseSuggestions(remoteBranches func() ([]string, error)) prComposer {
	if remoteBranches == nil {
		return c
	}

	branches, err := remoteBranches()
	if err != nil {
		return c
	}

	c.base.SetSuggestions(branches)
	c.base.ShowSuggestions = true

	return c
}

// withReviewerSuggestions shows the CODEOWNERS handles as the reviewers field's
// hint and completions, when the repository names any. A failure to read them
// is no reason to refuse the composer, so the field is simply left plain.
func (c prComposer) withReviewerSuggestions(codeOwners func() ([]string, error)) prComposer {
	if codeOwners == nil {
		return c
	}

	owners, err := codeOwners()
	if err != nil || len(owners) == 0 {
		return c
	}

	c.reviewers.Placeholder = strings.Join(owners, ", ")
	c.reviewers.SetSuggestions(owners)
	c.reviewers.ShowSuggestions = true

	return c
}

// browseURL links an issue, when there is an issue and a way to link it.
func (m Model) browseURL(issueKey jira.Key) string {
	if issueKey == "" || m.deps.Jira.BrowseURL == nil {
		return ""
	}

	return m.deps.Jira.BrowseURL(issueKey)
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
func (c prComposer) view(width, _ int) (string, string) {
	inner := max(1, width-prLabelWidth)
	c.title.SetWidth(inner)
	c.base.SetWidth(inner)
	c.reviewers.SetWidth(inner)
	c.assignees.SetWidth(inner)
	c.labels.SetWidth(inner)

	field := func(focus int, label string, input textinput.Model) string {
		return c.marks.marker(c.focus == focus) + fmt.Sprintf("%-9s ", label) + input.View()
	}

	checkbox := map[bool]string{false: "[ ]", true: "[x]"}[c.draft]
	lines := pinnedOutcome(c.styles, c.marks, c.send, "opening", width)
	lines = append(lines,
		field(prFieldTitle, "title", c.title),
		field(prFieldBase, "base", c.base),
		field(prFieldReviewers, "reviewers", c.reviewers),
		field(prFieldAssignees, "assignees", c.assignees),
		field(prFieldLabels, "labels", c.labels),
		fmt.Sprintf("  %-9s %s", "head", c.head),
		"  "+c.templateName()+c.marks.separator+checkbox+" draft",
		"",
	)

	body := strings.Split(wrap(strings.TrimRight(c.body, "\n"), width), "\n")
	lines = append(lines, body[:min(len(body), prBodyPreviewLines)]...)

	return "Open " + c.vocab.noun, strings.Join(lines, "\n")
}

// templateName names the template in use, and how many there are to choose
// from.
func (c prComposer) templateName() string {
	if len(c.templates) == 0 {
		return "no template in this repository"
	}

	return "template " + c.templates[c.template].Name +
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
		relabel(keys.closeOverlay, "discard"))
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

		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return c.open(m)
	case key.Matches(msg, m.keys.editBody):
		m.overlay = c

		return m, c.editBody(m)
	case key.Matches(msg, m.keys.nextTemplate) && c.canNextTemplate():
		c = c.withTemplate((c.template + 1) % len(c.templates))
	case key.Matches(msg, m.keys.toggleDraft):
		c.draft = !c.draft
	case key.Matches(msg, m.keys.nextField, m.keys.prevField):
		c = c.onFieldNav(msg, m.keys)
	default:
		c = c.typed(msg)
	}

	m.overlay = c

	return m, nil
}

// onFieldNav accepts a pending base completion when tab could take one, and
// otherwise moves to the next field.
func (c prComposer) onFieldNav(msg tea.KeyPressMsg, keys keyMap) prComposer {
	if c.focus == prFieldBase && key.Matches(msg, keys.nextField) && c.baseCanComplete() {
		return c.typed(msg)
	}

	if key.Matches(msg, keys.prevField) {
		return c.focusOn((c.focus + prFields - 1) % prFields)
	}

	return c.focusOn((c.focus + 1) % prFields)
}

// baseCanComplete reports a base suggestion that would extend what is typed, so
// tab completes it rather than moving on.
func (c prComposer) baseCanComplete() bool {
	suggestion := c.base.CurrentSuggestion()

	return suggestion != "" && suggestion != c.base.Value()
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
func (c prComposer) typed(msg tea.KeyPressMsg) prComposer {
	switch c.focus {
	case prFieldBase:
		c.base, _ = c.base.Update(msg)
	case prFieldReviewers:
		c.reviewers, _ = c.reviewers.Update(msg)
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

// editBody opens the editor on the body.
func (c prComposer) editBody(m Model) tea.Cmd {
	if m.deps.Editor.Edit == nil {
		return nil
	}

	return m.deps.Editor.Edit(c.body, prBodyHelp, func(text string, err error) tea.Msg {
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
