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
	subjects  []string
	issueKey  jira.Key
	issueURL  string
	body      string
	draft     bool
	edited    bool
	vocab     reviewVocab
	send      sendState
}

var _ editable = prComposer{}

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

// openPullRequestComposer proposes a pull request for the branch.
func (m Model) openPullRequestComposer() (Model, tea.Cmd) {
	branch := m.branch.branch
	subjects := loop.Subjects(branch.Commits)

	issueKey, _ := m.branchIssue()
	issue, _ := m.issues.find(issueKey)
	titleSource := convention.TitleSource(m.cfg.PullRequest.TitleSource)

	composer := prComposer{
		marks: m.marks, styles: m.styles,
		title:     newInput(convention.PullRequestTitleFrom(titleSource, subjects, string(issueKey), issue.Summary)),
		base:      newInput(branch.BaseName()),
		reviewers: newInput(""), assignees: newInput(""), labels: newInput(""),
		focus: prFieldTitle, head: branch.Name,
		subjects: subjects, issueKey: issueKey, issueURL: m.browseURL(issueKey), vocab: m.vocab,
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

	composer = composer.withTemplate(0)
	if m.prDraft.branch == branch.Name {
		composer = composer.restore(m.prDraft)
	}

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

	text := ""
	if index < len(c.templates) {
		text = c.templates[index].Body
	}

	c.body = convention.PullRequestBody(text, c.subjects, string(c.issueKey), c.issueURL)

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

// request is the pull request the composer describes.
func (c prComposer) request() forge.NewPullRequest {
	return forge.NewPullRequest{
		Title: strings.TrimSpace(c.title.Value()), Body: c.body, Head: c.head,
		Base: strings.TrimSpace(c.base.Value()), Draft: c.draft,
		Reviewers: splitList(c.reviewers.Value()),
		Assignees: splitList(c.assignees.Value()),
		Labels:    splitList(c.labels.Value()),
	}
}

// splitList reads a comma-separated field into its trimmed, non-empty entries,
// or nil when nothing was typed.
func splitList(text string) []string {
	var entries []string

	for entry := range strings.SplitSeq(text, ",") {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			entries = append(entries, trimmed)
		}
	}

	return entries
}

// open opens the pull request — pushing the branch first when origin does not
// have every commit, and opening only if the push succeeded.
func (c prComposer) open(m Model) (Model, tea.Cmd) {
	request := c.request()

	switch {
	case request.Title == "":
		c.send.err = errNoTitle
	case request.Base == "":
		c.send.err = errNoBase
	case m.dryRun:
		return m.closeOverlay().noticed(c.dryRunNotice(request, m.branch.branch.Pushed(), m.issueToLink())), nil
	case m.branch.branch.Pushed():
		return c.create(m)
	default:
		m.prDraft = c.snapshot()

		return m.startPush(func(pushed Model) (Model, tea.Cmd) {
			reload := pushed.loadBranch()
			pushed, create := c.create(pushed)

			return pushed, tea.Batch(reload, create)
		})
	}

	m.overlay = c

	return m, nil
}

// dryRunNotice says what opening the pull request would do, including the push
// that enter does first when origin does not have every commit.
func (c prComposer) dryRunNotice(request forge.NewPullRequest, pushed bool, linkTo jira.Key) string {
	open := "open \"" + request.Title + "\" from " + request.Head + " into " + request.Base
	if linkTo != "" {
		open += " and link it on " + string(linkTo)
	}

	if pushed {
		return "dry run: would " + open
	}

	return "dry run: would push " + request.Head + ", then " + open
}

// create asks the forge to open the pull request.
func (c prComposer) create(m Model) (Model, tea.Cmd) {
	c.send = starting()
	m.overlay = c
	create, request := m.deps.Forge.CreatePullRequest, c.request()

	return m, func() tea.Msg {
		pull, err := create(request)

		return pullCreated{pull: pull, err: err}
	}
}

// pullCreated reports how opening a pull request went.
type pullCreated struct {
	pull forge.PullRequest
	err  error
}

// apply shows the new pull request and starts on its CI, or keeps the composer
// open with the forge's reason. A pull that opened but whose reviewers could
// not be added is shown all the same, with a note, rather than lost.
func (msg pullCreated) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil && !msg.pull.Opened() {
		composer, open := m.overlay.(prComposer)
		if open {
			composer.send = composer.send.failed(msg.err)
			m.overlay = composer
		}

		return m, nil
	}

	notice := m.marks.done + " opened " + m.vocab.sigil + strconv.Itoa(msg.pull.Number) + " " + msg.pull.URL
	if msg.err != nil {
		notice += "; could not add every reviewer, assignee or label: " + forgeReason(msg.err)
	}

	m = m.noticed(notice)
	m.review = reviewState{pull: msg.pull, found: true, loaded: true}
	m.prDraft = prDraft{}

	cmds := tea.Batch(m.checkCI(), m.loadAuthor())

	// With a Jira issue, offer to link the pull request on it — so the team that
	// watches Jira learns of it — and then to move it to the review status. When
	// Jira cannot take the link, go straight to the status offer.
	issueKey, named := m.branchIssue()
	switch {
	case named && m.deps.Jira.LinkPullRequest != nil:
		m.overlay = issueLinker{marks: m.marks, styles: m.styles, vocab: m.vocab, issueKey: issueKey, pull: msg.pull}

		return m, cmds
	case named:
		picker, offer := m.offerReviewStatus(issueKey)

		return picker, tea.Batch(cmds, offer)
	default:
		return m.closeOverlay(), cmds
	}
}
